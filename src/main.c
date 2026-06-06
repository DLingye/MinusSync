/* SPDX-License-Identifier: AGPL-3.0-or-later */

#include "msync.h"
#include "quotes.h"

static void print_usage(void) {
    printf("Minus Sync (msync) Version %s Build %s - A simplified version control system\n", MSYNC_VERSION, MSYNC_BUILD);
    printf("Usage: msync <command> [options]\n\n");
    printf("Commands:\n");
    printf("  init                      Initialize a new msync repository\n");
    printf("  status                    Show working tree status\n");
    printf("  commit  -m <message>      Commit changes to local repository\n");
    printf("  log     [-n <count>] [--oneline] [--graphic]\n");
    printf("                           Show commit history\n");
    printf("  branch                    List branches\n");
    printf("  branch  <name>            Create a new branch\n");
    printf("  branch  -d <name>         Delete a branch\n");
    printf("  checkout <branch|commit>  Switch to branch or commit\n");
    printf("  remote add <name> <url>   Add a remote repository\n");
    printf("  remote list               List remote repositories\n");
    printf("  remote remove <name>      Remove a remote repository\n");
    printf("  push    <remote|url> [branch]  Push to remote repository\n");
    printf("  update  <remote|url> [branch]  Update from remote repository\n");
    printf("  clone   <url> [dir] [--name <n>]\n");
    printf("                           Clone and add remote (default: origin)\n");
    printf("  mirror once <url>         One-shot mirror from source\n");
    printf("  mirror start [url] [-i <s>] [--serve <p>]\n");
    printf("                           Start mirror daemon (pull + serve)\n");
    printf("  serve   [-p <port>]       Start server mode\n");
    printf("  ignore  add <pattern>      Add an ignore pattern\n");
    printf("  ignore  list               List ignore patterns\n");
    printf("  ignore  remove <pattern>   Remove an ignore pattern\n");
    printf("  fsck    [-v]               Verify repository integrity\n");
    printf("  gc      [--prune]          Garbage collect unreachable objects\n");
    printf("  config  <key> [value]     Get or set configuration\n");
    printf("\nRemote URL format: host:port or msync://host:port\n");
    printf("Use 'msync remote add <name> <url>' to save a remote.\n");
    printf("Default port: %d\n", MSYNC_DEFAULT_PORT);
}

