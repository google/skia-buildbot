#include <unistd.h>
#include <fcntl.h>
#include <sys/time.h>
#include <sys/resource.h>
#include <sys/ptrace.h>
#include <sys/syscall.h>
#include <sys/user.h>
#include <sys/types.h>
#include <sys/wait.h>

#include "seccomp_bpf.h"

#include <iostream>

using namespace std;

static bool install_syscall_filter() {
    struct sock_filter filter[] = {
        VALIDATE_ARCHITECTURE,
        /* Grab the system call number. */
        EXAMINE_SYSCALL,
        /* List allowed syscalls. Look up via ausyscall. */
        ALLOW_SYSCALL(exit_group),
        ALLOW_SYSCALL(exit),
        ALLOW_SYSCALL(stat),
        ALLOW_SYSCALL(fstat),
        ALLOW_SYSCALL(read),
        ALLOW_SYSCALL(write),
        ALLOW_SYSCALL(getdents),
        ALLOW_SYSCALL(close),
        ALLOW_SYSCALL(mmap),
        ALLOW_SYSCALL(mprotect),
        ALLOW_SYSCALL(munmap),
        ALLOW_SYSCALL(brk),
        ALLOW_SYSCALL(futex),
        ALLOW_SYSCALL(lseek),
        ALLOW_SYSCALL(set_tid_address),
        ALLOW_SYSCALL(set_robust_list),
        ALLOW_SYSCALL(rt_sigaction),
        ALLOW_SYSCALL(rt_sigprocmask),
        ALLOW_SYSCALL(getrlimit),
        ALLOW_SYSCALL(arch_prctl),
        ALLOW_SYSCALL(access),
        ALLOW_SYSCALL(fstatfs),
        ALLOW_SYSCALL(readlink),
        ALLOW_SYSCALL(fadvise64),
        ALLOW_SYSCALL(clock_gettime),
        ALLOW_SYSCALL(sysinfo),

        ALLOW_SYSCALL(getuid),
        ALLOW_SYSCALL(geteuid),
        ALLOW_SYSCALL(getgid),
        ALLOW_SYSCALL(getegid),

        ALLOW_SYSCALL(fcntl),

        ALLOW_SYSCALL(mremap),
        ALLOW_SYSCALL(statfs),
        ALLOW_SYSCALL(readlink),
        ALLOW_SYSCALL(getpid),
        ALLOW_SYSCALL(gettid),
        ALLOW_SYSCALL(tgkill),

        ALLOW_SYSCALL(ftruncate),
        ALLOW_SYSCALL(ioctl),
        ALLOW_SYSCALL(sched_yield),

        ALLOW_SYSCALL(clone),
        ALLOW_SYSCALL(wait4),
        ALLOW_SYSCALL(getrandom),
        ALLOW_SYSCALL(shmctl),
        ALLOW_SYSCALL(prlimit64),
        ALLOW_SYSCALL(dup),
        ALLOW_SYSCALL(chmod),
        ALLOW_SYSCALL(chown),
        ALLOW_SYSCALL(newfstatat),
        ALLOW_SYSCALL(pread64),
        ALLOW_SYSCALL(getdents64),

        TRACE_SYSCALL(mknod),
        TRACE_SYSCALL(link),
        TRACE_SYSCALL(rename),
        TRACE_SYSCALL(execve),
        TRACE_SYSCALL(mkdir),
        TRACE_SYSCALL(unlink),
        TRACE_SYSCALL(open),
        TRACE_SYSCALL(openat),

        // Uncomment the following when trying to figure out which new
        // syscall's are being made:

        // TRACE_ALL,
        // ALLOW_ALL,
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

     // Limit to 20 seconds of CPU.
     n.rlim_cur = 20;
     n.rlim_max = 20;
     if (setrlimit(RLIMIT_CPU, &n)) {
         perror("setrlimit(RLIMIT_CPU)");
     }

     // Limit to 1G of Address space.
     n.rlim_cur = 1000000000;
     n.rlim_max = 1000000000;
     if (setrlimit(RLIMIT_AS, &n)) {
         perror("setrlimit(RLIMIT_AS)");
     }
 }


int do_child(int argc, char **argv) {

    char *args[argc+1];

    memcpy(args, argv, argc * sizeof(char *));
    args[argc] = NULL;

    if (ptrace(PTRACE_TRACEME, 0, 0, 0)) {
        perror("ptrace");
        exit(-1);
    }
    kill(getpid(), SIGSTOP);

    setLimits();
    if (!install_syscall_filter()) {
        perror("Failed to install syscall filter");
        return -1;
    }

    (void)execvp(args[0], args);
    // if execvp returns, we couldn't run the child.  Probably
    // because the compile failed.  Let's kill ourselves so the
    // parent sees the signal and exits appropriately.
    perror("Couldn't run child.");
    kill(getpid(), SIGKILL);
    return -1;
}

// read_string copies a null-terminated string out
// of the child's address space, one character at a time.
// It allocates memory and returns it to the caller;
// it is the caller's responsibility to free it.
char *read_string(pid_t child, unsigned long addr) {
#define INITIAL_ALLOCATION 4096
    char *val = (char *) malloc(INITIAL_ALLOCATION);
    size_t allocated = INITIAL_ALLOCATION;
    size_t read = 0;
    unsigned long tmp;

    while (1) {
        if (read + sizeof tmp > allocated) {
            allocated *= 2;
            val = (char *) realloc(val, allocated);
        }

        tmp = ptrace(PTRACE_PEEKDATA, child, addr + read);
        if (errno != 0) {
            val[read] = 0;
            break;
        }
        memcpy(val + read, &tmp, sizeof tmp);
        if (memchr(&tmp, 0, sizeof tmp) != NULL) {
            break;
        }
        read += sizeof tmp;
    }
    return val;
}

void child_fail(pid_t child, const char* message) {
    perror(message);
    kill(child, SIGKILL);
    exit(-1);
}

const char *mkdir_allowed_prefixes[] = {
    "/tmp",
    "/var/cache/fontconfig",
    NULL,
};

const char *unlink_allowed_prefixes[] = {
    "/tmp",
    NULL,
};

const char *writing_allowed_prefixes[] = {
    "/tmp/",
    "/var/cache/fontconfig", // This dir is readonly in the container, so this is OK.
    NULL,
};

const char *link_allowed_prefixes[] = {
    "/tmp/",
    NULL,
};

const char *mknod_allowed_prefixes[] = {
    "/tmp/",
    NULL,
};

const char *rename_allowed_prefixes[] = {
    "/tmp/",
    NULL,
};

const char *readonly_allowed_prefixes[] = {
    "",
    "/etc/fonts",
    "/etc/fiddle/",
    "/etc/glvnd/",
    "/etc/ld.so.cache",
    "/lib/",
    "/mnt/pd0/",
    "/tmp/",
    "/usr/lib/",
    "/usr/local/share/fonts",
    "/usr/local/lib",
    "/usr/share/",
    "/sys/devices/",
    "/var/cache/fontconfig",
    "skia.conf",
    NULL,
};

void test_against_prefixes(pid_t child, const char * caller, char* name, const char** prefixes) {
    if (NULL != strstr(name, "../")) {
        perror(caller);
        perror(name);
        child_fail(child, "No relative paths...");
    }
    bool okay = false;
    for (; *prefixes != NULL; prefixes++) {
        if (!strncmp(*prefixes, name, strlen(*prefixes))) {
            okay = true;
            break;
        }
    }
    if (!okay) {
        perror(name);
        perror(caller);
        child_fail(child, "Invalid filename.");
    }
}

/*
 * The first six integer or pointer arguments are passed in registers RDI,
 * RSI, RDX, RCX (R10 in the Linux kernel interface), R8, and R9,
 * while XMM0, XMM1, XMM2, XMM3, XMM4, XMM5, XMM6 and XMM7 are used for
 * certain floating point arguments.
 */
int do_trace(pid_t child, char *allowed_exec) {
    int status;
    waitpid(child, &status, 0);
    ptrace(PTRACE_SETOPTIONS, child, 0, PTRACE_O_TRACEEXEC | PTRACE_O_TRACESECCOMP);
    ptrace(PTRACE_CONT, child, 0, 0);

    while(1) {
        waitpid(child, &status, 0);
        if (WIFEXITED(status)) {
            return 0;
        }
        if (WIFSIGNALED(status)) {
            cerr << "Signal: "  << WTERMSIG(status) << endl;
            perror("WIFSIGNALED");
            return 1;
        }

        if (status>>8 == (SIGTRAP | (PTRACE_EVENT_SECCOMP<<8))) {
            struct user_regs_struct regs;
            if(ptrace(PTRACE_GETREGS, child, NULL, &regs)) {
                  perror("The child failed...");
                  exit(-1);
            }

            int syscall = regs.orig_rax;
            if (syscall == SYS_execve) {
                char *name = read_string(child, regs.rdi);
                if (strcmp(name, allowed_exec)) {
                    child_fail(child, "Invalid exec.");
                }
                free(name);
            } else if (syscall == SYS_open) {
                char *name = read_string(child, regs.rdi);
                const char **prefixes = readonly_allowed_prefixes;
                int flags = regs.rsi;
                if (O_RDONLY != (flags & O_ACCMODE)) {
                    prefixes = writing_allowed_prefixes;
                }
                test_against_prefixes(child, "open", name, prefixes);
                free(name);
            } else if (syscall == SYS_openat) {
                char *name = read_string(child, regs.rsi);
                int flags = regs.rdx;
                const char **prefixes = readonly_allowed_prefixes;
                if (O_RDONLY != (flags & O_ACCMODE)) {
                    prefixes = writing_allowed_prefixes;
                }
                test_against_prefixes(child, "openat", name, prefixes);
                free(name);
            } else if (syscall == SYS_mkdir) {
                char *name = read_string(child, regs.rdi);
                test_against_prefixes(child, "mkdir", name, mkdir_allowed_prefixes);
                free(name);
            } else if (syscall == SYS_unlink) {
                char *name = read_string(child, regs.rdi);
                test_against_prefixes(child, "unlink", name, unlink_allowed_prefixes);
                free(name);
            } else if (syscall == SYS_mknod) {
                char *name = read_string(child, regs.rdi);
                test_against_prefixes(child, "mknod", name, mknod_allowed_prefixes);
                free(name);
            } else if (syscall == SYS_link) {
                char *name = read_string(child, regs.rdi);
                test_against_prefixes(child, "link", name, link_allowed_prefixes);
                free(name);
                name = read_string(child, regs.rsi);
                test_against_prefixes(child, "link", name, link_allowed_prefixes);
                free(name);
            } else if (syscall == SYS_rename) {
                char *name = read_string(child, regs.rdi);
                test_against_prefixes(child, "rename", name, rename_allowed_prefixes);
                free(name);
                name = read_string(child, regs.rsi);
                test_against_prefixes(child, "rename", name, rename_allowed_prefixes);
                free(name);
            } else {
                // this should never happen, but if we're in TRACE_ALL
                // mode for debugging, this lets me print out what system
                // calls are happening unexpectedly.
                cout << "WEIRD SYSTEM CALL: " << syscall << endl;
                child_fail(child, "Invalid system call.");
            }
        }
        ptrace(PTRACE_CONT, child, 0, 0);
    }
    return 0;
}

int main(int argc, char** argv) {
    pid_t child = fork();

    if (child == 0) {
        return do_child(argc-1, argv+1);
    } else {
        return do_trace(child, argv[1]);
    }
}
