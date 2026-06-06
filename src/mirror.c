#include "msync.h"
#include <signal.h>

#ifndef _WIN32
#include <sys/wait.h>
static volatile int mirror_running = 1;

static void mirror_sig_handler(int sig) {
    (void)sig;
    mirror_running = 0;
}
#endif

/* One-shot: pull all refs from source and update local tracking refs.
 * Returns 0 on success, -1 on error. */
int repo_mirror_once(const char *source_url) {
    char host[256];
    int port = MSYNC_DEFAULT_PORT;
    char path[MAX_PATH_LEN] = "";

    remote_resolve(source_url, host, &port, path, sizeof(path));

    /* Connect to source */
    int fd = net_connect(host, port);
    if (fd < 0) {
        fprintf(stderr, "mirror: failed to connect to %s:%d\n", host, port);
        return -1;
    }

    /* List all remote refs */
    char **ref_names;
    uint8_t *ref_hashes;
    int ref_count;

    if (proto_list_remote_refs(fd, &ref_names, &ref_hashes, &ref_count) != 0) {
        fprintf(stderr, "mirror: failed to list remote refs\n");
        net_close(fd);
        return -1;
    }

    if (ref_count == 0) {
        printf("mirror: remote repository is empty, nothing to mirror.\n");
        net_close(fd);
        return 0;
    }

    /* Collect all remote hashes the mirror needs */
    const uint8_t *want[256];
    int want_count = 0;
    int new_objects = 0;

    for (int i = 0; i < ref_count && want_count < 256; i++) {
        if (!object_exists(ref_hashes + i * HASH_RAW_SIZE)) {
            want[want_count++] = ref_hashes + i * HASH_RAW_SIZE;
            new_objects = 1;
        }
    }

    if (!new_objects) {
        printf("mirror: already up to date (%d refs).\n", ref_count);
        for (int i = 0; i < ref_count; i++) free(ref_names[i]);
        free(ref_names);
        free(ref_hashes);
        net_close(fd);
        return 0;
    }

    /* Fetch all needed objects */
    printf("mirror: fetching %d/%d refs from %s:%d...\n",
           want_count, ref_count, host, port);

    if (proto_fetch(fd, want, want_count, NULL, 0) != 0) {
        fprintf(stderr, "mirror: failed to fetch objects\n");
        for (int i = 0; i < ref_count; i++) free(ref_names[i]);
        free(ref_names);
        free(ref_hashes);
        net_close(fd);
        return -1;
    }

    net_close(fd);

    /* Update local tracking refs */
    char track_dir[MAX_PATH_LEN];
    snprintf(track_dir, sizeof(track_dir), "%s/mirror", MSYNC_REMOTES_DIR);
    if (!dir_exists(track_dir)) msync_mkdir(track_dir);

    int updated = 0;
    for (int i = 0; i < ref_count; i++) {
        /* ref_names[i] is like "refs/heads/master" or "refs/remotes/.../..." */
        /* Store as remotes/mirror/<rest-of-path> */
        const char *ref_path = ref_names[i];
        if (strncmp(ref_path, "refs/", 5) == 0) ref_path += 5;

        char local_ref[MAX_PATH_LEN];
        snprintf(local_ref, sizeof(local_ref), "remotes/mirror/%s", ref_path);

        uint8_t *rh = ref_hashes + i * HASH_RAW_SIZE;
        uint8_t old_hash[HASH_RAW_SIZE];

        if (ref_resolve(local_ref, old_hash) != 0 || hash_cmp(old_hash, rh) != 0) {
            ref_update(local_ref, rh);
            updated++;
        }
        free(ref_names[i]);
    }
    free(ref_names);
    free(ref_hashes);

    printf("mirror: synced %d refs (%d updated).\n", ref_count, updated);
    return 0;
}

/* Daemon mode: continuously mirror from source at the given interval.
 * If serve_port > 0, also serves the mirrored repository. */
int repo_mirror_daemon(const char *source_url, int interval_sec, int serve_port) {
#ifndef _WIN32
    /* Set up signal handlers for graceful shutdown */
    signal(SIGINT,  mirror_sig_handler);
    signal(SIGTERM, mirror_sig_handler);

    /* Start server in child process if requested */
    pid_t server_pid = 0;
    if (serve_port > 0) {
        server_pid = fork();
        if (server_pid < 0) {
            fprintf(stderr, "mirror: failed to fork server process\n");
            return -1;
        }
        if (server_pid == 0) {
            /* Child: run the server (inherits parent's stdio) */
            repo_serve(serve_port);
            _exit(0);
        }
        printf("mirror: server started on port %d (pid %d)\n", serve_port, server_pid);
        fflush(stdout);
        sleep(1); /* Give the child time to bind the port */
    }

    /* Main mirror loop */
    printf("mirror: daemon started, syncing from %s every %d seconds\n",
           source_url, interval_sec);
    printf("mirror: press Ctrl+C to stop\n");
    fflush(stdout);

    int cycle = 0;
    while (mirror_running) {
        if (cycle > 0) {
            /* Wait between cycles, but check for interrupt signal each second */
            int waited = 0;
            while (waited < interval_sec && mirror_running) {
                sleep(1);
                waited++;
            }
        }

        if (!mirror_running) break;

        time_t now = time(NULL);
        char time_str[32];
        strftime(time_str, sizeof(time_str), "%Y-%m-%d %H:%M:%S", localtime(&now));
        printf("\n[%s] mirror cycle #%d\n", time_str, cycle + 1);
        fflush(stdout);

        if (repo_mirror_once(source_url) != 0) {
            fprintf(stderr, "mirror: sync cycle failed, will retry in %d seconds\n",
                    interval_sec);
            fflush(stderr);
        }
        cycle++;
    }

    printf("\nmirror: shutting down...\n");
    fflush(stdout);

    /* Clean up server child */
    if (server_pid > 0) {
        kill(server_pid, SIGTERM);
        waitpid(server_pid, NULL, 0);
        printf("mirror: server stopped.\n");
    }
#else
    fprintf(stderr, "mirror: daemon mode is not supported on Windows.\n");
    fprintf(stderr, "Use 'msync mirror once <url>' with Windows Task Scheduler instead.\n");
    return -1;
#endif

    return 0;
}
