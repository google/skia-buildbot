package main

import (
	"fmt"
	"log"

	sec "github.com/seccomp/libseccomp-golang"
)

func id(sc string) sec.ScmpSyscall {
	id, err := sec.GetSyscallFromName(sc)
	if err != nil {
		log.Fatalf("Name not found: %s", err)
	}
	return id
}

func addFilters() {
	fmt.Println("addFilters")
	// filter, _ := sec.NewFilter(sec.ActErrno.SetReturnCode(int16(syscall.EPERM)))
	filter, _ := sec.NewFilter(sec.ActTrace)

	allowed := []string{
		"access",
		"arch_prctl",
		"brk",
		"clone",
		"close",
		"execve",
		"exit_group",
		"fcntl",
		"fstat",
		"futex",
		"getrlimit",
		"gettid",
		"getpid",
		"mkdirat",
		"mmap",
		"mprotect",
		"munmap",
		"newfstatat",
		"open",
		"pipe2",
		"prctl",
		"read",
		"readlinkat",
		"rt_sigaction",
		"rt_sigreturn",
		"rt_sigprocmask",
		"sched_getaffinity",
		"set_robust_list",
		"set_tid_address",
		"sigaltstack",
		"wait4",
		"waitpid",
		"waitid",
		"write",
	}
	for _, name := range allowed {
		fmt.Println("addRule", name)
		filter.AddRule(id(name), sec.ActAllow)
	}

	if !filter.IsValid() {
		log.Fatal("Invalid filter.")
	}

	if err := filter.Load(); err != nil {
		log.Fatalf("Failed to load: %s", err)
	}
	fmt.Println("loaded")
}
