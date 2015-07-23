package geventbus

import (
	"sort"
	"sync"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

func TestEventBus(t *testing.T) {
	testutils.SkipIfShort(t)

	eventBus, err := NewGlobalEventBus("127.0.0.1:4150")
	assert.Nil(t, err)
	defer eventBus.Close()

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
}
