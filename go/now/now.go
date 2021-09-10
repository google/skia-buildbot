// Package now provides a function to return the current time that is
// also easily overridden for testing.
package now

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type contextKeyType string

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
//    var mockTimeProvider = func() time.Time {
//      monotonicTime += 1
//	    return time.Unix(monotonicTime, 0).UTC()
//    }
//    ctx = context.WithValue(ctx, now.ContextKey, now.NowProvider(mockTimeProvider))
//
const ContextKey contextKeyType = "overwriteNow"

// NowProvider is the type of function that can also be passed as a context
// value. The function will be evaluated every time Now() is called with that
// context. NowProvider should be threadsafe if the context is used across
// threads.
// Clients that need the time to vary throughout tests should probably use TimeTravelCtx
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

// TimeTravelCtx is a test utility that makes it easy to change the apparent time. It embeds a
// context that contains a NowProvider to overwrite the time returned by now.Now(ctx). As an
// example of how this might be used in a test:
//     ctx := now.TimeTravelingContext(tsOne)
//     result1 := myTestFunction(ctx, "param one")
//     // simulate fast forwarding 2 minutes
//     ctx.SetTime(tsOne.Add(2 * time.Minute))
//     result2 := myTestFunction(ctx, "another param")
//     // do assertions on result1 and result2
type TimeTravelCtx struct {
	context.Context

	mutex sync.RWMutex
	ts    time.Time
}

// TimeTravelingContext returns a *TimeTravelCtx, using the given time and the background context.
func TimeTravelingContext(start time.Time) *TimeTravelCtx {
	t := &TimeTravelCtx{
		ts: start,
	}
	t.Context = context.WithValue(context.Background(), ContextKey, NowProvider(t.now))
	return t
}

// now() is a thread-safe NowProvider.
func (t *TimeTravelCtx) now() time.Time {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.ts
}

// SetTime updates the underlying time that will be returned by the embedded context's NowProvider.
// It is thread-safe.
func (t *TimeTravelCtx) SetTime(newTime time.Time) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.ts = newTime
}

// WithContext replaces the embedded context with one derived from the passed in context.
// It is thread-safe, but tests should strive to use it in a non-threaded way for simplicity.
func (t *TimeTravelCtx) WithContext(ctx context.Context) *TimeTravelCtx {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.Context = context.WithValue(ctx, ContextKey, NowProvider(t.now))
	return t
}
