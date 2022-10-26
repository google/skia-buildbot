// Package executil provides a mostly transparent way to make os/exec testable. It is inspired by
// https://npf.io/2015/06/testing-exec-command/ (which was inspired by the standard library's tests
// of os/exec). Basically, the helpers in this package replace a call to an arbitrary executable
// (and arguments) with a call to the underlying test binary, with a flag to run exactly one test.
// This test can then be a fake implementation of the binary, do assertions on the arguments, etc.
//
// See executil_test.go for example usages.
package executil

import (
	"context"
	"os"
	"os/exec"
	"sync"
)

const (
	// OverrideEnvironmentVariable is the environment variable that will be set if a test has been
	// invoked via CommandContext below and it should behave as if it is faking a call to an
	// executable. The value it is set to should be considered arbitrary and not relied upon.
	OverrideEnvironmentVariable = "SKIA_INFRA_OVERRIDE_TEST"

	// This is the key used in context.Value to correspond to a *fakeTestTracker object.
	overrideKey = "skia_infra_override_cmd"
)

// WithFakeTests returns a context.Context loaded with a special Value containing the given test
// names. When this Context is passed into this package's CommandContext, faked *exec.Cmd objects
// will be returned using the given test names. The first call to CommandContext will be faked
// with the first value of fakeTestNames, the second call to CommandContext will use the
// second value of fakeTestNames and so on. This panics if the provided context was one that
// already has fake tests associated with it.
func WithFakeTests(parent context.Context, fakeTestNames ...string) context.Context {
	if _, ok := parent.Value(overrideKey).(*fakeTestTracker); ok {
		panic("parent context already has fake tests associated with it")
	}
	return context.WithValue(parent, overrideKey, &fakeTestTracker{
		index:         0,
		fakeTestNames: fakeTestNames,
	})
}

// FakeTestsContext is a convenient wrapper around WithFakeTests using context.Background().
func FakeTestsContext(fakeTestNames ...string) context.Context {
	return WithFakeTests(context.Background(), fakeTestNames...)
}

// fakeTestTracker keeps track of which test we should fake out next. We have this be a struct and
// store the pointer to this struct in the ctx.Value so we can mutate the value without having
// to return a new context or something more complex. Contexts are meant to be thread safe, so this
// object has a mutex to avoid problems when being used synchronously, although in practice using
// this package in a multi-threaded fashion would likely lead to flaky tests.
type fakeTestTracker struct {
	index         int
	fakeTestNames []string
	mutex         sync.Mutex
}

// CommandContext looks for a special value on the provided context.Context (see WithFakeTests).
// If that value exists, it will use the next fake test value and return a faked *exec.Cmd. It
// panics if there are not enough fake tests that were provided to the original context. If the
// special value does not exist, it is a passthrough to os/exec.CommandContext.
func CommandContext(ctx context.Context, cmd string, args ...string) *exec.Cmd {
	if override, ok := ctx.Value(overrideKey).(*fakeTestTracker); ok {
		override.mutex.Lock()
		defer override.mutex.Unlock()
		// We are going to shell out to the current test executable...
		testBinary := os.Args[0]
		// ...and tell it to run the next faked test.
		if override.index >= len(override.fakeTestNames) {
			panic("Not enough fake tests provided")
		}
		fakeTest := override.fakeTestNames[override.index]
		override.index++
		// fakeTest is where the client has put their fake implementation of the given command.
		argsWithOverride := []string{"-test.run=" + fakeTest, "--", cmd}
		argsWithOverride = append(argsWithOverride, args...)
		fakedCmd := exec.CommandContext(ctx, testBinary, argsWithOverride...)
		fakedCmd.Env = []string{OverrideEnvironmentVariable + "=1"}
		return fakedCmd
	}
	// Did not find special Context value, so fall back to default impl
	return exec.CommandContext(ctx, cmd, args...)
}

// FakeCommandsReturned returns the count of how many times CommandContext was called using the
// given context. This is a proxy for the number of fake commands run.
func FakeCommandsReturned(ctx context.Context) int {
	if override, ok := ctx.Value(overrideKey).(*fakeTestTracker); ok {
		override.mutex.Lock()
		defer override.mutex.Unlock()
		return override.index
	}
	panic("A Context was passed in that was not produced by the executil package.")
}

// OriginalArgs returns the original arguments passed into a test function. Concretely, it looks
// at the osArgs and strips off the first 3 (the test binary, the test to run, and "--")
func OriginalArgs() []string {
	return os.Args[3:]
}

// IsCallingFakeCommand returns whether the current process is a test process that's running a
// mocked-out CLI invocation. This should be called at the beginning of each Test_FakeExe_... test
// and trigger an early return if false.
func IsCallingFakeCommand() bool {
	return os.Getenv(OverrideEnvironmentVariable) != ""
}
