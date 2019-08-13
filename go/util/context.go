package util

import (
	"context"
	"time"
)

// withoutCancelContext is a context.Context implementation which is not
// cancelable, even if the parent context is canceled.
type withoutCancelContext struct {
	context.Context
}

// See documentation for context.Context interface.
func (ctx *withoutCancelContext) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

// See documentation for context.Context interface.
func (ctx *withoutCancelContext) Done() <-chan struct{} {
	return nil
}

// See documentation for context.Context interface.
func (ctx *withoutCancelContext) Err() error {
	return nil
}

// WithoutCancel returns a context.Context which cannot be canceled, even
// if its parent is canceled.
func WithoutCancel(ctx context.Context) context.Context {
	return &withoutCancelContext{ctx}
}
