package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_driver/go/db"
	"go.skia.org/infra/task_driver/go/db/memory"
	"go.skia.org/infra/task_driver/go/display"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	project = flag.String("project_id", "", "GCE Project ID")
)

// Entry mimics logging.Entry, which for some reason does not include the
// jsonPayload field, and is not parsable via json.Unmarshal due to the Severity
// type.
type Entry struct {
	Labels      map[string]string `json:"labels"`
	JsonPayload td.Message        `json:"jsonPayload"`
}

func main() {
	common.Init()
	if *project == "" {
		sklog.Fatal("--project_id is required.")
	}
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, *project)
	if err != nil {
		sklog.Fatal(err)
	}
	topic := client.Topic(td.PUBSUB_TOPIC)
	if exists, err := topic.Exists(ctx); err != nil {
		sklog.Fatal(err)
	} else if !exists {
		topic, err = client.CreateTopic(ctx, td.PUBSUB_TOPIC)
		if err != nil {
			sklog.Fatal(err)
		}
	}
	hostname, err := os.Hostname()
	if err != nil {
		sklog.Fatal(err)
	}
	subName := fmt.Sprintf("pubsub_listener_%s", hostname)
	sub := client.Subscription(subName)
	if exists, err := sub.Exists(ctx); err != nil {
		sklog.Fatal(err)
	} else if !exists {
		sub, err = client.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
			Topic:       topic,
			AckDeadline: 10 * time.Second,
		})
		if err != nil {
			sklog.Fatal(err)
		}
	}

	// Create the TaskDriver DB.
	d := memory.NewInMemoryDB()

	for {
		sklog.Infof("Waiting for messages.")
		if err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
			var e Entry
			if err := json.Unmarshal(msg.Data, &e); err != nil {
				sklog.Fatal(err)
			}
			b, err := json.MarshalIndent(e.JsonPayload, "", "  ")
			if err != nil {
				sklog.Fatal(err)
			}
			if err := db.UpdateFromMessage(d, &e.JsonPayload); err != nil {
				sklog.Fatal(err)
			}
			msg.Ack()
			sklog.Infof("Received message: %s", string(b))
			t, err := d.GetTaskDriver(e.JsonPayload.TaskId)
			if err != nil {
				sklog.Fatal(err)
			}
			disp, err := display.TaskDriverForDisplay(t)
			if err != nil {
				sklog.Fatal(err)
			}
			b, err = json.MarshalIndent(disp, "", "  ")
			if err != nil {
				sklog.Fatal(err)
			}
			sklog.Infof("Full task driver: %s", string(b))
		}); err != nil {
			sklog.Fatal(err)
		}
	}
}
