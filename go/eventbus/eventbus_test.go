package eventbus

import (
	"sort"
	"sync"
	"testing"

	assert "github.com/stretchr/testify/require"
)

func TestEventBus(t *testing.T) {
	eventBus := New()
	var wg sync.WaitGroup

	ch := make(chan int, 5)
	eventBus.SubscribeAsync("topic1", func(e interface{}) {
		ch <- 1
		wg.Done()
	})
	eventBus.SubscribeAsync("topic2", func(e interface{}) {
		ch <- (e.(int)) + 1
		wg.Done()
	})
	eventBus.SubscribeAsync("topic2", func(e interface{}) {
		ch <- e.(int)
		wg.Done()
	})

	wg.Add(3)
	eventBus.Publish("topic1", nil)
	eventBus.Publish("topic2", 55)
	wg.Wait()
	assert.Equal(t, 3, len(ch))

	vals := []int{<-ch, <-ch, <-ch}
	sort.Ints(vals)
	assert.Equal(t, []int{1, 55, 56}, vals)
}
