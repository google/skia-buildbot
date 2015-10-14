package redisutil

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/skia-dev/glog"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/rtcache"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

const (
	// TEST_DATA_DIR  is the directory with data used for benchmarks.
	TEST_DATA_DIR = "./benchdata"

	// TEST_DATA_STORAGE_PATH is the folder in the test data bucket.
	// See go/testutils for details.
	TEST_DATA_STORAGE_PATH = "common-testdata/redislru-benchdata.tar.gz"

	// REDIS_SERVER_ADDRESS is the address of the redis server used for testing.
	REDIS_SERVER_ADDRESS = "127.0.0.1:6379"

	// REDIS_DB_RTCACHE is the database used in the readthrough cache test.
	REDIS_DB_RTCACHE = 1

	// REDIS_DB_PRIMITIVE_TESTS is the database used in testing the Redis primitives.
	REDIS_DB_PRIMITIVE_TESTS = 2
)

type TestStruct struct {
	NumDiffPixels     int
	PixelDiffPercent  float32
	PixelDiffFilePath string
	// Contains the maximum difference between the images for each R/G/B channel.
	MaxRGBADiffs []int
	// True if the dimensions of the compared images are different.
	DimDiffer bool
}

func TestRedisLRUCache(t *testing.T) {
	testutils.SkipIfShort(t)
	cache := NewRedisLRUCache("localhost:6379", 1, "test-di", util.UnitTestCodec())
	util.UnitTestLRUCache(t, cache)
}

func BenchmarkBigDataset(b *testing.B) {
	// Download the testdata and remove the testdata directory at the end.
	err := testutils.DownloadTestDataArchive(b, TEST_DATA_STORAGE_PATH, TEST_DATA_DIR)
	assert.Nil(b, err, "Unable to download testdata.")
	defer func() {
		util.LogErr(os.RemoveAll(TEST_DATA_DIR))
	}()

	// Load the data
	fileInfos, err := ioutil.ReadDir(TEST_DATA_DIR)
	assert.Nil(b, err)

	results := make(chan interface{}, len(fileInfos))
	var codec TestStructCodec
	counter := 0
	for _, fi := range fileInfos {
		if strings.HasSuffix(fi.Name(), ".json") {
			go func(fName string) {
				f, err := os.Open(fName)
				if err != nil {
					glog.Fatalf("Unable to open file %s", fName)
				}

				content, err := ioutil.ReadAll(f)
				if err != nil {
					glog.Fatalf("Unable to read file %s", fName)
				}

				v, err := codec.Decode(content)
				if err != nil {
					glog.Fatalf("Unable to decode file %s", fName)
				}
				if _, ok := v.(*TestStruct); !ok {
					glog.Fatalln("Expected to get instance of TestStruct")
				}

				// Store the filepath in this field to use as cache key.
				ts := v.(*TestStruct)
				ts.PixelDiffFilePath = fName
				results <- ts
			}(filepath.Join("./benchdata", fi.Name()))
			counter++
		}
	}

	groundTruth := make(map[string]interface{}, counter)
	cache := NewRedisLRUCache("localhost:6379", 1, "di", TestStructCodec(0))
	rlruCache := cache.(*RedisLRUCache)
	rlruCache.Purge()

	for i := 0; i < counter; i++ {
		ret := <-results
		ts := ret.(*TestStruct)
		groundTruth[ts.PixelDiffFilePath] = ret
	}

	glog.Infof("Done importing %d files. Starting bench.", len(groundTruth))

	b.ResetTimer()
	for k, v := range groundTruth {
		cache.Add(k, v)
	}

	assert.Equal(b, len(groundTruth), cache.Len())
	counter = 0
	for k, v := range groundTruth {
		found, ok := cache.Get(k)
		assert.True(b, ok)
		assert.Equal(b, v, found)
		counter++
		// if (counter % 1000) == 0 {
		// 	glog.Infof("Checked %d records.", counter)
		// }
	}
	b.StopTimer()

	// Cleanup code that should not be timed but deserves to be tested.
	rlruCache.Purge()
	assert.Equal(b, 0, cache.Len())
}

type TestStructCodec int

