#define FUSE_USE_VERSION 37

#include <assert.h>
#include <errno.h>
#include <fcntl.h>
#include <fuse.h>
#include <stddef.h>
#include <stdio.h>
#include <string.h>

extern const char *dummy_version;

const char *dummy_filename = "dummy-file.txt";
const char *dummy_file_contents = "Hello world!\n";

static void *dummy_init(struct fuse_conn_info *conn, struct fuse_config *cfg) {
  (void)conn;
  cfg->kernel_cache = 1;
  return NULL;
}

static int dummy_getattr(const char *path, struct stat *stbuf,
                         struct fuse_file_info *fi) {
  (void)fi;
  int res = 0;

  memset(stbuf, 0, sizeof(struct stat));
  if (strcmp(path, "/") == 0) {
    stbuf->st_mode = S_IFDIR | 0755;
    stbuf->st_nlink = 2;
  } else if (strcmp(path + 1, dummy_filename) == 0) {
    stbuf->st_mode = S_IFREG | 0444;
    stbuf->st_nlink = 1;
    stbuf->st_size = strlen(dummy_file_contents);
  } else {
    res = -ENOENT;
  }

  return res;
}

static int dummy_readdir(const char *path, void *buf, fuse_fill_dir_t filler,
                         off_t offset, struct fuse_file_info *fi,
                         enum fuse_readdir_flags flags) {
  (void)offset;
  (void)fi;
  (void)flags;

  if (strcmp(path, "/") != 0)
    return -ENOENT;

  filler(buf, ".", NULL, 0, 0);
  filler(buf, "..", NULL, 0, 0);
  filler(buf, dummy_filename, NULL, 0, 0);

  return 0;
}

static int dummy_open(const char *path, struct fuse_file_info *fi) {
  if (strcmp(path + 1, dummy_filename) != 0)
    return -ENOENT;

  if ((fi->flags & O_ACCMODE) != O_RDONLY)
    return -EACCES;

  return 0;
}

static int dummy_read(const char *path, char *buf, size_t size, off_t offset,
                      struct fuse_file_info *fi) {
  size_t len;
  (void)fi;
  if (strcmp(path + 1, dummy_filename) != 0)
    return -ENOENT;

  len = strlen(dummy_file_contents);
  if (offset < len) {
    if (offset + size > len)
      size = len - offset;
    memcpy(buf, dummy_file_contents + offset, size);
  } else
    size = 0;

  return size;
}

static const struct fuse_operations dummy_ops = {
  .init    = dummy_init,
  .getattr = dummy_getattr,
  .readdir = dummy_readdir,
  .open    = dummy_open,
  .read    = dummy_read,
};

int main(int argc, char *argv[]) {
  for (int i = 0; i < argc; i++) {
    if (strcmp(argv[i], "--version") == 0) {
      printf("dummy-fuse version: %s\n", dummy_version);
      break;
    }
  }
    
  int ret;
  struct fuse_args args = FUSE_ARGS_INIT(argc, argv);

  ret = fuse_main(args.argc, args.argv, &dummy_ops, NULL);
  fuse_opt_free_args(&args);
  return ret;
}
