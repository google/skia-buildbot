// Functions for the last mile of a fiddle, i.e. writing out
// draw.cpp and then calling fiddle_run to compile and execute
// the code.
package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"

	"go.skia.org/infra/fiddle/go/linenumbers"
	"go.skia.org/infra/fiddle/go/types"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/util"
)

const (
	// PREFIX is a format string for the code that makes it compilable.
	PREFIX = `#include "fiddle_main.h"
DrawOptions GetDrawOptions() {
  static const char *path = %s; // Either a string, or 0.
  return DrawOptions(%d, %d, true, true, true, true, path);
}

%s
`
)

// prepCodeToCompile adds the line numbers and the right prefix code
// to the fiddle so it compiles and links correctly.
//
//    fiddleRoot - The root of the fiddle working directory. See DESIGN.md.
//    code - The code to compile.
//    opts - The user's options about how to run that code.
//
// Returns the prepped code.
func prepCodeToCompile(fiddleRoot, code string, opts *types.Options) string {
	code = linenumbers.LineNumbers(code)
	sourceImage := "0"
	if opts.Source != 0 {
		filename := fmt.Sprintf("%d.png", opts.Source)
		sourceImage = fmt.Sprintf("%q", filepath.Join(fiddleRoot, "images", filename))
	}
	return fmt.Sprintf(PREFIX, sourceImage, opts.Width, opts.Height, code)
}

// WriteDrawCpp takes the given code, modifies it so that it can be compiled
// and then writes the "draw.cpp" file to the correct location, based on
// 'local'.
//
//    fiddleRoot - The root of the fiddle working directory. See DESIGN.md.
//    code - The code to compile.
//    opts - The user's options about how to run that code.
//    local - If true then we are running locally, so write the code to
//        fiddleRoot/src/draw.cpp.
//
// Returns the directory where the 'draw.cpp' file was written.
func WriteDrawCpp(fiddleRoot, code string, opts *types.Options, local bool) (string, error) {
	code = prepCodeToCompile(fiddleRoot, code, opts)
	dstDir := filepath.Join(fiddleRoot, "src")
	if !local {
		tmp := filepath.Join(fiddleRoot, "tmp")
		err := os.MkdirAll(tmp, 0755)
		if err != nil {
			return "", fmt.Errorf("Failed to create FIDDLE_ROOT/tmp: %s", err)
		}
		dstDir, err = ioutil.TempDir(tmp, "code")
		if err != nil {
			return "", fmt.Errorf("Failed to create temp dir for draw.cpp: %s", err)
		}
	} else {
		if _, err := os.Stat(dstDir); err != nil && os.IsNotExist(err) {
			err := os.MkdirAll(dstDir, 0755)
			if err != nil {
				return "", fmt.Errorf("Failed to create FIDDLE_ROOT/src: %s", err)
			}
		}
	}
	w, err := os.Create(filepath.Join(dstDir, "draw.cpp"))
	if err != nil {
		return dstDir, fmt.Errorf("Failed to open destination: %s", err)
	}
	defer util.Close(w)
	_, err = w.Write([]byte(code))
	if err != nil {
		return dstDir, fmt.Errorf("Failed to write draw.cpp file: %s", err)
	}
	return dstDir, nil
}

// GitHashTimeStamp finds the timestamp, in UTC, of the given checkout of Skia under fiddleRoot.
//
//    fiddleRoot - The root of the fiddle working directory. See DESIGN.md.
//    gitHash - The git hash of the version of Skia we have checked out.
//
// Returns the timestamp of the git commit in UTC.
func GitHashTimeStamp(fiddleRoot, gitHash string) (time.Time, error) {
	g, err := gitinfo.NewGitInfo(filepath.Join(fiddleRoot, "versions", gitHash), false, false)
	if err != nil {
		return time.Time{}, fmt.Errorf("Failed to create gitinfo: %s", err)
	}
	commit, err := g.Details(gitHash, false)
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
//
// Returns the parsed JSON that fiddle_run emits to stdout.
//
// If non-local this should run something like:
//
//     sudo systemd-nspawn -D /mnt/pd0/container/ --read-only --private-network \
//       --bind-ro /mnt/pd0/fiddle \
//       --bind-ro /mnt/pd0/fiddle/tmp/code288218027:/mnt/pd0/fiddle/src \
//       --bind /mnt/pd0/fiddle/tmp/code288218027:/mnt/pd0/fiddle/out \
//       xargs --arg-file=/dev/null \
//       /mnt/pd0/fiddle/bin/fiddle_run \
//       --fiddle_root /mnt/pd0/fiddle \
//       --git_hash 5280dcbae3affd73be5d5e0ff3db8823e26901e6
//       --alsologtostderr
//
// NOTE: When trying to run a binary that exists on a mounted directory under nspawn, it will fail with:
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
func Run(fiddleRoot, gitHash string, local bool, tmpDir string) (*types.Result, error) {
	machine := ""
	if !local {
		machine = path.Base(tmpDir)
	}
	name := "sudo"
	args := []string{
		"systemd-nspawn", "-D", "/mnt/pd0/container/",
		"--read-only",        // Mount the root file system as read only.
		"--private-network",  // Turn off networking.
		"--machine", machine, // Give the container a unique name, so we can run fiddles concurrently.
		"--bind-ro", "/mnt/pd0/fiddle", // Mount most of FIDDLE_ROOT as read-only.
		"--bind-ro", tmpDir + ":/mnt/pd0/fiddle/src", // Mount the user's draw.cpp file into /src.
		"--bind", tmpDir + ":/mnt/pd0/fiddle/out", // Also mount the same dir as draw.cpp as read/write to receive the executable.
		"xargs", "--arg-file=/dev/null", // See Note above for explanation of xargs.
		"/mnt/pd0/fiddle/bin/fiddle_run", "--fiddle_root", fiddleRoot, "--git_hash", gitHash, "--alsologtostderr",
	}
	if local {
		name = "fiddle_run"
		args = []string{"--fiddle_root", fiddleRoot, "--git_hash", gitHash, "--local"}
	}
	output := &bytes.Buffer{}
	runCmd := &exec.Command{
		Name:      name,
		Args:      args,
		LogStderr: true,
		Stdout:    output,
	}
	if err := exec.Run(runCmd); err != nil {
		return nil, fmt.Errorf("fiddle_run failed to run %#v: %s", *runCmd, err)
	}
	// Parse the output into types.Result.
	res := &types.Result{}
	if err := json.Unmarshal(output.Bytes(), res); err != nil {
		return nil, fmt.Errorf("Failed to decode results from run: %s", err)
	}
	return res, nil
}
