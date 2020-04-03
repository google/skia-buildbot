// Package adb is a simple wrapper around calling adb.
package adb

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils/unittest"
)

type cleanupFunc func()

func TestProperties_HappyPath(t *testing.T) {
	unittest.SmallTest(t)

	const adbResponseHappyPath = `
[ro.product.manufacturer]: [asus]
[ro.product.model]: [Nexus 7]
[ro.product.name]: [razor]
`
	want := map[string]string{
		"ro.product.manufacturer": "asus",
		"ro.product.model":        "Nexus 7",
		"ro.product.name":         "razor",
	}
	cleanup := fakeExecCommandContext(adbResponseHappyPath, "", 0)
	defer cleanup()
	ctx := context.Background()
	a := New()
	got, err := a.Properties(ctx)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// TestProperties_EmptyOutputFromAdb tests that we handle empty output from adb
// without error.
func TestProperties_EmptyOutputFromAdb(t *testing.T) {
	unittest.SmallTest(t)

	cleanup := fakeExecCommandContext("", "", 0)
	defer cleanup()

	ctx := context.Background()
	a := New()
	got, err := a.Properties(ctx)
	assert.NoError(t, err)
	assert.Empty(t, got)
}

// TestProperties_Error tests that we catch adb returning an error and that we
// capture the stderr output in the returned error.
func TestProperties_ErrFromAdbNonZeroExitCode(t *testing.T) {
	unittest.SmallTest(t)

	const exitCode = 123

	cleanup := fakeExecCommandContext("", "error: no devices/emulators found", exitCode)
	defer cleanup()

	ctx := context.Background()

	a := New()
	_, err := a.Properties(ctx)
	// Confirm that both the exit code and the abd stderr make it into the returned error.
	assert.Contains(t, err.Error(), fmt.Sprintf("exit status %d", exitCode))
	assert.Contains(t, err.Error(), "error: no devices/emulators found")
}

// An exec.CommandContext fake that actually executes another test in this file
// TestFakeAdbExecutable instead of the requested exe.
//
// See https://npf.io/2015/06/testing-exec-command/ for background on this technique.
func fakeExecCommandContext(stdout, stderr string, exitCode int) cleanupFunc {

	execCommandContext = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		extendedArgs := []string{"-test.run=TestFakeAdbExecutable", "--", command}
		extendedArgs = append(extendedArgs, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], extendedArgs...)
		// Since we are executing another process we can set environment
		// variables to send test data to that process.
		cmd.Env = []string{
			"EMULATE_ADB_EXECUTABLE=1",
			fmt.Sprintf("STDOUT=%s", base64.RawStdEncoding.EncodeToString([]byte(stdout))),
			fmt.Sprintf("STDERR=%s", base64.RawStdEncoding.EncodeToString([]byte(stderr))),
			fmt.Sprintf("EXIT_CODE=%d", exitCode),
		}
		return cmd
	}

	return func() {
		execCommandContext = exec.CommandContext
	}
}

// TestFakeAdbExecutable isn't really a test, but is used by
// fakeExecCommandContext to fake out exec.CommandContext().
func TestFakeAdbExecutable(t *testing.T) {
	unittest.SmallTest(t)
	if os.Getenv("EMULATE_ADB_EXECUTABLE") != "1" {
		return
	}
	if stdout := os.Getenv("STDOUT"); stdout != "" {
		b, err := base64.RawStdEncoding.DecodeString(stdout)
		if err != nil {
			sklog.Fatalf("Failed to base64 decode the expected stdout: %s", err)
		}
		fmt.Print(string(b))
	}
	if stderr := os.Getenv("STDERR"); stderr != "" {
		b, err := base64.RawStdEncoding.DecodeString(stderr)
		if err != nil {
			sklog.Fatalf("Failed to base64 decode the expected stdout: %s", err)
		}
		fmt.Fprint(os.Stderr, string(b))
	}
	exitCode, err := strconv.Atoi(os.Getenv("EXIT_CODE"))
	if err != nil {
		sklog.Fatal(err)
	}

	os.Exit(exitCode)
}

func TestDimensionsFromProperties_Success(t *testing.T) {
	unittest.SmallTest(t)

	adbResponse := strings.Join([]string{
		"[ro.product.manufacturer]: [Google]", // Ignored
		"[ro.product.model]: [Pixel 3a]",      // Ignored
		"[ro.build.id]: [QQ2A.200305.002]",    // device_os
		"[ro.product.brand]: [google]",        // device_os_flavor
		"[ro.build.type]: [user]",             // device_os_type
		"[ro.product.device]: [sargo]",        // device_type
		"[ro.product.system.brand]: [google]", // device_os_flavor (dup should be ignored)
		"[ro.product.system.brand]: [aosp]",   // device_os_flavor (should be converted to "android")
	}, "\n")

	cleanup := fakeExecCommandContext(adbResponse, "", 0)
	defer cleanup()
	ctx := context.Background()
	inputDim := map[string][]string{
		"foo": {"bar"},
	}
	a := New()
	got, err := a.DimensionsFromProperties(ctx, inputDim)
	require.NoError(t, err)
	expected := map[string][]string{
		"android_devices":  {"1"},
		"device_os":        {"Q", "QQ2A.200305.002"},
		"device_os_flavor": {"google", "android"},
		"device_os_type":   {"user"},
		"device_type":      {"sargo"},
		"os":               {"Android"},
		"foo":              {"bar"},
	}
	assert.Equal(t, expected, got)
}
