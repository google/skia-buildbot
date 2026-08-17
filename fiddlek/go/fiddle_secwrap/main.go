//go:build unix

// fiddle_secwrap is a wrapper through which fiddles and their builds are
// executed. It blocks certain syscalls and traces others, ensuring that
// user-supplied code cannot access certain parts of the system.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"unsafe"

	"github.com/google/shlex"
	"go.skia.org/infra/go/skerr"
	"golang.org/x/sys/unix"
)

func setLimits() {
	var rLimit unix.Rlimit
	rLimit.Cur = 20
	rLimit.Max = 20
	if err := unix.Setrlimit(unix.RLIMIT_CPU, &rLimit); err != nil {
		fmt.Fprintf(os.Stderr, "setrlimit(RLIMIT_CPU): %v\n", err)
	}

	// Limit to 2G of Address space.
	rLimit.Cur = 2000000000
	rLimit.Max = 2000000000
	if err := unix.Setrlimit(unix.RLIMIT_AS, &rLimit); err != nil {
		fmt.Fprintf(os.Stderr, "setrlimit(RLIMIT_AS): %v\n", err)
	}
}

func doChild(args []string) error {
	// See https://man7.org/linux/man-pages/man2/ptrace.2.html for information
	// about ptrace.
	if _, _, err := unix.Syscall(unix.SYS_PTRACE, unix.PTRACE_TRACEME, 0, 0); err != 0 {
		return skerr.Wrapf(err, "ptrace TRACEME")
	}
	if err := unix.Kill(unix.Getpid(), unix.SIGSTOP); err != nil {
		return skerr.Wrap(err)
	}

	setLimits()

	filter := buildSeccompFilter()
	prog := &unix.SockFprog{
		Len:    uint16(len(filter)),
		Filter: &filter[0],
	}

	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return skerr.Wrapf(err, "prctl(NO_NEW_PRIVS)")
	}

	if _, _, err := unix.Syscall6(unix.SYS_PRCTL, unix.PR_SET_SECCOMP, unix.SECCOMP_MODE_FILTER, uintptr(unsafe.Pointer(prog)), 0, 0, 0); err != 0 {
		return skerr.Wrapf(err, "prctl(SECCOMP)")
	}

	if err := unix.Exec(args[0], args, os.Environ()); err != nil {
		_ = unix.Kill(unix.Getpid(), unix.SIGKILL)
		return skerr.Wrapf(err, "Couldn't run child")
	}
	return nil
}

// Flags.
var (
	buildMode bool
)

var execveAllowedBinaries = []string{
	"/usr/bin/ar",
	"/usr/bin/clang",
	"/usr/bin/clang++",
	"/usr/bin/ld",
	"/usr/bin/ninja",
	"/usr/bin/nm",
	"/usr/lib/llvm-19/bin/clang",
	// The below is run by ninja, so it needs to be included here. If you add
	// any more shells, be sure to keep them in sync with isShellCommand.
	"/bin/sh",
}

func isShellCommand(name string) bool {
	return name == "/bin/sh" || name == "sh"
}

var (
	mkdirAllowedPrefixes    = []string{"/tmp", "/var/cache/fontconfig", "/home/skia/.cache/fontconfig"}
	unlinkAllowedPrefixes   = []string{"/tmp", "/home/skia/.cache/fontconfig"}
	writingAllowedPrefixes  = []string{"/tmp/", "/var/cache/fontconfig", "/home/skia/.cache/fontconfig", "/dev/null"}
	linkAllowedPrefixes     = []string{"/tmp/"}
	mknodAllowedPrefixes    = []string{"/tmp/"}
	renameAllowedPrefixes   = []string{"/tmp/"}
	readonlyAllowedPrefixes = []string{
		"/dev/null",
		"/dev/urandom",
		"/etc/arch-release",
		"/etc/debian_version",
		"/etc/env.d/",
		"/etc/fiddle/",
		"/etc/fonts",
		"/etc/gentoo-release",
		"/etc/glvnd/",
		"/etc/ld.so.cache",
		"/etc/ld.so.conf.d/",
		"/etc/ld.so.conf",
		"/etc/lsb-release",
		"/etc/os-release",
		"/etc/redhat-release",
		"/home/skia/.cache/fontconfig",
		"/lib/",
		"/mnt/pd0/",
		"/proc/self/cgroup",
		"/proc/self/cmdline",
		"/proc/self/exe",
		"/proc/self/maps",
		"/proc/self/mountinfo",
		"/proc/self/stat",
		"/proc/self/status",
		"/proc/self/task/",
		"/sys/fs/",
		"/sys/devices/system/cpu/",
		"/tmp",
		"/usr/etc/",
		"/usr/lib/",
		"/usr/lib32/",
		"/usr/local/lib",
		"/usr/local/share/fonts",
		"/usr/share/",
		"/var/cache/fontconfig",
		"skia.conf",
	}
)

