package eventbus

import (
	"sort"
	"testing"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
)

type testType struct {
	ID    int
	Value string
}

func TestEventBus(t *testing.T) {
	testutils.SmallTest(t)
	eventBus := New()

	ch := make(chan int, 5)
	eventBus.SubscribeAsync("channel1", func(e interface{}) { ch <- 1 })
	eventBus.SubscribeAsync("channel2", func(e interface{}) { ch <- (e.(int)) + 1 })
	eventBus.SubscribeAsync("channel2", func(e interface{}) { ch <- e.(int) })

	eventBus.Publish("channel1", nil)
	eventBus.Publish("channel2", 2)
	eventBus.(*MemEventBus).Wait("channel1")
	eventBus.(*MemEventBus).Wait("channel2")
	assert.Equal(t, 3, len(ch))

	vals := []int{<-ch, <-ch, <-ch}
	sort.Ints(vals)
	assert.Equal(t, []int{1, 2, 3}, vals)
}
