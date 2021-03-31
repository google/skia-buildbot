// Package now provides a function to return the current time that is
// also easily overridden for testing.
package now

import (
	"context"
	"fmt"
	"time"
)

type ContextKeyType string

// ContextKey is used by tests to make the time deterministic.
//
// That is, in a test, you can write a value into a context to use as the return
// value of Now().
//
//    var mockTime = time.Unix(0, 12).UTC()
//    ctx = context.WithValue(ctx, now.ContextKey, mockTime)
//
// The value set can also be a function that returns a time.Time.
//
//    var monotonicTime int64 = 0
//    var mockTimeProvider = func() {
//      monotonicTime += 1
//	    return time.Unix(monotonicTime, 0).UTC()
//    }
//    ctx = context.WithValue(ctx, now.ContextKey, now.NowProvider(mockTimeProvider))
//
const ContextKey ContextKeyType = "overwriteNow"

// NowProvider is the type of function that can also be passed as a context
// value. The function will be evaluated every time Now() is called with that
// context. NowProvider should be threadsafe if the context is used across
// threads.
type NowProvider func() time.Time

// Now returns the current time or the time from the context.
func Now(ctx context.Context) time.Time {
	if ts := ctx.Value(ContextKey); ts != nil {
		switch v := ts.(type) {
		case NowProvider:
			return v()
		case time.Time:
			return v
		default:
			panic(fmt.Sprintf("Unknown value for ContextKey: %v", v))
		}
	}
	return time.Now()
}
