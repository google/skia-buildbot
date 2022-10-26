// Package foundrybotrunner starts Foundry Bot to handle RBE requests and keeps it running.
package foundrybotrunner

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// Runner is an opaque type which, once instantiated, implies a Foundry Bot binary has been found.
type Runner struct {
	// path is the absolute path to a copy of Foundry Bot.
	path string
}

// New constructs a fresh Foundry Bot Runner. It exists to surface errors (like Foundry Bot being
// missing) before RunUntilCancelled is called, generally on a separate goroutine.
func New() (*Runner, error) {
	path, err := botPath()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &Runner{path: path}, nil
}

// botPath returns a path to the Foundry Bot binary sitting next to test_machine_monitor, returning
// an error if the former doesn't exist.
func botPath() (string, error) {
	tmm, err := os.Executable()
	if err != nil {
		return "", skerr.Wrapf(err, "couldn't compute path to Foundry Bot")
	}
	foundryBot := filepath.Join(filepath.Dir(tmm), "bot.1")
	if _, err := os.Stat(foundryBot); errors.Is(err, fs.ErrNotExist) {
		return "", skerr.Wrapf(err, "Foundry Bot not found at %s", foundryBot)
	}
	if err != nil {
		return "", skerr.Wrapf(err, "failed to stat() bot.1 in the same directory as test_machine_monitor")
	}
	return foundryBot, nil
}

// RunUntilCancelled runs the copy of Foundry Bot sitting next to test_machine_monitor, restarts it
// if it exits (whether successfully or with an error), and kills it if the passed-in context is
// cancelled or times out. Returns if the context comes to an end.
func (r *Runner) RunUntilCancelled(ctx context.Context) error {
	for {
		err := ctx.Err()
		if err != nil {
			return skerr.Wrapf(err, "context was cancelled or timed out")
		}
		cmd := executil.CommandContext(
			ctx,
			r.path,
			"-service_address=remotebuildexecution.googleapis.com:443",
			"-instance_name=projects/skia-rbe/instances/default_instance",
			"session",
			"-sandbox=none")
		sklog.Infof("Starting %q", cmd.String())
		if err := cmd.Run(); err == nil {
			sklog.Errorf("Foundry Bot exited without an error, which is unexpected.")
		}
	}
}
