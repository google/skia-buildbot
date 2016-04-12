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
)

func TestPrep(t *testing.T) {
	opts := &types.Options{
		Width:  128,
		Height: 256,
		Source: 2,
	}
	want := `#include "fiddle_main.h"
DrawOptions GetDrawOptions() {
  static const char *path = "/mnt/pd0/fiddle/images/2.png"; // Either a string, or 0.
  return DrawOptions(128, 256, true, true, true, true, path);
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
  return DrawOptions(128, 256, true, true, true, true, path);
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
	// Create a temp fiddleRoot that gets cleaned up.
	fiddleRoot, err := ioutil.TempDir("", "runner-test")
	assert.NoError(t, err)
	defer func() {
		err := os.RemoveAll(fiddleRoot)
		if err != nil {
			t.Logf("Failed to clean up fiddleRoot: %s", err)
		}
	}()

	opts := &types.Options{
		Width:  128,
		Height: 256,
		Source: 2,
	}
	// Test local=true.
	dir, err := WriteDrawCpp(fiddleRoot, "void draw(SkCanvas* canvas) {\n}", opts, true)
	assert.NoError(t, err)
	assert.Equal(t, dir, filepath.Join(fiddleRoot, "src"))

	// Test local=false.
	dir, err = WriteDrawCpp(fiddleRoot, "void draw(SkCanvas* canvas) {\n}", opts, false)
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
	// Now test local runs, first set up exec for testing.
	exec.SetRunForTesting(testRun)
	defer exec.SetRunForTesting(exec.DefaultRun)

	res, err := Run("fiddleroot/", "abcdef", true, "")
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, "fiddle_run --fiddle_root fiddleroot/ --git_hash abcdef --local", execString)

	res, err = Run("fiddleroot/", "abcdef", false, "/mnt/pd0/fiddle/tmp/draw0123")
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, "sudo systemd-nspawn -D /mnt/pd0/container/ --bind=/mnt/pd0/fiddle --bind /mnt/pd0/fiddle/tmp/draw0123:/mnt/pd0/fiddle/src xargs --arg-file=/dev/null /mnt/pd0/fiddle/bin/fiddle_run --fiddle_root fiddleroot/ --git_hash abcdef --alsologtostderr", execString)
}
