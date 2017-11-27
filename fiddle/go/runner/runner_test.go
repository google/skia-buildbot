package runner

import (
	"context"
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
	dir, err := WriteDrawCpp(checkout, fiddleRoot, "void draw(SkCanvas* canvas) {\n}", opts)
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(dir, filepath.Join(fiddleRoot, "tmp")))
}

// execStrings are the command lines that would have been run through exec.
var execStrings []string = []string{}

// testRun is a 'exec.Run' function to use for testing.
func testRun(cmd *exec.Command) error {
	_, err := cmd.Stdout.Write([]byte("{}"))
	if err != nil {
		return fmt.Errorf("Internal error writing: %s", err)
	}
	execStrings = append(execStrings, exec.DebugString(cmd))
	return nil
}

func TestRun(t *testing.T) {
	testutils.SmallTest(t)
	// Now test local runs, first set up exec for testing.
	ctx := exec.NewContext(context.Background(), testRun)

	opts := &types.Options{
		Duration: 2.0,
	}

	tmp, err := ioutil.TempDir("", "runner_test")
	assert.NoError(t, err)

	execStrings = []string{}
	res, err := Run(ctx, tmp+"/checkout/", tmp+"/fiddleroot/", tmp+"/depot_tools/", "abcdef", true, "", opts)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, fmt.Sprintf("sudo mount -t overlayfs -o lowerdir=%s/fiddleroot/versions/abcdef,upperdir=upper,workdir=work none overlay", tmp), execStrings[0])

	err = os.RemoveAll(tmp)
	assert.NoError(t, err)

	execStrings = []string{}
	res, err = Run(ctx, tmp+"/checkout/", tmp+"/fiddleroot/", tmp+"/depot_tools/", "abcdef", false, tmp+"/draw0123", opts)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, fmt.Sprintf("sudo mount -t overlay -o lowerdir=%s/fiddleroot/versions/abcdef,upperdir=%s/draw0123/upper,workdir=%s/draw0123/work none %s/draw0123/overlay", tmp, tmp, tmp, tmp), execStrings[0])

	err = os.RemoveAll(tmp)
	assert.NoError(t, err)
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
