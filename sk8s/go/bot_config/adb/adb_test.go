// Package adb is a simple wrapper around calling adb.
package adb

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

const adbResponseHappyPath = `
[ro.product.manufacturer]: [asus]
[ro.product.model]: [Nexus 7]
[ro.product.name]: [razor]
`

func TestProperties_HappyPath(t *testing.T) {
	unittest.SmallTest(t)

	execCommandContext = fakeExecCommandContext_HappyPath
	defer func() {
		execCommandContext = exec.CommandContext
	}()

	want := map[string]string{
		"ro.product.manufacturer": "asus",
		"ro.product.model":        "Nexus 7",
		"ro.product.name":         "razor",
	}
	ctx := context.Background()
	got, err := Properties(ctx)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// An exec.CommandContext fake that actually executes another test in this file
// TestFakeSwarmingExecutable_ExitCodeZero instead of the requested exe.
func fakeExecCommandContext_HappyPath(ctx context.Context, command string, args ...string) *exec.Cmd {
	extendedArgs := []string{"-test.run=TestFakeAdbExecutable_HappyPath", "--", command}
	extendedArgs = append(extendedArgs, args...)
	cmd := exec.CommandContext(ctx, os.Args[0], extendedArgs...)
	cmd.Env = []string{"EMULATE_ADB_EXECUTABLE=1"}
	return cmd
}

func TestFakeAdbExecutable_HappyPath(t *testing.T) {
	if os.Getenv("EMULATE_ADB_EXECUTABLE") != "1" {
		return
	}
	fmt.Println(adbResponseHappyPath)
	os.Exit(0)
}

/*
// TestProperties_EmptyOutputFromAdb tests that we handle empty output from adb
// without error.
func TestProperties_EmptyOutputFromAdb(t *testing.T) {
	unittest.SmallTest(t)
	ctx := adbtest.AdbMockHappy(t, "")
	got, err := Properties(ctx)
	assert.NoError(t, err)
	assert.Empty(t, got)
}

// TestProperties_Error tests that we catch adb returning an error and that we
// capture the stderr output in the returned error.
func TestProperties_Error(t *testing.T) {
	unittest.SmallTest(t)
	ctx := adbtest.AdbMockError(t, "error: no devices/emulators found")
	_, err := Properties(ctx)
	assert.Equal(t, err.Error(), "Failed to run adb shell getprop \"error: no devices/emulators found\": exit code 1")
}
*/
