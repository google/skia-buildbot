package util

import (
	"runtime"
	"sync"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestCondMonitor(t *testing.T) {
	unittest.MediumTest(t)

	// Define the id range and the number of concurrent calls for each id.
	nFnCalls := 50
	nIDs := 50
	mon := NewCondMonitor(1)
	var mutex sync.Mutex
	concurMap := map[int64]int{}
	errCh := make(chan error, nFnCalls*nIDs)
	var wg sync.WaitGroup
	fn := func(id, callID int64) {
		defer wg.Done()
		defer mon.Enter(id).Release()

		mutex.Lock()
		concurMap[id]++
		if concurMap[id] > 1 {
			errCh <- sklog.FmtErrorf("More than one thread with the same ID entered the critical section")
		}
		mutex.Unlock()

		// Give another goroutine a chance to run and simulate a little bit of work.
		runtime.Gosched()
		time.Sleep(10 * time.Millisecond)
		mutex.Lock()
		concurMap[id]--
		mutex.Unlock()
	}

	// Make lots of function calls
	for id := 1; id < nIDs+1; id++ {
		for callIdx := 0; callIdx < nFnCalls; callIdx++ {
			wg.Add(1)
			go fn(int64(id), int64(callIdx))
		}
	}
	wg.Wait()
	close(errCh)

	// Note: This will fail for the first error we encountered. That's ok.
	for err := range errCh {
		assert.NoError(t, err)
	}
}
