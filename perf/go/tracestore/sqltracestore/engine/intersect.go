package engine

import (
	"context"
)

// NewIntersect returns a channel of ordered int64s that is the intersection
// of the 'inputs' channels of ordered int64s.
//
// Ordered int64s means the int64s are monotonically increasing and there
// are no duplicates.
//
// The context is cancellable and cancelling will close the returned output channel.
//
// You might be tempted to use reflect.Select and listen on all channels at once
// instead of building a tree, but reflection is slow, and as each value
// arrives from a channel the channels you are passing to relect.Select needs
// to change so you need to do slice operations one every int64 that arrives
// from a channel, which is also slow.
func NewIntersect(ctx context.Context, inputs []<-chan int64) <-chan int64 {
	// Build a binary tree of channels by calling NewIntersect recursively.
	switch len(inputs) {
	case 0:
		ret := make(chan int64)
		close(ret)
		return ret
	case 1:
		return inputs[0]
	case 2:
		return newIntersect2(ctx, inputs[0], inputs[1])
	default:
		m := len(inputs) / 2
		return NewIntersect(ctx, []<-chan int64{
			NewIntersect(ctx, inputs[:m]),
			NewIntersect(ctx, inputs[m:]),
		})
	}
}

// newIntersect2 returns a channel of int64s that represents the ordered
// intersection of the two ordered channels a and b.
//
// The context is cancellable and cancelling will close the returned output channel.
func newIntersect2(ctx context.Context, a, b <-chan int64) <-chan int64 {
	out := make(chan int64, QueryEngineChannelSize)
	go func() {
		defer close(out)

		aValue := int64(-1)
		bValue := int64(-1)
		cancel := ctx.Done()
		ok := false
		for {
			if aValue == -1 {
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

			if bValue == -1 {
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
				aValue = int64(-1)
			} else if bValue < aValue {
				bValue = int64(-1)
			} else if aValue == bValue {
				out <- aValue
				aValue = int64(-1)
				bValue = int64(-1)
			}
		}
	}()

	return out
}
