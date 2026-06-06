#include "msync.h"

int repo_commit(const char *message) {
    if (!dir_exists(MSYNC_DIR)) {
        fprintf(stderr, "Not an msync repository.\n");
        return -1;
    }

    index_load();

    /* Build tree from working directory */
    uint8_t tree_hash[HASH_RAW_SIZE];
    if (tree_build(".", tree_hash) != 0) {
        fprintf(stderr, "Failed to build tree.\n");
        index_clear_entries();
        return -1;
    }

    /* Get parent commit */
    uint8_t parent_hash[HASH_RAW_SIZE];
    int has_parent = (head_get_hash(parent_hash) == 0);

    /* Get author/email from config */
    char author[256] = "unknown";
    char email[256] = "unknown@unknown";
    config_get("user.name", author, sizeof(author));
    config_get("user.email", email, sizeof(email));

    /* Create commit */
    uint8_t commit_hash[HASH_RAW_SIZE];
    if (commit_create(tree_hash, has_parent ? parent_hash : NULL,
                      author, email, message, commit_hash) != 0) {
        fprintf(stderr, "Failed to create commit.\n");
        index_clear_entries();
        return -1;
    }

    /* Update current branch ref */
    char ref[MAX_PATH_LEN];
    if (head_get_ref(ref, sizeof(ref)) != 0) {
        fprintf(stderr, "HEAD is not pointing to a branch.\n");
        index_clear_entries();
        return -1;
    }

    if (ref_update(ref, commit_hash) != 0) {
        fprintf(stderr, "Failed to update ref.\n");
        index_clear_entries();
        return -1;
    }

    /* Update index with new tree state */
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

    if (index_save() != 0) {
        fprintf(stderr, "Failed to save index.\n");
    }

    char hex[HASH_HEX_SIZE + 1];
    hash_to_hex(commit_hash, hex);
    printf("[%s %s] %s\n", ref + 11, hex, message);

    index_clear_entries();
    return 0;
}
