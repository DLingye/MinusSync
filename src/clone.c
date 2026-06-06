#include "msync.h"

int repo_clone(const char *remote_url, const char *dir, const char *remote_name) {
    /* Parse remote URL */
    char host[256];
    int port = MSYNC_DEFAULT_PORT;
    char path[MAX_PATH_LEN] = "";

    const char *url = remote_url;
    if (strncmp(url, "msync://", 8) == 0) url += 8;

    const char *colon = strchr(url, ':');
    const char *slash = strchr(url, '/');

    if (colon && (!slash || colon < slash)) {
        size_t hlen = colon - url;
        memcpy(host, url, hlen);
        host[hlen] = '\0';
        port = atoi(colon + 1);
        if (slash) snprintf(path, sizeof(path), "%s", slash + 1);
    } else if (slash) {
        size_t hlen = slash - url;
        memcpy(host, url, hlen);
        host[hlen] = '\0';
        if (!host[0]) snprintf(host, sizeof(host), "localhost");
        snprintf(path, sizeof(path), "%s", slash + 1);
    } else {
        snprintf(host, sizeof(host), "%s", url);
    }

    /* Use default remote name if not specified */
    const char *rname = remote_name ? remote_name : "origin";

    /* Create directory */
    if (dir_exists(dir)) {
        fprintf(stderr, "Directory '%s' already exists.\n", dir);
        return -1;
    }
    if (msync_mkdir(dir) != 0) {
        fprintf(stderr, "Failed to create directory '%s'.\n", dir);
        return -1;
    }

    /* Save original directory */
    char orig_dir[MAX_PATH_LEN];
    getcwd(orig_dir, sizeof(orig_dir));

    if (chdir(dir) != 0) {
        fprintf(stderr, "Failed to enter directory '%s'.\n", dir);
        return -1;
    }

    /* Init repo */
    if (repo_init() != 0) {
        chdir(orig_dir);
        return -1;
    }

    /* Connect to remote */
    printf("Cloning from %s:%d...\n", host, port);
    int fd = net_connect(host, port);
    if (fd < 0) {
        fprintf(stderr, "Failed to connect to %s:%d\n", host, port);
        chdir(orig_dir);
        return -1;
    }

    /* List remote refs */
    char **refs;
    uint8_t *hashes;
    int count;

    if (proto_list_remote_refs(fd, &refs, &hashes, &count) != 0) {
        fprintf(stderr, "Failed to list remote refs.\n");
        net_close(fd);
        chdir(orig_dir);
        return -1;
    }

    /* Find master/main branch */
    uint8_t master_hash[HASH_RAW_SIZE];
    char *master_ref_name = NULL;
    int found = 0;

    const char *branch_names[] = {"refs/heads/master", "refs/heads/main", NULL};
    for (int b = 0; branch_names[b] && !found; b++) {
        for (int i = 0; i < count; i++) {
            if (strcmp(refs[i], branch_names[b]) == 0) {
                memcpy(master_hash, hashes + i * HASH_RAW_SIZE, HASH_RAW_SIZE);
                master_ref_name = refs[i];
                found = 1;
                break;
            }
        }
    }

    if (!found && count > 0) {
        memcpy(master_hash, hashes, HASH_RAW_SIZE);
        master_ref_name = refs[0];
        found = 1;
    }

    if (!found) {
        fprintf(stderr, "Remote repository is empty.\n");
        for (int i = 0; i < count; i++) free(refs[i]);
        free(refs);
        free(hashes);
        net_close(fd);
        chdir(orig_dir);
        return 0;
    }

    /* Fetch all objects */
    printf("Fetching objects...\n");
    const uint8_t *want[] = { master_hash };
    if (proto_fetch(fd, want, 1, NULL, 0) != 0) {
        fprintf(stderr, "Failed to fetch objects.\n");
        for (int i = 0; i < count; i++) free(refs[i]);
        free(refs);
        free(hashes);
        net_close(fd);
        chdir(orig_dir);
        return -1;
    }

    net_close(fd);

    /* Checkout */
    uint8_t *data;
    size_t len;
    if (object_read(master_hash, &data, &len) != 0) {
        fprintf(stderr, "Failed to read commit.\n");
        chdir(orig_dir);
        return -1;
    }

    uint8_t tree_hash[HASH_RAW_SIZE];
    commit_parse(data, len, tree_hash, NULL, NULL, NULL, NULL, NULL, NULL);
    free(data);

    tree_checkout(tree_hash, ".");

    /* Get branch name from remote ref */
    const char *branch_short = master_ref_name + 11; /* after "refs/heads/" */
    char local_ref[MAX_PATH_LEN];
    snprintf(local_ref, sizeof(local_ref), "refs/heads/%s", branch_short);
    ref_update(local_ref, master_hash);
    head_set_ref(local_ref);

    /* Save remote URL to config */
    remote_add(rname, remote_url);

    /* Save remote tracking refs */
    char remote_dir[MAX_PATH_LEN];
    snprintf(remote_dir, sizeof(remote_dir), "%s/%s", MSYNC_REMOTES_DIR, rname);
    msync_mkdir(remote_dir);

    for (int i = 0; i < count; i++) {
        char track_ref[MAX_PATH_LEN];
        snprintf(track_ref, sizeof(track_ref), "remotes/%s/%s", rname, refs[i] + 11);
        ref_update(track_ref, hashes + i * HASH_RAW_SIZE);
        free(refs[i]);
    }
    free(refs);
    free(hashes);

    /* Build initial index */
    char **files;
    int fcount;
    file_list(".", &files, &fcount);
    index_reset();
    for (int i = 0; i < fcount; i++) {
        if (strncmp(files[i], ".msync", 6) == 0) { free(files[i]); continue; }
        struct stat st;
        if (stat(files[i], &st) != 0) { free(files[i]); continue; }
        uint8_t blob_hash[HASH_RAW_SIZE];
        if (blob_create(files[i], blob_hash) == 0) {
            index_add(files[i], blob_hash, (uint64_t)st.st_size,
                      (int64_t)st.st_mtime, (uint32_t)st.st_mode);
        }
        free(files[i]);
    }
    free(files);
    index_save();
    index_clear_entries();

    printf("Clone complete. Remote '%s' configured, %d files checked out.\n",
           rname, fcount);

    chdir(orig_dir);
    return 0;
}
