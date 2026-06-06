#include "msync.h"

int repo_checkout(const char *target) {
    if (!dir_exists(MSYNC_DIR)) {
        fprintf(stderr, "Not an msync repository.\n");
        return -1;
    }

    /* Try as branch name first */
    uint8_t hash[HASH_RAW_SIZE];
    char ref[MAX_PATH_LEN];
    snprintf(ref, sizeof(ref), "refs/heads/%s", target);

    if (ref_resolve(ref, hash) != 0) {
        /* Try as commit hash */
        if (hex_to_hash(target, hash) != 0) {
            fprintf(stderr, "Invalid branch or commit hash: %s\n", target);
            return -1;
        }
        if (!object_exists(hash)) {
            fprintf(stderr, "Commit not found: %s\n", target);
            return -1;
        }
    }

    /* For branch checkout, update HEAD */
    if (ref_resolve(ref, hash) == 0) {
        /* Check for uncommitted changes */
        index_load();
        int new_count, mod_count, del_count;
        char **names;
        int *states;
        status_check(&new_count, &mod_count, &del_count, &names, &states);
        int changes = new_count + mod_count + del_count;
        for (int i = 0; i < changes; i++) free(names[i]);
        free(names);
        free(states);
        index_clear_entries();

        if (changes > 0) {
            fprintf(stderr, "You have uncommitted changes. Commit or discard them first.\n");
            return -1;
        }

        head_set_ref(ref);
    }

    /* Checkout tree */
    uint8_t *data;
    size_t len;
    if (object_read(hash, &data, &len) != 0) {
        fprintf(stderr, "Failed to read commit.\n");
        return -1;
    }

    uint8_t tree_hash[HASH_RAW_SIZE];
    if (commit_parse(data, len, tree_hash, NULL, NULL, NULL, NULL, NULL, NULL) != 0) {
        free(data);
        fprintf(stderr, "Failed to parse commit.\n");
        return -1;
    }
    free(data);

    /* Remove existing files (keep .msync) */
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
    if (tree_checkout(tree_hash, ".") != 0) {
        fprintf(stderr, "Failed to checkout tree.\n");
        return -1;
    }

    /* Rebuild index to match the checked-out tree */
    char **idx_files;
    int idx_fcount;
    file_list(".", &idx_files, &idx_fcount);
    index_reset();
    for (int i = 0; i < idx_fcount; i++) {
        if (strncmp(idx_files[i], ".msync", 6) == 0) { free(idx_files[i]); continue; }
        struct stat st;
        if (stat(idx_files[i], &st) != 0) { free(idx_files[i]); continue; }
        uint8_t blob_hash[HASH_RAW_SIZE];
        if (blob_create(idx_files[i], blob_hash) == 0) {
            index_add(idx_files[i], blob_hash, (uint64_t)st.st_size,
                      (int64_t)st.st_mtime, (uint32_t)st.st_mode);
        }
        free(idx_files[i]);
    }
    free(idx_files);
    index_save();
    index_clear_entries();

    printf("Switched to '%s'.\n", target);
    return 0;
}
