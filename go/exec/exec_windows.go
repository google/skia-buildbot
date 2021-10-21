//go:build windows
// +build windows

package exec

import (
	"context"
)

// NoInterruptContext returns a context.Context instance which launches
// subprocesses in a difference process group so that they are not killed when
// this process is killed.
//
// On Windows, this function just returns withoutCancel(ctx).
func NoInterruptContext(ctx context.Context) context.Context {
	return withoutCancel(ctx)
}
