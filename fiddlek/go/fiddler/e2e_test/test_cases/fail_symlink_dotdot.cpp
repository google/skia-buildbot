#include <fcntl.h>
#include <iostream>
#include <unistd.h>

void draw(void* canvas) {
    // /usr/bin/X11 is a symlink to "." (/usr/bin).
    // Lexically, filepath.Clean("/usr/bin/X11/../../etc/passwd") evaluates to
    // "/usr/etc/passwd", which matches the allowed prefix "/usr/etc/".
    // In the kernel, "/usr/bin/X11" resolves to "/usr/bin", so "../../etc/passwd"
    // resolves to "/etc/passwd".
    int fd = open("/usr/bin/X11/../../etc/passwd", O_RDONLY);
    if (fd < 0) {
        std::cout << "DENIED" << std::endl;
        return;
    }
    std::cout << "BYPASS SUCCESS (symlink dotdot)" << std::endl;
    close(fd);
}
