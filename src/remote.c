#include "msync.h"

int remote_add(const char *name, const char *url) {
    char key[256];
    snprintf(key, sizeof(key), "remote.%s.url", name);
    if (config_set(key, url) != 0) return -1;
    printf("Added remote '%s' -> %s\n", name, url);
    return 0;
}

int remote_remove(const char *name) {
    char key[256];
    snprintf(key, sizeof(key), "remote.%s.url", name);
    if (config_unset(key) != 0) {
        fprintf(stderr, "Remote '%s' not found.\n", name);
        return -1;
    }
    printf("Removed remote '%s'.\n", name);
    return 0;
}

int remote_list(void) {
    char **keys;
    int count;
    config_list_keys("remote.", &keys, &count);

    if (count == 0) {
        printf("No remotes configured.\n");
        return 0;
    }

    printf("%-20s %s\n", "Name", "URL");
    printf("%-20s %s\n", "----", "---");

    for (int i = 0; i < count; i++) {
        /* Extract name from "remote.<name>.url" */
        const char *p = keys[i] + 7; /* skip "remote." */
        const char *dot = strrchr(p, '.');
        char name[128];
        size_t nlen = dot ? (size_t)(dot - p) : strlen(p);
        if (nlen >= sizeof(name)) nlen = sizeof(name) - 1;
        memcpy(name, p, nlen);
        name[nlen] = '\0';

        char url[256];
        if (config_get(keys[i], url, sizeof(url)) == 0) {
            printf("%-20s %s\n", name, url);
        }
        free(keys[i]);
    }
    free(keys);
    return 0;
}

/* Resolve a name-or-url to host, port, and path components.
 * First tries to look up <name_or_url> as a remote name in config.
 * If found, uses the configured URL; otherwise parses as raw host:port[/path]. */
int remote_resolve(const char *name_or_url, char *host, int *port,
                   char *path, size_t path_size) {
    /* Try as remote name */
    char key[256];
    snprintf(key, sizeof(key), "remote.%s.url", name_or_url);

    char resolved_url[256];
    const char *url = name_or_url;

    if (config_get(key, resolved_url, sizeof(resolved_url)) == 0) {
        url = resolved_url;
    }

    /* Parse URL */
    if (strncmp(url, "msync://", 8) == 0) url += 8;

    const char *colon = strchr(url, ':');
    const char *slash = strchr(url, '/');

    if (colon && (!slash || colon < slash)) {
        size_t hlen = colon - url;
        if (hlen >= 256) hlen = 255;
        memcpy(host, url, hlen);
        host[hlen] = '\0';
        *port = atoi(colon + 1);
        if (slash && path) {
            snprintf(path, path_size, "%s", slash + 1);
        } else if (path) {
            path[0] = '\0';
        }
    } else if (slash) {
        size_t hlen = slash - url;
        if (hlen >= 256) hlen = 255;
        memcpy(host, url, hlen);
        host[hlen] = '\0';
        if (!host[0]) snprintf(host, 256, "localhost");
        *port = MSYNC_DEFAULT_PORT;
        if (path) snprintf(path, path_size, "%s", slash + 1);
    } else {
        snprintf(host, 256, "%s", url);
        *port = MSYNC_DEFAULT_PORT;
        if (path) path[0] = '\0';
    }

    return 0;
}

/* Get the hash from remote tracking ref: refs/remotes/<name>/<branch> */
int remote_get_tracking_hash(const char *remote_name, const char *branch,
                             uint8_t *hash) {
    char ref[MAX_PATH_LEN];
    snprintf(ref, sizeof(ref), "refs/remotes/%s/%s", remote_name, branch);
    return ref_resolve(ref, hash);
}

/* --- Merge helpers --- */

/* Check if ancestor_hash is really an ancestor of descendant_hash by
 * walking parent chain. */
static int is_ancestor(const uint8_t *descendant_hash,
                       const uint8_t *ancestor_hash) {
    uint8_t cur[HASH_RAW_SIZE];
    hash_cpy(cur, descendant_hash);

    int depth = 0;
    while (depth < 1000 && object_exists(cur)) {
        if (hash_cmp(cur, ancestor_hash) == 0) return 1;

        uint8_t *data;
        size_t len;
        if (object_read(cur, &data, &len) != 0) return 0;

        uint8_t parent[HASH_RAW_SIZE];
        memset(parent, 0, HASH_RAW_SIZE);
        commit_parse(data, len, NULL, parent, NULL, NULL, NULL, NULL, NULL);
        free(data);

        if (parent[0] == 0) return 0; /* root - no more parents */
        hash_cpy(cur, parent);
        depth++;
    }
    return 0;
}

/* Returns 1 if remote_hash is a direct descendant of local_hash
 * (i.e., local can fast-forward to remote without merge). */
