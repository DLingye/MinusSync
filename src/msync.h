/* msync - A lightweight version control system
 * Copyright (C) 2026  msync contributors
 * SPDX-License-Identifier: AGPL-3.0-or-later
 * This program is free software licensed under the GNU AGPL v3 or later.
 * See the LICENSE file for the full license text.
 */

#ifndef MSYNC_H
#define MSYNC_H

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <sys/stat.h>
#include <time.h>
#include <errno.h>

#ifdef _WIN32
    #include <winsock2.h>
    #include <ws2tcpip.h>
    #include <windows.h>
    #include <direct.h>
    #define mkdir(p,m)  _mkdir(p)
    #define rmdir(p)    _rmdir(p)
    #define unlink(p)   _unlink(p)
    #define MSYNC_OS_WINDOWS
    typedef int socklen_t;
#else
    #include <unistd.h>
    #include <dirent.h>
    #include <sys/types.h>
    #include <sys/socket.h>
    #include <netinet/in.h>
    #include <arpa/inet.h>
    #include <netdb.h>
    #define SOCKET int
    #define INVALID_SOCKET (-1)
    #define SOCKET_ERROR   (-1)
    #define closesocket(s) close(s)
#endif

#include "sha256.h"

#define MSYNC_VERSION       "0.0606-5"
#define MSYNC_BUILD         "0005"
#define MSYNC_DEFAULT_PORT  65530
#define MSYNC_IGNORE_FILE   ".msyncign"

/* Status states */
#define STATUS_NEW     0
#define STATUS_MODIFIED 1
#define STATUS_DELETED  2
#define MSYNC_DIR           ".msync"
#define MSYNC_OBJECTS_DIR   ".msync/objects"
#define MSYNC_REFS_DIR      ".msync/refs"
#define MSYNC_HEADS_DIR     ".msync/refs/heads"
#define MSYNC_REMOTES_DIR   ".msync/refs/remotes"
#define MSYNC_HEAD_FILE     ".msync/HEAD"
#define MSYNC_INDEX_FILE    ".msync/index"
#define MSYNC_CONFIG_FILE   ".msync/config"
#define HASH_HEX_SIZE       64
#define HASH_RAW_SIZE       32
#define MAX_PATH_LEN        4096
#define MAX_MSG_LEN         8192
#define MAX_OBJ_SIZE        (64 * 1024 * 1024)
#define MAX_COMMIT_SIZE     65536

/* Object types */
#define OBJ_BLOB    0
#define OBJ_TREE    1
#define OBJ_COMMIT  2

/* Object storage */
int  object_write(const uint8_t *data, size_t len, uint8_t *hash_out);
int  object_read(const uint8_t *hash, uint8_t **data, size_t *len);
int  object_exists(const uint8_t *hash);
void hash_to_hex(const uint8_t *hash, char *hex);
int  hex_to_hash(const char *hex, uint8_t *hash);
void hash_cpy(uint8_t *dst, const uint8_t *src);
int  hash_cmp(const uint8_t *a, const uint8_t *b);

/* Blob */
int  blob_create(const char *filepath, uint8_t *hash_out);

/* Tree */
int  tree_build(const char *dirpath, uint8_t *hash_out);
int  tree_checkout(const uint8_t *hash, const char *target_dir);
int  tree_entries_parse(const uint8_t *data, size_t len,
                        char ***names, uint8_t **hashes, int *count);

/* Commit */
int  commit_create(const uint8_t *tree_hash, const uint8_t *parent_hash,
                   const char *author, const char *email,
                   const char *message, uint8_t *hash_out);
int  commit_parse(const uint8_t *data, size_t len,
                  uint8_t *tree_hash, uint8_t *parent_hash,
                  char *author, char *email, char *hostname,
                  char *message, time_t *timestamp);

