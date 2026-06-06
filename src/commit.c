#include "msync.h"

int commit_create(const uint8_t *tree_hash, const uint8_t *parent_hash,
                  const char *author, const char *email,
                  const char *message, uint8_t *hash_out) {
    char hostname[256];
    if (gethostname(hostname, sizeof(hostname)) != 0) {
        snprintf(hostname, sizeof(hostname), "unknown");
    }

    time_t now = time(NULL);

    /* Build commit content */
    char *content = (char *)malloc(MAX_COMMIT_SIZE);
    if (!content) return -1;
    int pos = 0;

    char tree_hex[HASH_HEX_SIZE + 1];
    hash_to_hex(tree_hash, tree_hex);

    pos += snprintf(content + pos, MAX_COMMIT_SIZE - pos, "tree %s\n", tree_hex);

    if (parent_hash) {
        char parent_hex[HASH_HEX_SIZE + 1];
        hash_to_hex(parent_hash, parent_hex);
        pos += snprintf(content + pos, MAX_COMMIT_SIZE - pos, "parent %s\n", parent_hex);
    }

    pos += snprintf(content + pos, MAX_COMMIT_SIZE - pos,
                    "author %s <%s> %ld\n", author, email, (long)now);
    pos += snprintf(content + pos, MAX_COMMIT_SIZE - pos,
                    "hostname %s\n", hostname);
    pos += snprintf(content + pos, MAX_COMMIT_SIZE - pos, "\n%s\n", message);

    /* Build object with header */
    size_t body_len = strlen(content);
    char header[64];
    int hlen = snprintf(header, sizeof(header), "commit %zu", body_len);

    size_t total = (size_t)hlen + 1 + body_len;
    uint8_t *buf = (uint8_t *)malloc(total);
    if (!buf) { free(content); return -1; }

    memcpy(buf, header, hlen);
    buf[hlen] = '\0';
    memcpy(buf + hlen + 1, content, body_len);

    int ret = object_write(buf, total, hash_out);
    free(buf);
    free(content);
    return ret;
}

int commit_parse(const uint8_t *data, size_t len,
                 uint8_t *tree_hash, uint8_t *parent_hash,
                 char *author, char *email, char *hostname,
                 char *message, time_t *timestamp) {
    /* Skip header */
    const uint8_t *p = data;
    while (*p != '\0' && p < data + len) p++;
    if (p >= data + len) return -1;
    p++; /* skip null */

    const char *s = (const char *)p;
    const char *end = (const char *)(data + len);

    if (hostname) hostname[0] = '\0';
    if (parent_hash) memset(parent_hash, 0, HASH_RAW_SIZE);
    if (timestamp) *timestamp = 0;

    while (s < end) {
        /* Stop at blank line (end of headers) */
        if (*s == '\n' || *s == '\0') break;

        const char *line_end = strchr(s, '\n');
        if (!line_end) line_end = end;

        if (strncmp(s, "tree ", 5) == 0 && tree_hash) {
            char hex[HASH_HEX_SIZE + 1];
            memcpy(hex, s + 5, HASH_HEX_SIZE);
            hex[HASH_HEX_SIZE] = '\0';
            hex_to_hash(hex, tree_hash);
        } else if (strncmp(s, "parent ", 7) == 0 && parent_hash) {
            char hex[HASH_HEX_SIZE + 1];
            memcpy(hex, s + 7, HASH_HEX_SIZE);
            hex[HASH_HEX_SIZE] = '\0';
            hex_to_hash(hex, parent_hash);
        } else if (strncmp(s, "author ", 7) == 0 && author && email) {
            const char *a = s + 7;
            const char *lt = strchr(a, '<');
            const char *gt = strchr(a, '>');
            if (lt && gt) {
                size_t name_len = lt - a - 1;
                memcpy(author, a, name_len);
                author[name_len] = '\0';

                size_t email_len = gt - lt - 1;
                memcpy(email, lt + 1, email_len);
                email[email_len] = '\0';

                if (timestamp) *timestamp = (time_t)atoll(gt + 2);
            }
        } else if (strncmp(s, "hostname ", 9) == 0 && hostname) {
            size_t hlen = line_end - (s + 9);
            memcpy(hostname, s + 9, hlen);
            hostname[hlen] = '\0';
        }

        s = line_end + 1;
    }

    /* Skip blank line separating headers from message */
    if (s < end && *s == '\n') s++;

    if (message && s < end) {
        size_t mlen = end - s;
        if (mlen > 0 && s[mlen - 1] == '\n') mlen--;
        if (mlen >= MAX_MSG_LEN) mlen = MAX_MSG_LEN - 1;
        memcpy(message, s, mlen);
        message[mlen] = '\0';
    }

    return 0;
}
