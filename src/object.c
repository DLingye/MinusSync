#include "msync.h"

static const char hex_chars[] = "0123456789abcdef";

void hash_to_hex(const uint8_t *hash, char *hex) {
    for (int i = 0; i < HASH_RAW_SIZE; i++) {
        hex[i*2]     = hex_chars[hash[i] >> 4];
        hex[i*2 + 1] = hex_chars[hash[i] & 0x0f];
    }
    hex[HASH_HEX_SIZE] = '\0';
}

static int hex_val(char c) {
    if (c >= '0' && c <= '9') return c - '0';
    if (c >= 'a' && c <= 'f') return c - 'a' + 10;
    if (c >= 'A' && c <= 'F') return c - 'A' + 10;
    return -1;
}

int hex_to_hash(const char *hex, uint8_t *hash) {
    if (strlen(hex) < HASH_HEX_SIZE) return -1;
    for (int i = 0; i < HASH_RAW_SIZE; i++) {
        int hi = hex_val(hex[i*2]);
        int lo = hex_val(hex[i*2 + 1]);
        if (hi < 0 || lo < 0) return -1;
        hash[i] = (uint8_t)((hi << 4) | lo);
    }
    return 0;
}

void hash_cpy(uint8_t *dst, const uint8_t *src) {
    memcpy(dst, src, HASH_RAW_SIZE);
}

int hash_cmp(const uint8_t *a, const uint8_t *b) {
    return memcmp(a, b, HASH_RAW_SIZE);
}

static int object_path(const uint8_t *hash, char *path, size_t size) {
    char hex[HASH_HEX_SIZE + 1];
    hash_to_hex(hash, hex);
    snprintf(path, size, "%s/%c%c/%s", MSYNC_OBJECTS_DIR, hex[0], hex[1], hex + 2);
    return 0;
}

int object_exists(const uint8_t *hash) {
    char path[MAX_PATH_LEN];
    object_path(hash, path, sizeof(path));
    return file_exists(path);
}

int object_write(const uint8_t *data, size_t len, uint8_t *hash_out) {
    sha256_hash(data, len, hash_out);

    char path[MAX_PATH_LEN];
    object_path(hash_out, path, sizeof(path));

    if (file_exists(path)) return 0;

    char hex[HASH_HEX_SIZE + 1];
    hash_to_hex(hash_out, hex);

    char dir[MAX_PATH_LEN];
    snprintf(dir, sizeof(dir), "%s/%c%c", MSYNC_OBJECTS_DIR, hex[0], hex[1]);
    if (!dir_exists(dir)) msync_mkdir(dir);

    return file_write(path, data, len);
}

int object_read(const uint8_t *hash, uint8_t **data, size_t *len) {
    char path[MAX_PATH_LEN];
    object_path(hash, path, sizeof(path));

    if (!file_exists(path)) return -1;
    return file_read(path, data, len);
}

/* --- blob --- */

int blob_create(const char *filepath, uint8_t *hash_out) {
    uint8_t *content;
    size_t clen;
    if (file_read(filepath, &content, &clen) != 0) return -1;

    char header[64];
    int hlen = snprintf(header, sizeof(header), "blob %zu", clen);

    size_t total = (size_t)hlen + 1 + clen;
    uint8_t *buf = (uint8_t *)malloc(total);
    if (!buf) { free(content); return -1; }

    memcpy(buf, header, hlen);
    buf[hlen] = '\0';
    memcpy(buf + hlen + 1, content, clen);

    int ret = object_write(buf, total, hash_out);
    free(buf);
    free(content);
    return ret;
}

/* --- tree --- */

typedef struct {
    char   *name;
    uint8_t hash[HASH_RAW_SIZE];
    uint32_t mode;
} tree_entry_t;

static int tree_entry_cmp(const void *a, const void *b) {
    return strcmp(((tree_entry_t *)a)->name, ((tree_entry_t *)b)->name);
}

