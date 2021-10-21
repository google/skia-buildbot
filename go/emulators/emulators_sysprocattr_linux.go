//go:build linux
// +build linux

package emulators

import "syscall"

// makeSysProcAttrWithPdeathsigSIGKILL returns a syscall.SysProcAttr struct such that any emulators
// started via an exec.Cmd will be killed as soon as the parent process (e.g. a Go test) dies.
//
// This function uses a Linux-only field of the syscall.SysProcAttr struct. Trying to compile this
// file for a non-Linux target will result in the following compilation error:
//
//     unknown field 'Pdeathsig' in struct literal of type syscall.SysProcAttr
//
// For this reason, this file is annotated with a "+build linux" build tag, which will exclude it
// from compilation for non-Linux targets. A separate, noop implementation for all other compilation
// targets is provided in a sibling file, so as not to break the build.
//
// See https://golang.org/cmd/go/#hdr-Build_constraints for more information on build tags.
func makeSysProcAttrWithPdeathsigSIGKILL() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		// Under Bazel and RBE, emulators are launched by each individual go_test Bazel target. The
		// below setting kills the emulator processes (and any other child processes) as soon as the
		// parent process (i.e. the test runner) dies.
		//
		// If we don't do this, the emulators will continue running indefinitely, and Bazel will
		// eventually time out while waiting for these child processes to die.
		//
		// This setting is Linux-only, but that's OK because our RBE instance consists of Linux
		// machines exclusively. Alternative approaches include adding a TestMain function to our
		// emulator tests that launches the emulators before running the test cases and kills them
		// afterwards, or leveraging the test_on_env Bazel macro to run an environment binary
		// alongside the tests which controls the emulators' lifecycle. Any of these approaches would
		// work on non-Linux OSes as well.
		Pdeathsig: syscall.SIGKILL,
	}
}
