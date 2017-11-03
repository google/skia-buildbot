package gevent

import (
	"os"
	"sort"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

const LOCAL_TOPIC = "testing-local-topic"
const SUBSCRIBER_1 = "buildbot-1"
const SUBSCRIBER_2 = "buildbot-2"

// Test structure that is send as the payload on the event channels.
type testType struct {
	ID        int
	Value     string
	TimeStamp uint64
}

func TestEventBus(t *testing.T) {
	testutils.MediumTest(t)

	if os.Getenv("PUBSUB_EMULATOR_HOST") == "" {
		t.Skip(`Skipping tests that require a local Cloud PubSub emulator.
Set the environment: $(gcloud beta emulators pubsub env-init)
Run the emulator: gcloud beta emulators pubsub start`)
	}

	testCodec := util.JSONCodec(&testType{})
	RegisterCodec("channel1", testCodec)
	RegisterCodec("channel2", testCodec)

	eventBus, err := New(common.PROJECT_ID, LOCAL_TOPIC, SUBSCRIBER_1)
	assert.NoError(t, err)

	eventBusTwo, err := New(common.PROJECT_ID, LOCAL_TOPIC, SUBSCRIBER_2)
	assert.NoError(t, err)

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
		if time.Now().Sub(startTime) > (time.Second * 10) {
			assert.FailNow(t, "Timeout: did not receive messages in time")
		}
		if len(ch) == 3 {
			break
		}
	}
	assert.Equal(t, 3, len(ch))
	vals := []int{<-ch, <-ch, <-ch}
	sort.Ints(vals)
	assert.Equal(t, []int{1, 2, 2}, vals)
}
