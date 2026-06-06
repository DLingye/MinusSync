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
            if (strncmp(relpath, ".msync", 6) != 0) {
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
            if (strncmp(relpath, ".msync", 6) != 0) {
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

int is_ignored(const char *path) {
    if (!file_exists(MSYNC_IGNORE_FILE)) return 0;

    uint8_t *data;
    size_t len;
    if (file_read(MSYNC_IGNORE_FILE, &data, &len) != 0) return 0;

    char *content = (char *)data;
    char *line = strtok(content, "\n\r");
    int ignored = 0;

    while (line) {
        while (*line == ' ' || *line == '\t') line++;
        if (*line && *line != '#') {
            if (strstr(path, line)) { ignored = 1; break; }
        }
        line = strtok(NULL, "\n\r");
    }

    free(data);
    return ignored;
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
