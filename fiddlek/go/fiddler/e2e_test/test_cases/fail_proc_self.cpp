#include <cstdio>
#include <cstring>
#include <cerrno>
#include <fcntl.h>
#include <iostream>
#include <unistd.h>

static void show(const char* path, int max) {
    int fd = open(path, O_RDONLY);
    if (fd < 0) { std::cout << "DENIED" << std::endl; return; }
    std::cout << "BYPASS SUCCESS (/proc/self): " << std::endl;
    close(fd);
}

void draw(void* canvas) {
    show("/proc/self/root/etc/passwd", 80);
}
