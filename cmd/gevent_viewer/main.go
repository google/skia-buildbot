package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"google.golang.org/api/option"

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
	topic          = flag.String("topic", "testing-gold-stage-eventbus", "Google Cloud PubSub topic of the eventbus.")
	subscriberName = flag.String("subscriber", "local-wien", "ID of the pubsub subscriber.")
	storageBucket  = flag.String("bucket", "wien", "ID of the pubsub subscriber.")
	projectID      = flag.String("project_id", common.PROJECT_ID, "Project ID of the Cloud project where the PubSub topic lives.")
	channels       = flag.String("channels", expstorage.EV_EXPSTORAGE_CHANGED, "Comma separated list of event channels.")
)

func main() {
	common.Init()

	if (*projectID == "") || (*topic == "") || (*subscriberName == "") || (*channels == "") {
		sklog.Fatalf("project_id, topic, subscriber and channels flags must all be set.")
	}

	if *storageBucket != "" {
		_, err := storage.NewClient(context.TODO())
		if err != nil {
			sklog.Fatalf("Unable to create storage client: %s", err)
		}
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
		opts := []option.ClientOption{option.WithCredentialsFile(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))}
		storageClient, err := storage.NewClient(context.TODO(), opts...)
		if err != nil {
			sklog.Fatalf("Unable to create storage client: %s", err)
		}
		gEventBus := eventBus.(*gevent.DistEventBus)
		if err := gEventBus.RegisterStorageEvents(*storageBucket, storageClient); err != nil {
			sklog.Fatalf("Error: %s", err)
		}
		sklog.Infof("Registered storage events")
		gEventBus.SubscribeAsync(gEventBus.StorageEventType(*storageBucket), func(evt interface{}) {
			fmt.Printf("Received Message for bucket %s: \n %s\n", *storageBucket, spew.Sdump(evt))
		})

	}

	// Wait forever as messages come in.
	select {}
}