var allowedCmdRegex = regexp.MustCompile(`^[a-zA-Z0-9\s\-\.\/_=\+:,*\?\@\"'\$]+$`)

func isAllowedExec(name string, allowedExec string, args, envp []string, buildMode bool) bool {
	if name == allowedExec {
		return true
	}
	if !buildMode {
		fmt.Fprintf(os.Stderr, "not in build mode but exec is %s\n", name)
		return false
	}
	isAllowedBinary := false
	for _, allowed := range execveAllowedBinaries {
		if allowed == name || filepath.Base(allowed) == name {
			isAllowedBinary = true
			break
		}
	}
	if !isAllowedBinary {
		fmt.Fprintf(os.Stderr, "%s is not in %v\n", name, execveAllowedBinaries)
		return false
	}
	if isShellCommand(name) {
		if len(args) != 3 || args[1] != "-c" {
			fmt.Fprintf(os.Stderr, "invalid shell command: %v\n", args)
			return false
		}
		cmd := args[2]
		cmdArgs, err := shlex.Split(cmd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed parsing command: %s\n", err)
			return false
		}
		if len(cmdArgs) == 0 {
			fmt.Fprintf(os.Stderr, "no args for shell command: %v\n", args)
			return false
		}
		name := cmdArgs[0]

		// Disallow nested shells.
		if isShellCommand(name) {
			fmt.Fprintf(os.Stderr, "disallowing nested shell command: %v\n", args)
			return false
		}
		return isAllowedExec(name, allowedExec, cmdArgs, envp, buildMode)
	}

	// Prevent shell injection, redirection, piping, command substitution, etc.
	fullCmd := strings.Join(args, " ")
	if !allowedCmdRegex.MatchString(fullCmd) {
		fmt.Fprintf(os.Stderr, "invalid characters in command: %s\n", fullCmd)
		return false
	}

	// Inspect for dangerous compiler flags that could allow arbitrary code
	// execution.
	if strings.Contains(fullCmd, "-fplugin=") ||
		strings.Contains(fullCmd, "-Xclang") ||
		strings.Contains(fullCmd, "-load") ||
		strings.Contains(fullCmd, "-specs=") {
		fmt.Fprintf(os.Stderr, "command contains illegal flags: %s\n", fullCmd)
		return false
	}

	// Prevent attackers from using LD_PRELOAD, LD_LIBRARY_PATH, or other
	// dangerous variables to hijack the process.
	for _, env := range envp {
		if strings.HasPrefix(env, "LD_") ||
			strings.HasPrefix(env, "DYLD_") ||
			strings.HasPrefix(env, "PYTHON") ||
			strings.HasPrefix(env, "PERL") ||
			strings.HasPrefix(env, "RUBY") ||
			strings.HasPrefix(env, "NODE_") ||
			strings.HasPrefix(env, "BASH_FUNC_") ||
			strings.Contains(env, "IFS=") {
			fmt.Fprintf(os.Stderr, "command has illegal environment variable: %s\n", env)
			return false
		}
	}

	return true
}

// testAgainstPrefixes verifies that the given name begins with one of the given
// prefixes. If the name is a relative path, it is resolved against the child's
// current working directory. Exits the process with a non-zero code if the
// name is invalid.
func testAgainstPrefixes(child int, caller string, name string, prefixes []string) {
	var normalized string
	if !strings.HasPrefix(name, "/") {
		cwd, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", child))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read CWD of child %d: %v\n", child, err)
			childFail(child, "Failed to read CWD.")
		}
		normalized = filepath.Join(cwd, name)
	} else {
		normalized = filepath.Clean(name)
	}

	okay := false
	for _, prefix := range prefixes {
		if strings.HasPrefix(normalized, prefix) {
			okay = true
			break
		}
	}
	if !okay {
		fmt.Fprintf(os.Stderr, "Invalid filename: %s (normalized: %s)\n", name, normalized)
		fmt.Fprintf(os.Stderr, "%s: %s\n", caller, name)
		childFail(child, "Invalid filename.")
	}
}

