package exec

import (
	"context"
	"syscall"
)

// NoInterruptContext returns a context.Context instance which launches
// subprocesses in a difference process group so that they are not killed when
// this process is killed.
//
// This function is a no-op on Windows.
func NoInterruptContext(ctx context.Context) context.Context {
	parent := getCtx(ctx)
	runFn := func(c *Command) error {
		c.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
		return parent.runFn(c)
	}
	return NewContext(ctx, runFn)
}
