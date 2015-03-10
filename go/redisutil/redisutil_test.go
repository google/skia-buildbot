package redisutil

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/skia-dev/glog"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

const (
	// TEST_DATA_DIR  is the directory with data used for benchmarks.
	TEST_DATA_DIR = "./benchdata"

	// TEST_DATA_STORAGE_PATH is the folder in the test data bucket.
	// See go/testutils for details.
	TEST_DATA_STORAGE_PATH = "common-testdata/redislru-benchdata.tar.gz"
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
	cache := NewRedisLRUCache("localhost:6379", 1, "test-di", TestStructCodec(0))
	testLRUCache(t, cache)
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

func testLRUCache(t *testing.T, cache util.LRUCache) {
	rlruCache := cache.(*RedisLRUCache)
	rlruCache.Purge()
	N := 256
	for i := 0; i < N; i++ {
		cache.Add(strconv.Itoa(i), i)
	}

	// Make sure out keys are correct
	assert.Equal(t, N, cache.Len())
	assert.Equal(t, N, len(rlruCache.Keys()))
	for _, k := range rlruCache.Keys() {
		assert.IsType(t, "", k)
		v, ok := cache.Get(k)
		assert.True(t, ok)
		assert.IsType(t, 0, v)
		assert.Equal(t, k, strconv.Itoa(v.(int)))
	}

	for i := 0; i < N; i++ {
		found, ok := cache.Get(strconv.Itoa(i))
		assert.True(t, ok)
		assert.IsType(t, 0, found)
		assert.Equal(t, found.(int), i)
	}

	for i := 0; i < N; i++ {
		_, ok := cache.Get(strconv.Itoa(i))
		assert.True(t, ok)
		oldLen := cache.Len()
		cache.Remove(strconv.Itoa(i))
		assert.Equal(t, oldLen-1, cache.Len())
	}
	assert.Equal(t, 0, cache.Len())

	// Add some TestStructs to make sure the codec works.
	for i := 0; i < N; i++ {
		strKey := "structkey-" + strconv.Itoa(i)
		ts := &TestStruct{
			PixelDiffFilePath: "somesting-" + strconv.Itoa(i),
			MaxRGBADiffs:      []int{i * 4, i*4 + 1, i*4 + 2, i*4 + 3},
		}
		cache.Add(strKey, ts)
		assert.Equal(t, i+1, cache.Len())
		foundTS, ok := cache.Get(strKey)
		assert.True(t, ok)
		assert.IsType(t, &TestStruct{}, foundTS)
		assert.Equal(t, ts, foundTS)
	}
}
