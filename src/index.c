#include "msync.h"

#define INDEX_MAGIC   0x4D53594E  /* "MSYN" */
#define INDEX_VERSION 1
#define MAX_ENTRIES   100000

typedef struct {
    char     *path;
    uint8_t   hash[HASH_RAW_SIZE];
    uint64_t  size;
    int64_t   mtime;
    uint32_t  mode;
} index_entry_t;

static index_entry_t *entries = NULL;
static int entry_count = 0;

int index_load(void) {
    if (!file_exists(MSYNC_INDEX_FILE)) return 0;

    uint8_t *data;
    size_t len;
    if (file_read(MSYNC_INDEX_FILE, &data, &len) != 0) return -1;

    if (len < 12) { free(data); return 0; }

    uint32_t magic = ((uint32_t)data[0] << 24) | ((uint32_t)data[1] << 16) |
                     ((uint32_t)data[2] << 8)  | (uint32_t)data[3];
    if (magic != INDEX_MAGIC) { free(data); return 0; }

    uint32_t version = ((uint32_t)data[4] << 24) | ((uint32_t)data[5] << 16) |
                       ((uint32_t)data[6] << 8)  | (uint32_t)data[7];
    if (version != INDEX_VERSION) { free(data); return 0; }

    entry_count = ((uint32_t)data[8] << 24) | ((uint32_t)data[9] << 16) |
                  ((uint32_t)data[10] << 8) | (uint32_t)data[11];

    entries = (index_entry_t *)calloc(entry_count, sizeof(index_entry_t));
    if (!entries) { free(data); return -1; }

    size_t pos = 12;
    for (int i = 0; i < entry_count; i++) {
        /* path (null-terminated) */
        entries[i].path = msync_strdup((char *)(data + pos));
        pos += strlen((char *)(data + pos)) + 1;

        /* hash */
        if (pos + HASH_RAW_SIZE > len) break;
        memcpy(entries[i].hash, data + pos, HASH_RAW_SIZE);
        pos += HASH_RAW_SIZE;

        /* size */
        if (pos + 8 > len) break;
        entries[i].size = ((uint64_t)data[pos] << 56) | ((uint64_t)data[pos+1] << 48) |
                          ((uint64_t)data[pos+2] << 40) | ((uint64_t)data[pos+3] << 32) |
                          ((uint64_t)data[pos+4] << 24) | ((uint64_t)data[pos+5] << 16) |
                          ((uint64_t)data[pos+6] << 8)  | (uint64_t)data[pos+7];
        pos += 8;

        /* mtime */
        if (pos + 8 > len) break;
        entries[i].mtime = ((int64_t)data[pos] << 56) | ((int64_t)data[pos+1] << 48) |
                           ((int64_t)data[pos+2] << 40) | ((int64_t)data[pos+3] << 32) |
                           ((int64_t)data[pos+4] << 24) | ((int64_t)data[pos+5] << 16) |
                           ((int64_t)data[pos+6] << 8)  | (int64_t)data[pos+7];
        pos += 8;

        /* mode */
        if (pos + 4 > len) break;
        entries[i].mode = ((uint32_t)data[pos] << 24) | ((uint32_t)data[pos+1] << 16) |
                          ((uint32_t)data[pos+2] << 8) | (uint32_t)data[pos+3];
        pos += 4;
    }

    free(data);
    return 0;
}

