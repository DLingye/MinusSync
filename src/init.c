#include "msync.h"

int repo_init(void) {
    if (dir_exists(MSYNC_DIR)) {
        fprintf(stderr, "Repository already exists.\n");
        return -1;
    }

    if (msync_mkdir(MSYNC_DIR) != 0) { perror("mkdir .msync"); return -1; }
    if (msync_mkdir(MSYNC_OBJECTS_DIR) != 0) { perror("mkdir objects"); return -1; }
    if (msync_mkdir(MSYNC_REFS_DIR) != 0) { perror("mkdir refs"); return -1; }
    if (msync_mkdir(MSYNC_HEADS_DIR) != 0) { perror("mkdir heads"); return -1; }
    if (msync_mkdir(MSYNC_REMOTES_DIR) != 0) { perror("mkdir remotes"); return -1; }

    /* Set HEAD to master */
    if (head_set_ref("refs/heads/master") != 0) {
        fprintf(stderr, "Failed to initialize HEAD.\n");
        return -1;
    }

    /* Write initial config */
    const char *conf = "[core]\nrepositoryformatversion = 0\n";
    file_write(MSYNC_CONFIG_FILE, (const uint8_t *)conf, strlen(conf));

    /* Create initial index */
    index_save();

    printf("Initialized empty msync repository in %s\n", MSYNC_DIR);
    return 0;
}
