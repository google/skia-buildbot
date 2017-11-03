package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gevent"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/expstorage"
)

// Command line flags.
var (
	topic          = flag.String("topic", "testing-gold-stage-eventbus", "Google Cloud PubSub topic of the eventbus.")
	subscriberName = flag.String("subscriber", "local-wien", "ID of the pubsub subscriber.")
	projectID      = flag.String("project_id", common.PROJECT_ID, "Project ID of the Cloud project where the PubSub topic lives.")
	channels       = flag.String("channels", expstorage.EV_EXPSTORAGE_CHANGED, "Comma separated list of event channels.")
)

func main() {
	common.Init()

	if (*projectID == "") || (*topic == "") || (*subscriberName == "") || (*channels == "") {
		sklog.Fatalf("project_id, topic, subscriber and channels flags must all be set.")
	}

	eventBus, err := gevent.New(common.PROJECT_ID, *topic, *subscriberName)
	if err != nil {
		sklog.Fatalf("Error createing event bus: %s", err)
	}

	allChannels := strings.Split(*channels, ",")
	for _, oneChannel := range allChannels {
		func(channelName string) {
			eventBus.SubscribeAsync(channelName, func(evt interface{}) {
				fmt.Printf("Received Message on channel %s:\n\n", channelName)
				fmt.Println(spew.Sdump(evt))
			})
		}(oneChannel)
	}

	// Wait forever as messages come in.
	select {}
}
