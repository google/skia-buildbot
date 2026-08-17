#include <unistd.h>

void draw(void* canvas) {
    char *newargv[] = { (char *)"/bin/ls", NULL };
    char *newenviron[] = { NULL };
    execve("/bin/ls", newargv, newenviron);
}
