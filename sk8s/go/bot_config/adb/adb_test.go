// Package adb is a simple wrapper around calling adb.
package adb

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
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
	cleanup := fakeExecCommandContextWithStdout(adbResponseHappyPath)
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

	cleanup := fakeExecCommandContextWithStdout("")
	defer cleanup()

	ctx := context.Background()
	got, err := Properties(ctx)
	assert.NoError(t, err)
	assert.Empty(t, got)
}

func fakeExecCommandContextWithStdout(stdout string) cleanupFunc {
	// An exec.CommandContext fake that actually executes another test in this file
	// TestFakeAdbExecutable_HappyPath instead of the requested exe.

	execCommandContext = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		extendedArgs := []string{"-test.run=TestFakeAdbExecutable", "--", command}
		extendedArgs = append(extendedArgs, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], extendedArgs...)
		cmd.Env = []string{
			"EMULATE_ADB_EXECUTABLE=1",
			fmt.Sprintf("STDOUT=%s", base64.RawStdEncoding.EncodeToString([]byte(stdout))),
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

	os.Exit(0)
}

/*

// TestProperties_Error tests that we catch adb returning an error and that we
// capture the stderr output in the returned error.
func TestProperties_Error(t *testing.T) {
	unittest.SmallTest(t)
	ctx := adbtest.AdbMockError(t, "error: no devices/emulators found")
	_, err := Properties(ctx)
	assert.Equal(t, err.Error(), "Failed to run adb shell getprop \"error: no devices/emulators found\": exit code 1")
}
*/
