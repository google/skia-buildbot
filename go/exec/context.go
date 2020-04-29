package exec

import (
	"context"
	"time"
)

// withoutCancelContext is a context.Context implementation which is not
// cancelable, even if the parent context is canceled.
type withoutCancelContext struct {
	context.Context
}

// Deadline implements the context.Context interface.
func (ctx *withoutCancelContext) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

// Done implements the context.Context interface.
func (ctx *withoutCancelContext) Done() <-chan struct{} {
	return nil
}

// Err implements the context.Context interface.
func (ctx *withoutCancelContext) Err() error {
	return nil
}

// withoutCancel returns a context.Context which cannot be canceled, even
// if its parent is canceled.
func withoutCancel(ctx context.Context) context.Context {
	return &withoutCancelContext{ctx}
}
