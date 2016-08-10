package redisutil

import (
	"crypto/rand"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
)

const (
	Q_NAME            = "mytype"
	Q_NAME_PRIMITIVES = "mytestq"
	N_TASKS           = 1000
	PACKAGE_SIZE      = 1024 * 512
)

// BytesCodec for testing.
type BytesCodec struct{}

func (b BytesCodec) Encode(data interface{}) ([]byte, error) {
	// Make a copy to simulate the generic case.
	return append([]byte(nil), data.([]byte)...), nil
}

func (b BytesCodec) Decode(byteData []byte) (interface{}, error) {
	// Make a copy to simulate the generic case.
	return append([]byte(nil), byteData...), nil
}

//. TODO(stephana): Re-enable when no longer flacky.
func xTestReadThroughCache(t *testing.T) {
	testutils.SkipIfShort(t)
	rp := NewRedisPool(REDIS_SERVER_ADDRESS, REDIS_DB_RTCACHE)
	defer testutils.CloseInTest(t, rp)

	assert.NoError(t, rp.FlushDB())
	randBytes := make([]byte, PACKAGE_SIZE)
	_, err := rand.Read(randBytes)
	assert.NoError(t, err)

	worker := func(priority int64, id string) (interface{}, error) {
		// Create a unique version of the random array.
		return []byte(id + string(randBytes)), nil
	}

	// create a worker queue for a given type
	codec := BytesCodec{}
	qRet, err := NewReadThroughCache(rp, Q_NAME, worker, codec, runtime.NumCPU()-2)
	assert.NoError(t, err)
	q := qRet.(*RedisRTC)
	defer q.shutdown()

	// Wait for 2 seconds to make sure wait for work function times out
	// at least once and queries the queueu directly.
	time.Sleep(1 * time.Second)

	// make sure all results arrive.
	var allDone sync.WaitGroup
	retCh := make(chan interface{}, N_TASKS)
	errCh := make(chan error, N_TASKS)

	for i := 0; i < N_TASKS; i++ {
		allDone.Add(1)
		go func(idx, priority int) {
			id := "id-" + fmt.Sprintf("%04d", idx)
			result, err := q.Get(int64(priority), false, id)
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
	for ret := range retCh {
		assert.IsType(t, []byte(""), ret)

		// Add the prefix size to PACKAGE_SIZE to account for prefix added above.
		assert.Equal(t, PACKAGE_SIZE+7, len(ret.([]byte)))
		found[string(ret.([]byte))] = true
	}

	// Make sure all strings are unique.
	assert.Equal(t, N_TASKS, len(found))
}
