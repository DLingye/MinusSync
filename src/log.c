#include "msync.h"

/* Branch color palette for graph mode */
static const char *colors[] = {
    "\033[31m", "\033[32m", "\033[33m", "\033[34m",
    "\033[35m", "\033[36m", "\033[91m", "\033[92m",
    "\033[93m", "\033[94m", "\033[95m", "\033[96m"
};
#define NUM_COLORS 12
#define COLOR_RESET "\033[0m"

/* Graph column tracking */
typedef struct {
    uint8_t  hash[HASH_RAW_SIZE];
    int      color_idx;
    int      active;       /* still has commits to show */
} graph_column_t;

typedef struct {
    uint8_t       hash[HASH_RAW_SIZE];
    uint8_t       parent1[HASH_RAW_SIZE];
    uint8_t       parent2[HASH_RAW_SIZE];
    char          message[MAX_MSG_LEN];
    char          author[256];
    char          email[256];
    char          hostname[256];
    time_t        timestamp;
    int           ref_count;
    char          refs[16][128];
    int           shown;        /* already displayed */
    int           column;       /* assigned graph column */
    int           color_idx;
} commit_info_t;

static commit_info_t *all_commits = NULL;
static int commit_total = 0;

/* Collect all commits reachable from given hash */
static void collect_commits(const uint8_t *hash, int depth) {
    if (depth > 2000 || !object_exists(hash)) return;

    /* Check if already collected */
    for (int i = 0; i < commit_total; i++) {
        if (hash_cmp(all_commits[i].hash, hash) == 0) return;
    }

    uint8_t *data;
    size_t len;
    if (object_read(hash, &data, &len) != 0) return;

    commit_total++;
    all_commits = (commit_info_t *)realloc(all_commits,
                                           commit_total * sizeof(commit_info_t));
    commit_info_t *c = &all_commits[commit_total - 1];
    memset(c, 0, sizeof(commit_info_t));
    hash_cpy(c->hash, hash);

    uint8_t tree[HASH_RAW_SIZE];
    commit_parse(data, len, tree, c->parent1,
                 c->author, c->email, c->hostname, c->message, &c->timestamp);
    free(data);

    /* Check if there's a second parent (merge commit) */
    memset(c->parent2, 0, HASH_RAW_SIZE);
    /* parent2 detection: scan raw commit data for second "parent " line */
    {
        const char *p = strstr((char *)data, "\nparent ");
        if (p) p = strstr(p + 1, "\nparent ");
        if (p) {
            /* Found second parent line */
            char phex[HASH_HEX_SIZE + 1];
            memcpy(phex, p + 8, HASH_HEX_SIZE);
            phex[HASH_HEX_SIZE] = '\0';
            hex_to_hash(phex, c->parent2);
        }
    }

    /* Recursively collect parents */
    if (c->parent1[0] != 0) collect_commits(c->parent1, depth + 1);
}

/* Simple qsort comparator: newest first */
static int commit_cmp(const void *a, const void *b) {
    const commit_info_t *ca = (const commit_info_t *)a;
    const commit_info_t *cb = (const commit_info_t *)b;
    if (ca->timestamp > cb->timestamp) return -1;
    if (ca->timestamp < cb->timestamp) return 1;
    return 0;
}

/* Find the index of a commit by hash */
static int find_commit_idx(const uint8_t *hash) {
    for (int i = 0; i < commit_total; i++) {
        if (hash_cmp(all_commits[i].hash, hash) == 0) return i;
    }
    return -1;
}

/* Annotate commits with ref names */
static void annotate_refs(void) {
    char **ref_names;
    uint8_t *ref_hashes;
    int ref_count;
    ref_list(&ref_names, &ref_hashes, &ref_count);

    for (int r = 0; r < ref_count; r++) {
        for (int c = 0; c < commit_total; c++) {
            if (hash_cmp(all_commits[c].hash, ref_hashes + r * HASH_RAW_SIZE) == 0) {
                if (all_commits[c].ref_count < 16) {
                    /* Shorten refs/heads/ -> empty, refs/remotes/ -> remote/ */
                    const char *display = ref_names[r];
                    snprintf(all_commits[c].refs[all_commits[c].ref_count],
                             sizeof(all_commits[c].refs[0]), "%s", display);
                    all_commits[c].ref_count++;
                }
            }
        }
        free(ref_names[r]);
    }
    free(ref_names);
    free(ref_hashes);

    /* Mark HEAD */
    char head_ref[MAX_PATH_LEN];
    if (head_get_ref(head_ref, sizeof(head_ref)) == 0) {
        uint8_t head_hash[HASH_RAW_SIZE];
        if (head_get_hash(head_hash) == 0) {
            for (int c = 0; c < commit_total; c++) {
                if (hash_cmp(all_commits[c].hash, head_hash) == 0) {
                    if (all_commits[c].ref_count < 16) {
                        /* Mark as HEAD */
                        for (int r = 0; r < all_commits[c].ref_count; r++) {
                            char refname[MAX_PATH_LEN];
                            snprintf(refname, sizeof(refname), "refs/heads/%s",
                                     all_commits[c].refs[r]);
                            if (strncmp(head_ref, refname, strlen(refname)) == 0 ||
                                strncmp(refname, head_ref, strlen(head_ref)) == 0) {
                                char tmp[128];
                                snprintf(tmp, sizeof(tmp), "HEAD -> %s",
                                         all_commits[c].refs[r]);
                                snprintf(all_commits[c].refs[r],
                                         sizeof(all_commits[c].refs[0]), "%s", tmp);
                            }
                        }
                    }
                    break;
                }
            }
        }
    }
}

