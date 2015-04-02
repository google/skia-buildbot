#include <unistd.h>
#include <fcntl.h>
#include <sys/time.h>
#include <sys/resource.h>
#include "seccomp_bpf.h"

static bool install_syscall_filter() {
    struct sock_filter filter[] = {
        /* Grab the system call number. */
        EXAMINE_SYSCALL,
        /* List allowed syscalls. */
        ALLOW_SYSCALL(exit_group),
        ALLOW_SYSCALL(exit),
        ALLOW_SYSCALL(fstat),
        ALLOW_SYSCALL(read),
        ALLOW_SYSCALL(write),
        ALLOW_SYSCALL(close),
        ALLOW_SYSCALL(mmap),
        ALLOW_SYSCALL(munmap),
        ALLOW_SYSCALL(brk),
        ALLOW_SYSCALL(futex),
        ALLOW_SYSCALL(lseek),
        KILL_PROCESS,
    };
    struct sock_fprog prog = {
        sizeof(filter)/sizeof(filter[0]),
        filter,
    };

    // Lock down the app so that it can't get new privs, such as setuid.
    // Calling this is a requirement for an unprivileged process to use mode
    // 2 seccomp filters, ala SECCOMP_MODE_FILTER, otherwise we'd have to be
    // root.
    if (prctl(PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0)) {
        perror("prctl(NO_NEW_PRIVS)");
        goto failed;
    }
    // Now call seccomp and restrict the system calls that can be made to only
    // the ones in the provided filter list.
    if (prctl(PR_SET_SECCOMP, SECCOMP_MODE_FILTER, &prog)) {
        perror("prctl(SECCOMP)");
        goto failed;
    }
    return true;

failed:
    if (errno == EINVAL) {
        fprintf(stderr, "SECCOMP_FILTER is not available. :(\n");
    }
    return false;
}

static void setLimits() {
    struct rlimit n;

    // Limit to 5 seconds of CPU.
    n.rlim_cur = 5;
    n.rlim_max = 5;
    if (setrlimit(RLIMIT_CPU, &n)) {
        perror("setrlimit(RLIMIT_CPU)");
    }

    // Limit to 150M of Address space.
    n.rlim_cur = 150000000;
    n.rlim_max = 150000000;
    if (setrlimit(RLIMIT_AS, &n)) {
        perror("setrlimit(RLIMIT_CPU)");
    }
}

int main(int argc, char** argv) {

    int executable = open(argv[1], O_RDONLY);
    setLimits();

    if (!install_syscall_filter()) {
        return 1;
    }

    char *env[] = {NULL};

    fexecve( executable, argv+1, env);
}