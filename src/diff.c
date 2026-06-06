#include "msync.h"

/* Simple line-by-line diff. Returns 0 if same, 1 if different, -1 on error. */
int diff_file(const char *path, const uint8_t *old_hash, const uint8_t *new_hash) {
    uint8_t *old_data = NULL, *new_data = NULL;
    size_t old_len = 0, new_len = 0;

    if (old_hash && object_exists(old_hash)) {
        object_read(old_hash, &old_data, &old_len);
    }

    if (new_hash && object_exists(new_hash)) {
        object_read(new_hash, &new_data, &new_len);
    }

    /* Skip blob headers */
    if (old_data && old_len > 0 && strncmp((char *)old_data, "blob ", 5) == 0) {
        uint8_t *p = old_data;
        while (*p != '\0' && p < old_data + old_len) p++;
        p++;
        memmove(old_data, p, old_len - (p - old_data));
        old_len -= (p - old_data);
    }

    if (new_data && new_len > 0 && strncmp((char *)new_data, "blob ", 5) == 0) {
        uint8_t *p = new_data;
        while (*p != '\0' && p < new_data + new_len) p++;
        p++;
        memmove(new_data, p, new_len - (p - new_data));
        new_len -= (p - new_data);
    }

    if (!old_data && !new_data) return 0;
    if (!old_data) {
        /* New file - show all lines as added */
        printf("diff --git a/%s b/%s\n", path, path);
        printf("new file\n");
        printf("--- /dev/null\n");
        printf("+++ b/%s\n", path);
        char *line = strtok((char *)new_data, "\n");
        while (line) {
            printf("+%s\n", line);
            line = strtok(NULL, "\n");
        }
        free(new_data);
        return 1;
    }
    if (!new_data) {
        /* Deleted file */
        printf("diff --git a/%s b/%s\n", path, path);
        printf("deleted file\n");
        printf("--- a/%s\n", path);
        printf("+++ /dev/null\n");
        char *line = strtok((char *)old_data, "\n");
        while (line) {
            printf("-%s\n", line);
            line = strtok(NULL, "\n");
        }
        free(old_data);
        return 1;
    }

    if (old_len == new_len && memcmp(old_data, new_data, old_len) == 0) {
        free(old_data);
        free(new_data);
        return 0;
    }

    /* Simple unified diff for text files */
    printf("diff --git a/%s b/%s\n", path, path);
    printf("--- a/%s\n", path);
    printf("+++ b/%s\n", path);

    /* Simple line-by-line comparison */
    char *old_copy = (char *)malloc(old_len + 1);
    char *new_copy = (char *)malloc(new_len + 1);
    if (!old_copy || !new_copy) { free(old_copy); free(new_copy); free(old_data); free(new_data); return -1; }
    memcpy(old_copy, old_data, old_len); old_copy[old_len] = '\0';
    memcpy(new_copy, new_data, new_len); new_copy[new_len] = '\0';

    char **old_lines = NULL, **new_lines = NULL;
    int old_count = 0, new_count = 0;

    char *tok = strtok(old_copy, "\n");
    while (tok) {
        old_count++;
        old_lines = (char **)realloc(old_lines, old_count * sizeof(char *));
        old_lines[old_count - 1] = tok;
        tok = strtok(NULL, "\n");
    }

    tok = strtok(new_copy, "\n");
    while (tok) {
        new_count++;
        new_lines = (char **)realloc(new_lines, new_count * sizeof(char *));
        new_lines[new_count - 1] = tok;
        tok = strtok(NULL, "\n");
    }

    /* Very simple diff: show all old and new lines interleaved */
    for (int i = 0; i < old_count; i++) {
        int found = 0;
        for (int j = 0; j < new_count && !found; j++) {
            if (strcmp(old_lines[i], new_lines[j]) == 0) {
                printf(" %s\n", old_lines[i]);
                found = 1;
            }
        }
        if (!found) printf("-%s\n", old_lines[i]);
    }
    for (int j = 0; j < new_count; j++) {
        int found = 0;
        for (int i = 0; i < old_count && !found; i++) {
            if (strcmp(new_lines[j], old_lines[i]) == 0) found = 1;
        }
        if (!found) printf("+%s\n", new_lines[j]);
    }

    free(old_copy);
    free(new_copy);
    free(old_lines);
    free(new_lines);
    free(old_data);
    free(new_data);
    return 1;
}

int status_check(int *new_count, int *mod_count, int *del_count,
                 char ***names, int **states) {
    *new_count = 0;
    *mod_count = 0;
    *del_count = 0;
    *names = NULL;
    *states = NULL;

    char **files;
    int fcount;
    file_list(".", &files, &fcount);

    /* Check for new and modified files */
    for (int i = 0; i < fcount; i++) {
        if (strncmp(files[i], ".msync", 6) == 0) { free(files[i]); continue; }

        struct stat st;
        if (stat(files[i], &st) != 0) { free(files[i]); continue; }

        uint8_t idx_hash[HASH_RAW_SIZE];
        uint64_t idx_size;
        int64_t idx_mtime;

        if (index_lookup(files[i], idx_hash, &idx_size, &idx_mtime) == 0) {
            /* File is in index - check if modified */
            if (idx_size != (uint64_t)st.st_size || idx_mtime != (int64_t)st.st_mtime) {
                /* Size or mtime changed - verify with hash */
                uint8_t cur_hash[HASH_RAW_SIZE];
                if (blob_create(files[i], cur_hash) == 0) {
                    if (hash_cmp(idx_hash, cur_hash) != 0) {
                        (*mod_count)++;
                        int total = *mod_count + *new_count + *del_count;
                        *names = (char **)realloc(*names, total * sizeof(char *));
                        *states = (int *)realloc(*states, total * sizeof(int));
                        (*names)[total - 1] = msync_strdup(files[i]);
                        (*states)[total - 1] = STATUS_MODIFIED;
                    }
                }
            }
        } else {
            /* Not in index - new file */
            (*new_count)++;
            int total = *mod_count + *new_count + *del_count;
            *names = (char **)realloc(*names, total * sizeof(char *));
            *states = (int *)realloc(*states, total * sizeof(int));
            (*names)[total - 1] = msync_strdup(files[i]);
            (*states)[total - 1] = STATUS_NEW;
        }
        free(files[i]);
    }
    free(files);

    /* Check for deleted files (in index but not on disk) */
    int idx_count = index_get_count();
    for (int i = 0; i < idx_count; i++) {
        const char *path = index_get_path(i);
        if (!file_exists(path) && !dir_exists(path)) {
            (*del_count)++;
            int total = *mod_count + *new_count + *del_count;
            *names = (char **)realloc(*names, total * sizeof(char *));
            *states = (int *)realloc(*states, total * sizeof(int));
            (*names)[total - 1] = msync_strdup(path);
            (*states)[total - 1] = STATUS_DELETED;
        }
    }

    return 0;
}
