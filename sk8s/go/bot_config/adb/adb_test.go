// Package adb is a simple wrapper around calling adb.
package adb

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/sk8s/go/bot_config/adb/adbtest"
)

func TestProperties_HappyPath(t *testing.T) {
	unittest.SmallTest(t)
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
	ctx := adbtest.AdbMockHappy(t, responseFromAdb)
	got, err := Properties(ctx)
	assert.NoError(t, err)
	assert.Equal(t, want, got)
}

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

func TestPackageVersion_HappyPath(t *testing.T) {
	unittest.SmallTest(t)
	errout := &bytes.Buffer{}
	ctx := adbtest.AdbMockHappy(t, `
			versionCode=8186436 targetSdk=23
			versionName=8.1.86 (2287566-436)
					`)
	got := packageVersion(ctx, errout, "com.google.android.gms")
	assert.Equal(t, got, []string{"8.1.86"})
	assert.Equal(t, errout.String(), "")
}

// TestPackageVersion_NoTrailingWhitespace confirms we parse correctly even when
// there is no trailing whitespace.
func TestPackageVersion_NoTrailingWhitespace(t *testing.T) {
	unittest.SmallTest(t)
	errout := &bytes.Buffer{}
	ctx := adbtest.AdbMockHappy(t, `
			versionCode=8186436 targetSdk=23
			versionName=8.1.86`)
	got := packageVersion(ctx, errout, "com.google.android.gms")
	assert.Equal(t, got, []string{"8.1.86"})
	assert.Equal(t, errout.String(), "")
}

// TestPackageVersion_EmptyResponse tests that we handle an empty response
// without error.
func TestPackageVersion_EmptyResponse(t *testing.T) {
	unittest.SmallTest(t)
	errout := &bytes.Buffer{}
	ctx := adbtest.AdbMockHappy(t, "")
	got := packageVersion(ctx, errout, "com.google.android.gms")
	assert.Equal(t, got, []string{})
	assert.Empty(t, errout.String())
}

// TestPackageVersion_AdbError tests that we catch adb returning an error and
// that we capture the stderr output in the returned error.
func TestPackageVersion_AdbError(t *testing.T) {
	unittest.SmallTest(t)
	errout := &bytes.Buffer{}
	ctx := adbtest.AdbMockError(t, "Failed to talk to device")
	got := packageVersion(ctx, errout, "com.google.android.gms")
	assert.Empty(t, got)
	assert.Equal(t, errout.String(), "Error: Failed to run adb dumpsys package \"Failed to talk to device\": exit code 1")
}
