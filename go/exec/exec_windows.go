// +build windows
package exec

import (
	"context"

	"go.skia.org/infra/go/util"
)

const WHICH = "where"

// NoInterruptContext returns a context.Context instance which launches
// subprocesses in a difference process group so that they are not killed when
// this process is killed.
//
// On Windows, this function just returns util.WithoutCancel(ctx).
func NoInterruptContext(ctx context.Context) context.Context {
	return util.WithoutCancel(ctx)
}
