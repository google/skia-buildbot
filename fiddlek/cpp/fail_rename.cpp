#include <stdio.h>

int main() {
    rename("/bin/echo", "/bin/sudo");
    return 0;
}
