package ssh

import (
	"context"
	"os/exec"
	"time"

	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/skerr"
)

const (
	commandTimeout = 20 * time.Second // chosen arbitrarily
)

type SSH interface {
	Run(ctx context.Context, userIP, cmd string, args ...string) (string, error)
}

// ExeImpl runs SSH via an executable that is assumed to be on the PATH.
type ExeImpl struct{}

func (e ExeImpl) Run(ctx context.Context, userIP, cmd string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	xargs := append([]string{"-oConnectTimeout=15", "-oBatchMode=yes",
		"-t", "-t", // These might not work on Windows
		userIP, cmd}, args...)
	cc := executil.CommandContext(ctx, "ssh", xargs...)
	b, err := cc.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			err = skerr.Wrapf(err, "ssh failed with stderr: %q", ee.Stderr)
		}
		return "", err
	}
	return string(b), nil
}
