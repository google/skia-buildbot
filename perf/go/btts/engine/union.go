package engine

import (
	"context"
)

// NewUnion takes the ordered values coming from all the 'inputs' channels
// and collates them into a single output channel, removing duplicates
// as they appear.
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
func NewUnion(ctx context.Context, inputs []<-chan string) <-chan string {
	// Build a binary tree of channels by calling NewUnion recursively.
	switch len(inputs) {
	case 0:
		ret := make(chan string)
		close(ret)
		return ret
	case 1:
		return inputs[0]
	case 2:
		return newUnion2(ctx, inputs[0], inputs[1])
	default:
		m := len(inputs) / 2
		return NewUnion(ctx, []<-chan string{
			NewUnion(ctx, inputs[:m]),
			NewUnion(ctx, inputs[m:]),
		})
	}
}

// newUnion2 returns a channel of strings that represents the ordered
// merge of the two ordered channels a and b, removing duplicates as
// they appear.
//
// The context is cancellable and cancelling will close the returned output channel.
func newUnion2(ctx context.Context, a, b <-chan string) <-chan string {
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
						// Channel is closed, just pump the rest of the b to out.
						if bValue != "" {
							out <- bValue
						}
						for v := range b {
							out <- v
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
						// Channel is closed, just pump the rest of the a to out.
						if aValue != "" {
							out <- aValue
						}
						for v := range a {
							out <- v
						}
						return
					}
				case <-cancel:
					return
				}
			}
			if aValue < bValue {
				out <- aValue
				aValue = ""
			} else if bValue < aValue {
				out <- bValue
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
