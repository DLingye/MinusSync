#include "msync.h"

int config_get(const char *key, char *value, size_t size) {
    if (!file_exists(MSYNC_CONFIG_FILE)) return -1;

    uint8_t *data;
    size_t len;
    if (file_read(MSYNC_CONFIG_FILE, &data, &len) != 0) return -1;

    char *content = (char *)data;
    char *line = strtok(content, "\n");
    int found = 0;

    char section[64] = "";
    while (line) {
        char *s = line;
        while (*s == ' ' || *s == '\t') s++;

        if (*s == '[') {
            char *end = strchr(s, ']');
            if (end) {
                size_t slen = end - s - 1;
                if (slen < sizeof(section)) {
                    memcpy(section, s + 1, slen);
                    section[slen] = '\0';
                }
            }
        } else if (*s && *s != '#' && *s != ';') {
            char *eq = strchr(s, '=');
            if (eq) {
                char *k = s;
                while (*k == ' ' || *k == '\t') k++;
                size_t klen = eq - k;
                while (klen > 0 && (k[klen-1] == ' ' || k[klen-1] == '\t')) klen--;

                char full_key[256];
                if (section[0]) {
                    snprintf(full_key, sizeof(full_key), "%.63s.%.*s", section, (int)klen, k);
                } else {
                    snprintf(full_key, sizeof(full_key), "%.*s", (int)klen, k);
                }

                if (strcmp(full_key, key) == 0) {
                    char *v = eq + 1;
                    while (*v == ' ' || *v == '\t') v++;
                    size_t vlen = strlen(v);
                    while (vlen > 0 && (v[vlen-1] == ' ' || v[vlen-1] == '\t' ||
                                        v[vlen-1] == '\r')) vlen--;
                    size_t cpy = vlen < size - 1 ? vlen : size - 1;
                    memcpy(value, v, cpy);
                    value[cpy] = '\0';
                    found = 1;
                    break;
                }
            }
        }
        line = strtok(NULL, "\n");
    }

    free(data);
    return found ? 0 : -1;
}

int config_set(const char *key, const char *value) {
    char *section = NULL;
    char *subkey = NULL;
    char buf[256];

    const char *dot = strchr(key, '.');
    if (dot) {
        size_t slen = dot - key;
        memcpy(buf, key, slen);
        buf[slen] = '\0';
        section = buf;
        subkey = (char *)(dot + 1);
    } else {
        subkey = (char *)key;
    }

    /* Read existing config */
    uint8_t *data = NULL;
    size_t len = 0;
    if (file_exists(MSYNC_CONFIG_FILE)) {
        file_read(MSYNC_CONFIG_FILE, &data, &len);
    }

    char new_content[65536];
    int pos = 0;

    if (data) {
        char *content = (char *)data;
        char *line = strtok(content, "\n");
        char cur_section[64] = "";
        int in_section = 0;
        int key_found = 0;

        while (line) {
            char *s = line;
            while (*s == ' ' || *s == '\t') s++;

            if (*s == '[') {
                if (in_section && !key_found && section) {
                    pos += snprintf(new_content + pos, sizeof(new_content) - pos,
                                    "\t%s = %s\n", subkey, value);
                    key_found = 1;
                }
                in_section = 0;
                char *end = strchr(s, ']');
                if (end && section) {
                    size_t slen = end - s - 1;
                    if (strncmp(s + 1, section, slen) == 0) in_section = 1;
                }
                pos += snprintf(new_content + pos, sizeof(new_content) - pos, "%s\n", line);
            } else if (*s && *s != '#' && *s != ';') {
                char *eq = strchr(s, '=');
                if (eq) {
                    char *kstr = s;
                    while (*kstr == ' ' || *kstr == '\t') kstr++;
                    size_t klen = eq - kstr;
                    while (klen > 0 && (kstr[klen-1] == ' ' || kstr[klen-1] == '\t')) klen--;

                    char full_key[256];
                    if (cur_section[0]) {
                        snprintf(full_key, sizeof(full_key), "%.63s.%.*s", cur_section, (int)klen, kstr);
                    } else {
                        snprintf(full_key, sizeof(full_key), "%.*s", (int)klen, kstr);
                    }

                    if (strcmp(full_key, key) == 0) {
                        pos += snprintf(new_content + pos, sizeof(new_content) - pos,
                                        "\t%s = %s\n", subkey, value);
                        key_found = 1;
                    } else {
                        pos += snprintf(new_content + pos, sizeof(new_content) - pos, "%s\n", line);
                    }
                }
            } else {
                pos += snprintf(new_content + pos, sizeof(new_content) - pos, "%s\n", line);
            }
            line = strtok(NULL, "\n");
        }

        if (!key_found) {
            if (section) {
                pos += snprintf(new_content + pos, sizeof(new_content) - pos,
                                "[%s]\n\t%s = %s\n", section, subkey, value);
            } else {
                pos += snprintf(new_content + pos, sizeof(new_content) - pos,
                                "%s = %s\n", key, value);
            }
        }

        free(data);
    } else {
        if (section) {
            pos += snprintf(new_content, sizeof(new_content),
                            "[%s]\n\t%s = %s\n", section, subkey, value);
        } else {
            pos += snprintf(new_content, sizeof(new_content),
                            "%s = %s\n", key, value);
        }
    }

    return file_write(MSYNC_CONFIG_FILE, (uint8_t *)new_content, strlen(new_content));
}