int merge_is_fast_forward(const uint8_t *local_hash, const uint8_t *remote_hash) {
    if (hash_cmp(local_hash, remote_hash) == 0) return 1;
    return is_ancestor(remote_hash, local_hash);
}

/* Simple merge: if remote is fast-forward from local, result = remote.
 * If local is fast-forward from remote, result = local (nothing to do).
 * Otherwise, finds common ancestor and creates a merge commit.
 * Returns 0 on success, -1 on error, 1 if merge conflict needs resolution. */
int merge_commits(const uint8_t *local_hash, const uint8_t *remote_hash,
                  uint8_t *result_hash) {
    if (hash_cmp(local_hash, remote_hash) == 0) {
        hash_cpy(result_hash, local_hash);
        return 0;
    }

    /* Fast-forward: remote is ahead of local */
    if (is_ancestor(remote_hash, local_hash)) {
        hash_cpy(result_hash, remote_hash);
        return 0;
    }

    /* Local is ahead of remote - nothing to change */
    if (is_ancestor(local_hash, remote_hash)) {
        hash_cpy(result_hash, local_hash);
        return 0;
    }

    /* Divergent: find common ancestor by walking from local */
    uint8_t common[HASH_RAW_SIZE];
    uint8_t cur[HASH_RAW_SIZE];
    hash_cpy(cur, local_hash);
    int found_common = 0;
    int depth = 0;

    while (depth < 1000 && object_exists(cur)) {
        if (is_ancestor(remote_hash, cur)) {
            hash_cpy(common, cur);
            found_common = 1;
            break;
        }

        uint8_t *data;
        size_t len;
        if (object_read(cur, &data, &len) != 0) break;

        uint8_t parent[HASH_RAW_SIZE];
        memset(parent, 0, HASH_RAW_SIZE);
        commit_parse(data, len, NULL, parent, NULL, NULL, NULL, NULL, NULL);
        free(data);

        if (parent[0] == 0) break;
        hash_cpy(cur, parent);
        depth++;
    }

    if (!found_common) {
        /* No common ancestor, can't auto-merge */
        return 1;
    }

    /* For a true merge, we would need to diff and combine trees.
     * For now, create a merge commit with both parents pointing to
     * the remote tree (user should resolve manually if needed). */
    uint8_t *rdata;
    size_t rlen;
    if (object_read(remote_hash, &rdata, &rlen) != 0) return -1;

    uint8_t remote_tree[HASH_RAW_SIZE];
    char author[256] = "";
    char email[256] = "";
    commit_parse(rdata, rlen, remote_tree, NULL, author, email, NULL, NULL, NULL);
    free(rdata);

    /* Create merge commit with remote's tree and local as second parent */
    config_get("user.name", author, sizeof(author));
    config_get("user.email", email, sizeof(email));
    if (!author[0]) snprintf(author, sizeof(author), "unknown");
    if (!email[0]) snprintf(email, sizeof(email), "unknown@unknown");

    /* Build merge commit content manually with two parents */
    char hostname[256] = "unknown";
    gethostname(hostname, sizeof(hostname));

    time_t now = time(NULL);

    char *content = (char *)malloc(MAX_COMMIT_SIZE);
    if (!content) return -1;
    int pos = 0;

    char hex[HASH_HEX_SIZE + 1];

    hash_to_hex(remote_tree, hex);
    pos += snprintf(content + pos, MAX_COMMIT_SIZE - pos, "tree %s\n", hex);

    hash_to_hex(local_hash, hex);
    pos += snprintf(content + pos, MAX_COMMIT_SIZE - pos, "parent %s\n", hex);

    hash_to_hex(remote_hash, hex);
    pos += snprintf(content + pos, MAX_COMMIT_SIZE - pos, "parent %s\n", hex);

    pos += snprintf(content + pos, MAX_COMMIT_SIZE - pos,
                    "author %s <%s> %ld\n", author, email, (long)now);
    pos += snprintf(content + pos, MAX_COMMIT_SIZE - pos,
                    "hostname %s\n", hostname);
    pos += snprintf(content + pos, MAX_COMMIT_SIZE - pos,
                    "\nMerge branch from remote\n");

    size_t body_len = strlen(content);
    char header[64];
    int hlen = snprintf(header, sizeof(header), "commit %zu", body_len);

    size_t total = (size_t)hlen + 1 + body_len;
    uint8_t *buf = (uint8_t *)malloc(total);
    if (!buf) { free(content); return -1; }

    memcpy(buf, header, hlen);
    buf[hlen] = '\0';
    memcpy(buf + hlen + 1, content, body_len);

    int ret = object_write(buf, total, result_hash);
    free(buf);
    free(content);

    return (ret == 0) ? 0 : -1;
}
