#include "msync.h"
#include <signal.h>

static volatile int keep_running = 1;

#ifndef _WIN32
static void sig_handler(int sig) {
    (void)sig;
    keep_running = 0;
}
#endif

static int handle_list(int client_fd) {
    char **names;
    uint8_t *hashes;
    int count;

    if (ref_list(&names, &hashes, &count) != 0) {
        net_sendline(client_fd, "0");
        return 0;
    }

    char buf[MAX_PATH_LEN];
    snprintf(buf, sizeof(buf), "%d", count);
    net_sendline(client_fd, buf);

    for (int i = 0; i < count; i++) {
        char hex[HASH_HEX_SIZE + 1];
        hash_to_hex(hashes + i * HASH_RAW_SIZE, hex);
        char full_ref[MAX_PATH_LEN];
        snprintf(full_ref, sizeof(full_ref), "refs/%s", names[i]);
        snprintf(buf, sizeof(buf), "%s %s", hex, full_ref);
        net_sendline(client_fd, buf);
        free(names[i]);
    }
    free(names);
    free(hashes);

    net_recvline(client_fd, buf, sizeof(buf)); /* Wait for OK */
    return 0;
}

static int handle_fetch(int client_fd) {
    char buf[MAX_MSG_LEN];
    uint8_t want_hashes[128][HASH_RAW_SIZE];
    uint8_t have_hashes[1024][HASH_RAW_SIZE];
    int want_count = 0, have_count = 0;

    while (1) {
        if (net_recvline(client_fd, buf, sizeof(buf)) != 0) return -1;

        if (strncmp(buf, "DONE", 4) == 0) break;

        if (strncmp(buf, "WANT ", 5) == 0) {
            if (want_count < 128) {
                hex_to_hash(buf + 5, want_hashes[want_count++]);
            }
        } else if (strncmp(buf, "HAVE ", 5) == 0) {
            if (have_count < 1024) {
                hex_to_hash(buf + 5, have_hashes[have_count++]);
            }
        }
    }

    /* Collect objects to send */
    /* Walk from each WANT back to HAVEs or root */
    uint8_t *send_hashes = NULL;
    int send_count = 0;

    for (int w = 0; w < want_count; w++) {
        uint8_t cur[HASH_RAW_SIZE];
        memcpy(cur, want_hashes[w], HASH_RAW_SIZE);

        int depth = 0;
        while (depth < 100 && object_exists(cur)) {
            /* Don't send if client has it */
            int client_has = 0;
            for (int h = 0; h < have_count; h++) {
                if (hash_cmp(cur, have_hashes[h]) == 0) {
                    client_has = 1;
                    break;
                }
            }

            /* Don't send if we already queued it */
            int already_sending = 0;
            for (int s = 0; s < send_count; s++) {
                if (hash_cmp(cur, send_hashes + s * HASH_RAW_SIZE) == 0) {
                    already_sending = 1;
                    break;
                }
            }

            if (!client_has && !already_sending) {
                send_count++;
                send_hashes = (uint8_t *)realloc(send_hashes, send_count * HASH_RAW_SIZE);
                memcpy(send_hashes + (send_count - 1) * HASH_RAW_SIZE, cur, HASH_RAW_SIZE);

                /* Also send the tree */
                uint8_t *data;
                size_t len;
                if (object_read(cur, &data, &len) == 0) {
                    uint8_t tree_hash[HASH_RAW_SIZE];
                    commit_parse(data, len, tree_hash, NULL, NULL, NULL, NULL, NULL, NULL);
                    free(data);

                    if (!client_has) {
                        /* Check if tree is already queued */
                        int tree_queued = 0;
                        for (int s = 0; s < send_count; s++) {
                            if (hash_cmp(tree_hash, send_hashes + s * HASH_RAW_SIZE) == 0) {
                                tree_queued = 1;
                                break;
                            }
                        }
                        if (!tree_queued && object_exists(tree_hash)) {
                            send_count++;
                            send_hashes = (uint8_t *)realloc(send_hashes, send_count * HASH_RAW_SIZE);
                            memcpy(send_hashes + (send_count - 1) * HASH_RAW_SIZE, tree_hash, HASH_RAW_SIZE);

                            /* Add tree entries (blobs and subtrees) */
                            uint8_t *tdata;
                            size_t tlen;
                            if (object_read(tree_hash, &tdata, &tlen) == 0) {
                                char **names = NULL;
                                uint8_t *entry_hashes = NULL;
                                int ecount = 0;
                                tree_entries_parse(tdata, tlen, &names, &entry_hashes, &ecount);
                                free(tdata);
                                for (int e = 0; e < ecount; e++) {
                                    uint8_t *eh = entry_hashes + e * HASH_RAW_SIZE;
                                    if (object_exists(eh)) {
                                        int queued = 0;
                                        for (int s = 0; s < send_count; s++) {
                                            if (hash_cmp(eh, send_hashes + s * HASH_RAW_SIZE) == 0) {
                                                queued = 1; break;
                                            }
                                        }
                                        if (!queued) {
                                            send_count++;
                                            send_hashes = (uint8_t *)realloc(send_hashes, send_count * HASH_RAW_SIZE);
                                            memcpy(send_hashes + (send_count - 1) * HASH_RAW_SIZE, eh, HASH_RAW_SIZE);
                                        }
                                    }
                                    free(names[e]);
                                }
                                free(names);
                                free(entry_hashes);
                            }
                        }
                    }
                }

                /* Move to parent */
                uint8_t *cdata;
                size_t clen;
                if (object_read(cur, &cdata, &clen) == 0) {
                    uint8_t parent[HASH_RAW_SIZE];
                    memset(parent, 0, HASH_RAW_SIZE);
                    commit_parse(cdata, clen, NULL, parent, NULL, NULL, NULL, NULL, NULL);
                    free(cdata);

                    if (parent[0] != 0 && object_exists(parent)) {
                        memcpy(cur, parent, HASH_RAW_SIZE);
                        depth++;
                        continue;
                    }
                }
            }
            break;
        }
    }

    /* Send pack */
    snprintf(buf, sizeof(buf), "PACK %d", send_count);
    net_sendline(client_fd, buf);

    for (int i = 0; i < send_count; i++) {
        uint8_t *objdata;
        size_t objlen;
        if (object_read(send_hashes + i * HASH_RAW_SIZE, &objdata, &objlen) != 0)
            continue;

        /* Hash */
        net_send(client_fd, send_hashes + i * HASH_RAW_SIZE, HASH_RAW_SIZE);

        /* Type */
        uint8_t type = OBJ_BLOB;
        if (strncmp((char *)objdata, "commit ", 7) == 0) type = OBJ_COMMIT;
        else if (strncmp((char *)objdata, "tree ", 5) == 0) type = OBJ_TREE;
        net_send(client_fd, &type, 1);

        /* Size */
        uint32_t osize = htonl((uint32_t)objlen);
        net_send(client_fd, (uint8_t *)&osize, 4);

        /* Data */
        net_send(client_fd, objdata, objlen);
        free(objdata);
    }
    free(send_hashes);

    /* Wait for client OK */
    if (net_recvline(client_fd, buf, sizeof(buf)) != 0) return -1;

    return 0;
}

