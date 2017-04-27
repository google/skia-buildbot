package eventbus

import (
	"sort"
	"sync"
	"testing"

	"go.skia.org/infra/go/testutils"

	assert "github.com/stretchr/testify/require"
)

const LOCAL_TOPIC = "local-topic"
const SYNC_MSG = -1

type testType struct {
	ID    int
	Value string
}

func init() {
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
	eventBus.Wait("topic1")
	eventBus.Wait("topic2")
	assert.Equal(t, 3, len(ch))

	vals := []int{<-ch, <-ch, <-ch}
	sort.Ints(vals)
	assert.Equal(t, []int{1, 2, 3}, vals)
}

func TestSubTopics(t *testing.T) {
	testutils.MediumTest(t)
	testutils.SkipIfShort(t)

	const N_NUMBERS = 200
	const ALL_NUMBERS_EVENT = "allNumbers"
	const EVEN_NUMBERS_EVENT = "evenNumbers"

	RegisterSubTopic(ALL_NUMBERS_EVENT, EVEN_NUMBERS_EVENT, func(data interface{}) bool {
		i, ok := data.(int)
		if !ok {
			return false
		}
		return i%2 == 0
	})

	eventBus := New()
	allCh := make(chan int, N_NUMBERS*3)
	evenCh := make(chan int, N_NUMBERS*3)
	eventBus.SubscribeAsync(ALL_NUMBERS_EVENT, func(e interface{}) { allCh <- e.(int) })
	eventBus.SubscribeAsync(EVEN_NUMBERS_EVENT, func(e interface{}) { evenCh <- e.(int) })

	allExpected := []int{}
	evenExpected := []int{}
	for i := 0; i < N_NUMBERS; i++ {
		eventBus.Publish(ALL_NUMBERS_EVENT, i)
		allExpected = append(allExpected, i)
		if i%2 == 0 {
			evenExpected = append(evenExpected, i)
		}
	}

	eventBus.Wait(ALL_NUMBERS_EVENT)
	close(allCh)
	close(evenCh)

	assert.Equal(t, N_NUMBERS, len(allCh))
	compChan(t, allExpected, allCh)
	compChan(t, evenExpected, evenCh)
}

func compChan(t assert.TestingT, exp []int, ch <-chan int) {
	actual := []int{}
	for v := range ch {
		actual = append(actual, v)
	}
	sort.Ints(actual)
	assert.Equal(t, exp, actual)
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
