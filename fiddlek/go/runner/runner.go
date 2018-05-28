// Functions for the last mile of a fiddle, i.e. writing out
// draw.cpp and then calling fiddle_run to compile and execute
// the code.
package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"

	"go.skia.org/infra/fiddlek/go/linenumbers"
	"go.skia.org/infra/fiddlek/go/types"
	"go.skia.org/infra/go/git/gitinfo"
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
	client      = httputils.NewTimeoutClient()
)

func toGrMipMapped(b bool) string {
	if b {
		return "GrMipMapped::kYes"
	} else {
		return "GrMipMapped::kNo"
	}
}

// prepCodeToCompile adds the line numbers and the right prefix code
// to the fiddle so it compiles and links correctly.
//
//    code - The code to compile.
//    opts - The user's options about how to run that code.
//
// Returns the prepped code.
func prepCodeToCompile(code string, opts *types.Options) string {
	code = linenumbers.LineNumbers(code)
	sourceImage := "0"
	if opts.Source != 0 {
		filename := fmt.Sprintf("%d.png", opts.Source)
		// TODO Move string to const.
		sourceImage = fmt.Sprintf("%q", filepath.Join("/tmp/skia/skia/images", filename))
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
func ValidateOptions(opts *types.Options) error {
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

// GitHashTimeStamp finds the timestamp, in UTC, of the given checkout of Skia under fiddleRoot.
//
//    fiddleRoot - The root of the fiddle working directory. See DESIGN.md.
//    gitHash - The git hash of the version of Skia we have checked out.
//
// Returns the timestamp of the git commit in UTC.
func GitHashTimeStamp(ctx context.Context, fiddleRoot, gitHash string) (time.Time, error) {
	g, err := gitinfo.NewGitInfo(ctx, filepath.Join(fiddleRoot, "versions", gitHash), false, false)
	if err != nil {
		return time.Time{}, fmt.Errorf("Failed to create gitinfo: %s", err)
	}
	commit, err := g.Details(ctx, gitHash, false)
	if err != nil {
		return time.Time{}, fmt.Errorf("Failed to retrieve info on the git commit %s: %s", gitHash, err)
	}
	return commit.Timestamp.In(time.UTC), nil
}

// Run executes fiddle_run and then parses the JSON output into types.Results.
//
//    fiddleRoot - The root of the fiddle working directory. See DESIGN.md.
//    gitHash - The git hash of the version of Skia we have checked out.
//    local - Boolean, true if we are running locally, else we should execute
//        fiddle_run under fiddle_secwrap.
//    tmpDir - The directory outside the container to mount as FIDDLE_ROOT/src
//        that contains the user's draw.cpp file. Only used if local is false.
//    opts - The compile time options that are passed to draw.cpp.
//    preserve - Should the overlay mount be preserverd?
//
// Returns the parsed JSON that fiddle_run emits to stdout.
//
// If non-local this should run something like:
//
// sudo systemd-nspawn -D /mnt/pd0/container/ --read-only --private-network
//  --machine foo
//  --overlay=/mnt/pd0/fiddle:/tmp:/mnt/pd0/fiddle
//  --bind-ro /tmp/draw.cpp:/mnt/pd0/fiddle/versions/d6dd44140d8dd6d18aba1dfe9edc5582dcd73d2f/tools/fiddle/draw.cpp
//  xargs --arg-file=/dev/null \
//    /mnt/pd0/fiddle/bin/fiddle_run \
//    --fiddle_root /mnt/pd0/fiddle \
//    --git_hash 5280dcbae3affd73be5d5e0ff3db8823e26901e6 \
//    --alsologtostderr
//
// OVERLAY
//    The use of --overlay=/mnt/pd0/fiddle:/tmp:/mnt/pd0/fiddle sets up an interesting directory
//    /mnt/pd0/fiddle in the container, where the entire contents of the host's
//    /mnt/pd0/fiddle is available read-only, and if the container tries to write
//    to any files there, or create new files, they end up in /tmp and the first
//    directory stays untouched. See https://www.kernel.org/doc/Documentation/filesystems/overlayfs.txt
//    and the documentation for the --overlay flag of systemd-nspawn.
//
// Why xargs?
//    When trying to run a binary that exists on a mounted directory under nspawn, it will fail with:
//
//    $ sudo systemd-nspawn -D /mnt/pd0/container/ --bind=/mnt/pd0/fiddle /mnt/pd0/fiddle/bin/fiddle_run
//    Directory /mnt/pd0/container lacks the binary to execute or doesn't look like a binary tree. Refusing.
//
// That's because nspawn is looking for the exe before doing the bindings. The
// fix? A pure hack, insert "xargs --arg-file=/dev/null " before the command
// you want to run. Since xargs exists in the container this will proceed to
// the point of making the bindings and then xargs will be able to execute the
// exe within the container.
//
func Run(ctx context.Context, local bool, req *types.FiddleContext) (*types.Result, error) {
	req.Code = prepCodeToCompile(req.Code, &req.Options)
	runTotal.Inc(1)

	var output bytes.Buffer
	// If not local then use the k8s api to pick an open fiddler pod to send
	// the request to. Send a GET / to each on until you find an idle instance.
	if local {
		b, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("Failed to encode request: %s", err)
		}
		body := bytes.NewReader(b)
		resp, err := client.Post("http://localhost:8000/run", "application/json", body)
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
		return nil, fmt.Errorf("Failed to decode results from run: %s", err)
	}
	return res, nil
}
