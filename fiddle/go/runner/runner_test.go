package runner

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/fiddle/go/types"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/testutils"
)

func TestPrep(t *testing.T) {
	testutils.SmallTest(t)
	opts := &types.Options{
		Width:  128,
		Height: 256,
		Source: 2,
	}
	want := `#include "fiddle_main.h"
DrawOptions GetDrawOptions() {
  static const char *path = "/mnt/pd0/fiddle/images/2.png"; // Either a string, or 0.
  return DrawOptions(128, 256, true, true, true, true, false, false, false, path);
}

#line 1
void draw(SkCanvas* canvas) {
#line 2
}
`
	got := prepCodeToCompile("/mnt/pd0/fiddle/", "void draw(SkCanvas* canvas) {\n}", opts)
	assert.Equal(t, want, got)

	opts = &types.Options{
		Width:  128,
		Height: 256,
		Source: 0,
	}
	want = `#include "fiddle_main.h"
DrawOptions GetDrawOptions() {
  static const char *path = 0; // Either a string, or 0.
  return DrawOptions(128, 256, true, true, true, true, false, false, false, path);
}

#line 1
void draw(SkCanvas* canvas) {
#line 2
}
`
	got = prepCodeToCompile("/mnt/pd0/fiddle/", "void draw(SkCanvas* canvas) {\n}", opts)
	assert.Equal(t, want, got)

	opts = &types.Options{
		Width:    128,
		Height:   256,
		Source:   0,
		SRGB:     true,
		F16:      false,
		TextOnly: true,
	}
	want = `#include "fiddle_main.h"
DrawOptions GetDrawOptions() {
  static const char *path = 0; // Either a string, or 0.
  return DrawOptions(128, 256, true, true, true, true, true, false, true, path);
}

#line 1
void draw(SkCanvas* canvas) {
#line 2
}
`
	got = prepCodeToCompile("/mnt/pd0/fiddle/", "void draw(SkCanvas* canvas) {\n}", opts)
	assert.Equal(t, want, got)
}

func TestWriteDrawCpp(t *testing.T) {
	testutils.SmallTest(t)
	// Create a temp fiddleRoot that gets cleaned up.
	fiddleRoot, err := ioutil.TempDir("", "runner-test")
	assert.NoError(t, err)
	defer func() {
		err := os.RemoveAll(fiddleRoot)
		if err != nil {
			t.Logf("Failed to clean up fiddleRoot: %s", err)
		}
	}()

	// Create a temp checkout that gets cleaned up.
	checkout, err := ioutil.TempDir("", "runner-test")
	assert.NoError(t, err)
	defer func() {
		err := os.RemoveAll(checkout)
		if err != nil {
			t.Logf("Failed to clean up checkout: %s", err)
		}
	}()
	err = os.MkdirAll(filepath.Join(checkout, "tools", "fiddle"), 0777)
	assert.NoError(t, err)

	opts := &types.Options{
		Width:  128,
		Height: 256,
		Source: 2,
	}
	// Test local=true.
	dir, err := WriteDrawCpp(checkout, fiddleRoot, "void draw(SkCanvas* canvas) {\n}", opts, true)
	assert.NoError(t, err)
	assert.Equal(t, dir, filepath.Join(checkout, "skia", "tools", "fiddle"))

	// Test local=false.
	dir, err = WriteDrawCpp(checkout, fiddleRoot, "void draw(SkCanvas* canvas) {\n}", opts, false)
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(dir, filepath.Join(fiddleRoot, "tmp")))
}

// execString is the command line that would have been run through exec.
var execString string

// testRun is a 'exec.Run' function to use for testing.
func testRun(cmd *exec.Command) error {
	_, err := cmd.Stdout.Write([]byte("{}"))
	if err != nil {
		return fmt.Errorf("Internal error writing: %s", err)
	}
	execString = exec.DebugString(cmd)
	return nil
}

func TestRun(t *testing.T) {
	testutils.SmallTest(t)
	// Now test local runs, first set up exec for testing.
	exec.SetRunForTesting(testRun)
	defer exec.SetRunForTesting(exec.DefaultRun)

	opts := &types.Options{
		Duration: 2.0,
	}

	res, err := Run("checkout/", "fiddleroot/", "depot_tools/", "abcdef", true, "", opts)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, "fiddle_run --fiddle_root fiddleroot/ --git_hash abcdef --local --alsologtostderr --duration 2.000000", execString)

	res, err = Run("checkout/", "fiddleroot/", "depot_tools/", "abcdef", false, "/mnt/pd0/fiddle/tmp/draw0123", opts)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, "sudo systemd-nspawn -D /mnt/pd0/container/ --read-only --private-network --machine draw0123 --overlay fiddleroot/:/mnt/pd0/fiddle/tmp/draw0123:fiddleroot/ --bind-ro /mnt/pd0/fiddle/tmp/draw0123/draw.cpp:checkout/skia/tools/fiddle/draw.cpp xargs --arg-file=/dev/null /mnt/pd0/fiddle/bin/fiddle_run --fiddle_root fiddleroot/ --git_hash abcdef --duration 2.000000", execString)
}

func TestValidateOptions(t *testing.T) {
	testutils.SmallTest(t)
	testCases := []struct {
		value         *types.Options
		errorExpected bool
		message       string
	}{
		{
			value: &types.Options{
				Animated: true,
				Duration: -1,
			},
			errorExpected: true,
			message:       "negative duration",
		},
	}

	for _, tc := range testCases {
		if got, want := ValidateOptions(tc.value) != nil, tc.errorExpected; got != want {
			t.Errorf("Failed case Got %v Want %v: %s", got, want, tc.message)
		}
	}
}
