package recentschannel

import (
	"fmt"
	"sync"
)

// recentschannel.Ch is a buffered channel on which sending never blocks (though receiving still
// may). A Send() to a full Ch evicts the oldest item in the buffer, leaving the most recent ones.
//
// recentschannel.Ch is useful for receiving nondeterministic heartbeats, such as results of network
// polls, which cannot be replaced with Tickers without incurring latency. Unlike with normal
// channels, there is no risk that a heartbeat sender will block on a full channel until the
// listener completes some possibly lengthy work. The advantage over ordinary nonblocking sends is
// that the receiver sees one of the newest values (THE newest if channel capacity is 1) rather than
// the oldest.
type Ch[T any] struct {
	ch    chan T
	mutex sync.Mutex
}

func New[T any](size int) *Ch[T] {
	if size < 1 {
		panic(fmt.Sprintf("recentschannel.Ch size must be at least 1; got %d.", size))
		// Otherwise, we would necessarily block on sends.
	}
	return &Ch[T]{
		ch: make(chan T, size),
	}
}

func (t *Ch[T]) Send(value T) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if len(t.ch) == cap(t.ch) {
		// Channel full. Dump 1 entry on the floor to make room for new one. Don't block if somebody
		// else beat us to it.
		select {
		case <-t.ch:
		default:
		}
	}
	t.ch <- value
}

func (t *Ch[T]) Recv() <-chan T {
	return t.ch
}
