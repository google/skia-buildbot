package eventbus

import (
	"sort"
	"testing"

	assert "github.com/stretchr/testify/require"
)

func TestEventBus(t *testing.T) {
	eventBus := New()

	ch := make(chan int, 5)
	eventBus.SubscribeAsync("topic1", func(e interface{}) { ch <- 1 })
	eventBus.SubscribeAsync("topic2", func(e interface{}) { ch <- (e.(int)) + 1 })
	eventBus.SubscribeAsync("topic2", func(e interface{}) { ch <- e.(int) })

	eventBus.Publish("topic1", nil)
	eventBus.Publish("topic2", 2)
	eventBus.Wait("topic1")
	eventBus.Wait("topic2")
	assert.Equal(t, 3, len(ch))

	vals := []int{<-ch, <-ch, <-ch}
	sort.Ints(vals)
	assert.Equal(t, []int{1, 2, 3}, vals)
}
