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
)

const (
	// OverrideEnvironmentVariable is the environment variable that will be set if a test has been
	// invoked via CommandContext below and it should behave as if it is faking a call to an
	// executable. The value it is set to should be considered arbitrary and not relied upon.
	OverrideEnvironmentVariable = "SKIA_INFRA_OVERRIDE_TEST"

	// This is the key used in context.Value to correspond to a *fakeTests object.
	overrideKey = "skia_infra_override_cmd"
)

// TestContextFrom returns a context.Context loaded with a special Value containing the given test
// names. When this Context is passed into this package's CommandContext, faked *exec.Cmd objects
// will be returned using the given test names. The first call to CommandContext will be faked
// with the first value of testsToCallInstead, the second call to CommandContext will use the
// second value of testsToCallInstead and so on. This panics if the provided context was one that
// already has fake tests associated with it.
func TestContextFrom(parent context.Context, testsToCallInstead ...string) context.Context {
	if _, ok := parent.Value(overrideKey).(*fakeTests); ok {
		panic("parent context already has fake tests associated with it")
	}
	return context.WithValue(parent, overrideKey, &fakeTests{
		index:              0,
		testsToCallInstead: testsToCallInstead,
	})
}

// TestContext is a convenient wrapper around TestContextFrom using context.Background().
func TestContext(testsToCallInstead ...string) context.Context {
	return TestContextFrom(context.Background(), testsToCallInstead...)
}

// fakeTests keeps track of which test we should fake out next. We have this be a struct and
// store the pointer to this struct in the ctx.Value so we can mutate the value without having
// to return a new context or something more complex.
type fakeTests struct {
	index              int
	testsToCallInstead []string
}

// CommandContext looks for a special value on the provided context.Context (see TestContextFrom).
// If that value exists, it will use the next fake test value and return a faked *exec.Cmd. It
// panics if there are not enough fake tests that were provided to the original context. If the
// special value does not exist, it is a passthrough to os/exec.CommandContext.
func CommandContext(ctx context.Context, cmd string, args ...string) *exec.Cmd {
	if override, ok := ctx.Value(overrideKey).(*fakeTests); ok {
		// We are going to shell out to the current test executable...
		testBinary := os.Args[0]
		// ...and tell it to run the next faked test.
		if override.index >= len(override.testsToCallInstead) {
			panic("Not enough fake tests provided")
		}
		fakeTest := override.testsToCallInstead[override.index]
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
// given context. This is a proxy for the amount of fake commands run.
func FakeCommandsReturned(ctx context.Context) int {
	if override, ok := ctx.Value(overrideKey).(*fakeTests); ok {
		return override.index
	}
	panic("A Context was passed in that was not produced by the executil package.")
}

// OriginalArgs returns the original arguments passed into a test function. Concretely, it looks
// at the osArgs and strips off the first 3 (the test binary, the test to run, and "--")
func OriginalArgs() []string {
	return os.Args[3:]
}
