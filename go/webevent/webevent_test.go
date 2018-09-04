package webevent

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/testutils"
)

const (
	channelOne = "ch-one"
	channelTwo = "ch-two"
)

type testType struct {
	ID    int
	Value string
}

type wrapper struct {
	subID int
	evt   *Event
}

func TestEventDispatcher(t *testing.T) {
	testutils.SmallTest(t)
	eventBus := eventbus.New()

	dispatcher := NewEventDispatcher(eventBus)
	oneMessages := make(chan *wrapper, 100)
	twoMessages := make(chan *wrapper, 100)
	doneCh := make(chan bool)

	sub_1, err := dispatcher.Subscribe(channelOne, channelTwo)
	assert.NoError(t, err)
	collect(sub_1, 1, oneMessages, doneCh)

	sub_2, err := dispatcher.Subscribe(channelOne)
	assert.NoError(t, err)
	collect(sub_2, 2, twoMessages, doneCh)

	eventBus.Publish(channelOne, &testType{1, "value 1"}, false)
	eventBus.Publish(channelOne, &testType{2, "value 2"}, false)
	eventBus.Publish(channelOne, &testType{3, "value 3"}, false)
	eventBus.Publish(channelOne, &testType{4, "value 4"}, false)
	eventBus.Publish(channelTwo, &testType{5, "value 5"}, false)
	eventBus.Publish(channelTwo, &testType{6, "value 6"}, false)
	sub_1.Cancel()
	eventBus.Publish(channelOne, &testType{7, "value 7"}, false)
	eventBus.Publish(channelTwo, &testType{8, "value 8"}, false)
	sub_2.Cancel()
	eventBus.Publish(channelOne, &testType{9, "value 9"}, false)
	eventBus.Publish(channelTwo, &testType{10, "value 10"}, false)

	time.Sleep(5 * time.Second)
	close(doneCh)

	oneRet := drainCh(oneChannel)
	twoRet := drainCh(twoChannel)

	assert.False(t, true)
}

func collect(sub *Subscription, subID int, targetCh chan *wrapper, doneCh chan bool) {
	go func() {
		for {
			select {
			case <-doneCh:
				return
			case evt := <-sub.Channel:
				targetCh <- &wrapper{
					subID: subID,
					evt:   evt,
				}
			}
		}

	}()
}
