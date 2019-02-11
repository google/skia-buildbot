#include <unistd.h>

int main() {
    link("/bin/echo", "/bin/sudo");
    return 0;
}
