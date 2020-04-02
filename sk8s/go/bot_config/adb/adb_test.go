// Package adb is a simple wrapper around calling adb.
package adb

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils/unittest"
)

type cleanupFunc func()

const adbResponseHappyPath = `
[ro.product.manufacturer]: [asus]
[ro.product.model]: [Nexus 7]
[ro.product.name]: [razor]
`

func TestProperties_HappyPath(t *testing.T) {
	unittest.SmallTest(t)

	want := map[string]string{
		"ro.product.manufacturer": "asus",
		"ro.product.model":        "Nexus 7",
		"ro.product.name":         "razor",
	}
	cleanup := fakeExecCommandContextWithStdout(adbResponseHappyPath, "", 0)
	defer cleanup()
	ctx := context.Background()
	got, err := Properties(ctx)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// TestProperties_EmptyOutputFromAdb tests that we handle empty output from adb
// without error.
func TestProperties_EmptyOutputFromAdb(t *testing.T) {
	unittest.SmallTest(t)

	cleanup := fakeExecCommandContextWithStdout("", "", 0)
	defer cleanup()

	ctx := context.Background()
	got, err := Properties(ctx)
	assert.NoError(t, err)
	assert.Empty(t, got)
}

// TestProperties_Error tests that we catch adb returning an error and that we
// capture the stderr output in the returned error.
func TestProperties_ErrFromAdbNonZeroExitCode(t *testing.T) {
	unittest.SmallTest(t)

	const exitCode = 123

	cleanup := fakeExecCommandContextWithStdout("", "error: no devices/emulators found", exitCode)
	defer cleanup()

	ctx := context.Background()

	_, err := Properties(ctx)
	// Confirm that both the exit code and the abd stderr make it into the returned error.
	assert.Contains(t, err.Error(), fmt.Sprintf("exit status %d", exitCode))
	assert.Contains(t, err.Error(), "error: no devices/emulators found")
}

func fakeExecCommandContextWithStdout(stdout, stderr string, exitCode int) cleanupFunc {
	// An exec.CommandContext fake that actually executes another test in this file
	// TestFakeAdbExecutable_HappyPath instead of the requested exe.

	execCommandContext = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		extendedArgs := []string{"-test.run=TestFakeAdbExecutable", "--", command}
		extendedArgs = append(extendedArgs, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], extendedArgs...)
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

func TestFakeAdbExecutable(t *testing.T) {
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
