#include <fcntl.h>
#include <iostream>
#include <sys/stat.h>
#include <unistd.h>

void draw(void* canvas) {
    mkdir("/tmp/sub", 0755);
    mkdir("/tmp/sub/deep", 0755);
    chdir("/tmp/sub/deep");
    int dirfd = open("/etc/fiddle/source", O_RDONLY | O_DIRECTORY);
    if (dirfd < 0) {
        std::cout << "DENIED" << std::endl;
        return;
    }
    int fd = openat(dirfd, "../../passwd", O_RDONLY);
    if (fd < 0) {
        std::cout << "DENIED" << std::endl;
        close(dirfd);
        return;
    }
    std::cout << "BYPASS SUCCESS (openat dirfd)" << std::endl;
    close(fd);
    close(dirfd);
}
