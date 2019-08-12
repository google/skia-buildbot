package util

import (
	"context"
	"time"
)

// ignoreCancelContext is a context.Context implementation which is not
// cancelable, even if the parent context is canceled.
type ignoreCancelContext struct {
	context.Context
}

// See documentation for context.Context interface.
func (ctx *ignoreCancelContext) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

// See documentation for context.Context interface.
func (ctx *ignoreCancelContext) Done() <-chan struct{} {
	return nil
}

// See documentation for context.Context interface.
func (ctx *ignoreCancelContext) Err() error {
	return nil
}

// NonCancelableContext returns a context.Context which cannot be canceled, even
// if its parent is canceled.
func NonCancelableContext(ctx context.Context) context.Context {
	return &ignoreCancelContext{ctx}
}
