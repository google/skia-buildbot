// +build !windows

package exec

import (
	"context"
	"syscall"

	"go.skia.org/infra/go/util"
)

const WHICH = "which"

// NoInterruptContext returns a context.Context instance which launches
// subprocesses in a difference process group so that they are not killed when
// this process is killed.
//
// On Windows, this function just returns util.WithoutCancel(ctx).
func NoInterruptContext(ctx context.Context) context.Context {
	parent := getCtx(ctx)
	runFn := func(ctx context.Context, c *Command) error {
		c.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
		return parent.runFn(ctx, c)
	}
	return NewContext(util.WithoutCancel(ctx), runFn)
}
