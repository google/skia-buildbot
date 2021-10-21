//go:build !windows
// +build !windows

package exec

import (
	"context"
	"syscall"
)

// NoInterruptContext returns a context.Context instance which launches
// subprocesses in a difference process group so that they are not killed when
// this process is killed.
//
// On Windows, this function just returns withoutCancel(ctx).
func NoInterruptContext(ctx context.Context) context.Context {
	parent := getCtx(ctx)
	runFn := func(ctx context.Context, c *Command) error {
		c.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
		return parent.runFn(ctx, c)
	}
	return NewContext(withoutCancel(ctx), runFn)
}
