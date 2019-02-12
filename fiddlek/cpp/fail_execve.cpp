#include <unistd.h>

int main() {
    char *newargv[] = { NULL, (char *)"hello", (char *)"world", NULL };
    char *newenviron[] = { NULL };
    execve("/bin/echo", newargv, newenviron);
    return 0;
}
