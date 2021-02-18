// +build !linux

package emulators

import "syscall"

// makeSysProcAttrWithPdeathsigSIGKILL is a noop version of the Linux-only version of this function
// implemented in a sibling file. It always panics because it indicates that we're trying to run
// emulator tests under a non-Linux operating system, which is currently not supported.
func makeSysProcAttrWithPdeathsigSIGKILL() *syscall.SysProcAttr {
	panic("Running emulator tests on non-Linux operating systems is currently not supported.")
}
