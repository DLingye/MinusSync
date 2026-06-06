#include "msync.h"

#ifdef _WIN32
    #include <windows.h>
#else
    #include <dirent.h>
    #include <unistd.h>
#endif

int msync_mkdir(const char *path) {
    char tmp[MAX_PATH_LEN];
    char *p = NULL;
    size_t len;

    snprintf(tmp, sizeof(tmp), "%s", path);
    len = strlen(tmp);
    if (tmp[len - 1] == '/') tmp[len - 1] = 0;

    for (p = tmp + 1; *p; p++) {
        if (*p == '/') {
            *p = 0;
#ifdef _WIN32
            mkdir(tmp);
#else
            mkdir(tmp, 0755);
#endif
            *p = '/';
        }
    }
#ifdef _WIN32
    return mkdir(tmp);
#else
    return mkdir(tmp, 0755);
#endif
}

int file_exists(const char *path) {
    struct stat st;
    return stat(path, &st) == 0 && S_ISREG(st.st_mode);
}

int dir_exists(const char *path) {
    struct stat st;
    return stat(path, &st) == 0 && S_ISDIR(st.st_mode);
}

int file_read(const char *path, uint8_t **data, size_t *len) {
    FILE *f = fopen(path, "rb");
    if (!f) return -1;

    fseek(f, 0, SEEK_END);
    long flen = ftell(f);
    if (flen < 0 || flen > MAX_OBJ_SIZE) { fclose(f); return -1; }
    fseek(f, 0, SEEK_SET);

    *len = (size_t)flen;
    *data = (uint8_t *)malloc(*len + 1);
    if (!*data) { fclose(f); return -1; }

    size_t n = fread(*data, 1, *len, f);
    fclose(f);

    if (n != *len) { free(*data); *data = NULL; return -1; }
    (*data)[*len] = '\0';
    return 0;
}

int file_write(const char *path, const uint8_t *data, size_t len) {
    FILE *f = fopen(path, "wb");
    if (!f) return -1;

    size_t n = fwrite(data, 1, len, f);
    fclose(f);
    return (n == len) ? 0 : -1;
}

void file_list_recursive(const char *dir, char ***files, int *count, const char *base) {
#ifdef _WIN32
    WIN32_FIND_DATA fd;
    HANDLE hFind;
    char pattern[MAX_PATH_LEN];
    snprintf(pattern, sizeof(pattern), "%s\\*", dir);

    hFind = FindFirstFile(pattern, &fd);
    if (hFind == INVALID_HANDLE_VALUE) return;

    do {
        if (strcmp(fd.cFileName, ".") == 0 || strcmp(fd.cFileName, "..") == 0) continue;
        char fullpath[MAX_PATH_LEN];
        snprintf(fullpath, sizeof(fullpath), "%s\\%s", dir, fd.cFileName);

        char relpath[MAX_PATH_LEN];
        if (base) snprintf(relpath, sizeof(relpath), "%s/%s", base, fd.cFileName);
        else snprintf(relpath, sizeof(relpath), "%s", fd.cFileName);

        if (fd.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY) {
            if (strncmp(relpath, ".msync", 6) != 0 && !is_ignored(relpath)) {
                file_list_recursive(fullpath, files, count, relpath);
            }
        } else {
            if (is_ignored(relpath)) continue;
            (*count)++;
            *files = (char **)realloc(*files, (*count) * sizeof(char *));
            (*files)[*count - 1] = msync_strdup(relpath);
        }
    } while (FindNextFile(hFind, &fd));
    FindClose(hFind);
#else
    DIR *d = opendir(dir);
    if (!d) return;

    struct dirent *entry;
    while ((entry = readdir(d)) != NULL) {
        if (strcmp(entry->d_name, ".") == 0 || strcmp(entry->d_name, "..") == 0)
            continue;

        char fullpath[MAX_PATH_LEN];
        snprintf(fullpath, sizeof(fullpath), "%s/%s", dir, entry->d_name);

        char relpath[MAX_PATH_LEN];
        if (base) snprintf(relpath, sizeof(relpath), "%s/%s", base, entry->d_name);
        else snprintf(relpath, sizeof(relpath), "%s", entry->d_name);

        struct stat st;
        if (stat(fullpath, &st) != 0) continue;

        if (S_ISDIR(st.st_mode)) {
            if (strncmp(relpath, ".msync", 6) != 0 && !is_ignored(relpath)) {
                file_list_recursive(fullpath, files, count, relpath);
            }
        } else if (S_ISREG(st.st_mode)) {
            if (is_ignored(relpath)) continue;
            (*count)++;
            *files = (char **)realloc(*files, (*count) * sizeof(char *));
            (*files)[*count - 1] = msync_strdup(relpath);
        }
    }
    closedir(d);
#endif
}