static int handle_push(int client_fd) {
    char buf[MAX_MSG_LEN];

    if (net_recvline(client_fd, buf, sizeof(buf)) != 0) return -1;

    if (strncmp(buf, "UPDATE ", 7) != 0) {
        net_sendline(client_fd, "ERR Invalid command");
        return -1;
    }

    /* Parse UPDATE refname oldhash newhash */
    char refname[MAX_PATH_LEN];
    char old_hex[HASH_HEX_SIZE + 1];
    char new_hex[HASH_HEX_SIZE + 1];

    if (sscanf(buf, "UPDATE %s %s %s", refname, old_hex, new_hex) != 3) {
        net_sendline(client_fd, "ERR Invalid format");
        return -1;
    }

    uint8_t old_hash[HASH_RAW_SIZE];
    uint8_t new_hash[HASH_RAW_SIZE];
    uint8_t zero_hash[HASH_RAW_SIZE];
    memset(zero_hash, 0, HASH_RAW_SIZE);

    hex_to_hash(old_hex, old_hash);
    hex_to_hash(new_hex, new_hash);

    /* Validate old_hash matches current ref (non-fast-forward check) */
    uint8_t current_hash[HASH_RAW_SIZE];
    int ref_exists = (ref_resolve(refname, current_hash) == 0);

    if (ref_exists && hash_cmp(old_hash, zero_hash) != 0) {
        /* Client specified an old_hash - validate it matches */
        if (hash_cmp(old_hash, current_hash) != 0) {
            /* Conflict: remote has moved since client last synced */
            char cur_hex[HASH_HEX_SIZE + 1];
            hash_to_hex(current_hash, cur_hex);
            cur_hex[8] = '\0';
            printf("[push] Rejected non-fast-forward push for %s (expected %s, got %s)\n",
                   refname, cur_hex, "different");
            net_sendline(client_fd, "ERR Non-fast-forward");
            return -1;
        }
    }

    /* Request the pack */
    net_sendline(client_fd, "WANT_PACK");

    /* Receive pack */
    if (net_recvline(client_fd, buf, sizeof(buf)) != 0) return -1;
    if (strncmp(buf, "PACK", 4) != 0) {
        net_sendline(client_fd, "ERR Expected pack");
        return -1;
    }

    int obj_count = atoi(buf + 5);
    for (int i = 0; i < obj_count; i++) {
        uint8_t hash[HASH_RAW_SIZE];
        if (net_recv(client_fd, hash, HASH_RAW_SIZE) != 0) {
            net_sendline(client_fd, "ERR Receive error");
            return -1;
        }

        uint8_t type;
        if (net_recv(client_fd, &type, 1) != 0) return -1;

        uint32_t osize;
        if (net_recv(client_fd, (uint8_t *)&osize, 4) != 0) return -1;
        osize = ntohl(osize);

        uint8_t *objdata = (uint8_t *)malloc(osize);
        if (!objdata) return -1;
        if (net_recv(client_fd, objdata, osize) != 0) { free(objdata); return -1; }

        uint8_t computed[HASH_RAW_SIZE];
        sha256_hash(objdata, osize, computed);

        if (hash_cmp(computed, hash) != 0) {
            free(objdata);
            net_sendline(client_fd, "ERR Hash mismatch");
            return -1;
        }
        object_write(objdata, osize, hash);
        free(objdata);
    }

    /* Update ref */
    ref_update(refname, new_hash);

    net_sendline(client_fd, "OK");
    printf("[push] Updated %s\n", refname);
    return 0;
}

