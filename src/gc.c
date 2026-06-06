#include "msync.h"

/* Collect all objects reachable from commit hash into a hash set */
static void mark_reachable(const uint8_t *hash, uint8_t **marked, int *mcount, int depth) {
    if (depth > 10000) return;
    if (!object_exists(hash)) return;

    /* Check if already marked */
    for (int i = 0; i < *mcount; i++) {
        if (hash_cmp(*marked + i * HASH_RAW_SIZE, hash) == 0) return;
    }

    /* Mark this object */
    (*mcount)++;
    *marked = (uint8_t *)realloc(*marked, (*mcount) * HASH_RAW_SIZE);
    memcpy(*marked + (*mcount - 1) * HASH_RAW_SIZE, hash, HASH_RAW_SIZE);

    /* Read object to find references */
    uint8_t *data;
    size_t len;
    if (object_read(hash, &data, &len) != 0) return;

    if (strncmp((char *)data, "commit ", 7) == 0) {
        uint8_t tree_hash[HASH_RAW_SIZE];
        uint8_t parent_hash[HASH_RAW_SIZE];
        commit_parse(data, len, tree_hash, parent_hash, NULL, NULL, NULL, NULL, NULL);
        free(data);

        /* Mark tree */
        mark_reachable(tree_hash, marked, mcount, depth + 1);

        /* Mark parent */
        if (parent_hash[0] != 0) {
            mark_reachable(parent_hash, marked, mcount, depth + 1);
        }
    } else if (strncmp((char *)data, "tree ", 5) == 0) {
        char **names = NULL;
        uint8_t *hashes = NULL;
        int count = 0;
        tree_entries_parse(data, len, &names, &hashes, &count);
        free(data);

        for (int i = 0; i < count; i++) {
            mark_reachable(hashes + i * HASH_RAW_SIZE, marked, mcount, depth + 1);
            free(names[i]);
        }
        free(names);
        free(hashes);
    } else {
        /* blob — no further references */
        free(data);
    }
}

int repo_gc(int prune) {
    if (!dir_exists(MSYNC_DIR)) {
        fprintf(stderr, "Not an msync repository.\n");
        return -1;
    }

    printf("Garbage collecting...\n");

    /* Step 1: Collect all reachable objects from all refs */
    uint8_t *marked = NULL;
    int mcount = 0;

    char **ref_names;
    uint8_t *ref_hashes;
    int ref_count;
    ref_list(&ref_names, &ref_hashes, &ref_count);

    for (int i = 0; i < ref_count; i++) {
        mark_reachable(ref_hashes + i * HASH_RAW_SIZE, &marked, &mcount, 0);
        free(ref_names[i]);
    }
    free(ref_names);
    free(ref_hashes);

    printf("  Reachable objects: %d\n", mcount);

    /* Step 2: Scan object store for unreachable objects */
    int total = 0;
    int removable = 0;
    uint64_t removable_bytes = 0;

    char **dirs = NULL;
    int dircount = 0;

    DIR *d = opendir(MSYNC_OBJECTS_DIR);
    if (!d) {
        free(marked);
        printf("  Object store is empty, nothing to do.\n");
        return 0;
    }

    struct dirent *entry;
    while ((entry = readdir(d)) != NULL) {
        if (entry->d_name[0] == '.') continue;

        char subdir[MAX_PATH_LEN];
        snprintf(subdir, sizeof(subdir), "%s/%s", MSYNC_OBJECTS_DIR, entry->d_name);
        DIR *sd = opendir(subdir);
        if (!sd) continue;

        struct dirent *f;
        while ((f = readdir(sd)) != NULL) {
            if (f->d_name[0] == '.') continue;
            total++;

            char hex_str[HASH_HEX_SIZE + 1];
            snprintf(hex_str, sizeof(hex_str), "%s%s", entry->d_name, f->d_name);
            uint8_t h[HASH_RAW_SIZE];
            if (hex_to_hash(hex_str, h) != 0) continue;

            int found = 0;
            for (int i = 0; i < mcount; i++) {
                if (hash_cmp(h, marked + i * HASH_RAW_SIZE) == 0) {
                    found = 1;
                    break;
                }
            }

            if (!found) {
                removable++;
                char objpath[MAX_PATH_LEN];
                snprintf(objpath, sizeof(objpath), "%s/%s", subdir, f->d_name);

                struct stat st;
                if (stat(objpath, &st) == 0) {
                    removable_bytes += (uint64_t)st.st_size;
                }

                if (prune) {
                    if (unlink(objpath) == 0) {
                        printf("  Removed: %s\n", hex_str);
                    }
                }
            }
        }
        closedir(sd);

        /* If directory is empty after pruning, remove it */
        if (prune) {
            rmdir(subdir); /* fails silently if not empty */
        }
    }
    closedir(d);

    printf("  Total objects:     %d\n", total);
    printf("  Reachable:         %d\n", mcount);
    printf("  Unreachable:       %d\n", removable);
    printf("  Recoverable space: %lu bytes (%.1f KB)\n",
           (unsigned long)removable_bytes, removable_bytes / 1024.0);

    if (removable > 0 && !prune) {
        printf("\n  %d unreachable objects found.\n", removable);
        printf("  Run 'msync gc --prune' to remove them.\n");
    } else if (prune && removable > 0) {
        printf("\n  Pruned %d objects, freed %.1f KB.\n",
               removable, removable_bytes / 1024.0);
    } else if (removable == 0) {
        printf("\n  Repository is clean, no garbage found.\n");
    }

    free(marked);
    return 0;
}
