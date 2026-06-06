#include "msync.h"

#ifdef _WIN32
static int winsock_initialized = 0;
static void init_winsock(void) {
    if (!winsock_initialized) {
        WSADATA wsa;
        WSAStartup(MAKEWORD(2, 2), &wsa);
        winsock_initialized = 1;
    }
}
#endif

int net_connect(const char *host, int port) {
#ifdef _WIN32
    init_winsock();
#endif
    SOCKET fd = socket(AF_INET, SOCK_STREAM, 0);
    if (fd == INVALID_SOCKET) return -1;

    struct sockaddr_in addr;
    memset(&addr, 0, sizeof(addr));
    addr.sin_family = AF_INET;
    addr.sin_port = htons((uint16_t)port);

    struct hostent *he = gethostbyname(host);
    if (he) {
        memcpy(&addr.sin_addr, he->h_addr_list[0], (size_t)he->h_length);
    } else {
        addr.sin_addr.s_addr = inet_addr(host);
        if (addr.sin_addr.s_addr == INADDR_NONE) {
            closesocket(fd);
            return -1;
        }
    }

    if (connect(fd, (struct sockaddr *)&addr, sizeof(addr)) < 0) {
        closesocket(fd);
        return -1;
    }
    return (int)fd;
}

int net_listen(int port) {
#ifdef _WIN32
    init_winsock();
#endif
    SOCKET fd = socket(AF_INET, SOCK_STREAM, 0);
    if (fd == INVALID_SOCKET) return -1;

    int opt = 1;
    setsockopt(fd, SOL_SOCKET, SO_REUSEADDR,
#ifdef _WIN32
               (const char *)&opt, sizeof(opt));
#else
               &opt, sizeof(opt));
#endif

    struct sockaddr_in addr;
    memset(&addr, 0, sizeof(addr));
    addr.sin_family = AF_INET;
    addr.sin_addr.s_addr = INADDR_ANY;
    addr.sin_port = htons((uint16_t)port);

    if (bind(fd, (struct sockaddr *)&addr, sizeof(addr)) < 0) {
        closesocket(fd);
        return -1;
    }

    if (listen(fd, 5) < 0) {
        closesocket(fd);
        return -1;
    }
    return (int)fd;
}

int net_accept(int server_fd) {
    struct sockaddr_in client;
    socklen_t len = sizeof(client);
    return (int)accept(server_fd, (struct sockaddr *)&client, &len);
}

int net_send(int fd, const uint8_t *data, size_t len) {
    size_t sent = 0;
    while (sent < len) {
        int n = (int)send(fd,
#ifdef _WIN32
                          (const char *)(data + sent),
#else
                          data + sent,
#endif
                          len - sent, 0);
        if (n <= 0) return -1;
        sent += (size_t)n;
    }
    return 0;
}

int net_recv(int fd, uint8_t *data, size_t len) {
    size_t recvd = 0;
    while (recvd < len) {
        int n = (int)recv(fd,
#ifdef _WIN32
                          (char *)(data + recvd),
#else
                          data + recvd,
#endif
                          len - recvd, 0);
        if (n <= 0) return -1;
        recvd += (size_t)n;
    }
    return 0;
}

int net_sendline(int fd, const char *line) {
    size_t len = strlen(line);
    if (net_send(fd, (const uint8_t *)line, len) != 0) return -1;
    return net_send(fd, (const uint8_t *)"\n", 1);
}

int net_recvline(int fd, char *buf, size_t size) {
    size_t i = 0;
    while (i < size - 1) {
        uint8_t c;
        if (recv(fd,
#ifdef _WIN32
                 (char *)&c,
#else
                 &c,
#endif
                 1, 0) != 1) return -1;

        if (c == '\n') break;
        if (c != '\r') buf[i++] = c;
    }
    buf[i] = '\0';
    return 0;
}

void net_close(int fd) {
    if (fd >= 0) closesocket(fd);
}

/* --- Protocol functions --- */