// childFail logs an error, kills the subprocess, and exits with code -1.
func childFail(child int, message string) {
	fmt.Fprintf(os.Stderr, "%s\n", message)
	if err := unix.Kill(child, unix.SIGKILL); err != nil {
		fmt.Fprintf(os.Stderr, "failed to kill subprocess: %s\n", err)
	}
	os.Exit(-1)
}

// readString copies a string out of the child's address space.
func readString(pid int, addr uint64) string {
	var buf bytes.Buffer

	// Read one 64-bit register's worth of bytes at a time.
	data := make([]byte, 8)
	readBytes := 0
	const maxReadBytes = 128 * 1024 // 128 KB limit
	for {
		if readBytes >= maxReadBytes {
			childFail(pid, fmt.Sprintf("readString hit maxReadBytes (%d)", maxReadBytes))
		}
		n, err := unix.PtracePeekData(pid, uintptr(addr), data)
		if err != nil || n == 0 {
			break
		}
		idx := bytes.IndexByte(data[:n], 0)
		if idx >= 0 {
			buf.Write(data[:idx])
			break
		}
		buf.Write(data[:n])
		addr += 8
		readBytes += n
	}
	return buf.String()
}

// readStringArray reads a null-terminated array of string pointers from the child's address space.
func readStringArray(pid int, addr uint64) []string {
	var strs []string

	// Read one 64-bit register's worth of bytes at a time.
	ptrData := make([]byte, 8)
	const maxElements = 4096 // Limit array size to prevent infinite loops
	for i := 0; ; i++ {
		if i == maxElements {
			childFail(pid, fmt.Sprintf("readStringArray hit maxElements (%d)", maxElements))
		}
		n, err := unix.PtracePeekData(pid, uintptr(addr), ptrData)
		if err != nil || n != 8 {
			break
		}
		strAddr := binary.LittleEndian.Uint64(ptrData)
		if strAddr == 0 {
			break
		}
		strs = append(strs, readString(pid, strAddr))
		addr += 8
	}
	return strs
}

