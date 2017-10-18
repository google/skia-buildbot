package main

import (
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gevent"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

type testType struct {
	ID        int
	Value     string
	TimeStamp uint64
}

func main() {
	common.Init()
	testCodec := util.JSONCodec(&testType{})

	eventBus, err := gevent.New(common.PROJECT_ID, "stephana-wien-local-test-topic-two", "stephana-receiver", testCodec, nil)
	if err != nil {
		sklog.Fatal(err)
	}

	ch := make(chan *testType, 1000000)
	eventBus.SubscribeAsync("topic1", func(e interface{}) {
		// sklog.Infof("Received message.")
		ch <- e.(*testType)
	})

	sklog.Infof("Starting to receive.")
	counter := 0
	startTime := time.Now()
	uniqueIDs := map[int]bool{}
	for evt := range ch {
		uniqueIDs[evt.ID] = true

		counter++
		if counter%10000 == 0 {
			secs := float64(time.Now().Sub(startTime)) / float64(time.Second)
			sklog.Infof("Receive %d Messages in %f. Unique: %d", counter, secs, len(uniqueIDs))
		}
	}
}
