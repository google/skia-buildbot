package rtcache

import (
	"container/heap"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

const (
	N_TASKS      = 1000
	PACKAGE_SIZE = 1024 * 512
)

func TestPriorityQueue(t *testing.T) {
	testutils.SmallTest(t)
	pq := &priorityQueue{}
	vals := []*workItem{
		{id: "0", priority: 0},
		{id: "1", priority: 1},
		{id: "2", priority: 2},
		{id: "3", priority: 3},
		{id: "4", priority: 4},
		{id: "5", priority: 5},
		{id: "6", priority: 6},
		{id: "7", priority: 7},
		{id: "8", priority: 8},
		{id: "9", priority: 9},
	}
	indices := rand.Perm(len(vals))
	for _, idx := range indices {
		heap.Push(pq, vals[idx])
	}
	assert.Equal(t, len(vals), len(*pq))
	result := ""
	for len(*pq) > 0 {
		item := heap.Pop(pq).(*workItem)
		result += item.id
	}
	assert.Equal(t, "0123456789", result)
}

func TestReadThroughCache(t *testing.T) {
	testutils.MediumTest(t)

	randBytes := make([]byte, PACKAGE_SIZE)
	_, err := rand.Read(randBytes)
	assert.NoError(t, err)

	worker := func(priority int64, id string) (interface{}, error) {
		// Create a unique version of the random array.
		return []byte(id + string(randBytes)), nil
	}

	// create a worker queue for a given type
	q, err := New(worker, 10000, runtime.NumCPU()-2)
	assert.NoError(t, err)

	// make sure all results arrive.
	var allDone sync.WaitGroup
	retCh := make(chan interface{}, N_TASKS)
	errCh := make(chan error, N_TASKS)

	for i := 0; i < N_TASKS; i++ {
		allDone.Add(1)
		go func(idx, priority int) {
			// time.Sleep(time.Second * 5)
			id := "id-" + fmt.Sprintf("%07d", idx)
			result, err := q.Get(int64(priority), id)
			if err != nil {
				errCh <- err
			} else {
				retCh <- result
			}

			allDone.Done()
		}(i, i)
	}
	allDone.Wait()

	close(errCh)
	close(retCh)

	if len(errCh) > 0 {
		for err := range errCh {
			fmt.Printf("Error: %s", err)
		}
		assert.Fail(t, "Received above error messages.")
	}

	assert.Equal(t, 0, len(errCh))
	found := make(map[string]bool, N_TASKS)
	resultIds := make([]string, 0, len(retCh))
	resultVals := make([][]byte, 0, len(retCh))
	for ret := range retCh {
		assert.IsType(t, []byte(""), ret)
		resultVal := ret.([]byte)
		resultIds = append(resultIds, string(resultVal[:10]))
		resultVals = append(resultVals, resultVal)

		// Add the prefix size to PACKAGE_SIZE to account for prefix added above.
		assert.Equal(t, PACKAGE_SIZE+10, len(ret.([]byte)))
		found[string(ret.([]byte))] = true
	}

	// Make sure all strings are unique.
	assert.Equal(t, N_TASKS, len(found))
	for i, resultID := range resultIds {
		val, err := q.Get(0, resultID)
		assert.NoError(t, err)
		assert.Equal(t, resultVals[i], val)
	}

	assert.True(t, q.Contains("id-0000000"))
	assert.False(t, q.Contains("some-random-never-before-seen-key"))
	q.(*MemReadThroughCache).shutdown()
}

func TestErrHandling(t *testing.T) {
	testutils.SmallTest(t)
	errWorker := func(priority int64, id string) (interface{}, error) {
		return nil, fmt.Errorf("id: %v", time.Now())
	}

	testID := "id-1"
	q, err := New(errWorker, 10000, runtime.NumCPU())
	assert.NoError(t, err)
	_, err = q.Get(1, testID)
	assert.Error(t, err)
	time.Sleep(time.Millisecond)
	_, err = q.Get(1, testID)
	_, errTwo := q.Get(1, testID)
	assert.Error(t, errTwo)
	assert.Equal(t, err, errTwo)
	q.(*MemReadThroughCache).errCache.Flush()
	time.Sleep(time.Millisecond)
	_, errThree := q.Get(1, testID)
	assert.Error(t, errThree)
	assert.NotEqual(t, err, errThree)
}
