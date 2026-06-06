#include "msync.h"

static int errors   = 0;
static int warnings = 0;
static int checked  = 0;

#define ERR(fmt, ...)  do { fprintf(stderr, "  ERROR: " fmt "\n", ##__VA_ARGS__); errors++; } while(0)
#define WARN(fmt, ...) do { fprintf(stderr, "  WARN:  " fmt "\n", ##__VA_ARGS__); warnings++; } while(0)
#define INFO(fmt, ...) do { if (verbose) printf("  " fmt "\n", ##__VA_ARGS__); } while(0)

static int verbose = 0;
static uint8_t *reachable = NULL;
static int reachable_count = 0;

/* Record an object as reachable */
static void mark_reachable_fsck(const uint8_t *hash) {
    for (int i = 0; i < reachable_count; i++) {
        if (hash_cmp(reachable + i * HASH_RAW_SIZE, hash) == 0) return;
    }
    reachable_count++;
    reachable = (uint8_t *)realloc(reachable, (size_t)reachable_count * HASH_RAW_SIZE);
    memcpy(reachable + (reachable_count - 1) * HASH_RAW_SIZE, hash, HASH_RAW_SIZE);
}

/* Verify a single object's hash matches its content */
static int verify_object_hash(const uint8_t *hash, const char *label) {
    mark_reachable_fsck(hash);
    uint8_t *data;
    size_t len;
    if (object_read(hash, &data, &len) != 0) {
        char hex[HASH_HEX_SIZE + 1];
        hash_to_hex(hash, hex);
        ERR("missing %s object %s", label, hex);
        return -1;
    }
    uint8_t computed[HASH_RAW_SIZE];
    sha256_hash(data, len, computed);
    if (hash_cmp(computed, hash) != 0) {
        char hex[HASH_HEX_SIZE + 1];
        hash_to_hex(hash, hex);
        ERR("corrupted %s object %s (hash mismatch)", label, hex);
        free(data);
        return -1;
    }
    free(data);
    checked++;
    return 0;
}

/* Recursively verify a tree and all its entries */
static int verify_tree(const uint8_t *hash, const char *path) {
    if (verify_object_hash(hash, "tree") != 0) return -1;

    uint8_t *data;
    size_t len;
    if (object_read(hash, &data, &len) != 0) return -1;

    char **names = NULL;
    uint8_t *hashes = NULL;
    int count = 0;

    int ret = tree_entries_parse(data, len, &names, &hashes, &count);
    free(data);

    if (ret != 0) {
        char hex[HASH_HEX_SIZE + 1];
        hash_to_hex(hash, hex);
        ERR("failed to parse tree %s", hex);
        return -1;
    }

    for (int i = 0; i < count; i++) {
        char sub[MAX_PATH_LEN];
        snprintf(sub, sizeof(sub), "%s/%s", path, names[i]);

        uint8_t *entry_data;
        size_t entry_len;
        if (object_read(hashes + i * HASH_RAW_SIZE, &entry_data, &entry_len) != 0) {
            char hex[HASH_HEX_SIZE + 1];
            hash_to_hex(hashes + i * HASH_RAW_SIZE, hex);
            ERR("missing object %s referenced by tree (path: %s)", hex, sub);
            free(names[i]);
            continue;
        }

        if (strncmp((char *)entry_data, "tree ", 5) == 0) {
            INFO("tree  %s", sub);
            verify_tree(hashes + i * HASH_RAW_SIZE, sub);
        } else {
            INFO("blob  %s", sub);
            verify_object_hash(hashes + i * HASH_RAW_SIZE, "blob");
        }
        free(entry_data);
        free(names[i]);
    }
    free(names);
    free(hashes);
    return 0;
}

/* Verify a commit chain (relies on global reachable set for cycle detection) */
static int verify_commit_chain(const uint8_t *hash, int depth) {
    if (depth > 10000) {
        WARN("commit chain too deep (>10000), stopping traversal");
        return 0;
    }

    /* Check for cycles via the global reachable set */
    int already_seen = 0;
    for (int i = 0; i < reachable_count; i++) {
        if (hash_cmp(reachable + i * HASH_RAW_SIZE, hash) == 0) {
            already_seen = 1;
            break;
        }
    }
    if (already_seen) return 0;

    if (verify_object_hash(hash, "commit") != 0) return -1;

    uint8_t *data;
    size_t len;
    if (object_read(hash, &data, &len) != 0) return -1;

    uint8_t tree_hash[HASH_RAW_SIZE];
    uint8_t parent_hash[HASH_RAW_SIZE];
    char author[256], email[256], hostname[256], message[MAX_MSG_LEN];
    time_t ts;

    commit_parse(data, len, tree_hash, parent_hash, author, email, hostname, message, &ts);
    free(data);

    char hex[HASH_HEX_SIZE + 1];
    hash_to_hex(hash, hex);

    /* Check author info */
    if (!author[0]) WARN("commit %s has empty author", hex);
    if (!email[0])  WARN("commit %s has empty email", hex);

    /* Check that tree is valid */
    INFO("commit %s (%s)", hex, message);
    verify_tree(tree_hash, "");

    /* Recurse to parent (check if parent hash is non-zero) */
    {
        int has_parent = 0;
        for (int k = 0; k < HASH_RAW_SIZE; k++) {
            if (parent_hash[k] != 0) { has_parent = 1; break; }
        }
        if (has_parent) {
            verify_commit_chain(parent_hash, depth + 1);
        }
    }

    return 0;
}

