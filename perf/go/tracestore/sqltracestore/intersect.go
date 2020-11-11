package sqltracestore

import (
	"context"
)

const channelSize = 1000

// newIntersect returns a channel of ordered strings that is the intersection
// of the 'inputs' channels of ordered strings.
//
// Ordered strings means the strings are monotonically increasing and there
// are no duplicates.
//
// The context is cancellable and cancelling will close the returned output channel.
//
// You might be tempted to use reflect.Select and listen on all channels at once
// instead of building a tree, but reflection is slow, and as each value
// arrives from a channel the channels you are passing to relect.Select needs
// to change so you need to do slice operations one every string that arrives
// from a channel, which is also slow.
func newIntersect(ctx context.Context, inputs []<-chan traceIDForSQL) <-chan traceIDForSQL {
	// Build a binary tree of channels by calling NewIntersect recursively.
	switch len(inputs) {
	case 0:
		ret := make(chan traceIDForSQL)
		close(ret)
		return ret
	case 1:
		return inputs[0]
	case 2:
		return newIntersect2(ctx, inputs[0], inputs[1])
	default:
		m := len(inputs) / 2
		return newIntersect(ctx, []<-chan traceIDForSQL{
			newIntersect(ctx, inputs[:m]),
			newIntersect(ctx, inputs[m:]),
		})
	}
}

// newIntersect2 returns a channel of strings that represents the ordered
// intersection of the two ordered channels a and b.
//
// The context is cancellable and cancelling will close the returned output channel.
func newIntersect2(ctx context.Context, a, b <-chan traceIDForSQL) <-chan traceIDForSQL {
	out := make(chan traceIDForSQL, channelSize)
	go func() {
		defer close(out)

		var aValue traceIDForSQL = ""
		var bValue traceIDForSQL = ""
		cancel := ctx.Done()
		ok := false
		for {
			if aValue == "" {
				select {
				case aValue, ok = <-a:
					if !ok {
						// Channel is closed, we can't possibly have more matches.

						// Drain the b channel. If we don't then the Union feeding
						// the b channel may never get a chance to see the cancelled
						// context and we get a Go routine leak.
						for range b {
						}
						return
					}
				case <-cancel:
					return
				}
			}

			if bValue == "" {
				select {
				case bValue, ok = <-b:
					if !ok {
						// Channel is closed, we can't possibly have more matches.

						// Drain the a channel. If we don't then the Union feeding
						// the a channel may never get a chance to see the cancelled
						// context and we get a Go routine leak.
						for range a {
						}
						return
					}
				case <-cancel:
					return
				}
			}
			if aValue < bValue {
				aValue = traceIDForSQL("")
			} else if bValue < aValue {
				bValue = ""
			} else if aValue == bValue {
				out <- aValue
				aValue = traceIDForSQL("")
				bValue = traceIDForSQL("")
			}
		}
	}()

	return out
}
