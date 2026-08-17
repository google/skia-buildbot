#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <unistd.h>

void draw(void *canvas) {
    mknod("/etc/networks", 0644, 0);
}