int index_save(void) {
    /* Calculate size */
    size_t total = 12; /* magic + version + count */
    for (int i = 0; i < entry_count; i++) {
        total += strlen(entries[i].path) + 1;
        total += HASH_RAW_SIZE + 8 + 8 + 4;
    }

    uint8_t *data = (uint8_t *)calloc(1, total);
    if (!data) return -1;

    /* Magic */
    data[0] = (INDEX_MAGIC >> 24) & 0xff;
    data[1] = (INDEX_MAGIC >> 16) & 0xff;
    data[2] = (INDEX_MAGIC >> 8)  & 0xff;
    data[3] = INDEX_MAGIC & 0xff;

    /* Version */
    data[4] = (INDEX_VERSION >> 24) & 0xff;
    data[5] = (INDEX_VERSION >> 16) & 0xff;
    data[6] = (INDEX_VERSION >> 8)  & 0xff;
    data[7] = INDEX_VERSION & 0xff;

    /* Count */
    data[8]  = (entry_count >> 24) & 0xff;
    data[9]  = (entry_count >> 16) & 0xff;
    data[10] = (entry_count >> 8)  & 0xff;
    data[11] = entry_count & 0xff;

    size_t pos = 12;
    for (int i = 0; i < entry_count; i++) {
        /* path */
        size_t plen = strlen(entries[i].path) + 1;
        memcpy(data + pos, entries[i].path, plen);
        pos += plen;

        /* hash */
        memcpy(data + pos, entries[i].hash, HASH_RAW_SIZE);
        pos += HASH_RAW_SIZE;

        /* size */
        data[pos++] = (entries[i].size >> 56) & 0xff;
        data[pos++] = (entries[i].size >> 48) & 0xff;
        data[pos++] = (entries[i].size >> 40) & 0xff;
        data[pos++] = (entries[i].size >> 32) & 0xff;
        data[pos++] = (entries[i].size >> 24) & 0xff;
        data[pos++] = (entries[i].size >> 16) & 0xff;
        data[pos++] = (entries[i].size >> 8)  & 0xff;
        data[pos++] = entries[i].size & 0xff;

        /* mtime */
        data[pos++] = (entries[i].mtime >> 56) & 0xff;
        data[pos++] = (entries[i].mtime >> 48) & 0xff;
        data[pos++] = (entries[i].mtime >> 40) & 0xff;
        data[pos++] = (entries[i].mtime >> 32) & 0xff;
        data[pos++] = (entries[i].mtime >> 24) & 0xff;
        data[pos++] = (entries[i].mtime >> 16) & 0xff;
        data[pos++] = (entries[i].mtime >> 8)  & 0xff;
        data[pos++] = entries[i].mtime & 0xff;

        /* mode */
        data[pos++] = (entries[i].mode >> 24) & 0xff;
        data[pos++] = (entries[i].mode >> 16) & 0xff;
        data[pos++] = (entries[i].mode >> 8)  & 0xff;
        data[pos++] = entries[i].mode & 0xff;
    }

    int ret = file_write(MSYNC_INDEX_FILE, data, total);
    free(data);
    return ret;
}

int index_add(const char *path, const uint8_t *hash, uint64_t size, int64_t mtime, uint32_t mode) {
    /* Look for existing entry */
    for (int i = 0; i < entry_count; i++) {
        if (strcmp(entries[i].path, path) == 0) {
            free(entries[i].path);
            entries[i].path = msync_strdup(path);
            memcpy(entries[i].hash, hash, HASH_RAW_SIZE);
            entries[i].size = size;
            entries[i].mtime = mtime;
            entries[i].mode = mode;
            return 0;
        }
    }

    /* New entry */
    entry_count++;
    entries = (index_entry_t *)realloc(entries, entry_count * sizeof(index_entry_t));
    entries[entry_count - 1].path = msync_strdup(path);
    memcpy(entries[entry_count - 1].hash, hash, HASH_RAW_SIZE);
    entries[entry_count - 1].size = size;
    entries[entry_count - 1].mtime = mtime;
    entries[entry_count - 1].mode = mode;
    return 0;
}

int index_remove(const char *path) {
    for (int i = 0; i < entry_count; i++) {
        if (strcmp(entries[i].path, path) == 0) {
            free(entries[i].path);
            if (i < entry_count - 1) {
                memmove(&entries[i], &entries[i+1],
                        (entry_count - i - 1) * sizeof(index_entry_t));
            }
            entry_count--;
            return 0;
        }
    }
    return -1;
}

int index_lookup(const char *path, uint8_t *hash, uint64_t *size, int64_t *mtime) {
    for (int i = 0; i < entry_count; i++) {
        if (strcmp(entries[i].path, path) == 0) {
            if (hash)  memcpy(hash, entries[i].hash, HASH_RAW_SIZE);
            if (size)  *size  = entries[i].size;
            if (mtime) *mtime = entries[i].mtime;
            return 0;
        }
    }
    return -1;
}

int index_get_count(void) {
    return entry_count;
}

const char *index_get_path(int i) {
    if (i < 0 || i >= entry_count) return NULL;
    return entries[i].path;
}

void index_clear_entries(void) {
    for (int i = 0; i < entry_count; i++) free(entries[i].path);
    free(entries);
    entries = NULL;
    entry_count = 0;
}

/* Reset in-memory index without touching disk. Use before rebuilding. */
void index_reset(void) {
    index_clear_entries();
}
