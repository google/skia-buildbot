#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <unistd.h>

int main() {
    mknod("/etc/networks", 0644, 0);
    return 0;
}
