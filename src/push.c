#include "msync.h"

int repo_push(const char *remote_arg, const char *branch) {
    if (!dir_exists(MSYNC_DIR)) {
        fprintf(stderr, "Not an msync repository.\n");
        return -1;
    }

    /* Resolve remote name or raw URL */
    char host[256];
    int port = MSYNC_DEFAULT_PORT;
    char path[MAX_PATH_LEN] = "";
    char remote_name[128] = "";

    /* Check if remote_arg is a configured remote name */
    char url_key[256];
    snprintf(url_key, sizeof(url_key), "remote.%s.url", remote_arg);
    if (config_has(url_key)) {
        /* It's a configured remote - save the name */
        snprintf(remote_name, sizeof(remote_name), "%s", remote_arg);
    }

    remote_resolve(remote_arg, host, &port, path, sizeof(path));

    /* Get the branch name */
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

    /* Get current local commit hash */
    char refname[MAX_PATH_LEN];
    snprintf(refname, sizeof(refname), "refs/heads/%s", branch_name);

    uint8_t new_hash[HASH_RAW_SIZE];
    if (ref_resolve(refname, new_hash) != 0) {
        fprintf(stderr, "Branch '%s' has no commits.\n", branch_name);
        return -1;
    }

    /* Get remote tracking hash (what we think the remote has) */
    uint8_t old_hash[HASH_RAW_SIZE];
    int has_old = 0;
    if (remote_name[0]) {
        has_old = (remote_get_tracking_hash(remote_name, branch_name, old_hash) == 0);
    }

    /* Connect to remote */
    printf("Connecting to %s:%d...\n", host, port);
    int fd = net_connect(host, port);
    if (fd < 0) {
        fprintf(stderr, "Failed to connect to %s:%d\n", host, port);
        return -1;
    }

    /* Push with old_hash so server can detect conflicts */
    printf("Pushing branch '%s'...\n", branch_name);

    uint8_t *old_ptr = has_old ? old_hash : NULL;
    int ret = proto_push(fd, refname, old_ptr, new_hash);

    net_close(fd);

    if (ret == 0) {
        printf("Push successful.\n");

        /* Update remote tracking ref */
        if (remote_name[0]) {
            char remote_ref[MAX_PATH_LEN];
            snprintf(remote_ref, sizeof(remote_ref), "refs/remotes/%s/%s",
                     remote_name, branch_name);
            char remote_ref_path[MAX_PATH_LEN];
            snprintf(remote_ref_path, sizeof(remote_ref_path),
                     "%s/%s", MSYNC_REMOTES_DIR, remote_name);

            if (!dir_exists(remote_ref_path)) msync_mkdir(remote_ref_path);
            ref_update(remote_ref, new_hash);
        }
    } else if (ret == -2) {
        fprintf(stderr, "Push rejected: the remote has newer commits.\n");
        fprintf(stderr, "Run 'msync update %s' first to integrate remote changes.\n",
                remote_name[0] ? remote_name : remote_arg);
    } else {
        fprintf(stderr, "Push failed: unable to reach remote or transfer error.\n");
    }

    return ret;
}
