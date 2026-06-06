#include "msync.h"

int repo_update(const char *remote_arg, const char *branch) {
    if (!dir_exists(MSYNC_DIR)) {
        fprintf(stderr, "Not an msync repository.\n");
        return -1;
    }

    /* Resolve remote */
    char host[256];
    int port = MSYNC_DEFAULT_PORT;
    char path[MAX_PATH_LEN] = "";
    char remote_name[128] = "";

    /* Check if it's a configured remote */
    char url_key[256];
    snprintf(url_key, sizeof(url_key), "remote.%s.url", remote_arg);
    if (config_has(url_key)) {
        snprintf(remote_name, sizeof(remote_name), "%s", remote_arg);
    }

    remote_resolve(remote_arg, host, &port, path, sizeof(path));

    /* Get branch */
    char branch_name[256];
    if (!branch) {
        char ref[MAX_PATH_LEN];
        if (head_get_ref(ref, sizeof(ref)) != 0) {
            fprintf(stderr, "Not on any branch.\n");
            return -1;
        }
        strncpy(branch_name, ref + 11, sizeof(branch_name) - 1);
        branch_name[sizeof(branch_name) - 1] = '\0';
    } else {
        snprintf(branch_name, sizeof(branch_name), "%s", branch);
    }

    /* Get local hash before checking remote */
    char local_ref[MAX_PATH_LEN];
    snprintf(local_ref, sizeof(local_ref), "refs/heads/%s", branch_name);
    uint8_t local_hash[HASH_RAW_SIZE];
    int has_local = (ref_resolve(local_ref, local_hash) == 0);

    /* Connect */
    printf("Connecting to %s:%d...\n", host, port);
    int fd = net_connect(host, port);
    if (fd < 0) {
        fprintf(stderr, "Failed to connect to %s:%d\n", host, port);
        return -1;
    }

    /* Get remote refs */
    char **refs;
    uint8_t *hashes;
    int count;

    if (proto_list_remote_refs(fd, &refs, &hashes, &count) != 0) {
        fprintf(stderr, "Failed to list remote refs.\n");
        net_close(fd);
        return -1;
    }

    /* Find our branch */
    uint8_t remote_hash[HASH_RAW_SIZE];
    char remote_ref_name[MAX_PATH_LEN];
    snprintf(remote_ref_name, sizeof(remote_ref_name), "refs/heads/%s", branch_name);
    int found = 0;

    for (int i = 0; i < count; i++) {
        if (strcmp(refs[i], remote_ref_name) == 0) {
            memcpy(remote_hash, hashes + i * HASH_RAW_SIZE, HASH_RAW_SIZE);
            found = 1;
            break;
        }
    }

    if (!found) {
        fprintf(stderr, "Remote branch '%s' not found.\n", branch_name);
        for (int i = 0; i < count; i++) free(refs[i]);
        free(refs);
        free(hashes);
        net_close(fd);
        return -1;
    }

    /* Already up to date? */
    if (has_local && hash_cmp(local_hash, remote_hash) == 0) {
        printf("Already up to date.\n");
        for (int i = 0; i < count; i++) free(refs[i]);
        free(refs);
        free(hashes);
        net_close(fd);
        return 0;
    }

    /* Conflict detection */
    if (has_local && !merge_is_fast_forward(local_hash, remote_hash)) {
        /* Check if local is ahead of remote (local has extra commits) */
        if (!merge_is_fast_forward(remote_hash, local_hash)) {
            /* Divergent histories */
            char lhex[HASH_HEX_SIZE + 1], rhex[HASH_HEX_SIZE + 1];
            hash_to_hex(local_hash, lhex); lhex[8] = '\0';
            hash_to_hex(remote_hash, rhex); rhex[8] = '\0';
            printf("Local and remote have diverged.\n");
            printf("  Local:  %s\n", lhex);
            printf("  Remote: %s\n", rhex);
            printf("Attempting merge...\n");
        }
    }

    /* Fetch */
    printf("Fetching updates for '%s'...\n", branch_name);

    const uint8_t *want[] = { remote_hash };
    const uint8_t *have_arr[32];
    int have_count = 0;

    if (has_local) {
        have_arr[0] = local_hash;
        have_count = 1;
    }

    if (proto_fetch(fd, want, 1, have_arr, have_count) != 0) {
        fprintf(stderr, "Failed to fetch objects.\n");
        for (int i = 0; i < count; i++) free(refs[i]);
        free(refs);
        free(hashes);
        net_close(fd);
        return -1;
    }

    net_close(fd);

    /* Determine the target commit hash */
    uint8_t target_hash[HASH_RAW_SIZE];
    int merge_done = 0;

    if (has_local && !merge_is_fast_forward(local_hash, remote_hash)) {
        /* Not fast-forward - attempt merge */
        uint8_t result_hash[HASH_RAW_SIZE];
        int merge_ret = merge_commits(local_hash, remote_hash, result_hash);

        if (merge_ret == 0) {
            hash_cpy(target_hash, result_hash);
            merge_done = 1;
            printf("Merge successful.\n");
        } else {
            /* Merge conflict - use remote version as basis but warn */
            fprintf(stderr, "Merge conflict detected.\n");
            fprintf(stderr, "Local and remote changes overlap.\n");
            fprintf(stderr, "Using remote version. Local-only commits preserved in history.\n");
            fprintf(stderr, "Use 'msync checkout <commit>' to inspect local changes.\n");
            hash_cpy(target_hash, remote_hash);
        }
    } else {
        /* Fast-forward: just use remote hash */
        hash_cpy(target_hash, remote_hash);
    }

    /* Checkout */
    uint8_t *data;
    size_t len;
    if (object_read(target_hash, &data, &len) != 0) {
        fprintf(stderr, "Failed to read target commit.\n");
        for (int i = 0; i < count; i++) free(refs[i]);
        free(refs); free(hashes);
        return -1;
    }

    uint8_t tree_hash[HASH_RAW_SIZE];
    commit_parse(data, len, tree_hash, NULL, NULL, NULL, NULL, NULL, NULL);
    free(data);

    /* Remove old tracked files */
    char **files;
    int fcount;
    file_list(".", &files, &fcount);
    for (int i = 0; i < fcount; i++) {
        if (strncmp(files[i], ".msync", 6) == 0) { free(files[i]); continue; }
        unlink(files[i]);
        free(files[i]);
    }
    free(files);

    /* Checkout tree */
    tree_checkout(tree_hash, ".");

    /* Update local ref */
    ref_update(local_ref, target_hash);

    /* Update remote tracking ref */
    if (remote_name[0]) {
        char remote_track_ref[MAX_PATH_LEN];
        snprintf(remote_track_ref, sizeof(remote_track_ref), "remotes/%s/%s",
                 remote_name, branch_name);
        char remote_track_path[MAX_PATH_LEN];
        snprintf(remote_track_path, sizeof(remote_track_path), "%s/%s",
                 MSYNC_REMOTES_DIR, remote_name);
        if (!dir_exists(remote_track_path)) msync_mkdir(remote_track_path);
        ref_update(remote_track_ref, remote_hash);
    }

    /* Rebuild index */
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

    for (int i = 0; i < count; i++) free(refs[i]);
    free(refs);
    free(hashes);

    char hex[HASH_HEX_SIZE + 1];
    hash_to_hex(target_hash, hex);
    hex[7] = '\0';
    if (merge_done) {
        printf("Updated (merged) to %s.\n", hex);
    } else {
        printf("Updated to %s.\n", hex);
    }

    return 0;
}