/* Check for duplicate objects (same content, different hash — should not happen with SHA-256 but worth checking) */
static int scan_objects_store(void) {
    if (!dir_exists(MSYNC_OBJECTS_DIR)) return 0;

    DIR *d = opendir(MSYNC_OBJECTS_DIR);
    if (!d) return 0;

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

            char objpath[MAX_PATH_LEN];
            snprintf(objpath, sizeof(objpath), "%s/%s", subdir, f->d_name);

            /* Read the object and verify hash */
            uint8_t *data;
            size_t dlen;
            if (file_read(objpath, &data, &dlen) != 0) {
                ERR("cannot read object file %s/%s", entry->d_name, f->d_name);
                continue;
            }

            uint8_t computed[HASH_RAW_SIZE];
            sha256_hash(data, dlen, computed);

            char hex[HASH_HEX_SIZE + 1];
            hash_to_hex(computed, hex);

            /* Check that filename matches hash */
            char expected[68];
            snprintf(expected, sizeof(expected), "%c%c/%s", hex[0], hex[1], hex + 2);
            char actual[68];
            snprintf(actual, sizeof(actual), "%s/%s", entry->d_name, f->d_name);

            if (strcmp(actual, expected) != 0) {
                ERR("object file %s has wrong location (expected %s)", actual, expected);
            }

            free(data);
        }
        closedir(sd);
    }
    closedir(d);
    return 0;
}

int repo_fsck(int verb) {
    verbose = verb;
    errors = 0;
    warnings = 0;
    checked = 0;

    if (!dir_exists(MSYNC_DIR)) {
        fprintf(stderr, "Not an msync repository.\n");
        return -1;
    }

    printf("Checking msync repository integrity...\n\n");

    /* 1. Check HEAD */
    printf("--- HEAD ---\n");
    char head_ref[MAX_PATH_LEN];
    if (head_get_ref(head_ref, sizeof(head_ref)) != 0) {
        if (file_exists(MSYNC_HEAD_FILE)) {
            ERR("HEAD file is corrupted");
        } else {
            ERR("HEAD file is missing");
        }
    } else {
        /* Try resolving the HEAD ref */
        uint8_t head_hash[HASH_RAW_SIZE];
        if (head_get_hash(head_hash) != 0) {
            WARN("HEAD (%s) points to non-existent ref", head_ref);
        } else {
            INFO("HEAD -> %s", head_ref);
        }
    }

    /* 2. Check all refs */
    printf("\n--- Refs ---\n");
    char **ref_names;
    uint8_t *ref_hashes;
    int ref_count;
    ref_list(&ref_names, &ref_hashes, &ref_count);

    /* Reset global reachable set */
    free(reachable);
    reachable = NULL;
    reachable_count = 0;

    for (int i = 0; i < ref_count; i++) {
        char hex[HASH_HEX_SIZE + 1];
        hash_to_hex(ref_hashes + i * HASH_RAW_SIZE, hex);

        if (!object_exists(ref_hashes + i * HASH_RAW_SIZE)) {
            ERR("ref %s points to missing commit %s", ref_names[i], hex);
        } else {
            uint8_t *d;
            size_t dl;
            if (object_read(ref_hashes + i * HASH_RAW_SIZE, &d, &dl) == 0) {
                if (strncmp((char *)d, "commit ", 7) != 0) {
                    ERR("ref %s points to non-commit object %s", ref_names[i], hex);
                } else {
                    INFO("ref   %s -> %.8s", ref_names[i], hex);
                    verify_commit_chain(ref_hashes + i * HASH_RAW_SIZE, 0);
                }
                free(d);
            }
        }
        free(ref_names[i]);
    }
    free(ref_names);
    free(ref_hashes);

    /* 3. Scan object store for file-level issues */
    printf("\n--- Object Store ---\n");
    scan_objects_store();

    /* 4. Find unreachable (dangling) objects */
    printf("\n--- Reachability ---\n");
    int total_objects = 0;
    int dangling = 0;
    {
        DIR *d = opendir(MSYNC_OBJECTS_DIR);
        if (d) {
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
                    total_objects++;

                    char hex_str[HASH_HEX_SIZE + 1];
                    snprintf(hex_str, sizeof(hex_str), "%s%s", entry->d_name, f->d_name);
                    uint8_t h[HASH_RAW_SIZE];
                    if (hex_to_hash(hex_str, h) == 0) {
                        int found = 0;
                        for (int s = 0; s < reachable_count; s++) {
                            if (hash_cmp(h, reachable + s * HASH_RAW_SIZE) == 0) {
                                found = 1;
                                break;
                            }
                        }
                        if (!found) {
                            dangling++;
                            if (verbose) WARN("dangling object: %.16s", hex_str);
                        }
                    }
                }
                closedir(sd);
            }
            closedir(d);
        }
    }
    INFO("reachable: %d objects", reachable_count);
    if (dangling > 0) {
        WARN("%d dangling (unreachable) objects (use 'msync gc' to clean)", dangling);
    }

    free(reachable);
    reachable = NULL;
    reachable_count = 0;

    /* Summary */
    printf("\n--- Summary ---\n");
    printf("Objects checked: %d\n", checked);
    printf("Total objects:   %d\n", total_objects);
    printf("Errors:          %d\n", errors);
    printf("Warnings:        %d\n", warnings);
    printf("Dangling:        %d\n", dangling);

    if (errors > 0) {
        printf("\nRepository has CORRUPTION. Some objects are missing or damaged.\n");
        return 1;
    }
    if (warnings > 0) {
        printf("\nRepository is OK with warnings.\n");
    } else {
        printf("\nRepository is clean.\n");
    }

    return errors > 0 ? 1 : 0;
}
