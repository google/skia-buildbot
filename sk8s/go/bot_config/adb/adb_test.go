// Package adb is a simple wrapper around calling adb.
package adb

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/exec"
)

// Returns a context that mocks out a response when calling exec.Run().
func adbMockHappy(t *testing.T, response string) context.Context {
	mock := exec.CommandCollector{}
	mock.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		_, err := cmd.Stdout.Write([]byte(response))
		assert.NoError(t, err)
		return nil
	})
	return exec.NewContext(context.Background(), mock.Run)
}

// Returns a context that mocks out an error when calling exec.Run().
//
// Also mocks out the stderr output from adb.
func adbMockError(t *testing.T, stderr string) context.Context {
	mock := exec.CommandCollector{}
	mock.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		_, err := cmd.Stderr.Write([]byte(stderr))
		assert.NoError(t, err)
		return fmt.Errorf("exit code 1")

	})
	return exec.NewContext(context.Background(), mock.Run)
}

func TestProperties_HappyPath(t *testing.T) {
	want := map[string]string{
		"ro.product.manufacturer": "asus",
		"ro.product.model":        "Nexus 7",
		"ro.product.name":         "razor",
	}
	responseFromAdb := `
[ro.product.manufacturer]: [asus]
[ro.product.model]: [Nexus 7]
[ro.product.name]: [razor]
	`
	ctx := adbMockHappy(t, responseFromAdb)
	got, err := Properties(ctx)
	assert.NoError(t, err)
	assert.Equal(t, got, want)
}

func TestProperties_EmptyOutputFromAdb(t *testing.T) {
	want := map[string]string{}
	ctx := adbMockHappy(t, "")
	got, err := Properties(ctx)
	assert.NoError(t, err)
	assert.Equal(t, got, want)
}

func TestProperties_Error(t *testing.T) {
	ctx := adbMockError(t, "error: no devices/emulators found")
	_, err := Properties(ctx)
	assert.Equal(t, err.Error(), "Failed to run adb shell getprop \"error: no devices/emulators found\": exit code 1")
}

func TestPackageVersion_HappyPath(t *testing.T) {
	errout := &bytes.Buffer{}
	ctx := adbMockHappy(t, `
			versionCode=8186436 targetSdk=23
			versionName=8.1.86 (2287566-436)
					`)
	got := packageVersion(ctx, errout, "com.google.android.gms")
	assert.Equal(t, got, []string{"8.1.86"})
	assert.Equal(t, errout.String(), "")
}

func TestPackageVersion_NoTrailingWhitespace(t *testing.T) {
	errout := &bytes.Buffer{}
	ctx := adbMockHappy(t, `
			versionCode=8186436 targetSdk=23
			versionName=8.1.86`)
	got := packageVersion(ctx, errout, "com.google.android.gms")
	assert.Equal(t, got, []string{"8.1.86"})
	assert.Equal(t, errout.String(), "")
}

func TestPackageVersion_EmptyResponse(t *testing.T) {
	errout := &bytes.Buffer{}
	ctx := adbMockHappy(t, "")
	got := packageVersion(ctx, errout, "com.google.android.gms")
	assert.Equal(t, got, []string{})
	assert.Equal(t, errout.String(), "")
}

func TestPackageVersion_AdbError(t *testing.T) {
	errout := &bytes.Buffer{}
	ctx := adbMockError(t, "Failed to talk to device")
	got := packageVersion(ctx, errout, "com.google.android.gms")
	assert.Equal(t, got, []string{})
	assert.Equal(t, errout.String(), "Error: Failed to run adb dumpsys package \"Failed to talk to device\": exit code 1")
}