/* --- Graph mode --- */
static int graph_columns[64];    /* column -> commit_idx, -1 if empty */
static int graph_col_count = 0;

static int find_or_alloc_column(int commit_idx) {
    /* Check if this commit already has a column */
    for (int i = 0; i < graph_col_count; i++) {
        if (graph_columns[i] == commit_idx) return i;
    }
    /* Find empty slot */
    for (int i = 0; i < graph_col_count; i++) {
        if (graph_columns[i] == -1) {
            graph_columns[i] = commit_idx;
            return i;
        }
    }
    /* Add new column */
    if (graph_col_count < 64) {
        graph_columns[graph_col_count] = commit_idx;
        return graph_col_count++;
    }
    return -1;
}

static void print_graph_line(int cur_idx) {
    commit_info_t *c = &all_commits[cur_idx];
    char hex[HASH_HEX_SIZE + 1];
    hash_to_hex(c->hash, hex);

    int col = c->column;
    int color_n = c->color_idx % NUM_COLORS;

    /* Draw graph columns */
    for (int i = 0; i <= col && i < graph_col_count; i++) {
        if (i == col) {
            /* This commit's column */
            printf("%s*%s", colors[color_n], COLOR_RESET);
        } else if (graph_columns[i] >= 0) {
            /* Active line passing through */
            int ci = graph_columns[i];
            if (ci >= 0 && ci < commit_total) {
                printf("%s|%s", colors[all_commits[ci].color_idx % NUM_COLORS], COLOR_RESET);
            }
        } else {
            printf(" ");
        }
    }

    /* Commit hash and refs */
    printf(" %s%.8s%s", colors[color_n], hex, COLOR_RESET);

    if (c->ref_count > 0) {
        printf(" (");
        for (int i = 0; i < c->ref_count; i++) {
            const char *r = c->refs[i];
            if (strncmp(r, "HEAD", 4) == 0)
                printf("\033[1;36m%s\033[0m", r);
            else if (strncmp(r, "heads/", 6) == 0)
                printf("\033[1;32m%s\033[0m", r + 6);
            else if (strncmp(r, "remotes/", 8) == 0)
                printf("\033[1;31m%s\033[0m", r + 8);
            else
                printf("%s", r);
            if (i + 1 < c->ref_count) printf(", ");
        }
        printf(")");
    }

    printf(" %s\n", c->message);

    /* Update columns: this commit's column now points to its parent */
    int parent_idx = find_commit_idx(c->parent1);
    if (parent_idx >= 0 && all_commits[parent_idx].shown == 0) {
        graph_columns[col] = parent_idx;
    } else {
        graph_columns[col] = -1;
    }
}

