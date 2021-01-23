package gevent

import (
	"regexp"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/eventbus"
)

const (
	PROJECT_ID             = "test-project"
	LOCAL_TOPIC            = "testing-local-topic"
	SUBSCRIBER_1           = "buildbot-1"
	SUBSCRIBER_2           = "buildbot-2"
	SUBSCRIBER_STORAGE_EVT = "buildbot-storage-evt"

	// TEST_BUCKET is not actually accessed, it's just used to test synthetic storate events.
	TEST_BUCKET = "skia-not-existing-gm"
	TEST_PREFIX = "dm-json-v1"
)

// Test structure that is send as the payload on the event channels.
type testType struct {
	ID        int
	Value     string
	TimeStamp uint64
}

func TestEventBus(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresPubSubEmulator(t)

	testCodec := util.NewJSONCodec(&testType{})
	RegisterCodec("channel1", testCodec)
	RegisterCodec("channel2", testCodec)

	eventBus, err := New(PROJECT_ID, LOCAL_TOPIC, SUBSCRIBER_1)
	require.NoError(t, err)

	eventBusTwo, err := New(PROJECT_ID, LOCAL_TOPIC, SUBSCRIBER_2)
	require.NoError(t, err)

	ch := make(chan int, 5)
	eventBus.SubscribeAsync("channel1", func(e interface{}) {
		ch <- e.(*testType).ID
	})

	eventBus.SubscribeAsync("channel2", func(e interface{}) {
		ch <- e.(*testType).ID
	})

	eventBus.SubscribeAsync("channel2", func(e interface{}) {
		ch <- e.(*testType).ID
	})

	now := uint64(time.Now().UnixNano()) / uint64(time.Millisecond)
	eventBusTwo.Publish("channel1", &testType{
		ID:        1,
		Value:     "value 1",
		TimeStamp: now,
	}, true)
	eventBusTwo.Publish("channel2", &testType{
		ID:        2,
		Value:     "value 2",
		TimeStamp: now + 10,
	}, true)

	// Give the messages 10 seconds to process.
	startTime := time.Now()
	for {
		time.Sleep(time.Second)
		if time.Since(startTime) > (time.Second * 10) {
			require.FailNow(t, "Timeout: did not receive messages in time")
		}
		if len(ch) == 3 {
			break
		}
	}
	require.Equal(t, 3, len(ch))
	vals := []int{<-ch, <-ch, <-ch}
	sort.Ints(vals)
	require.Equal(t, []int{1, 2, 2}, vals)
}

func TestSynStorageEvents(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresPubSubEmulator(t)

	eventBus, err := New(PROJECT_ID, LOCAL_TOPIC, SUBSCRIBER_STORAGE_EVT)
	require.NoError(t, err)

	// Disable actual subscription to the bucket. It's not possible to test right now, but
	// if the subscription fails or doesn't work we will know immediately when deploying.
	eventBus.(*distEventBus).disableGCSSubscriptions = true

	targetFileRegExp := regexp.MustCompile(`.*\.json`)
	storageEvtChan, err := eventBus.RegisterStorageEvents(TEST_BUCKET, TEST_PREFIX, targetFileRegExp, nil)
	require.NoError(t, err)

	evtCh := make(chan interface{}, 1)
	eventBus.SubscribeAsync(storageEvtChan, func(evt interface{}) {
		evtCh <- evt
	})

	now := util.TimeStamp(time.Microsecond)
	testObjID := TEST_PREFIX + "/2018/11/01/15/89468e1cc434e93baeed282fd0c250b1d963c017/linux_xfa_rel/1541086007/pixel/dm.json"
	evt := eventbus.NewStorageEvent(TEST_BUCKET, testObjID, now, "5bf5542e57a662120b400c4cff7e9c40")
	eventBus.PublishStorageEvent(evt)

	require.NoError(t, testutils.EventuallyConsistent(50*time.Millisecond, func() error {
		select {
		case evt := <-evtCh:
			sEvt := evt.(*eventbus.StorageEvent)
			require.Equal(t, TEST_BUCKET, sEvt.BucketID)
			require.Equal(t, testObjID, sEvt.ObjectID)
			require.Equal(t, now, sEvt.TimeStamp)
			return nil
		default:
			return testutils.TryAgainErr
		}
	}))
}
