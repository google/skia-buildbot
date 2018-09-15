package eventbus

import (
	"fmt"
	"regexp"
	"sort"
	"testing"
	"time"

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

	eventBus.Publish("channel1", nil, false)
	eventBus.Publish("channel2", 2, false)
	testutils.EventuallyConsistent(time.Second*3, func() error {
		if len(ch) < 3 {
			return testutils.TryAgainErr
		}
		return nil
	})
	assert.Equal(t, 3, len(ch))

	vals := []int{<-ch, <-ch, <-ch}
	sort.Ints(vals)
	assert.Equal(t, []int{1, 2, 3}, vals)
}

const (
	TEST_BUCKET = "test-bucket"
	TEST_PREFIX = "some/path"
	JSON_REGEX  = `^.*\.json$`
)

func TestSynStorageEvents(t *testing.T) {
	testutils.SmallTest(t)

	eventBus := New()

	noPrefixEvt, err := eventBus.RegisterStorageEvents(TEST_BUCKET, "", nil, nil)
	assert.NoError(t, err)

	withPrefixEvt, err := eventBus.RegisterStorageEvents(TEST_BUCKET, TEST_PREFIX, nil, nil)
	assert.NoError(t, err)

	jsonRegex := regexp.MustCompile(JSON_REGEX)
	noPrefixRegExEvt, err := eventBus.RegisterStorageEvents(TEST_BUCKET, "", jsonRegex, nil)

	chNoPrefix := make(chan interface{}, 100)
	chWithPrefix := make(chan interface{}, 100)
	chNoPrefixRegEx := make(chan interface{}, 100)
	ch := make(chan interface{}, 100)
	eventBus.SubscribeAsync(noPrefixEvt, func(e interface{}) {
		chNoPrefix <- e
		ch <- e
	})
	eventBus.SubscribeAsync(withPrefixEvt, func(e interface{}) {
		chWithPrefix <- e
		ch <- e

	})
	eventBus.SubscribeAsync(noPrefixRegExEvt, func(e interface{}) {
		chNoPrefixRegEx <- e
		ch <- e

	})

	nEvents := 10
	for i := 0; i < nEvents; i++ {
		eventBus.PublishStorageEvent(TEST_BUCKET, fmt.Sprintf("whatever/path/somefile-%d", i))
		eventBus.PublishStorageEvent(TEST_BUCKET, fmt.Sprintf(TEST_PREFIX+"/whatever/path/somefile-%d", i))
		eventBus.PublishStorageEvent(TEST_BUCKET, fmt.Sprintf(TEST_PREFIX+"/whatever/path/somefile-%d.json", i))
	}

	testutils.EventuallyConsistent(time.Second*10, func() error {
		if len(chNoPrefix) < (nEvents*3) ||
			len(chWithPrefix) < (2*nEvents) ||
			len(chNoPrefixRegEx) < nEvents {
			return testutils.TryAgainErr
		}
		return nil
	})

	assert.Equal(t, 3*nEvents, len(chNoPrefix))
	assert.Equal(t, 2*nEvents, len(chWithPrefix))
	assert.Equal(t, nEvents, len(chNoPrefixRegEx))
}
