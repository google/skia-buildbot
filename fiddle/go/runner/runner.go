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
	"path/filepath"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/fiddle/go/linenumbers"
	"go.skia.org/infra/fiddle/go/types"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/util"
)

const (
	// PREFIX is a format string for the code that makes it compilable.
	PREFIX = `#include "fiddle_main.h"
DrawOptions GetDrawOptions() {
  static const char *path = %s; // Either a string, or 0.
  return DrawOptions(%d, %d, true, true, %v, %v, %v, %v, %v, path);
}

%s
`
)

var (
	runTotal    = metrics2.GetCounter("run-total", nil)
	runFailures = metrics2.GetCounter("run-failures", nil)
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
	pdf := true
	skp := true
	if opts.Animated {
		pdf = false
		skp = false
	}
	return fmt.Sprintf(PREFIX, sourceImage, opts.Width, opts.Height, pdf, skp, opts.SRGB, opts.F16, opts.TextOnly, code)
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
	return nil
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
// Returns a temp directory. Depending on 'local' this is where the 'draw.cpp' file was written.
func WriteDrawCpp(checkout, fiddleRoot, code string, opts *types.Options) (string, error) {
	code = prepCodeToCompile(fiddleRoot, code, opts)
	dstDir := filepath.Join(fiddleRoot, "src")
	var err error
	tmpDir := filepath.Join(fiddleRoot, "tmp")
	if err := os.MkdirAll(tmpDir, os.ModePerm); err != nil && err != os.ErrExist {
		return "", fmt.Errorf("Failed to create temp dir for draw.cpp: %s", err)
	}
	dstDir, err = ioutil.TempDir(tmpDir, "code")
	sklog.Infof("Created tmp dir: %s %s", dstDir, err)
	if err != nil {
		return "", fmt.Errorf("Failed to create temp dir for draw.cpp: %s", err)
	}

	drawPath := filepath.Join(dstDir, "upper", "skia", "tools", "fiddle")
	if err := os.MkdirAll(drawPath, 0755); err != nil {
		return dstDir, fmt.Errorf("failed to create dir %q: %s", drawPath, err)
	}

	sklog.Infof("About to write to: %s", drawPath)
	w, err := os.Create(filepath.Join(drawPath, "draw.cpp"))
	sklog.Infof("Create: %v %v", *w, err)
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
func Run(checkout, fiddleRoot, depotTools, gitHash string, local bool, tmpDir string, opts *types.Options) (*types.Result, error) {
	/*
		Do the equivalent of the following bash script that creates the overlayfs
		mount.

			#!/bin/bash

			set -e -x

			RUNID=runid2222
			GITHASH=c34a946d5a975ba8b8cd51f79b55174a5ec0f99f
			FIDDLE_ROOT=/usr/local/google/home/jcgregorio/projects/temp
			TMP_DIR=${FIDDLE_ROOT}/tmp/${RUNID}

			mkdir --parents ${TMP_DIR}/upper/skia/tools/fiddle
			mkdir ${TMP_DIR}/work
			mkdir ${TMP_DIR}/overlay
			cp ${TMP_DIR}/draw.cpp ${TMP_DIR}/upper/skia/tools/fiddle/draw.cpp

			LOWER=${FIDDLE_ROOT}/versions/${GITHASH}
			UPPER=${TMP_DIR}/upper
			WORK=${TMP_DIR}/work
			OVERLAY=${TMP_DIR}/overlay

			sudo mount -t overlay -o lowerdir=$LOWER,upperdir=$UPPER,workdir=$WORK none ${OVERLAY}

	*/
	runTotal.Inc(1)
	upper := filepath.Join(tmpDir, "upper")
	work := filepath.Join(tmpDir, "work")
	overlay := filepath.Join(tmpDir, "overlay")
	lower := filepath.Join(fiddleRoot, "versions", gitHash)
	if err := os.MkdirAll(upper, 0755); err != nil {
		return nil, fmt.Errorf("fiddle_run failed to create dir %q: %s", upper, err)
	}
	if err := os.MkdirAll(work, 0755); err != nil {
		return nil, fmt.Errorf("fiddle_run failed to create dir %q: %s", work, err)
	}
	if err := os.MkdirAll(overlay, 0755); err != nil {
		return nil, fmt.Errorf("fiddle_run failed to create dir %q: %s", overlay, err)
	}
	mounttype := "overlay"
	if local {
		mounttype = "overlayfs"
	}
	name := "sudo"
	args := []string{
		"mount", "-t", mounttype,
		"-o",
		fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lower, upper, work),
		"none",
		overlay,
	}
	output := &bytes.Buffer{}
	mountCmd := &exec.Command{
		Name:      name,
		Args:      args,
		LogStderr: true,
		Stdout:    output,
	}
	if err := exec.Run(mountCmd); err != nil {
		return nil, fmt.Errorf("mount failed to run %#v: %s", *mountCmd, err)
	}

	// Queue up the umount in a defer.
	defer func() {
		name = "sudo"
		args = []string{
			"umount", overlay,
		}
		output = &bytes.Buffer{}
		umountCmd := &exec.Command{
			Name:      name,
			Args:      args,
			LogStderr: true,
			Stdout:    output,
		}

		if err := exec.Run(umountCmd); err != nil {
			sklog.Errorf("umount failed to run %#v: %s", *umountCmd, err)
		}
	}()

	// Run fiddle_run.
	name = filepath.Join(fiddleRoot, "bin", "fiddle_run")
	if local {
		name = "fiddle_run"
	}
	args = []string{"--fiddle_root", fiddleRoot, "--checkout", overlay, "--git_hash", gitHash, "--alsologtostderr"}
	args = append(args, "--duration", fmt.Sprintf("%f", opts.Duration))
	if local {
		args = append(args, "--local")
	}
	output = &bytes.Buffer{}
	runCmd := &exec.Command{
		Name:      name,
		Args:      args,
		LogStderr: true,
		Stdout:    output,
	}
	if err := exec.Run(runCmd); err != nil {
		runFailures.Inc(1)
		return nil, fmt.Errorf("fiddle_run failed to run %#v: %s", *runCmd, err)
	}

	// Parse the output into types.Result.
	res := &types.Result{}
	if err := json.Unmarshal(output.Bytes(), res); err != nil {
		sklog.Errorf("Received erroneous output: %q", output.String())
		return nil, fmt.Errorf("Failed to decode results from run: %s", err)
	}
	return res, nil
}