int tree_build(const char *dirpath, uint8_t *hash_out) {
    char **files;
    int fcount;
    file_list(dirpath, &files, &fcount);

    tree_entry_t *entries = (tree_entry_t *)malloc(fcount * sizeof(tree_entry_t));
    int ecount = 0;

    for (int i = 0; i < fcount; i++) {
        char fullpath[MAX_PATH_LEN];
        snprintf(fullpath, sizeof(fullpath), "%s/%s", dirpath, files[i]);

        struct stat st;
        if (stat(fullpath, &st) != 0) continue;

        entries[ecount].name = files[i];
        entries[ecount].mode = (uint32_t)st.st_mode;

        if (S_ISDIR(st.st_mode)) {
            if (tree_build(fullpath, entries[ecount].hash) != 0) {
                continue; /* entries[ecount] is dead; file_list_free cleans up files[i] */
            }
        } else {
            if (blob_create(fullpath, entries[ecount].hash) != 0) {
                continue;
            }
        }
        ecount++;
    }

    qsort(entries, ecount, sizeof(tree_entry_t), tree_entry_cmp);

    /* Compute total size */
    size_t total = 0;
    for (int i = 0; i < ecount; i++) {
        total += snprintf(NULL, 0, "%o %s", entries[i].mode, entries[i].name);
        total += 1 + HASH_RAW_SIZE;
    }

    char header[64];
    int hlen = snprintf(header, sizeof(header), "tree %zu", total);

    size_t bufsize = (size_t)hlen + 1 + total;
    uint8_t *buf = (uint8_t *)malloc(bufsize);
    if (!buf) { file_list_free(files, fcount); free(entries); return -1; }

    memcpy(buf, header, hlen);
    buf[hlen] = '\0';

    size_t pos = (size_t)hlen + 1;
    for (int i = 0; i < ecount; i++) {
        int n = snprintf((char *)buf + pos, bufsize - pos,
                         "%o %s", entries[i].mode, entries[i].name);
        pos += n;
        buf[pos++] = '\0';
        memcpy(buf + pos, entries[i].hash, HASH_RAW_SIZE);
        pos += HASH_RAW_SIZE;
    }

    int ret = object_write(buf, pos, hash_out);
    free(buf);
    free(entries);
    file_list_free(files, fcount);
    return ret;
}

int tree_entries_parse(const uint8_t *data, size_t len,
                       char ***names, uint8_t **hashes, int *count) {
    /* Skip header: "tree <size>\0" */
    const uint8_t *p = data;
    while (*p != '\0' && p < data + len) p++;
    if (p >= data + len) return -1;
    p++; /* skip null */

    *count = 0;
    *names = NULL;
    *hashes = NULL;

    while (p < data + len) {
        const uint8_t *start = p;
        while (*p != '\0' && p < data + len) p++;
        if (p >= data + len) break;
        p++; /* skip null */

        /* start points to "mode name" */
        const char *s = (const char *)start;
        const char *spc = strchr(s, ' ');
        if (!spc) break;

        char *name = msync_strdup(spc + 1);

        (*count)++;
        *names = (char **)realloc(*names, (*count) * sizeof(char *));
        *hashes = (uint8_t *)realloc(*hashes, (*count) * HASH_RAW_SIZE);
        (*names)[*count - 1] = name;
        memcpy((*hashes) + (*count - 1) * HASH_RAW_SIZE, p, HASH_RAW_SIZE);
        p += HASH_RAW_SIZE;
    }
    return 0;
}

int tree_checkout(const uint8_t *hash, const char *target_dir) {
    uint8_t *data;
    size_t len;
    if (object_read(hash, &data, &len) != 0) return -1;

    char **names = NULL;
    uint8_t *hashes = NULL;
    int count = 0;

    if (tree_entries_parse(data, len, &names, &hashes, &count) != 0) {
        free(data);
        return -1;
    }

    if (!dir_exists(target_dir)) msync_mkdir(target_dir);

    for (int i = 0; i < count; i++) {
        char path[MAX_PATH_LEN];
        snprintf(path, sizeof(path), "%s/%s", target_dir, names[i]);

        uint8_t *entry_data;
        size_t entry_len;
        if (object_read(hashes + i * HASH_RAW_SIZE, &entry_data, &entry_len) != 0)
            continue;

        /* Check type */
        if (strncmp((char *)entry_data, "tree ", 5) == 0) {
            tree_checkout(hashes + i * HASH_RAW_SIZE, path);
        } else if (strncmp((char *)entry_data, "blob ", 5) == 0) {
            const uint8_t *p = entry_data;
            while (*p != '\0' && p < entry_data + entry_len) p++;
            if (p >= entry_data + entry_len) { free(entry_data); continue; }
            p++;
            if ((size_t)(p - entry_data) > entry_len) { free(entry_data); continue; }
            file_write(path, p, entry_len - (size_t)(p - entry_data));
        }
        free(entry_data);
    }

    free(data);
    for (int i = 0; i < count; i++) free(names[i]);
    free(names);
    free(hashes);
    return 0;
}