/* Index */
int  index_load(void);
int  index_save(void);
int  index_add(const char *path, const uint8_t *hash, uint64_t size, int64_t mtime, uint32_t mode);
int  index_remove(const char *path);
int  index_lookup(const char *path, uint8_t *hash, uint64_t *size, int64_t *mtime);
int  index_get_count(void);
const char *index_get_path(int i);
void index_clear_entries(void);
void index_reset(void);

/* Refs */
int  ref_resolve(const char *ref, uint8_t *hash);
int  ref_update(const char *ref, const uint8_t *hash);
int  ref_list(char ***names, uint8_t **hashes, int *count);
int  ref_delete(const char *ref);

/* HEAD */
int  head_get_ref(char *ref, size_t size);
int  head_set_ref(const char *ref);
int  head_get_hash(uint8_t *hash);

/* Diff */
int  diff_file(const char *path, const uint8_t *old_hash, const uint8_t *new_hash);
int  status_check(int *new_count, int *mod_count, int *del_count, char ***names, int **states);

/* Networking */
int  net_connect(const char *host, int port);
int  net_listen(int port);
int  net_accept(int server_fd);
int  net_send(int fd, const uint8_t *data, size_t len);
int  net_recv(int fd, uint8_t *data, size_t len);
int  net_sendline(int fd, const char *line);
int  net_recvline(int fd, char *buf, size_t size);
void net_close(int fd);

/* Protocol */
int  proto_fetch(int fd, const uint8_t *want_hashes[], int want_count,
                 const uint8_t *have_hashes[], int have_count);
int  proto_push(int fd, const char *refname, const uint8_t *old_hash,
                const uint8_t *new_hash);
int  proto_list_remote_refs(int fd, char ***refs, uint8_t **hashes, int *count);
int  proto_send_objects(int fd, const uint8_t *old_hash, const uint8_t *new_hash);
void collect_all_objects(const uint8_t *hash, const uint8_t *stop_hash,
                         uint8_t **all_hashes, int *hash_count);

/* Config */
int  config_get(const char *key, char *value, size_t size);
int  config_has(const char *key);
int  config_set(const char *key, const char *value);
int  config_unset(const char *key);
int  config_list_keys(const char *prefix, char ***keys, int *count);
int  config_load(void);

/* Remote */
int  remote_add(const char *name, const char *url);
int  remote_remove(const char *name);
int  remote_list(void);
int  remote_resolve(const char *name_or_url, char *host, int *port, char *path, size_t path_size);
int  remote_get_tracking_hash(const char *remote_name, const char *branch, uint8_t *hash);

/* Merge / conflict */
int  merge_is_fast_forward(const uint8_t *local_hash, const uint8_t *remote_hash);
int  merge_commits(const uint8_t *local_hash, const uint8_t *remote_hash, uint8_t *result_hash);

/* Util */
int  msync_mkdir(const char *path);
int  file_exists(const char *path);
int  dir_exists(const char *path);
int  file_read(const char *path, uint8_t **data, size_t *len);
int  file_write(const char *path, const uint8_t *data, size_t len);
void file_list(const char *dir, char ***files, int *count);
void file_list_free(char **files, int count);
int  is_ignored(const char *path);
int  ignore_add(const char *pattern);
int  ignore_list(void);
int  ignore_remove(const char *pattern);
void strip_newline(char *s);
char *msync_strdup(const char *s);
void msync_free(void *p);

/* Repository operations */
int  repo_init(void);
int  repo_commit(const char *message);
int  repo_status(void);
int  repo_log(int max_count, int oneline, int graphic);
int  repo_branch_list(void);
int  repo_branch_create(const char *name);
int  repo_branch_delete(const char *name);
int  repo_checkout(const char *target);
int  repo_push(const char *remote_url, const char *branch);
int  repo_update(const char *remote_url, const char *branch);
int  repo_clone(const char *remote_url, const char *dir, const char *remote_name);
int  repo_serve(int port);
int  repo_mirror_once(const char *source_url);
int  repo_mirror_daemon(const char *source_url, int interval_sec, int serve_port);
int  repo_fsck(int verbose);
int  repo_gc(int prune);

#endif
