package eventbus

import (
	"sort"
	"testing"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
)

const LOCAL_TOPIC = "local-topic"
const SYNC_MSG = -1

type testType struct {
	ID    int
	Value string
}

func TestEventBus(t *testing.T) {
	testutils.MediumTest(t)
	eventBus := New()

	ch := make(chan int, 5)
	eventBus.SubscribeAsync("topic1", func(e interface{}) { ch <- 1 })
	eventBus.SubscribeAsync("topic2", func(e interface{}) { ch <- (e.(int)) + 1 })
	eventBus.SubscribeAsync("topic2", func(e interface{}) { ch <- e.(int) })

	eventBus.Publish("topic1", nil)
	eventBus.Publish("topic2", 2)
	eventBus.(*memEventBus).wait("topic1")
	eventBus.(*memEventBus).wait("topic2")
	assert.Equal(t, 3, len(ch))

	vals := []int{<-ch, <-ch, <-ch}
	sort.Ints(vals)
	assert.Equal(t, []int{1, 2, 3}, vals)
}
