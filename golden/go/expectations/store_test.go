package expectations

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
)

func TestEventHandler_SynchronousHandler_CallbacksCalledInOrder(t *testing.T) {
	unittest.SmallTest(t)

	eh := NewEventDispatcherForTesting()

	expectedDelta := ID{
		Grouping: "abc",
		Digest:   "def",
	}

	counter := 0
	eh.ListenForChange(func(d ID) {
		assert.Equal(t, d, expectedDelta)
		assert.Equal(t, 0, counter)
		// This change of Grouping shouldn't affect future calls
		d.Grouping = "Oh no, this changed; hopefully it doesn't mess up future tests"
		counter++
	})
	eh.ListenForChange(func(d ID) {
		assert.Equal(t, d, expectedDelta)
		assert.Equal(t, 1, counter)
		counter++
	})
	// Make sure callbacks haven't happened yet.
	require.Equal(t, 0, counter)

	// Send a copy to notify to make sure mutations don't affect anything
	eh.NotifyChange(ID{
		Grouping: "abc",
		Digest:   "def",
	})
	assert.Equal(t, 2, counter)
}

func TestEventHandler_AsynchronousHandler_CallbacksCalledMultipleTimes(t *testing.T) {
	unittest.SmallTest(t)

	eh := NewEventDispatcher()

	expectedDelta := ID{
		Grouping: "abc",
		Digest:   "def",
	}

	firstCallbackCount := int32(0)
	secondCallbackCount := int32(0)

	wg := sync.WaitGroup{}
	wg.Add(4)

	eh.ListenForChange(func(d ID) {
		defer wg.Done()
		assert.Equal(t, d, expectedDelta)
		atomic.AddInt32(&firstCallbackCount, 1)
	})
	eh.ListenForChange(func(d ID) {
		defer wg.Done()
		assert.Equal(t, d, expectedDelta)
		atomic.AddInt32(&secondCallbackCount, 1)
	})

	// Send two notifications asynchronously, to make sure there aren't any race conditions
	// (as would be detected by go test -race).
	go eh.NotifyChange(ID{
		Grouping: "abc",
		Digest:   "def",
	})
	go eh.NotifyChange(ID{
		Grouping: "abc",
		Digest:   "def",
	})

	wg.Wait()
	// Make sure each callback was called exactly twice
	assert.Equal(t, int32(2), firstCallbackCount)
	assert.Equal(t, int32(2), secondCallbackCount)
}
