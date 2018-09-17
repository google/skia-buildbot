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
	assert.NoError(t, testutils.EventuallyConsistent(time.Second*3, func() error {
		if len(ch) < 3 {
			return testutils.TryAgainErr
		}
		return nil
	}))

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
	// now := time.Now().Unix()
	for i := 0; i < nEvents; i++ {
		eventBus.PublishStorageEvent(NewStorageEvent(TEST_BUCKET,
			fmt.Sprintf("whatever/path/somefile-%d", i)))
		eventBus.PublishStorageEvent(NewStorageEvent(TEST_BUCKET,
			fmt.Sprintf(TEST_PREFIX+"/whatever/path/somefile-%d", i)))
		eventBus.PublishStorageEvent(NewStorageEvent(TEST_BUCKET,
			fmt.Sprintf(TEST_PREFIX+"/whatever/path/somefile-%d.json", i)))
	}

	assert.NoError(t, testutils.EventuallyConsistent(time.Second*10, func() error {
		if len(chNoPrefix) < (nEvents*3) ||
			len(chWithPrefix) < (2*nEvents) ||
			len(chNoPrefixRegEx) < nEvents {
			return testutils.TryAgainErr
		}
		return nil
	}))
}

func TestNotificationsMap(t *testing.T) {
	testutils.SmallTest(t)

	notifyMap := NewNotificationsMap()
	notifyID := notifyMap.GetNotificationID(TEST_BUCKET, TEST_PREFIX)
	evtType_1 := notifyMap.Add(notifyID, nil)
	assert.Equal(t, []string{evtType_1}, notifyMap.Matches(TEST_BUCKET, TEST_PREFIX+"/path.json"))
	assert.Equal(t, []string{}, notifyMap.Matches(TEST_BUCKET, "other-prefix/path.json"))

	evtType_2 := notifyMap.Add(notifyID, regexp.MustCompile(`^.*\.json$`))

	compareSortStrings(t,
		[]string{evtType_1, evtType_2},
		notifyMap.Matches(TEST_BUCKET, TEST_PREFIX+"/path.json"))
	compareSortStrings(t,
		[]string{evtType_1},
		notifyMap.Matches(TEST_BUCKET, TEST_PREFIX+"/path.txt"))

	compareSortStrings(t,
		[]string{evtType_1, evtType_2},
		notifyMap.MatchesByID(notifyID, TEST_PREFIX+"/path.json"))

	compareSortStrings(t,
		[]string{evtType_1},
		notifyMap.MatchesByID(notifyID, TEST_PREFIX+"/path.txt"))
}

func compareSortStrings(t *testing.T, expected, actual []string) {
	sort.Strings(expected)
	sort.Strings(actual)
	assert.Equal(t, expected, actual)
}