int config_has(const char *key) {
    char dummy[16];
    return config_get(key, dummy, sizeof(dummy)) == 0;
}

int config_load(void) {
    if (!file_exists(MSYNC_CONFIG_FILE)) return 0;
    return 0; /* config is read on demand */
}

int config_unset(const char *key) {
    if (!file_exists(MSYNC_CONFIG_FILE)) return -1;

    uint8_t *data;
    size_t len;
    if (file_read(MSYNC_CONFIG_FILE, &data, &len) != 0) return -1;

    char *content = (char *)data;
    char new_content[65536];
    int pos = 0;

    const char *dot = strchr(key, '.');
    char section[64] = "";
    if (dot) {
        size_t slen = dot - key;
        if (slen < sizeof(section)) {
            memcpy(section, key, slen);
            section[slen] = '\0';
        }
    }

    char *line = strtok(content, "\n");
    char cur_section[64] = "";
    int removed = 0;

    while (line) {
        char *s = line;
        while (*s == ' ' || *s == '\t') s++;

        if (*s == '[') {
            char *end = strchr(s, ']');
            if (end) {
                size_t slen = end - s - 1;
                if (slen < sizeof(cur_section)) {
                    memcpy(cur_section, s + 1, slen);
                    cur_section[slen] = '\0';
                }
            }
            pos += snprintf(new_content + pos, sizeof(new_content) - pos, "%s\n", line);
        } else if (*s && *s != '#' && *s != ';') {
            char *eq = strchr(s, '=');
            if (eq) {
                char *kstr = s;
                while (*kstr == ' ' || *kstr == '\t') kstr++;
                size_t klen = eq - kstr;
                while (klen > 0 && (kstr[klen-1] == ' ' || kstr[klen-1] == '\t')) klen--;

                char full_key[256];
                if (cur_section[0]) {
                    snprintf(full_key, sizeof(full_key), "%.63s.%.*s", cur_section, (int)klen, kstr);
                } else {
                    snprintf(full_key, sizeof(full_key), "%.*s", (int)klen, kstr);
                }

                if (strcmp(full_key, key) == 0) {
                    /* Skip this line */
                    removed = 1;
                } else {
                    pos += snprintf(new_content + pos, sizeof(new_content) - pos, "%s\n", line);
                }
            } else {
                pos += snprintf(new_content + pos, sizeof(new_content) - pos, "%s\n", line);
            }
        } else {
            pos += snprintf(new_content + pos, sizeof(new_content) - pos, "%s\n", line);
        }
        line = strtok(NULL, "\n");
    }

    free(data);
    if (removed) {
        file_write(MSYNC_CONFIG_FILE, (uint8_t *)new_content, strlen(new_content));
    }
    return removed ? 0 : -1;
}

int config_list_keys(const char *prefix, char ***keys, int *count) {
    *keys = NULL;
    *count = 0;

    if (!file_exists(MSYNC_CONFIG_FILE)) return 0;

    uint8_t *data;
    size_t len;
    if (file_read(MSYNC_CONFIG_FILE, &data, &len) != 0) return 0;

    char *content = (char *)data;
    char *line = strtok(content, "\n");
    char section[64] = "";

    while (line) {
        char *s = line;
        while (*s == ' ' || *s == '\t') s++;

        if (*s == '[') {
            char *end = strchr(s, ']');
            if (end) {
                size_t slen = end - s - 1;
                if (slen < sizeof(section)) {
                    memcpy(section, s + 1, slen);
                    section[slen] = '\0';
                }
            }
        } else if (*s && *s != '#' && *s != ';') {
            char *eq = strchr(s, '=');
            if (eq) {
                char *kstr = s;
                while (*kstr == ' ' || *kstr == '\t') kstr++;
                size_t klen = eq - kstr;
                while (klen > 0 && (kstr[klen-1] == ' ' || kstr[klen-1] == '\t')) klen--;

                char full_key[256];
                if (section[0]) {
                    snprintf(full_key, sizeof(full_key), "%.63s.%.*s", section, (int)klen, kstr);
                } else {
                    snprintf(full_key, sizeof(full_key), "%.*s", (int)klen, kstr);
                }

                if (strncmp(full_key, prefix, strlen(prefix)) == 0) {
                    (*count)++;
                    *keys = (char **)realloc(*keys, (*count) * sizeof(char *));
                    (*keys)[*count - 1] = msync_strdup(full_key);
                }
            }
        }
        line = strtok(NULL, "\n");
    }

    free(data);
    return 0;
}
