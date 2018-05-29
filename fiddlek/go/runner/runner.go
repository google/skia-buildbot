// Functions for the last mile of a fiddle, i.e. writing out
// draw.cpp and then calling fiddle_run to compile and execute
// the code.
package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"

	"go.skia.org/infra/fiddlek/go/linenumbers"
	"go.skia.org/infra/fiddlek/go/types"
)

const (
	// PREFIX is a format string for the code that makes it compilable.
	PREFIX = `#include "fiddle_main.h"
DrawOptions GetDrawOptions() {
  static const char *path = %s; // Either a string, or 0.
  return DrawOptions(%d, %d, true, true, %v, %v, %v, %v, %v, path, %s, %d, %d, %d, %s);
}

%s
`
)

var (
	runTotal    = metrics2.GetCounter("run_total", nil)
	runFailures = metrics2.GetCounter("run_failures", nil)
)

func toGrMipMapped(b bool) string {
	if b {
		return "GrMipMapped::kYes"
	} else {
		return "GrMipMapped::kNo"
	}
}

type Runner struct {
	sourceDir string
	client    *http.Client
	localUrl  string
}

func New(sourceDir string) *Runner {
	return &Runner{
		sourceDir: sourceDir,
		client:    httputils.NewTimeoutClient(),
		localUrl:  "http://localhost:8000/run",
	}
}

// prepCodeToCompile adds the line numbers and the right prefix code
// to the fiddle so it compiles and links correctly.
//
//    code - The code to compile.
//    opts - The user's options about how to run that code.
//
// Returns the prepped code.
func (r *Runner) prepCodeToCompile(code string, opts *types.Options) string {
	code = linenumbers.LineNumbers(code)
	sourceImage := "0"
	if opts.Source != 0 {
		filename := fmt.Sprintf("%d.png", opts.Source)
		sourceImage = fmt.Sprintf("%q", filepath.Join(r.sourceDir, filename))
	}
	pdf := true
	skp := true
	if opts.Animated {
		pdf = false
		skp = false
	}
	offscreen_mipmap := toGrMipMapped(opts.OffScreenMipMap)
	source_mipmap := toGrMipMapped(opts.SourceMipMap)
	offscreen_width := opts.OffScreenWidth
	if offscreen_width == 0 {
		offscreen_width = 64
	}
	offscreen_height := opts.OffScreenHeight
	if offscreen_height == 0 {
		offscreen_height = 64
	}
	return fmt.Sprintf(PREFIX, sourceImage, opts.Width, opts.Height, pdf, skp, opts.SRGB, opts.F16, opts.TextOnly, source_mipmap, offscreen_width, offscreen_height, opts.OffScreenSampleCount, offscreen_mipmap, code)
}

// ValidateOptions validates that the options make sense.
func (r *Runner) ValidateOptions(opts *types.Options) error {
	if opts.Animated {
		if opts.Duration <= 0 {
			return fmt.Errorf("Animation duration must be > 0.")
		}
		if opts.Duration > 60 {
			return fmt.Errorf("Animation duration must be < 60s.")
		}
	} else {
		opts.Duration = 0
	}
	if opts.OffScreen {
		if opts.OffScreenMipMap {
			if opts.OffScreenTexturable == false {
				return fmt.Errorf("OffScreenMipMap can only be true if OffScreenTexturable is true.")
			}
		}
		if opts.OffScreenWidth <= 0 || opts.OffScreenHeight <= 0 {
			return fmt.Errorf("OffScreen Width and Height must be > 0.")
		}
		if opts.OffScreenSampleCount < 0 {
			return fmt.Errorf("OffScreen SampleCount must be >= 0.")
		}
	}
	return nil
}

// Run executes fiddle_run and then parses the JSON output into types.Results.
//
//    local - Boolean, true if we are running locally, else we should execute
//        fiddle_run under fiddle_secwrap.
func (r *Runner) Run(local bool, req *types.FiddleContext) (*types.Result, error) {
	reqToSend := *req
	reqToSend.Code = r.prepCodeToCompile(req.Code, &req.Options)
	runTotal.Inc(1)

	var output bytes.Buffer
	// If not local then use the k8s api to pick an open fiddler pod to send
	// the request to. Send a GET / to each on until you find an idle instance.
	if local {
		b, err := json.Marshal(reqToSend)
		if err != nil {
			return nil, fmt.Errorf("Failed to encode request: %s", err)
		}
		body := bytes.NewReader(b)
		resp, err := r.client.Post(r.localUrl, "application/json", body)
		if err != nil {
			return nil, fmt.Errorf("Failed to send request: %s", err)
		}
		defer util.Close(resp.Body)
		_, err = io.Copy(&output, resp.Body)
		sklog.Infof("Got response: %q", output.String())
		if err != nil {
			return nil, fmt.Errorf("Failed to read response: %s", err)
		}
	}

	// Parse the output into types.Result.
	res := &types.Result{}
	if err := json.Unmarshal(output.Bytes(), res); err != nil {
		sklog.Errorf("Received erroneous output: %q", output.String())
		return nil, fmt.Errorf("Failed to decode results from run: %s, %q", err, output.String())
	}
	return res, nil
}
