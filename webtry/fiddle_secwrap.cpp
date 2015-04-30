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
        /* List allowed syscalls. */
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
        TRACE_SYSCALL(execve),
        TRACE_OPENS_FOR_READS_ONLY(open, 1),
        TRACE_OPENS_FOR_READS_ONLY(openat, 2),
        // TRACE_ALL,
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


int do_child(int argc, char **argv) {

    char *args[argc+1];

    memcpy(args, argv, argc * sizeof(char *));
    args[argc] = NULL;

    if (ptrace(PTRACE_TRACEME, 0, 0, 0)) {
        perror("ptrace");
        exit(1);
    }
    kill(getpid(), SIGSTOP);

    setLimits();
    if (!install_syscall_filter()) {
        return 1;
    }

    return execvp(args[0], args);
}

// read_string copies a null-terminated string out
// of the child's address space, one character at a time.
// It allocates memory and returns it to the caller;
// it is the caller's responsibility to free it.
char *read_string(pid_t child, unsigned long addr) {
#define INTIAL_ALLOCATION 4096
    char *val = (char *) malloc(INTIAL_ALLOCATION);
    size_t allocated = INTIAL_ALLOCATION
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


int do_trace(pid_t child, char *allowed_exec) {
    int status;
    waitpid(child, &status, 0);
    ptrace(PTRACE_SETOPTIONS, child, 0, PTRACE_O_TRACEEXEC | PTRACE_O_TRACESECCOMP);
    ptrace(PTRACE_CONT, child, 0, 0);

#define CHILD_FAIL(message) \
    perror(message); \
    kill(child, SIGKILL); \
    exit(1)

    while(1) {
        waitpid(child, &status, 0);
        if (WIFEXITED(status)) {
            return 0;
        }
        if (WIFSIGNALED(status)) {
            return 1;
        }

        if (status>>8 == (SIGTRAP | (PTRACE_EVENT_SECCOMP<<8))) {
            struct user_regs_struct regs; 
            if(ptrace(PTRACE_GETREGS, child, NULL, &regs)) {
                  perror("The child failed...");
                  exit(1);
            }

            int syscall = regs.orig_rax;
            if (syscall == SYS_execve) {
                char *name = read_string( child, regs.rdi );
                if (strcmp(name, allowed_exec)) {
                    CHILD_FAIL( "Invalid exec." );
                }
                free(name);
            } else if (syscall == SYS_open) {
                char *name = read_string( child, regs.rdi );
                if (NULL != strstr(name, "..")) {
                    CHILD_FAIL( "No relative paths..." );
                }
                int flags = regs.rsi;
                if (O_RDONLY != (flags & O_ACCMODE)) {
                            CHILD_FAIL( "No writing to files..." );
                }
                const char *allowed_prefixes[] = { "/usr/share/fonts", "/etc/ld.so.cache", "/lib/", "/usr/lib/", "skia.conf" };
                bool okay = false;
                for (unsigned int i = 0 ; i < sizeof(allowed_prefixes) / sizeof(allowed_prefixes[0]) ; i++) {
                    if (!strncmp(allowed_prefixes[i], name, strlen(allowed_prefixes[i]))) {
                        okay = true;
                        break;
                    }
                }
                if (!okay) {
                    CHILD_FAIL( "Invalid open." );
                }
                free(name);
            } else if (syscall == SYS_openat) {
                char *name = read_string( child, regs.rsi );
                if (NULL != strstr(name, "..")) {
                    CHILD_FAIL( "No relative paths..." );
                }
                int flags = regs.rdx;
                if (O_RDONLY != (flags & O_ACCMODE)) {
                            CHILD_FAIL( "No writing to files..." );
                }
                if (strncmp(name, "/usr/share/fonts", strlen("/usr/share/fonts"))) {
                    CHILD_FAIL( "Invalid openat." );
                }
                free(name);
            } else {
                // this should never happen, but if we're in TRACE_ALL
                // mode for debugging, this lets me print out what system
                // calls are happening unexpectedly.
                cout << "WEIRD SYSTEM CALL: " << syscall << endl;
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
