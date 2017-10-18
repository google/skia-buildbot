package main

import (
	"fmt"
	"time"

	"go.skia.org/infra/go/timer"

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

func nowMS() uint64 {
	return uint64(time.Now().UnixNano()) / uint64(time.Millisecond)
}

func main() {
	common.Init()

	testCodec := util.JSONCodec(&testType{})
	eventBus, err := gevent.New(common.PROJECT_ID, "stephana-wien-local-test-topic-two", "stephana-sender", testCodec, nil)
	if err != nil {
		sklog.Fatalf("Error: %s", err)
	}

	nMessages := 100000
	startTime := time.Now()
	t := timer.New("Sender: ")
	for i := 0; i < nMessages; i++ {
		eventBus.Publish("topic1", &testType{
			ID:        i + 1,
			Value:     "value " + fmt.Sprintf("%d", i+1),
			TimeStamp: nowMS(),
		})
	}

	t.Stop()
	secs := time.Now().Sub(startTime) / time.Second
	sklog.Infof("Sent: %d message in %d seconds. %f per second", nMessages, secs, float64(nMessages)/float64(secs))
	select {}
}
