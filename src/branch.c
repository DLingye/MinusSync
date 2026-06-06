#include "msync.h"

int repo_branch_list(void) {
    if (!dir_exists(MSYNC_DIR)) {
        fprintf(stderr, "Not an msync repository.\n");
        return -1;
    }

    char **names;
    uint8_t *hashes;
    int count;

    if (ref_list(&names, &hashes, &count) != 0) return -1;

    char current_ref[MAX_PATH_LEN];
    int has_head = (head_get_ref(current_ref, sizeof(current_ref)) == 0);

    for (int i = 0; i < count; i++) {
        if (strncmp(names[i], "remotes/", 8) == 0) continue;

        int is_current = has_head && strcmp(names[i], current_ref + 5) == 0;

        char hex[HASH_HEX_SIZE + 1];
        hash_to_hex(hashes + i * HASH_RAW_SIZE, hex);
        hex[7] = '\0'; /* Short hash */

        if (is_current) {
            printf("* \033[32m%-30s\033[0m %s\n", names[i], hex);
        } else {
            printf("  %-30s %s\n", names[i], hex);
        }

        free(names[i]);
    }
    free(names);
    free(hashes);
    return 0;
}

int repo_branch_create(const char *name) {
    if (!dir_exists(MSYNC_DIR)) {
        fprintf(stderr, "Not an msync repository.\n");
        return -1;
    }

    uint8_t hash[HASH_RAW_SIZE];
    if (head_get_hash(hash) != 0) {
        fprintf(stderr, "No commits yet. Branch created but no commit to point to.\n");
        memset(hash, 0, HASH_RAW_SIZE);
    }

    char ref[MAX_PATH_LEN];
    snprintf(ref, sizeof(ref), "refs/heads/%s", name);

    char path[MAX_PATH_LEN];
    snprintf(path, sizeof(path), "%s/%s", MSYNC_HEADS_DIR, name);
    if (file_exists(path)) {
        fprintf(stderr, "Branch '%s' already exists.\n", name);
        return -1;
    }

    if (ref_update(ref, hash) != 0) {
        fprintf(stderr, "Failed to create branch.\n");
        return -1;
    }

    printf("Created branch '%s'.\n", name);
    return 0;
}

int repo_branch_delete(const char *name) {
    if (!dir_exists(MSYNC_DIR)) {
        fprintf(stderr, "Not an msync repository.\n");
        return -1;
    }

    char ref[MAX_PATH_LEN];
    snprintf(ref, sizeof(ref), "refs/heads/%s", name);

    char current_ref[MAX_PATH_LEN];
    if (head_get_ref(current_ref, sizeof(current_ref)) == 0) {
        if (strcmp(current_ref, ref) == 0) {
            fprintf(stderr, "Cannot delete current branch.\n");
            return -1;
        }
    }

    if (ref_delete(ref) != 0) {
        fprintf(stderr, "Branch '%s' not found.\n", name);
        return -1;
    }

    printf("Deleted branch '%s'.\n", name);
    return 0;
}
