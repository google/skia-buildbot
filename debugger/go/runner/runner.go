// Package runner provides funcs to run skiaserve either in or outside of a container.
package runner

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/util"
)

// GetCurrentGitHash is a function that returns the current Git hash that
// skiaserve was built at, usually provided by buildskia.ContinuousBuilder.
type GetCurrentGitHash func() string

// Runner runs skiaserve either in or outside of a chroot jail container.
type Runner struct {
	// workRoot is the directory where we check out versions of Skia.
	//
	// See the description of how workRoot is used by
	// buildskia.ContinuousBuilder.
	workRoot string

	// imageDir is the directory that contains the chroot jail image.
	imageDir string

	// local is true if we are running locally, and thus skiaserve shouldn't be
	// run in a container.
	local bool

	// getHash is a func that returns the last good git hash that skiaserve
	// was built at.
	getHash GetCurrentGitHash
}

// Start a single instance of skiaserve running at the given port.
//
// This func doesn't return until skiaserve exits.
//
// NOTE: When trying to run a binary that exists on a mounted directory under
// nspawn, it will fail with:
//
//    $ sudo systemd-nspawn -D /mnt/pd0/container/ --bind=/mnt/pd0/foo /mnt/pd0/blahblah/someexe
//    Directory /mnt/pd0/container lacks the binary to execute or doesn't look like a binary tree. Refusing.
//
// That's because nspawn is looking for the exe before doing the bindings. The
// fix? A pure hack, insert "xargs --arg-file=/dev/null " before the command
// you want to run. Since xargs exists in the container this will proceed to
// the point of making the bindings and then xargs will be able to execute the
// exe within the container.
func (c *Runner) Start(port int) error {
	hash := c.getHash()
	machine := fmt.Sprintf("debug%05d", port)
	name := "sudo"
	skiaserve := filepath.Join(c.workRoot, "versions", hash, "out", "Release", "skiaserve")
	args := []string{
		"systemd-nspawn", "-D", c.imageDir,
		"--read-only",        // Mount the root file system as read only.
		"--machine", machine, // Give the container a unique name, so we can run fiddles concurrently.
		"--bind-ro", c.workRoot, // Mount workRoot as read-only.
		"xargs", "--arg-file=/dev/null", // See Note above for explanation of xargs.
		skiaserve, "--port", fmt.Sprintf("%d", port), "--hosted",
	}
	if c.local {
		name = skiaserve
		args = []string{"--port", fmt.Sprintf("%d", port), "--hosted"}
	}
	runCmd := &exec.Command{
		Name:      name,
		Args:      args,
		LogStderr: true,
		LogStdout: true,
	}
	if err := exec.Run(runCmd); err != nil {
		return fmt.Errorf("skaiserve failed to run %#v: %s", *runCmd, err)
	}
	glog.Infof("Returned from running skiaserve.")
	return nil
}

// New creates a new Runner.
//
//   workRoot - The directory where we check out versions of Skia.
//   imageDir - The directory that contains the chroot jail image.
//   local - True if we are running locally, and thus skiaserve shouldn't be
//       run in a container.
//   getHash - A func that returns the last good git hash that skiaserve
//       was built at.
func New(workRoot, imageDir string, getHash GetCurrentGitHash, local bool) *Runner {
	return &Runner{
		workRoot: workRoot,
		imageDir: imageDir,
		local:    local,
		getHash:  getHash,
	}
}

// Stop, when called from a different Go routine, will cause the skiaserve
// running on the given port to exit and thus the container to exit.
func Stop(port int) {
	resp, _ := http.Get(fmt.Sprintf("http://localhost:%d/quitquitquit", port))
	if resp != nil && resp.Body != nil {
		util.Close(resp.Body)
	}
}