static void handle_client(int client_fd) {
    char buf[MAX_MSG_LEN];

    while (net_recvline(client_fd, buf, sizeof(buf)) == 0) {
        printf("[server] Command: %s\n", buf);

        if (strcmp(buf, "LIST") == 0) {
            handle_list(client_fd);
        } else if (strcmp(buf, "FETCH") == 0) {
            handle_fetch(client_fd);
        } else if (strcmp(buf, "PUSH") == 0) {
            handle_push(client_fd);
        } else if (strcmp(buf, "QUIT") == 0) {
            break;
        } else {
            net_sendline(client_fd, "ERR Unknown command");
            break;
        }
    }
}

int repo_serve(int port) {
    if (!dir_exists(MSYNC_DIR)) {
        fprintf(stderr, "Not an msync repository. Run 'msync init' first.\n");
        return -1;
    }

#ifndef _WIN32
    signal(SIGINT, sig_handler);
    signal(SIGTERM, sig_handler);
#endif

    int server_fd = net_listen(port);
    if (server_fd < 0) {
        fprintf(stderr, "Failed to listen on port %d\n", port);
        return -1;
    }

    printf("msync server listening on port %d\n", port);
    printf("Press Ctrl+C to stop.\n");

    while (keep_running) {
        int client_fd = net_accept(server_fd);
        if (client_fd < 0) {
            if (!keep_running) break;
            continue;
        }

        printf("[server] Client connected.\n");
        handle_client(client_fd);
        net_close(client_fd);
        printf("[server] Client disconnected.\n");
    }

    net_close(server_fd);
    printf("\nServer stopped.\n");
    return 0;
}