func (t TestStructCodec) Encode(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (t TestStructCodec) Decode(data []byte) (interface{}, error) {
	var v TestStruct
	err := json.Unmarshal(data, &v)
	return &v, err
}

type StringCodec struct{}

func (s StringCodec) Encode(data interface{}) ([]byte, error) {
	return []byte(data.(string)), nil
}

func (s StringCodec) Decode(byteData []byte) (interface{}, error) {
	return string(byteData), nil
}

type testStruct struct {
	Name    string `redis:"name"`
	Content []byte `redis:"content"`
}

func TestRedisPrimitives(t *testing.T) {
	testutils.SkipIfShort(t)

	rp := NewRedisPool(REDIS_SERVER_ADDRESS, REDIS_DB_PRIMITIVE_TESTS)
	assert.Nil(t, rp.FlushDB())

	// create a worker queue for a given type
	codec := StringCodec{}
	qRet, err := NewReadThroughCache(rp, Q_NAME_PRIMITIVES, nil, codec, runtime.NumCPU()-2)
	assert.Nil(t, err)

	// Cast to WorkerQueue since we are testing internals.
	q := qRet.(*RedisRTC)

	inProgress, err := q.inProgress()
	assert.Nil(t, err)
	assert.Equal(t, 0, len(inProgress))

	ID_1, ID_2, ID_3, ID_4, ID_5 := "id_1", "id_2", "id_3", "id_4", "id_5"
	p1 := rtcache.PriorityTimeCombined(3)
	found, err := q.enqueue(ID_1, p1)
	assert.Nil(t, err)
	assert.False(t, found)
	time.Sleep(time.Millisecond * 1)

	found, err = q.enqueue(ID_2, rtcache.PriorityTimeCombined(1))
	assert.Nil(t, err)
	assert.False(t, found)
	time.Sleep(time.Millisecond * 1)

	p3 := rtcache.PriorityTimeCombined(1)
	found, err = q.enqueue(ID_3, p3)
	assert.Nil(t, err)
	assert.False(t, found)
	time.Sleep(time.Millisecond * 1)

	p2 := rtcache.PriorityTimeCombined(0)
	found, err = q.enqueue(ID_2, p2)
	assert.Nil(t, err)
	assert.False(t, found)

	dequedItem, itemsLeft, err := q.dequeue()
	assert.Nil(t, err)
	assert.Equal(t, &workerTask{"id_2", p2}, dequedItem)
	assert.Equal(t, 2, itemsLeft)

	dequedItem, itemsLeft, err = q.dequeue()
	assert.Nil(t, err)
	assert.Equal(t, &workerTask{"id_3", p3}, dequedItem)
	assert.Equal(t, 1, itemsLeft)

	dequedItem, itemsLeft, err = q.dequeue()
	assert.Nil(t, err)
	assert.Equal(t, &workerTask{"id_1", p1}, dequedItem)
	assert.Equal(t, 0, itemsLeft)

	dequedItem, itemsLeft, err = q.dequeue()
	assert.Nil(t, err)
	assert.Nil(t, dequedItem)
	assert.Equal(t, 0, itemsLeft)

	inProgress, err = q.inProgress()
	assert.Nil(t, err)
	sort.Strings(inProgress)
	assert.Equal(t, []string{"id_1", "id_2", "id_3"}, inProgress)

	found, err = q.enqueue(ID_4, 1)
	assert.Nil(t, err)
	assert.False(t, found)
	time.Sleep(time.Millisecond * 5)

	found, err = q.enqueue(ID_5, 1)
	assert.Nil(t, err)
	assert.False(t, found)

	inQueue, err := q.inQueue(100)
	assert.Nil(t, err)
	assert.Equal(t, []string{ID_4, ID_5}, inQueue)
	time.Sleep(time.Millisecond * 5)

	found, err = q.enqueue(ID_5, 0)
	assert.Nil(t, err)
	assert.False(t, found)

	inQueue, err = q.inQueue(100)
	assert.Nil(t, err)
	assert.Equal(t, []string{ID_5, ID_4}, inQueue)

	// Test listening to a list.
	const N_MESSAGES = 10000
	const TEST_LIST = "mytestlist"
	assert.Nil(t, rp.FlushDB())

	listCh := rp.List(TEST_LIST)

	go func() {
		for i := 0; i < N_MESSAGES; i++ {
			id := "id-" + strconv.Itoa(i)
			if err := rp.AppendList(TEST_LIST, []byte(id)); err != nil {
				panic(fmt.Sprintf("AddToList failed: %s", err))
			}
		}
	}()

	tick := time.Tick(60 * time.Second)
	for i := 0; i < N_MESSAGES; i++ {
		select {
		case <-listCh:
		case <-tick:
			assert.FailNow(t, "Timeout in testing list channel")
		}
	}

	// Test hash save and load
	const TEST_HASH_KEY = "my-test-hash"
	ts1 := testStruct{
		Name:    "myName",
		Content: []byte("my content"),
	}
	var ts2 testStruct

	assert.Nil(t, rp.SaveHash(TEST_HASH_KEY, &ts1))
	foundHash, err := rp.LoadHashToStruct(TEST_HASH_KEY, &ts2)
	assert.Nil(t, err)
	assert.True(t, foundHash)
	assert.Equal(t, ts1, ts2)

	assert.Nil(t, rp.DeleteKey(TEST_HASH_KEY))
	foundHash, err = rp.LoadHashToStruct(TEST_HASH_KEY, &ts2)
	assert.Nil(t, err)
	assert.False(t, foundHash)
}