void file_list(const char *dir, char ***files, int *count) {
    *files = NULL;
    *count = 0;
    file_list_recursive(dir, files, count, NULL);
}

void file_list_free(char **files, int count) {
    for (int i = 0; i < count; i++) free(files[i]);
    free(files);
}

/* Glob matching: supports *, ?, **, and directory prefix matching.
 * p: pattern (never contains trailing / after parsing)
 * s: string (file path, never starts with /)
 * is_dir: 1 if s is a directory */
static int glob_match(const char *p, const char *s, int is_dir) {
    /* Handle **: match any number of path components */
    if (p[0] == '*' && p[1] == '*') {
        p += 2;
        if (*p == '/') p++;       /* skip / after ** */
        if (*p == '\0') return 1; /* ** matches everything */
        /* Try matching at every position */
        const char *sp = s;
        while (*sp) {
            if (glob_match(p, sp, is_dir)) return 1;
            /* Move to next path component */
            while (*sp && *sp != '/') sp++;
            if (*sp == '/') sp++;
        }
        return glob_match(p, sp, is_dir);
    }

    while (*p && *s) {
        if (*p == '*') {
            p++;
            if (*p == '\0') {
                /* Trailing * matches everything except / in filename */
                while (*s && *s != '/') s++;
                return *s == '\0';
            }
            /* Try matching at every position within current component */
            const char *sp = s;
            while (*sp) {
                if (glob_match(p, sp, is_dir)) return 1;
                if (*sp == '/') break;
                sp++;
            }
            return glob_match(p, sp, is_dir);
        }
        if (*p == '?') {
            if (*s == '/') return 0;
            p++; s++;
            continue;
        }
        if (*p != *s) return 0;
        p++; s++;
    }

    /* Pattern exhausted: string must be at end or at / */
    if (*p == '\0') {
        if (*s == '\0') return 1;
        if (is_dir && *s != '\0') return 0;
        return (*s == '\0');
    }

    return 0;
}

int is_ignored(const char *path) {
    if (!file_exists(MSYNC_IGNORE_FILE)) return 0;

    uint8_t *data;
    size_t len;
    if (file_read(MSYNC_IGNORE_FILE, &data, &len) != 0) return 0;

    /* Determine if path is a directory */
    struct stat st;
    int is_dir = (stat(path, &st) == 0 && S_ISDIR(st.st_mode));

    char *content = (char *)data;
    char *line = strtok(content, "\n\r");
    int ignored = 0;

    while (line) {
        /* Trim leading whitespace */
        while (*line == ' ' || *line == '\t') line++;

        if (*line == '\0' || *line == '#') {
            line = strtok(NULL, "\n\r");
            continue;
        }

        int negate = 0;
        if (*line == '!') {
            negate = 1;
            line++;
        }

        /* Remove trailing whitespace */
        size_t llen = strlen(line);
        while (llen > 0 && (line[llen-1] == ' ' || line[llen-1] == '\t' || line[llen-1] == '\r')) {
            line[--llen] = '\0';
        }

        int dir_only = 0;
        if (llen > 0 && line[llen-1] == '/') {
            dir_only = 1;
            line[--llen] = '\0';
        }

        if (llen == 0) {
            line = strtok(NULL, "\n\r");
            continue;
        }

        /* Remove leading / for root-only match indicator */
        int root_only = 0;
        if (line[0] == '/') {
            root_only = 1;
            line++;
            llen--;
        }

        /* Determine the string to match against */
        const char *match_str = path;

        if (root_only) {
            /* Strip leading directory components for root match */
            /* Match just the filename (no path separators) */
            const char *last = strrchr(path, '/');
            match_str = last ? last + 1 : path;
        }

        if (!strchr(line, '/') && !root_only) {
            /* Pattern without /: match against filename only */
            const char *last = strrchr(path, '/');
            match_str = last ? last + 1 : path;
        }

        if (dir_only && !is_dir) {
            line = strtok(NULL, "\n\r");
            continue;
        }

        if (glob_match(line, match_str, is_dir)) {
            ignored = negate ? 0 : 1;
        }

        line = strtok(NULL, "\n\r");
    }

    free(data);
    return ignored;
}

