package geventbus

import (
	"encoding/json"
	"sort"
	"sync"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

// TODO(stephana): Disable until fixed.
func xTestEventBus(t *testing.T) {
	testutils.SkipIfShort(t)

	eventBus, err := NewNSQEventBus("127.0.0.1:4150")
	assert.Nil(t, err)

	ch := make(chan string, 100)
	var wg sync.WaitGroup

	callbackFn := func(data []byte) {
		ch <- string(data)
		wg.Done()
	}

	assert.Nil(t, eventBus.SubscribeAsync("topic1", callbackFn))
	assert.Nil(t, eventBus.SubscribeAsync("topic2", callbackFn))
	assert.Nil(t, eventBus.SubscribeAsync("topic2", callbackFn))

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