int proto_list_remote_refs(int fd, char ***refs, uint8_t **hashes, int *count) {
    net_sendline(fd, "LIST");

    char buf[MAX_PATH_LEN];
    if (net_recvline(fd, buf, sizeof(buf)) != 0) return -1;

    *count = atoi(buf);
    *refs = (char **)calloc(*count, sizeof(char *));
    *hashes = (uint8_t *)calloc(*count, HASH_RAW_SIZE);

    for (int i = 0; i < *count; i++) {
        if (net_recvline(fd, buf, sizeof(buf)) != 0) {
            for (int j = 0; j < i; j++) free((*refs)[j]);
            free(*refs);
            free(*hashes);
            *refs = NULL;
            *hashes = NULL;
            *count = 0;
            return -1;
        }
        uint8_t *null_pos = (uint8_t *)strchr(buf, '\0');
        if (null_pos && null_pos < (uint8_t *)buf + sizeof(buf)) {
            /* line contains hash + space + refname */
        }

        char hash_hex[HASH_HEX_SIZE + 1];
        memcpy(hash_hex, buf, HASH_HEX_SIZE);
        hash_hex[HASH_HEX_SIZE] = '\0';
        hex_to_hash(hash_hex, (*hashes) + i * HASH_RAW_SIZE);

        (*refs)[i] = msync_strdup(buf + HASH_HEX_SIZE + 1);
    }
    net_sendline(fd, "OK");
    return 0;
}

int proto_fetch(int fd, const uint8_t *want_hashes[], int want_count,
                const uint8_t *have_hashes[], int have_count) {
    char buf[MAX_PATH_LEN];

    net_sendline(fd, "FETCH");

    /* Send want hashes */
    for (int i = 0; i < want_count; i++) {
        char hex[HASH_HEX_SIZE + 1];
        hash_to_hex(want_hashes[i], hex);
        snprintf(buf, sizeof(buf), "WANT %s", hex);
        net_sendline(fd, buf);
    }

    /* Send have hashes */
    for (int i = 0; i < have_count; i++) {
        char hex[HASH_HEX_SIZE + 1];
        hash_to_hex(have_hashes[i], hex);
        snprintf(buf, sizeof(buf), "HAVE %s", hex);
        net_sendline(fd, buf);
    }
    net_sendline(fd, "DONE");

    /* Receive pack */
    if (net_recvline(fd, buf, sizeof(buf)) != 0) return -1;
    if (strncmp(buf, "PACK", 4) != 0) return -1;

    int obj_count = atoi(buf + 5);
    for (int i = 0; i < obj_count; i++) {
        uint8_t hash[HASH_RAW_SIZE];
        if (net_recv(fd, hash, HASH_RAW_SIZE) != 0) return -1;

        if (object_exists(hash)) {
            /* Skip known object */
            uint8_t type;
            uint32_t osize;
            if (net_recv(fd, &type, 1) != 0) return -1;
            if (net_recv(fd, (uint8_t *)&osize, 4) != 0) return -1;
            osize = ntohl(osize);
            /* Skip data */
            uint8_t dummy[4096];
            size_t remaining = osize;
            while (remaining > 0) {
                size_t chunk = remaining > sizeof(dummy) ? sizeof(dummy) : remaining;
                if (net_recv(fd, dummy, chunk) != 0) return -1;
                remaining -= chunk;
            }
            continue;
        }

        uint8_t type;
        uint32_t osize;
        if (net_recv(fd, &type, 1) != 0) return -1;
        if (net_recv(fd, (uint8_t *)&osize, 4) != 0) return -1;
        osize = ntohl(osize);

        uint8_t *objdata = (uint8_t *)malloc(osize);
        if (!objdata) return -1;
        if (net_recv(fd, objdata, osize) != 0) { free(objdata); return -1; }

        uint8_t computed[HASH_RAW_SIZE];
        sha256_hash(objdata, osize, computed);
        if (hash_cmp(computed, hash) != 0) {
            free(objdata);
            return -1;
        }
        object_write(objdata, osize, computed);
        free(objdata);
    }
    net_sendline(fd, "OK");
    return 0;
}

