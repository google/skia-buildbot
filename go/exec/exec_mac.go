//go:build darwin

package exec

import (
	"context"
)

// NoInterruptContext returns a context.Context instance which launches
// subprocesses in a difference process group so that they are not killed when
// this process is killed.
//
// On Mac, this function just returns withoutCancel(ctx).
// TODO(kjlubick, borenet) is this sufficient?
func NoInterruptContext(ctx context.Context) context.Context {
	return withoutCancel(ctx)
}
