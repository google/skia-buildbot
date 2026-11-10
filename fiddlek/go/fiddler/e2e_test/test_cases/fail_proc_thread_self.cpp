#include <fcntl.h>
#include <iostream>
#include <unistd.h>

void draw(void* canvas) {
    // Attempt to bypass path prefix checks via /proc/thread-self/cwd after
    // changing the child's working directory to /etc.
    if (chdir("/etc") != 0) {
        std::cout << "CHDIR FAILED" << std::endl;
        return;
    }
    int fd = open("/proc/thread-self/cwd/passwd", O_RDONLY);
    if (fd < 0) {
        std::cout << "DENIED" << std::endl;
        return;
    }
    std::cout << "BYPASS SUCCESS (proc thread-self cwd)" << std::endl;
    close(fd);
}