int proto_push(int fd, const char *refname, const uint8_t *old_hash,
               const uint8_t *new_hash) {
    char buf[MAX_PATH_LEN];

    net_sendline(fd, "PUSH");

    /* Send ref update */
    char old_hex[HASH_HEX_SIZE + 1], new_hex[HASH_HEX_SIZE + 1];
    char old_zero[HASH_HEX_SIZE + 1];
    memset(old_zero, '0', HASH_HEX_SIZE);
    old_zero[HASH_HEX_SIZE] = '\0';

    if (old_hash) {
        hash_to_hex(old_hash, old_hex);
    } else {
        memcpy(old_hex, old_zero, HASH_HEX_SIZE + 1);
    }
    hash_to_hex(new_hash, new_hex);

    snprintf(buf, sizeof(buf), "UPDATE %s %s %s", refname, old_hex, new_hex);
    net_sendline(fd, buf);

    if (net_recvline(fd, buf, sizeof(buf)) != 0) return -1;
    if (strncmp(buf, "ERR Non-fast-forward", 20) == 0) {
        return -2; /* Conflict: server has newer commits */
    }
    if (strncmp(buf, "ERR", 3) == 0) {
        return -1; /* Other server error */
    }
    if (strncmp(buf, "WANT_PACK", 9) == 0) {
        /* Server wants the objects */
        proto_send_objects(fd, old_hash, new_hash);
        if (net_recvline(fd, buf, sizeof(buf)) != 0) return -1;
        if (strncmp(buf, "OK", 2) == 0) return 0;
        return -1;
    }
    if (strncmp(buf, "OK", 2) == 0) return 0;
    return -1;
}

void collect_all_objects(const uint8_t *hash, const uint8_t *stop_hash,
                         uint8_t **all_hashes, int *hash_count) {
    if (!hash || !object_exists(hash)) return;

    /* Check if we've reached the stop point */
    if (stop_hash && hash_cmp(hash, stop_hash) == 0) return;

    /* Check if already collected */
    for (int i = 0; i < *hash_count; i++) {
        if (hash_cmp(*all_hashes + i * HASH_RAW_SIZE, hash) == 0) return;
    }

    /* Add this object */
    (*hash_count)++;
    *all_hashes = (uint8_t *)realloc(*all_hashes, (*hash_count) * HASH_RAW_SIZE);
    memcpy(*all_hashes + (*hash_count - 1) * HASH_RAW_SIZE, hash, HASH_RAW_SIZE);

    /* Read object to determine type */
    uint8_t *data;
    size_t len;
    if (object_read(hash, &data, &len) != 0) return;

    if (strncmp((char *)data, "commit ", 7) == 0) {
        uint8_t tree_hash[HASH_RAW_SIZE];
        uint8_t parent_hash[HASH_RAW_SIZE];
        memset(parent_hash, 0, HASH_RAW_SIZE);
        commit_parse(data, len, tree_hash, parent_hash, NULL, NULL, NULL, NULL, NULL);

        /* Collect tree */
        collect_all_objects(tree_hash, stop_hash, all_hashes, hash_count);

        /* Collect tree entries */
        uint8_t *tdata;
        size_t tlen;
        if (object_read(tree_hash, &tdata, &tlen) == 0) {
            char **names = NULL;
            uint8_t *entry_hashes = NULL;
            int ecount = 0;
            tree_entries_parse(tdata, tlen, &names, &entry_hashes, &ecount);
            for (int e = 0; e < ecount; e++) {
                collect_all_objects(entry_hashes + e * HASH_RAW_SIZE, stop_hash,
                                    all_hashes, hash_count);
                free(names[e]);
            }
            free(names);
            free(entry_hashes);
            free(tdata);
        }

        /* Collect parent */
        if (parent_hash[0] != 0) {
            collect_all_objects(parent_hash, stop_hash, all_hashes, hash_count);
        }
    }
    free(data);
}

int proto_send_objects(int fd, const uint8_t *old_hash, const uint8_t *new_hash) {
    uint8_t *all_hashes = NULL;
    int hash_count = 0;

    collect_all_objects(new_hash, old_hash, &all_hashes, &hash_count);

    /* Send pack header */
    char buf[64];
    snprintf(buf, sizeof(buf), "PACK %d", hash_count);
    net_sendline(fd, buf);

    /* Send each object */
    for (int i = 0; i < hash_count; i++) {
        uint8_t *objdata;
        size_t objlen;
        if (object_read(all_hashes + i * HASH_RAW_SIZE, &objdata, &objlen) != 0)
            continue;

        net_send(fd, all_hashes + i * HASH_RAW_SIZE, HASH_RAW_SIZE);

        /* Type */
        uint8_t type = OBJ_BLOB;
        if (strncmp((char *)objdata, "commit ", 7) == 0) type = OBJ_COMMIT;
        else if (strncmp((char *)objdata, "tree ", 5) == 0) type = OBJ_TREE;
        net_send(fd, &type, 1);

        /* Size */
        uint32_t osize = htonl((uint32_t)objlen);
        net_send(fd, (uint8_t *)&osize, 4);

        /* Data */
        net_send(fd, objdata, objlen);
        free(objdata);
    }
    return 0;
}
