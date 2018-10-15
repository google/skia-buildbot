package main

import (
	"context"
	"flag"
	"fmt"
	"regexp"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/davecgh/go-spew/spew"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gevent"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/expstorage"
	_ "go.skia.org/infra/golden/go/tryjobstore" // Import registers event codecs in that package.
)

// Command line flags.
var (
	channels        = flag.String("channels", expstorage.EV_EXPSTORAGE_CHANGED, "Comma separated list of event channels.")
	objectPrefix    = flag.String("object_prefix", "", "Prefix of the storage path that should be watched.")
	objectRegExpStr = flag.String("object_regex", "", "Regex that must be matched by the object id")
	projectID       = flag.String("project_id", common.PROJECT_ID, "Project ID of the Cloud project where the PubSub topic lives.")
	storageBucket   = flag.String("bucket", "", "ID of the pubsub subscriber.")
	subscriberName  = flag.String("subscriber", "local-wien", "ID of the pubsub subscriber.")
	topic           = flag.String("topic", "testing-gold-stage-eventbus", "Google Cloud PubSub topic of the eventbus.")
)

func main() {
	common.Init()

	if (*projectID == "") || (*topic == "") || (*subscriberName == "") || (*channels == "") {
		sklog.Fatalf("project_id, topic, subscriber and channels flags must all be set.")
	}

	eventBus, err := gevent.New(*projectID, *topic, *subscriberName)
	if err != nil {
		sklog.Fatalf("Error creating event bus: %s", err)
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

	if *storageBucket != "" {
		storageClient, err := storage.NewClient(context.TODO())
		if err != nil {
			sklog.Fatalf("Unable to create storage client: %s", err)
		}

		var objRegEx *regexp.Regexp
		if *objectRegExpStr != "" {
			objRegEx = regexp.MustCompile(*objectRegExpStr)
		}

		eventType, err := eventBus.RegisterStorageEvents(*storageBucket, *objectPrefix, objRegEx, storageClient)
		if err != nil {
			sklog.Fatalf("Error: %s", err)
		}

		sklog.Infof("Registered storage events. Eventtype: %s", eventType)
		eventBus.SubscribeAsync(eventType, func(evt interface{}) {
			sklog.Infof("Received Message for bucket %s: \n %s\n", *storageBucket, spew.Sdump(evt))
		})
	}

	// Wait forever as messages come in.
	select {}
}
