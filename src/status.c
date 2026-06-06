#include "msync.h"

int repo_status(void) {
    if (!dir_exists(MSYNC_DIR)) {
        fprintf(stderr, "Not an msync repository.\n");
        return -1;
    }

    index_load();

    int new_count, mod_count, del_count;
    char **names;
    int *states;

    status_check(&new_count, &mod_count, &del_count, &names, &states);

    if (new_count == 0 && mod_count == 0 && del_count == 0) {
        printf("Nothing to commit, working tree clean.\n");
        index_clear_entries();
        return 0;
    }

    /* Show current branch */
    char ref[MAX_PATH_LEN];
    if (head_get_ref(ref, sizeof(ref)) == 0) {
        printf("On branch %s\n", ref + 11); /* skip "refs/heads/" */
    } else {
        printf("HEAD detached\n");
    }

    if (new_count > 0) {
        printf("\nNew files:\n");
        for (int i = 0; i < new_count + mod_count + del_count; i++) {
            if (states[i] == STATUS_NEW)
                printf("  \033[32mnew:  %s\033[0m\n", names[i]);
        }
    }

    if (mod_count > 0) {
        printf("\nModified files:\n");
        for (int i = 0; i < new_count + mod_count + del_count; i++) {
            if (states[i] == STATUS_MODIFIED)
                printf("  \033[33mmod:  %s\033[0m\n", names[i]);
        }
    }

    if (del_count > 0) {
        printf("\nDeleted files:\n");
        for (int i = 0; i < new_count + mod_count + del_count; i++) {
            if (states[i] == STATUS_DELETED)
                printf("  \033[31mdel:  %s\033[0m\n", names[i]);
        }
    }

    for (int i = 0; i < new_count + mod_count + del_count; i++)
        free(names[i]);
    free(names);
    free(states);
    index_clear_entries();
    return 0;
}
