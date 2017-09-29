package workerpool

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
)

func TestWorkerPool(t *testing.T) {
	testutils.SmallTest(t)

	// Basic functionality.
	p := New(3)
	count := 0
	mtx := sync.Mutex{}
	for i := 0; i < 5; i++ {
		p.Go(func() {
			mtx.Lock()
			defer mtx.Unlock()
			count++
		})
	}
	p.Wait()
	assert.Equal(t, 5, count)

	// After Wait(), p.Go() and p.Wait() should panic.
	assert.Panics(t, func() {
		p.Go(func() {
			return
		})
	})
	assert.Panics(t, func() {
		p.Wait()
	})

	// Ensure that we can Reset() the pool and then use p.Go() and p.Wait().
	p.Reset()
	p.Go(func() {
		count++
	})
	p.Wait()
	assert.Equal(t, 6, count)

	// Can't reset without calling Wait.
	p.Reset()
	assert.Panics(t, func() {
		p.Reset()
	})
}
