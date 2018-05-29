package runner

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/fiddlek/go/types"
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
	r := New("/etc/fiddle/source")
	want := `#include "fiddle_main.h"
DrawOptions GetDrawOptions() {
  static const char *path = "/etc/fiddle/source/2.png"; // Either a string, or 0.
  return DrawOptions(128, 256, true, true, true, true, false, false, false, path, GrMipMapped::kNo, 64, 64, 0, GrMipMapped::kNo);
}

#line 1
void draw(SkCanvas* canvas) {
#line 2
}
`
	got := r.prepCodeToCompile("void draw(SkCanvas* canvas) {\n}", opts)
	assert.Equal(t, want, got)

	opts = &types.Options{
		Width:  128,
		Height: 256,
		Source: 0,
	}
	want = `#include "fiddle_main.h"
DrawOptions GetDrawOptions() {
  static const char *path = 0; // Either a string, or 0.
  return DrawOptions(128, 256, true, true, true, true, false, false, false, path, GrMipMapped::kNo, 64, 64, 0, GrMipMapped::kNo);
}

#line 1
void draw(SkCanvas* canvas) {
#line 2
}
`
	got = r.prepCodeToCompile("void draw(SkCanvas* canvas) {\n}", opts)
	assert.Equal(t, want, got)

	opts = &types.Options{
		Width:        128,
		Height:       256,
		Source:       0,
		SourceMipMap: true,
		SRGB:         true,
		F16:          false,
		TextOnly:     true,
	}
	want = `#include "fiddle_main.h"
DrawOptions GetDrawOptions() {
  static const char *path = 0; // Either a string, or 0.
  return DrawOptions(128, 256, true, true, true, true, true, false, true, path, GrMipMapped::kYes, 64, 64, 0, GrMipMapped::kNo);
}

#line 1
void draw(SkCanvas* canvas) {
#line 2
}
`
	got = r.prepCodeToCompile("void draw(SkCanvas* canvas) {\n}", opts)
	assert.Equal(t, want, got)

	opts = &types.Options{
		Width:                128,
		Height:               256,
		Source:               0,
		SRGB:                 true,
		F16:                  false,
		TextOnly:             true,
		SourceMipMap:         true,
		OffScreenWidth:       128,
		OffScreenHeight:      256,
		OffScreenSampleCount: 2,
		OffScreenMipMap:      true,
	}
	want = `#include "fiddle_main.h"
DrawOptions GetDrawOptions() {
  static const char *path = 0; // Either a string, or 0.
  return DrawOptions(128, 256, true, true, true, true, true, false, true, path, GrMipMapped::kYes, 128, 256, 2, GrMipMapped::kYes);
}

#line 1
void draw(SkCanvas* canvas) {
#line 2
}
`
	got = r.prepCodeToCompile("void draw(SkCanvas* canvas) {\n}", opts)
	assert.Equal(t, want, got)

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

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Hit server.")
		fmt.Fprintln(w, "{}")
	}))
	defer ts.Close()

	r := New("/etc/fiddle/source")
	r.localUrl = ts.URL
	fmt.Println(ts.URL)
	req := &types.FiddleContext{
		// Fill this out.
	}
	_, err := r.Run(true, req)
	// Check response.
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
		{
			value: &types.Options{
				OffScreen:           true,
				OffScreenTexturable: true,
				OffScreenMipMap:     true,
				OffScreenWidth:      64,
				OffScreenHeight:     64,
			},
			errorExpected: false,
			message:       "offscreen texturable can be mipmap",
		},
		{
			value: &types.Options{
				OffScreen:           true,
				OffScreenTexturable: false,
				OffScreenMipMap:     true,
				OffScreenWidth:      64,
				OffScreenHeight:     64,
			},
			errorExpected: true,
			message:       "no offscreen texturable, so can't be mipmap",
		},
		{
			value: &types.Options{
				OffScreen:       true,
				OffScreenWidth:  0,
				OffScreenHeight: 64,
			},
			errorExpected: true,
			message:       "width and height > 0",
		},
		{
			value: &types.Options{
				OffScreen:            true,
				OffScreenSampleCount: -1,
				OffScreenWidth:       64,
				OffScreenHeight:      64,
			},
			errorExpected: true,
			message:       "No negative int",
		},
	}

	r := New("/etc/fiddle/source")
	for _, tc := range testCases {
		if got, want := r.ValidateOptions(tc.value) != nil, tc.errorExpected; got != want {
			t.Errorf("Failed case Got %v Want %v: %s", got, want, tc.message)
		}
	}
}
