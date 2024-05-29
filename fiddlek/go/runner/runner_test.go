package runner

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/fiddlek/go/types"
)

const fakeNamespace = "fake_namespace"

func TestPrepCodeToCompile_OptionsInjectedIntoString(t *testing.T) {
	const fakePath = "/etc/fiddle/source"
	const codeToPrep = "void draw(SkCanvas* canvas) {\n}"

	test := func(name string, opts types.Options, expectedOutput string) {
		t.Run(name, func(t *testing.T) {
			// This path does not have to exist on disk (but it should show up in the outputs)
			r, err := New(true, fakePath, fakeNamespace)
			require.NoError(t, err)
			assert.Equal(t, r.prepCodeToCompile(codeToPrep, opts), expectedOutput)
		})
	}

	test("default source", types.Options{Width: 128, Height: 256},
		`#include "fiddle_main.h"
DrawOptions GetDrawOptions() {
  static const char *path = 0; // Either a string, or 0.
  return DrawOptions(128, 256, true, true, true, true, false, false, false, path, skgpu::Mipmapped::kNo, 64, 64, 0, skgpu::Mipmapped::kNo);
}

#line 1
void draw(SkCanvas* canvas) {
}
`)

	test("uses png 2 as source", types.Options{Width: 128, Height: 256, Source: 2},
		`#include "fiddle_main.h"
DrawOptions GetDrawOptions() {
  static const char *path = "/etc/fiddle/source/2.png"; // Either a string, or 0.
  return DrawOptions(128, 256, true, true, true, true, false, false, false, path, skgpu::Mipmapped::kNo, 64, 64, 0, skgpu::Mipmapped::kNo);
}

#line 1
void draw(SkCanvas* canvas) {
}
`)

	test("some options set", types.Options{Width: 128, Height: 256, SourceMipMap: true, SRGB: true, TextOnly: true},
		`#include "fiddle_main.h"
DrawOptions GetDrawOptions() {
  static const char *path = 0; // Either a string, or 0.
  return DrawOptions(128, 256, true, true, true, true, true, false, true, path, skgpu::Mipmapped::kYes, 64, 64, 0, skgpu::Mipmapped::kNo);
}

#line 1
void draw(SkCanvas* canvas) {
}
`)

	test("bells and whistles", types.Options{
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
	}, `#include "fiddle_main.h"
DrawOptions GetDrawOptions() {
  static const char *path = 0; // Either a string, or 0.
  return DrawOptions(128, 256, true, true, true, true, true, false, true, path, skgpu::Mipmapped::kYes, 128, 256, 2, skgpu::Mipmapped::kYes);
}

#line 1
void draw(SkCanvas* canvas) {
}
`)
}

func TestRun_CompileFailed_ErrorShowsUpInResults(t *testing.T) {
	const codeSample = "This is invalid C++ code"

	// This is a fake fiddler that our runner will try to talk to.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, http.MethodPost)
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		// The full body is a JSON blob. Let's spot check that it contains the code we sent in.
		assert.Contains(t, string(body), codeSample)
		_, err = fmt.Fprintln(w, `{"Errors": "Compile Failed."}`)
		require.NoError(t, err)
	}))
	defer ts.Close()

	r, err := New(true, "/etc/fiddle/source", fakeNamespace)
	assert.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = withLocalRunURL(ctx, ts.URL)

	require.NoError(t, r.Start(ctx))

	req := &types.FiddleContext{
		Code: codeSample,
	}
	res, err := r.Run(ctx, true, req)
	assert.NoError(t, err)
	assert.Equal(t, "Compile Failed.", res.Errors)
}

func TestValidateOptions_ValidOptions_Success(t *testing.T) {
	test := func(name string, opts types.Options) {
		t.Run(name, func(t *testing.T) {
			assert.NoError(t, ValidateOptions(opts))
		})
	}

	test("empty", types.Options{})

	test("happy case", types.Options{Width: 10, Height: 20})

	test("offscreen texturable can be mipmap", types.Options{
		OffScreen:           true,
		OffScreenTexturable: true,
		OffScreenMipMap:     true,
		OffScreenWidth:      64,
		OffScreenHeight:     64,
	})
}

func TestValidateOptions_InValidOptionsReturnError(t *testing.T) {
	test := func(name string, opts types.Options) {
		t.Run(name, func(t *testing.T) {
			assert.Error(t, ValidateOptions(opts))
		})
	}

	test("negative duration", types.Options{
		Animated: true,
		Duration: -1,
	})

	test("no offscreen texturable, so can't be mipmap", types.Options{
		OffScreen:           true,
		OffScreenTexturable: false,
		OffScreenMipMap:     true,
		OffScreenWidth:      64,
		OffScreenHeight:     64,
	})

	test("offscreen width and height must be > 0", types.Options{
		OffScreen:       true,
		OffScreenWidth:  0,
		OffScreenHeight: 64,
	})

	test("negative sample count", types.Options{
		OffScreen:            true,
		OffScreenSampleCount: -1,
		OffScreenWidth:       64,
		OffScreenHeight:      64,
	})
}
