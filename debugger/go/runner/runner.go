// Package runner provides funcs to run skiaserve either in or outside of a container.
package runner

import (
	"context"
	"fmt"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	SKIASERVE = "/bin/skiaserve"
)

// Runner runs skiaserve.
type Runner struct{}

// Start a single instance of skiaserve running at the given port.
//
// This func doesn't return until skiaserve exits.
//
func (c *Runner) Start(ctx context.Context, port int) error {
	runCmd := &exec.Command{
		Name:      SKIASERVE,
		Args:      []string{"--port", fmt.Sprintf("%d", port), "--source", "", "--hosted"},
		LogStderr: true,
		LogStdout: true,
	}
	if err := exec.Run(ctx, runCmd); err != nil {
		return fmt.Errorf("skaiserve failed to run %#v: %s", *runCmd, err)
	}
	sklog.Infof("Returned from running skiaserve.")
	return nil
}

// New creates a new Runner.
//
func New() *Runner {
	return &Runner{}
}

// Stop, when called from a different Go routine, will cause the skiaserve
// running on the given port to exit and thus the container to exit.
func Stop(port int) {
	client := httputils.NewTimeoutClient()
	resp, _ := client.Get(fmt.Sprintf("http://localhost:%d/quitquitquit", port))
	if resp != nil && resp.Body != nil {
		util.Close(resp.Body)
	}
}