int main(int argc, char *argv[]) {
    if (argc < 2) {
        print_usage();
        srand((unsigned int)time(NULL));
        int idx = rand() % QUOTE_COUNT;
        printf("\n  \033[36m%s\033[0m\n", quotes[idx]);
        return 0;
    }

    const char *cmd = argv[1];

    if (strcmp(cmd, "init") == 0) {
        return repo_init();

    } else if (strcmp(cmd, "status") == 0) {
        return repo_status();

    } else if (strcmp(cmd, "commit") == 0) {
        const char *msg = NULL;
        for (int i = 2; i < argc; i++) {
            if (strcmp(argv[i], "-m") == 0 && i + 1 < argc) {
                msg = argv[++i];
            } else if (strncmp(argv[i], "-m", 2) == 0 && strlen(argv[i]) > 2) {
                msg = argv[i] + 2;
            }
        }
        if (!msg) {
            fprintf(stderr, "Usage: msync commit -m <message>\n");
            return 1;
        }
        return repo_commit(msg);

    } else if (strcmp(cmd, "log") == 0) {
        int max_count = 10;
        int oneline = 0;
        int graphic = 0;
        for (int i = 2; i < argc; i++) {
            if (strcmp(argv[i], "-n") == 0 && i + 1 < argc) {
                max_count = atoi(argv[++i]);
            } else if (strcmp(argv[i], "--oneline") == 0) {
                oneline = 1;
            } else if (strcmp(argv[i], "--graphic") == 0) {
                graphic = 1;
            }
        }
        return repo_log(max_count, oneline, graphic);

    } else if (strcmp(cmd, "branch") == 0) {
        if (argc == 2) {
            return repo_branch_list();
        } else if (strcmp(argv[2], "-d") == 0 && argc >= 4) {
            return repo_branch_delete(argv[3]);
        } else {
            return repo_branch_create(argv[2]);
        }

    } else if (strcmp(cmd, "checkout") == 0) {
        if (argc < 3) {
            fprintf(stderr, "Usage: msync checkout <branch|commit>\n");
            return 1;
        }
        return repo_checkout(argv[2]);

    } else if (strcmp(cmd, "push") == 0) {
        if (argc < 3) {
            fprintf(stderr, "Usage: msync push <url> [branch]\n");
            return 1;
        }
        return repo_push(argv[2], argc >= 4 ? argv[3] : NULL);

    } else if (strcmp(cmd, "update") == 0 || strcmp(cmd, "pull") == 0) {
        if (argc < 3) {
            fprintf(stderr, "Usage: msync update <url> [branch]\n");
            return 1;
        }
        return repo_update(argv[2], argc >= 4 ? argv[3] : NULL);

    } else if (strcmp(cmd, "clone") == 0) {
        if (argc < 3) {
            fprintf(stderr, "Usage: msync clone [--name <remote>] <url> [directory]\n");
            return 1;
        }
        const char *clone_url  = NULL;
        const char *dir        = NULL;
        const char *remote_nm  = "origin";

        for (int i = 2; i < argc; i++) {
            if (strcmp(argv[i], "--name") == 0 && i + 1 < argc) {
                remote_nm = argv[++i];
            } else if (!clone_url && argv[i][0] != '-') {
                clone_url = argv[i];
            } else if (clone_url && !dir && argv[i][0] != '-') {
                dir = argv[i];
            }
        }
        if (!clone_url) {
            fprintf(stderr, "Usage: msync clone [--name <remote>] <url> [directory]\n");
            return 1;
        }
        if (!dir) dir = "msync_repo";
        return repo_clone(clone_url, dir, remote_nm);

    } else if (strcmp(cmd, "mirror") == 0) {
        if (argc < 3) {
            fprintf(stderr, "Usage: msync mirror <once|start> [args...]\n");
            return 1;
        }
        if (strcmp(argv[2], "once") == 0) {
            if (argc < 4) {
                fprintf(stderr, "Usage: msync mirror once <source-url>\n");
                return 1;
            }
            return repo_mirror_once(argv[3]);
        } else if (strcmp(argv[2], "start") == 0) {
            char src_buf[256] = "";
            int interval = 60;
            int serve_port = 0;

            for (int i = 3; i < argc; i++) {
                if ((strcmp(argv[i], "-i") == 0 || strcmp(argv[i], "--interval") == 0)
                    && i + 1 < argc) {
                    interval = atoi(argv[++i]);
                } else if (strcmp(argv[i], "--serve") == 0 && i + 1 < argc) {
                    serve_port = atoi(argv[++i]);
                } else if (!src_buf[0] && argv[i][0] != '-') {
                    snprintf(src_buf, sizeof(src_buf), "%s", argv[i]);
                }
            }

            /* Fall back to config if no URL on command line */
            if (!src_buf[0]) {
                if (config_get("mirror.source", src_buf, sizeof(src_buf)) != 0) {
                    fprintf(stderr, "Usage: msync mirror start <source-url> [options]\n");
                    fprintf(stderr, "  Or set 'msync config mirror.source <url>' first.\n");
                    return 1;
                }

                char cfg_int[32];
                if (config_get("mirror.interval", cfg_int, sizeof(cfg_int)) == 0) {
                    interval = atoi(cfg_int);
                }
                char cfg_port[32];
                if (config_get("mirror.serve-port", cfg_port, sizeof(cfg_port)) == 0) {
                    serve_port = atoi(cfg_port);
                }
            }

            return repo_mirror_daemon(src_buf, interval, serve_port);
        } else {
            fprintf(stderr, "Unknown mirror command: %s\n", argv[2]);
            fprintf(stderr, "Usage: msync mirror <once|start>\n");
            return 1;
        }

    } else if (strcmp(cmd, "serve") == 0) {
        int port = MSYNC_DEFAULT_PORT;
        for (int i = 2; i < argc; i++) {
            if (strcmp(argv[i], "-p") == 0 && i + 1 < argc) {
                port = atoi(argv[++i]);
            }
        }
        return repo_serve(port);

    } else if (strcmp(cmd, "remote") == 0) {
        if (argc < 3) {
            fprintf(stderr, "Usage: msync remote <add|list|remove> [args...]\n");
            return 1;
        }
        if (strcmp(argv[2], "add") == 0) {
            if (argc < 5) {
                fprintf(stderr, "Usage: msync remote add <name> <url>\n");
                return 1;
            }
            return remote_add(argv[3], argv[4]);
        } else if (strcmp(argv[2], "remove") == 0 || strcmp(argv[2], "rm") == 0) {
            if (argc < 4) {
                fprintf(stderr, "Usage: msync remote remove <name>\n");
                return 1;
            }
            return remote_remove(argv[3]);
        } else if (strcmp(argv[2], "list") == 0 || strcmp(argv[2], "ls") == 0) {
            return remote_list();
        } else {
            fprintf(stderr, "Unknown remote command: %s\n", argv[2]);
            fprintf(stderr, "Usage: msync remote <add|list|remove>\n");
            return 1;
        }

    } else if (strcmp(cmd, "ignore") == 0) {
        if (argc < 3) {
            fprintf(stderr, "Usage: msync ignore <add|list|remove> [pattern]\n");
            return 1;
        }
        if (strcmp(argv[2], "add") == 0) {
            if (argc < 4) {
                fprintf(stderr, "Usage: msync ignore add <pattern>\n");
                return 1;
            }
            return ignore_add(argv[3]);
        } else if (strcmp(argv[2], "list") == 0 || strcmp(argv[2], "ls") == 0) {
            return ignore_list();
        } else if (strcmp(argv[2], "remove") == 0 || strcmp(argv[2], "rm") == 0) {
            if (argc < 4) {
                fprintf(stderr, "Usage: msync ignore remove <pattern>\n");
                return 1;
            }
            return ignore_remove(argv[3]);
        } else {
            fprintf(stderr, "Unknown ignore command: %s\n", argv[2]);
            return 1;
        }

    } else if (strcmp(cmd, "fsck") == 0) {
        int verbose = 0;
        for (int i = 2; i < argc; i++) {
            if (strcmp(argv[i], "-v") == 0 || strcmp(argv[i], "--verbose") == 0)
                verbose = 1;
        }
        return repo_fsck(verbose);

    } else if (strcmp(cmd, "gc") == 0) {
        int prune = 0;
        for (int i = 2; i < argc; i++) {
            if (strcmp(argv[i], "--prune") == 0) prune = 1;
        }
        return repo_gc(prune);

    } else if (strcmp(cmd, "config") == 0) {
        if (argc < 3) {
            fprintf(stderr, "Usage: msync config <key> [value]\n");
            return 1;
        }
        if (argc == 3) {
            char value[256];
            if (config_get(argv[2], value, sizeof(value)) == 0) {
                printf("%s\n", value);
            } else {
                printf("(not set)\n");
            }
        } else {
            config_set(argv[2], argv[3]);
        }
        return 0;

    } else if (strcmp(cmd, "help") == 0 || strcmp(cmd, "--help") == 0 ||
               strcmp(cmd, "-h") == 0) {
        print_usage();
        return 0;

    } else if (strcmp(cmd, "--version") == 0 || strcmp(cmd, "-v") == 0) {
        printf("msync version %s\n", MSYNC_VERSION);
        return 0;

    } else {
        fprintf(stderr, "Unknown command: %s\n", cmd);
        fprintf(stderr, "Run 'msync help' for usage.\n");
        return 1;
    }
}
