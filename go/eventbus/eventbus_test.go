package eventbus

import (
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"go.skia.org/infra/go/geventbus"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"

	assert "github.com/stretchr/testify/require"
)

const GLOBAL_TOPIC = "global-topic"
const LOCAL_TOPIC = "local-topic"
const SYNC_MSG = -1

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

func TestEventBusGlobally(t *testing.T) {
	testutils.SkipIfShort(t)

	messages := []*testType{
		&testType{0, "message-1"},
		&testType{1, "message-2"},
		&testType{2, "message-3"},
		&testType{3, "message-4"},
	}

	globalEventBus, err := geventbus.NewNSQEventBus(NSQD_ADDR)
	assert.Nil(t, err)

	secondGlobalBus, err := geventbus.NewNSQEventBus(NSQD_ADDR)
	assert.Nil(t, err)

	// Use atomic ints to sync the callback functions.
	firstMap := newAtomicMap()
	firstEventBus := New(globalEventBus)
	firstEventBus.SubscribeAsync(GLOBAL_TOPIC, func(e interface{}) {
		data := e.(*testType)
		if data.ID == SYNC_MSG {
			firstMap.setReady()
			return
		}
		firstMap.Add(data.ID, data)
	})

	secondMap := newAtomicMap()
	errCh := make(chan error, 100)
	assert.Nil(t, secondGlobalBus.SubscribeAsync(GLOBAL_TOPIC, geventbus.JSONCallback(&testType{}, func(data interface{}, err error) {
		if err != nil {
			errCh <- err
			return
		}

		if data.(*testType).ID == SYNC_MSG {
			secondMap.setReady()
			return
		}

		d := data.(*testType)
		secondMap.Add(d.ID, d)
	})))

	for !firstMap.isReady() && !secondMap.isReady() {
		firstEventBus.Publish(GLOBAL_TOPIC, &testType{SYNC_MSG, "ignore"})
	}

	for _, m := range messages {
		firstEventBus.Publish(GLOBAL_TOPIC, m)
	}

	lmsg := len(messages)
	for ((firstMap.Len() < lmsg) || (secondMap.Len() < lmsg)) && (len(errCh) == 0) {
		time.Sleep(time.Millisecond * 10)
	}

	if len(errCh) > 0 {
		close(errCh)
		for err = range errCh {
			fmt.Printf("Error: %s\n", err)
		}
		assert.FailNow(t, "Received too many error messages.")
	}
}

type atomicMap struct {
	m     map[int]*testType
	mutex sync.Mutex
	ready bool
}

func newAtomicMap() *atomicMap {
	return &atomicMap{
		m:     map[int]*testType{},
		ready: false,
	}
}

func (a *atomicMap) Add(k int, v *testType) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.m[k] = v
}

func (a *atomicMap) Len() int {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	return len(a.m)
}

func (a *atomicMap) setReady() {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.ready = true
}

func (a *atomicMap) isReady() bool {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	return a.ready
}