static void print_graph(int max_count) {
    if (commit_total == 0) {
        printf("No commits.\n");
        return;
    }

    /* Sort by timestamp, newest first */
    qsort(all_commits, commit_total, sizeof(commit_info_t), commit_cmp);

    /* Assign colors and initial columns */
    memset(graph_columns, -1, sizeof(graph_columns));
    graph_col_count = 0;

    /* First pass: assign colors to branches by tracing from heads */
    char **ref_names;
    uint8_t *ref_hashes;
    int ref_count;
    ref_list(&ref_names, &ref_hashes, &ref_count);

    int color_idx = 0;
    for (int r = 0; r < ref_count; r++) {
        /* Only local branches get distinct colors */
        if (strncmp(ref_names[r], "heads/", 6) == 0) {
            uint8_t cur[HASH_RAW_SIZE];
            hash_cpy(cur, ref_hashes + r * HASH_RAW_SIZE);
            int depth = 0;
            while (depth < 1000 && object_exists(cur)) {
                int ci = find_commit_idx(cur);
                if (ci >= 0 && all_commits[ci].color_idx == 0) {
                    all_commits[ci].color_idx = color_idx;
                }
                /* Follow parent */
                uint8_t *d;
                size_t dl;
                if (object_read(cur, &d, &dl) != 0) break;
                uint8_t parent[HASH_RAW_SIZE];
                memset(parent, 0, HASH_RAW_SIZE);
                commit_parse(d, dl, NULL, parent, NULL, NULL, NULL, NULL, NULL);
                free(d);
                if (parent[0] == 0) break;
                hash_cpy(cur, parent);
                depth++;
            }
            color_idx = (color_idx + 1) % NUM_COLORS;
        }
        free(ref_names[r]);
    }
    free(ref_names);
    free(ref_hashes);

    /* Default color for unassigned commits */
    for (int i = 0; i < commit_total; i++) {
        if (all_commits[i].color_idx == 0 && all_commits[i].ref_count == 0) {
            int parent_idx = find_commit_idx(all_commits[i].parent1);
            if (parent_idx >= 0) {
                all_commits[i].color_idx = all_commits[parent_idx].color_idx;
            }
        }
    }

    /* Display: walk commits in sorted order */
    int shown_count = 0;
    for (int i = 0; i < commit_total && shown_count < max_count; i++) {
        commit_info_t *c = &all_commits[i];
        if (c->shown) continue;

        /* Assign column for this commit */
        int pidx = find_commit_idx(c->parent1);
        if (pidx >= 0 && all_commits[pidx].column > 0 && all_commits[pidx].shown == 0) {
            c->column = all_commits[pidx].column;
        } else {
            c->column = find_or_alloc_column(i);
        }

        print_graph_line(i);
        c->shown = 1;
        shown_count++;
    }
}

/* --- Main log function --- */

int repo_log(int max_count, int oneline, int graphic) {
    if (!dir_exists(MSYNC_DIR)) {
        fprintf(stderr, "Not an msync repository.\n");
        return -1;
    }

    if (graphic) {
        /* Collect all commits from all branches */
        commit_total = 0;
        all_commits = NULL;

        char **ref_names;
        uint8_t *ref_hashes;
        int ref_count;
        ref_list(&ref_names, &ref_hashes, &ref_count);

        for (int r = 0; r < ref_count; r++) {
            collect_commits(ref_hashes + r * HASH_RAW_SIZE, 0);
            free(ref_names[r]);
        }
        free(ref_names);
        free(ref_hashes);

        if (commit_total == 0) {
            uint8_t head_hash[HASH_RAW_SIZE];
            if (head_get_hash(head_hash) == 0) {
                collect_commits(head_hash, 0);
            }
        }

        if (commit_total == 0) {
            fprintf(stderr, "No commits yet.\n");
            return -1;
        }

        annotate_refs();
        print_graph(max_count);

        free(all_commits);
        all_commits = NULL;
        commit_total = 0;
        return 0;
    }

    /* Standard log and --oneline */
    uint8_t hash[HASH_RAW_SIZE];
    if (head_get_hash(hash) != 0) {
        fprintf(stderr, "No commits yet.\n");
        return -1;
    }

    int count = 0;
    while (count < max_count && object_exists(hash)) {
        uint8_t *data;
        size_t len;
        if (object_read(hash, &data, &len) != 0) break;

        uint8_t tree_hash[HASH_RAW_SIZE];
        uint8_t parent_hash[HASH_RAW_SIZE];
        char author[256] = "";
        char email[256] = "";
        char hostname[256] = "";
        char message[MAX_MSG_LEN] = "";
        time_t timestamp = 0;

        commit_parse(data, len, tree_hash, parent_hash,
                     author, email, hostname, message, &timestamp);

        char hex[HASH_HEX_SIZE + 1];
        hash_to_hex(hash, hex);

        if (oneline) {
            /* One-line format */
            printf("\033[33m%.8s\033[0m %s\n", hex, message);
        } else {
            /* Full format */
            char time_str[64];
            struct tm *tm_info = localtime(&timestamp);
            strftime(time_str, sizeof(time_str), "%Y-%m-%d %H:%M:%S", tm_info);

            printf("\033[33mcommit %s\033[0m\n", hex);
            printf("Author: %s <%s>\n", author, email);
            printf("Host:   %s\n", hostname[0] ? hostname : "unknown");
            printf("Date:   %s\n\n", time_str);
            printf("    %s\n\n", message);
        }

        free(data);
        count++;

        if (parent_hash[0] == 0 && parent_hash[1] == 0) break;
        hash_cpy(hash, parent_hash);
    }

    return 0;
}