// buildSeccompFilter builds a set of filters used to allow, deny, or trace
// (in order to make the decision based on their properties) syscalls.
// See the following for more detail:
//
// https://outflux.net/teach-seccomp/
// A tutorial on how to use seccomp. This repository formerly had a stripped-
// down copy of seccomp-bpf.h from this tutorial. That has been folded into
// this function.
//
// https://www.kernel.org/doc/Documentation/networking/filter.txt
// Detailed documentation of the features used here, including the BPF
// instruction set.
//
// https://man7.org/linux/man-pages/man2/seccomp.2.html
// Man page for seccomp itself.
//
// https://datatracker.ietf.org/doc/rfc9669/
// A detailed description of the BPF Instruction Set Architecture.
func buildSeccompFilter() []unix.SockFilter {
	// Offset for seccomp_data struct
	const syscallNr = 0
	const archNr = 4

	// BPF instruction helper macros
	stmt := func(code uint16, k uint32) unix.SockFilter {
		return unix.SockFilter{Code: code, K: k}
	}
	jump := func(code uint16, k uint32, jt uint8, jf uint8) unix.SockFilter {
		return unix.SockFilter{Code: code, K: k, Jt: jt, Jf: jf}
	}

	allowSyscall := func(syscallNum uint32) []unix.SockFilter {
		return []unix.SockFilter{
			jump(unix.BPF_JMP|unix.BPF_JEQ|unix.BPF_K, syscallNum, 0, 1),
			stmt(unix.BPF_RET|unix.BPF_K, unix.SECCOMP_RET_ALLOW),
		}
	}

	traceSyscall := func(syscallNum uint32) []unix.SockFilter {
		return []unix.SockFilter{
			jump(unix.BPF_JMP|unix.BPF_JEQ|unix.BPF_K, syscallNum, 0, 1),
			stmt(unix.BPF_RET|unix.BPF_K, unix.SECCOMP_RET_TRACE),
		}
	}

	filter := []unix.SockFilter{
		// VALIDATE_ARCHITECTURE
		stmt(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS, archNr),
		jump(unix.BPF_JMP|unix.BPF_JEQ|unix.BPF_K, unix.AUDIT_ARCH_X86_64, 1, 0),
		stmt(unix.BPF_RET|unix.BPF_K, unix.SECCOMP_RET_KILL_PROCESS),

		// EXAMINE_SYSCALL
		stmt(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS, syscallNr),
	}

	allowedSyscalls := []uint32{
		unix.SYS_ACCESS,
		unix.SYS_ARCH_PRCTL,
		unix.SYS_BRK,
		unix.SYS_CHDIR,
		unix.SYS_CHMOD,
		unix.SYS_CHOWN,
		unix.SYS_CLOCK_GETTIME,
		unix.SYS_CLONE,
		unix.SYS_CLONE3,
		unix.SYS_CLOSE,
		unix.SYS_DUP,
		unix.SYS_DUP2,
		unix.SYS_EXIT_GROUP,
		unix.SYS_EXIT,
		unix.SYS_FACCESSAT,
		unix.SYS_FACCESSAT2,
		unix.SYS_FADVISE64,
		unix.SYS_FCHDIR,
		unix.SYS_FCNTL,
		unix.SYS_FSTAT,
		unix.SYS_FSTATFS,
		unix.SYS_FTRUNCATE,
		unix.SYS_FUTEX,
		unix.SYS_GETCWD,
		unix.SYS_GETDENTS,
		unix.SYS_GETDENTS64,
		unix.SYS_GETEGID,
		unix.SYS_GETEUID,
		unix.SYS_GETGID,
		unix.SYS_GETPGRP,
		unix.SYS_GETPID,
		unix.SYS_GETPPID,
		unix.SYS_GETRANDOM,
		unix.SYS_GETRESGID,
		unix.SYS_GETRESUID,
		unix.SYS_GETRLIMIT,
		unix.SYS_GETRUSAGE,
		unix.SYS_GETTID,
		unix.SYS_GETUID,
		unix.SYS_IOCTL,
		unix.SYS_LSEEK,
		unix.SYS_LSTAT,
		unix.SYS_MADVISE,
		unix.SYS_MMAP,
		unix.SYS_MPROTECT,
		unix.SYS_MREMAP,
		unix.SYS_MUNMAP,
		unix.SYS_NEWFSTATAT,
		unix.SYS_PIPE,
		unix.SYS_PIPE2,
		unix.SYS_PPOLL,
		unix.SYS_PREAD64,
		unix.SYS_PRLIMIT64,
		unix.SYS_PWRITE64,
		unix.SYS_READ,
		unix.SYS_READLINK,
		unix.SYS_RSEQ,
		unix.SYS_RT_SIGACTION,
		unix.SYS_RT_SIGPENDING,
		unix.SYS_RT_SIGPROCMASK,
		unix.SYS_RT_SIGRETURN,
		unix.SYS_RT_SIGSUSPEND,
		unix.SYS_SCHED_GETAFFINITY,
		unix.SYS_SCHED_YIELD,
		unix.SYS_SET_ROBUST_LIST,
		unix.SYS_SET_TID_ADDRESS,
		unix.SYS_SETPGID,
		unix.SYS_SHMCTL,
		unix.SYS_SIGALTSTACK,
		unix.SYS_STAT,
		unix.SYS_STATFS,
		unix.SYS_STATX,
		unix.SYS_SYSINFO,
		unix.SYS_TGKILL,
		unix.SYS_UMASK,
		unix.SYS_UNAME,
		unix.SYS_VFORK,
		unix.SYS_WAIT4,
		unix.SYS_WAITID,
		unix.SYS_WRITE,
	}
	for _, nr := range allowedSyscalls {
		filter = append(filter, allowSyscall(nr)...)
	}

	tracedSyscalls := []uint32{
		unix.SYS_EXECVE,
		unix.SYS_LINK,
		unix.SYS_LINKAT,
		unix.SYS_MKDIR,
		unix.SYS_MKDIRAT,
		unix.SYS_MKNOD,
		unix.SYS_MKNODAT,
		unix.SYS_OPEN,
		unix.SYS_OPENAT,
		unix.SYS_OPENAT2,
		unix.SYS_READLINK,
		unix.SYS_READLINKAT,
		unix.SYS_RENAME,
		unix.SYS_RENAMEAT,
		unix.SYS_RENAMEAT2,
		unix.SYS_RMDIR,
		unix.SYS_UNLINK,
		unix.SYS_UNLINKAT,
	}
	for _, nr := range tracedSyscalls {
		filter = append(filter, traceSyscall(nr)...)
	}

	// Default to KILL_PROCESS for production.
	filter = append(filter, stmt(unix.BPF_RET|unix.BPF_K, unix.SECCOMP_RET_KILL_PROCESS))

	return filter
}

