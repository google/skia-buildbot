package geventbus

import (
	"encoding/json"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

// TODO(stephana): Disable until fixed.
func TestEventBus(t *testing.T) {
	testutils.SkipIfShort(t)

	eventBus, err := NewNSQEventBus("127.0.0.1:4150")
	assert.Nil(t, err)

	ch := make(chan string, 100)
	var wg sync.WaitGroup

	callbackFn := func(ready *int32) func([]byte) {
		return func(data []byte) {
			if string(data) == "ready" {
				atomic.StoreInt32(ready, 0)
				return
			}
			ch <- string(data)
			wg.Done()
		}
	}

	var ready_1 int32 = -1
	var ready_2 int32 = -1
	var ready_3 int32 = -1
	assert.Nil(t, eventBus.SubscribeAsync("topic1", callbackFn(&ready_1)))
	assert.Nil(t, eventBus.SubscribeAsync("topic2", callbackFn(&ready_2)))
	assert.Nil(t, eventBus.SubscribeAsync("topic2", callbackFn(&ready_3)))

	for atomic.LoadInt32(&ready_1)+atomic.LoadInt32(&ready_2)+atomic.LoadInt32(&ready_3) < 0 {
		assert.Nil(t, eventBus.Publish("topic1", []byte("ready")))
		assert.Nil(t, eventBus.Publish("topic2", []byte("ready")))
		time.Sleep(time.Millisecond)
	}

	wg.Add(3)
	assert.Nil(t, eventBus.Publish("topic1", []byte("0")))
	assert.Nil(t, eventBus.Publish("topic2", []byte("msg-01")))
	wg.Wait()

	assert.True(t, len(ch) >= 3)
	close(ch)

	vals := []string{}
	for val := range ch {
		vals = append(vals, val)
	}

	sort.Strings(vals)
	assert.Equal(t, []string{"0", "msg-01", "msg-01"}, vals)

	assert.Nil(t, eventBus.Close())
}

func TestJSONHelper(t *testing.T) {
	type myTestType struct {
		A int
		B string
	}

	testInstance := &myTestType{5, "hello"}
	f := JSONCallback(&myTestType{}, func(data interface{}, err error) {
		assert.Nil(t, err)
		assert.IsType(t, &myTestType{}, data)
		assert.Equal(t, testInstance, data)
	})
	jsonBytes, err := json.Marshal(testInstance)
	assert.Nil(t, err)
	f(jsonBytes)

	testArr := []*myTestType{&myTestType{1, "1"}, &myTestType{2, "2"}}
	f = JSONCallback([]*myTestType{}, func(data interface{}, err error) {
		assert.Nil(t, err)
		assert.IsType(t, []*myTestType{}, data)
		assert.Equal(t, testArr, data)
	})
	jsonBytes, err = json.Marshal(testArr)
	assert.Nil(t, err)
	f(jsonBytes)
}
