package eventbus

import (
	"sort"
	"testing"
	"time"

	"go.skia.org/infra/go/geventbus"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"

	assert "github.com/stretchr/testify/require"
)

const GLOBAL_TOPIC = "global-topic"
const LOCAL_TOPIC = "local-topic"

const NSQD_ADDR = "127.0.0.1:4150"

type testType struct {
	ID    int
	Value string
}

func init() {
	RegisterGlobalEvent(GLOBAL_TOPIC, util.JSONCodec(&testType{}))
}

func TestEventBus(t *testing.T) {
	eventBus := New(nil)

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

// TODO(stephana): TestEventBusGlobally is temporarilly disabled because it is
// flaky. To be re-enable once fixed.
func xTestEventBusGlobally(t *testing.T) {
	testutils.SkipIfShort(t)

	globalEventBus, err := geventbus.NewNSQEventBus(NSQD_ADDR)
	assert.Nil(t, err)

	secondGlobalBus, err := geventbus.NewNSQEventBus(NSQD_ADDR)
	assert.Nil(t, err)

	eventBus := New(globalEventBus)
	ch := make(chan interface{}, 100)
	eventBus.SubscribeAsync(GLOBAL_TOPIC, func(e interface{}) { ch <- e })

	secondCh := make(chan interface{}, 100)
	errCh := make(chan error, 100)
	assert.Nil(t, secondGlobalBus.SubscribeAsync(GLOBAL_TOPIC, geventbus.JSONCallback(&testType{}, func(data interface{}, err error) {
		if err != nil {
			errCh <- err
		} else {
			secondCh <- data
		}
	})))

	eventBus.Publish(GLOBAL_TOPIC, &testType{0, "message-1"})
	eventBus.Publish(GLOBAL_TOPIC, &testType{1, "message-2"})
	eventBus.Publish(GLOBAL_TOPIC, &testType{2, "message-3"})
	eventBus.Publish(GLOBAL_TOPIC, &testType{3, "message-4"})
	time.Sleep(5 * time.Second)
	assert.Equal(t, 4, len(ch))
	assert.Equal(t, 4, len(secondCh))
	close(ch)
	close(secondCh)

	found := map[int]bool{}
	for m := range ch {
		assert.IsType(t, &testType{}, m)
		temp := m.(*testType)
		assert.False(t, found[temp.ID])
		found[temp.ID] = true
	}

	for m := range secondCh {
		assert.IsType(t, &testType{}, m)
	}
}
