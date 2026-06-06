#include "msync.h"

int ref_resolve(const char *ref, uint8_t *hash) {
    char path[MAX_PATH_LEN];

    if (strncmp(ref, "refs/", 5) == 0) {
        snprintf(path, sizeof(path), ".msync/%s", ref);
    } else if (strncmp(ref, "remotes/", 8) == 0) {
        snprintf(path, sizeof(path), ".msync/refs/%s", ref);
    } else {
        snprintf(path, sizeof(path), ".msync/refs/heads/%s", ref);
    }

    if (!file_exists(path)) return -1;

    uint8_t *data;
    size_t len;
    if (file_read(path, &data, &len) != 0) return -1;
    if (len < HASH_HEX_SIZE) { free(data); return -1; }

    ((char *)data)[HASH_HEX_SIZE] = '\0';
    int ret = hex_to_hash((char *)data, hash);
    free(data);
    return ret;
}

int ref_update(const char *ref, const uint8_t *hash) {
    char path[MAX_PATH_LEN];

    if (strncmp(ref, "refs/", 5) == 0) {
        snprintf(path, sizeof(path), ".msync/%s", ref);
    } else if (strncmp(ref, "remotes/", 8) == 0) {
        snprintf(path, sizeof(path), ".msync/refs/%s", ref);
    } else {
        snprintf(path, sizeof(path), ".msync/refs/heads/%s", ref);
    }

    /* Ensure parent directory exists */
    char dir[MAX_PATH_LEN];
    snprintf(dir, sizeof(dir), "%s", path);
    char *slash = strrchr(dir, '/');
    if (slash) {
        *slash = '\0';
        if (!dir_exists(dir)) msync_mkdir(dir);
    }

    char hex[HASH_HEX_SIZE + 1];
    hash_to_hex(hash, hex);
    hex[HASH_HEX_SIZE] = '\n';

    return file_write(path, (uint8_t *)hex, HASH_HEX_SIZE + 1);
}

int ref_delete(const char *ref) {
    char path[MAX_PATH_LEN];
    snprintf(path, sizeof(path), ".msync/%s", ref);
    if (!file_exists(path)) return -1;
    return unlink(path);
}

int ref_list(char ***names, uint8_t **hashes, int *count) {
    *count = 0;
    *names = NULL;
    *hashes = NULL;

    /* List heads */
    if (dir_exists(MSYNC_HEADS_DIR)) {
        char **head_files;
        int head_count;
        file_list(MSYNC_HEADS_DIR, &head_files, &head_count);

        for (int i = 0; i < head_count; i++) {
            char refname[MAX_PATH_LEN];
            snprintf(refname, sizeof(refname), "refs/heads/%s", head_files[i]);

            char *display = msync_strdup(refname + 5); /* skip "refs/" */

            uint8_t hash[HASH_RAW_SIZE];
            if (ref_resolve(refname, hash) == 0) {
                (*count)++;
                *names = (char **)realloc(*names, (*count) * sizeof(char *));
                *hashes = (uint8_t *)realloc(*hashes, (*count) * HASH_RAW_SIZE);
                (*names)[*count - 1] = display;
                memcpy((*hashes) + (*count - 1) * HASH_RAW_SIZE, hash, HASH_RAW_SIZE);
            } else {
                free(display);
            }
        }
        file_list_free(head_files, head_count);
    }

    /* List remotes */
    if (dir_exists(MSYNC_REMOTES_DIR)) {
        char **remote_dirs;
        int rd_count;
        file_list(MSYNC_REMOTES_DIR, &remote_dirs, &rd_count);

        for (int i = 0; i < rd_count; i++) {
            char branch_dir[MAX_PATH_LEN];
            snprintf(branch_dir, sizeof(branch_dir), "%s/%s", MSYNC_REMOTES_DIR, remote_dirs[i]);

            char **branch_files;
            int bf_count;
            file_list(branch_dir, &branch_files, &bf_count);

            for (int j = 0; j < bf_count; j++) {
                char refname[MAX_PATH_LEN];
                snprintf(refname, sizeof(refname), "refs/remotes/%s/%s", remote_dirs[i], branch_files[j]);

                char *display = msync_strdup(refname + 5);

                uint8_t hash[HASH_RAW_SIZE];
                if (ref_resolve(refname, hash) == 0) {
                    (*count)++;
                    *names = (char **)realloc(*names, (*count) * sizeof(char *));
                    *hashes = (uint8_t *)realloc(*hashes, (*count) * HASH_RAW_SIZE);
                    (*names)[*count - 1] = display;
                    memcpy((*hashes) + (*count - 1) * HASH_RAW_SIZE, hash, HASH_RAW_SIZE);
                } else {
                    free(display);
                }
            }
            file_list_free(branch_files, bf_count);
        }
        file_list_free(remote_dirs, rd_count);
    }

    return 0;
}

/* HEAD */

int head_get_ref(char *ref, size_t size) {
    if (!file_exists(MSYNC_HEAD_FILE)) return -1;

    uint8_t *data;
    size_t len;
    if (file_read(MSYNC_HEAD_FILE, &data, &len) != 0) return -1;

    char *content = (char *)data;
    strip_newline(content);

    if (strncmp(content, "ref: ", 5) == 0) {
        snprintf(ref, size, "%s", content + 5);
        free(data);
        return 0;
    }

    free(data);
    return -1;
}

int head_set_ref(const char *ref) {
    char content[MAX_PATH_LEN];
    snprintf(content, sizeof(content), "ref: %s\n", ref);
    return file_write(MSYNC_HEAD_FILE, (uint8_t *)content, strlen(content));
}

int head_get_hash(uint8_t *hash) {
    char ref[MAX_PATH_LEN];
    if (head_get_ref(ref, sizeof(ref)) != 0) return -1;
    return ref_resolve(ref, hash);
}
