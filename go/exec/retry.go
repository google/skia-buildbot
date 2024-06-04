package exec

import (
	"context"

	"github.com/cenkalti/backoff/v4"
)

// WithRetryContext enables retries with exponential backoff according to the
// given settings.
func WithRetryContext(ctx context.Context, settings backoff.BackOff) context.Context {
	parent := getCtx(ctx)
	runFn := func(ctx context.Context, c *Command) error {
		run := func() error {
			return parent.runFn(ctx, c)
		}
		return backoff.Retry(run, settings)
	}
	return NewContext(ctx, runFn)
}