func doTrace(child int, allowedExec string) int {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var status unix.WaitStatus
	_, err := unix.Wait4(child, &status, 0, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Initial wait4 failed: %v\n", err)
		return -1
	}

	options := unix.PTRACE_O_TRACEEXEC |
		unix.PTRACE_O_TRACESECCOMP |
		unix.PTRACE_O_TRACECLONE |
		unix.PTRACE_O_TRACEFORK |
		unix.PTRACE_O_TRACEVFORK |
		unix.PTRACE_O_EXITKILL
	err = unix.PtraceSetOptions(child, options)
	if err != nil {
		fmt.Fprintf(os.Stderr, "PtraceSetOptions failed: %v\n", err)
		return -1
	}

	err = unix.PtraceSyscall(child, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "PtraceSyscall failed: %v\n", err)
		return -1
	}

	syscallEntry := make(map[int]bool)

	for {
		wpid, err := unix.Wait4(-1, &status, unix.WALL, nil)
		if err != nil {
			if err == unix.ECHILD {
				break
			}
			continue
		}

		if status.Exited() {
			delete(syscallEntry, wpid)
			if wpid == child {
				return status.ExitStatus()
			}
			continue
		}

		if status.Signaled() {
			delete(syscallEntry, wpid)
			if wpid == child {
				return 1
			}
			continue
		}

		if status.Stopped() {
			sig := status.StopSignal()
			if sig == unix.SIGTRAP {
				event := uint32(status) >> 16
				if event == unix.PTRACE_EVENT_SECCOMP {
					var regs unix.PtraceRegs
					if err := unix.PtraceGetRegs(wpid, &regs); err != nil {
						fmt.Fprintf(os.Stderr, "The child %d failed to get regs: %v\n", wpid, err)
						continue
					}

					syscallNum := regs.Orig_rax
					// Uncomment for very verbose logging of all traced syscalls.
					// fmt.Fprintf(os.Stderr, "Traced syscall: %d\n", syscallNum)

					switch syscallNum {
					case unix.SYS_EXECVE:
						name := readString(wpid, regs.Rdi)
						args := readStringArray(wpid, regs.Rsi)
						envp := readStringArray(wpid, regs.Rdx)
						if !isAllowedExec(name, allowedExec, args, envp, buildMode) {
							fmt.Fprintf(os.Stderr, "Invalid exec: %v\n", args)
							childFail(wpid, "Invalid exec.")
						}
					case unix.SYS_OPEN:
						name := readString(wpid, regs.Rdi)
						flags := regs.Rsi
						prefixes := readonlyAllowedPrefixes
						if (flags & unix.O_ACCMODE) != unix.O_RDONLY {
							prefixes = writingAllowedPrefixes
						}
						testAgainstPrefixes(wpid, "open", name, prefixes)
					case unix.SYS_OPENAT, unix.SYS_OPENAT2:
						name := readString(wpid, regs.Rsi)
						flags := regs.Rdx
						prefixes := readonlyAllowedPrefixes
						if (flags & unix.O_ACCMODE) != unix.O_RDONLY {
							prefixes = writingAllowedPrefixes
						}
						testAgainstPrefixes(wpid, "openat", name, prefixes)
					case unix.SYS_MKDIR:
						name := readString(wpid, regs.Rdi)
						testAgainstPrefixes(wpid, "mkdir", name, mkdirAllowedPrefixes)
					case unix.SYS_MKDIRAT:
						name := readString(wpid, regs.Rsi)
						testAgainstPrefixes(wpid, "mkdirat", name, mkdirAllowedPrefixes)
					case unix.SYS_RMDIR:
						name := readString(wpid, regs.Rdi)
						testAgainstPrefixes(wpid, "rmdir", name, mkdirAllowedPrefixes)
					case unix.SYS_UNLINK:
						name := readString(wpid, regs.Rdi)
						testAgainstPrefixes(wpid, "unlink", name, unlinkAllowedPrefixes)
					case unix.SYS_UNLINKAT:
						name := readString(wpid, regs.Rsi)
						testAgainstPrefixes(wpid, "unlinkat", name, unlinkAllowedPrefixes)
					case unix.SYS_MKNOD:
						name := readString(wpid, regs.Rdi)
						testAgainstPrefixes(wpid, "mknod", name, mknodAllowedPrefixes)
					case unix.SYS_MKNODAT:
						name := readString(wpid, regs.Rsi)
						testAgainstPrefixes(wpid, "mknodat", name, mknodAllowedPrefixes)
					case unix.SYS_LINK:
						name := readString(wpid, regs.Rdi)
						testAgainstPrefixes(wpid, "link", name, linkAllowedPrefixes)
						name2 := readString(wpid, regs.Rsi)
						testAgainstPrefixes(wpid, "link", name2, linkAllowedPrefixes)
					case unix.SYS_LINKAT:
						name := readString(wpid, regs.Rsi)
						testAgainstPrefixes(wpid, "linkat", name, linkAllowedPrefixes)
						name2 := readString(wpid, regs.R10)
						testAgainstPrefixes(wpid, "linkat", name2, linkAllowedPrefixes)
					case unix.SYS_RENAME:
						name := readString(wpid, regs.Rdi)
						testAgainstPrefixes(wpid, "rename", name, renameAllowedPrefixes)
						name2 := readString(wpid, regs.Rsi)
						testAgainstPrefixes(wpid, "rename", name2, renameAllowedPrefixes)
					case unix.SYS_RENAMEAT, unix.SYS_RENAMEAT2:
						name := readString(wpid, regs.Rsi)
						testAgainstPrefixes(wpid, "renameat*", name, renameAllowedPrefixes)
						name2 := readString(wpid, regs.R10)
						testAgainstPrefixes(wpid, "renameat*", name2, renameAllowedPrefixes)
					case unix.SYS_READLINK:
						name := readString(wpid, regs.Rdi)
						testAgainstPrefixes(wpid, "readlink", name, readonlyAllowedPrefixes)
					case unix.SYS_READLINKAT:
						name := readString(wpid, regs.Rsi)
						testAgainstPrefixes(wpid, "readlinkat", name, readonlyAllowedPrefixes)
					default:
						// childFail will kill the process
						childFail(wpid, fmt.Sprintf("Untracked system call: %d", syscallNum))
					}
				} else if event == 0 {
					// Traditional ptrace syscall trap
					var regs unix.PtraceRegs
					if err := unix.PtraceGetRegs(wpid, &regs); err == nil {
						if !syscallEntry[wpid] {
							syscallEntry[wpid] = true
						} else {
							syscallEntry[wpid] = false
						}
					}
				}
			}
		}

		// If the child process received a signal other than SIGTRAP (which
		// ptrace itself uses to pause the process), forward that signal to
		// the child process.
		var signal int
		if status.Stopped() && status.StopSignal() != unix.SIGTRAP {
			signal = int(status.StopSignal())
		}
		if err := unix.PtraceSyscall(wpid, signal); err != nil && err != unix.ESRCH {
			fmt.Fprintf(os.Stderr, "failed ptrace: %s\n", err)
		}
	}
	return 0
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--child" {
		if err := doChild(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(-1)
		}
		return
	}

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: secwrap [--build] <allowed_exec> [checkout_path] <args...>\n")
		os.Exit(-1)
	}

	args := os.Args[1:]
	if args[0] == "--build" {
		buildMode = true
		args = args[1:]
	}

	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: secwrap [--build] <allowed_exec> [checkout_path] <args...>\n")
		os.Exit(-1)
	}

	allowedExec := args[0]
	args = args[1:]

	var checkout string
	if len(args) > 0 && strings.HasPrefix(args[0], "/") {
		checkout = args[0]
		if !strings.HasSuffix(checkout, "/") {
			checkout += "/"
		}
		writingAllowedPrefixes = append(writingAllowedPrefixes, checkout)
		linkAllowedPrefixes = append(linkAllowedPrefixes, checkout)
		unlinkAllowedPrefixes = append(unlinkAllowedPrefixes, checkout)
		renameAllowedPrefixes = append(renameAllowedPrefixes, checkout)
		readonlyAllowedPrefixes = append(readonlyAllowedPrefixes, checkout)
		args = args[1:]
	}

	writingAllowedPrefixes = append(writingAllowedPrefixes, "/dev/null")
	readonlyAllowedPrefixes = append(
		readonlyAllowedPrefixes,
		"/dev/null",
		"/dev/urandom",
		"/usr/bin/",
		"/usr/include/",
		"/usr/local/include/",
		"/bin/",
		"/lib64/",
		"/usr/lib64/",
	)

	cmd := exec.Command(os.Args[0], append([]string{"--child"}, args...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start child: %v\n", err)
		os.Exit(-1)
	}

	exitCode := doTrace(cmd.Process.Pid, allowedExec)
	os.Exit(exitCode)
}
