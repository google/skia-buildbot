package engine

import (
	"context"
)

// NewIntersect returns a channel of ordered strings that is the intersection
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
func NewIntersect(ctx context.Context, inputs []<-chan string) <-chan string {
	// Build a binary tree of channels by calling NewIntersect recursively.
	switch len(inputs) {
	case 0:
		ret := make(chan string)
		close(ret)
		return ret
	case 1:
		return inputs[0]
	case 2:
		return newIntersect2(ctx, inputs[0], inputs[1])
	default:
		m := len(inputs) / 2
		return NewIntersect(ctx, []<-chan string{
			NewIntersect(ctx, inputs[:m]),
			NewIntersect(ctx, inputs[m:]),
		})
	}
}

// newIntersect2 returns a channel of strings that represents the ordered
// intersection of the two ordered channels a and b.
//
// The context is cancellable and cancelling will close the returned output channel.
func newIntersect2(ctx context.Context, a, b <-chan string) <-chan string {
	out := make(chan string, QUERY_ENGINE_CHANNEL_SIZE)
	go func() {
		defer close(out)

		aValue := ""
		bValue := ""
		cancel := ctx.Done()
		ok := false
		for {
			if aValue == "" {
				select {
				case aValue, ok = <-a:
					if !ok {
						// Channel is closed, we can't possibly have more matches.
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
						return
					}
				case <-cancel:
					return
				}
			}
			if aValue < bValue {
				aValue = ""
			} else if bValue < aValue {
				bValue = ""
			} else if aValue == bValue {
				out <- aValue
				aValue = ""
				bValue = ""
			}
		}
	}()

	return out
}
