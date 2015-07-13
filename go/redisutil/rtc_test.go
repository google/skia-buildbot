package redisutil

import (
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"testing"

	"go.skia.org/infra/go/testutils"

	assert "github.com/stretchr/testify/require"
)

const (
	Q_NAME            = "mytype"
	Q_NAME_PRIMITIVES = "mytestq"
	N_TASKS           = 10000
)

func TestReadThroughCache(t *testing.T) {
	testutils.SkipIfShort(t)

	runtime.GOMAXPROCS(runtime.NumCPU() - 1)

	rp := NewRedisPool(REDIS_SERVER_ADDRESS, REDIS_DB_RTCACHE)
	assert.Nil(t, rp.FlushDB())

	worker := func(priority int64, id string) (interface{}, error) {
		// Run a few calculations in a loop.
		result := 0
		for i := 0; i < 10; i++ {
			result += i
		}

		// Do the work
		return id + "-" + strconv.Itoa(result), nil
	}

	// create a worker queue for a given type
	codec := StringCodec{}
	qRet, err := NewReadThroughCache(rp, Q_NAME, nil, codec, runtime.NumCPU()-2)
	assert.Nil(t, err)
	q := qRet.(*RedisRTC)

	// make sure all results arrive.
	var allDone sync.WaitGroup
	retCh := make(chan interface{}, N_TASKS)
	errCh := make(chan error, N_TASKS)

	for i := 0; i < N_TASKS; i++ {
		allDone.Add(1)
		go func(idx, priority int) {
			id := "id-" + strconv.Itoa(idx)
			result, err := q.Get(int64(priority), false, id)
			if err != nil {
				errCh <- err
			} else {
				retCh <- result
			}

			allDone.Done()
		}(i, i)
	}

	q.worker = worker
	assert.Nil(t, q.startWorkers(runtime.NumCPU()-2))
	allDone.Wait()

	close(errCh)
	close(retCh)

	if len(errCh) > 0 {
		for err := range errCh {
			fmt.Printf("Error: %s", err)
		}
		assert.True(t, false)
	}

	assert.Equal(t, 0, len(errCh))
	found := make(map[string]bool, N_TASKS)
	for ret := range retCh {
		assert.IsType(t, "", ret)
		found[ret.(string)] = true
	}

	assert.Equal(t, N_TASKS, len(found))
}
