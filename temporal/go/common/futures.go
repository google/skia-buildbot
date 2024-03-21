package common

import (
	"go.temporal.io/sdk/workflow"
)

// NewFutureWithFutures returns a future that waits for all the given futures to be fulfilled.
//
// The given list can contain a nil future, which will be a no-op. A new future will always be
// returned regardless of the given list containing any pending future.
// The context error will be populated to the returned future but it doesn't have any returned
// values from the given list of futures.
func NewFutureWithFutures(ctx workflow.Context, futures ...workflow.Future) workflow.Future {
	ct := 0
	f, s := workflow.NewFuture(ctx)
	sel := workflow.NewSelector(ctx)

	for _, f := range futures {
		if f == nil {
			continue
		}
		ct++
		sel.AddFuture(f, func(f workflow.Future) {})
	}

	workflow.Go(ctx, func(gCtx workflow.Context) {
		for i := 0; i < ct; i++ {
			sel.Select(gCtx)
		}
		s.Set(nil, gCtx.Err())
	})
	return f
}