/* Add a pattern to .msyncign */
int ignore_add(const char *pattern) {
    if (!pattern || !pattern[0]) return -1;

    /* Read existing content */
    uint8_t *data = NULL;
    size_t len = 0;
    char new_content[65536];
    int pos = 0;

    if (file_exists(MSYNC_IGNORE_FILE)) {
        if (file_read(MSYNC_IGNORE_FILE, &data, &len) == 0) {
            /* Check if pattern already exists */
            char *content = (char *)data;
            char *line = strtok(content, "\n\r");
            while (line) {
                while (*line == ' ' || *line == '\t') line++;
                if (strcmp(line, pattern) == 0) {
                    free(data);
                    return 0; /* Already present */
                }
                pos += snprintf(new_content + pos, sizeof(new_content) - pos,
                                "%s\n", line);
                line = strtok(NULL, "\n\r");
            }
            free(data);
        }
    }

    /* Append new pattern */
    pos += snprintf(new_content + pos, sizeof(new_content) - pos,
                    "%s\n", pattern);

    return file_write(MSYNC_IGNORE_FILE, (uint8_t *)new_content, (size_t)pos);
}

/* List patterns in .msyncign */
int ignore_list(void) {
    if (!file_exists(MSYNC_IGNORE_FILE)) {
        printf("No .msyncign file found. Add patterns with 'msync ignore add <pattern>'.\n");
        return 0;
    }

    uint8_t *data;
    size_t len;
    if (file_read(MSYNC_IGNORE_FILE, &data, &len) != 0) return -1;

    printf("Ignore patterns (%s):\n", MSYNC_IGNORE_FILE);
    char *content = (char *)data;
    char *line = strtok(content, "\n\r");
    while (line) {
        while (*line == ' ' || *line == '\t') line++;
        if (*line && *line != '#') {
            printf("  %s\n", line);
        }
        line = strtok(NULL, "\n\r");
    }
    free(data);
    return 0;
}

/* Remove a pattern from .msyncign */
int ignore_remove(const char *pattern) {
    if (!file_exists(MSYNC_IGNORE_FILE)) return -1;

    uint8_t *data;
    size_t len;
    if (file_read(MSYNC_IGNORE_FILE, &data, &len) != 0) return -1;

    char new_content[65536];
    int pos = 0;
    int removed = 0;

    char *content = (char *)data;
    char *line = strtok(content, "\n\r");
    while (line) {
        char trimmed[4096];
        char *s = line;
        while (*s == ' ' || *s == '\t') s++;
        snprintf(trimmed, sizeof(trimmed), "%s", s);
        /* Also trim trailing whitespace */
        size_t tl = strlen(trimmed);
        while (tl > 0 && (trimmed[tl-1] == ' ' || trimmed[tl-1] == '\t' || trimmed[tl-1] == '\r'))
            trimmed[--tl] = '\0';

        if (strcmp(trimmed, pattern) != 0) {
            pos += snprintf(new_content + pos, sizeof(new_content) - pos,
                            "%s\n", line);
        } else {
            removed = 1;
        }
        line = strtok(NULL, "\n\r");
    }

    free(data);
    if (removed) {
        file_write(MSYNC_IGNORE_FILE, (uint8_t *)new_content, (size_t)pos);
    }
    return removed ? 0 : -1;
}

void strip_newline(char *s) {
    size_t len = strlen(s);
    while (len > 0 && (s[len-1] == '\n' || s[len-1] == '\r')) {
        s[len-1] = '\0';
        len--;
    }
}

char *msync_strdup(const char *s) {
    if (!s) return NULL;
    size_t len = strlen(s) + 1;
    char *d = (char *)malloc(len);
    if (d) memcpy(d, s, len);
    return d;
}

void msync_free(void *p) {
    if (p) free(p);
}
