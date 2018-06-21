package exec

import "context"

// NoInterruptContext returns a context.Context instance which launches
// subprocesses in a difference process group so that they are not killed when
// this process is killed.
//
// This function is a no-op on Windows.
func NoInterruptContext(ctx context.Context) context.Context {
	return ctx
}
