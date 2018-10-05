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
	"go.skia.org/infra/go/test_automation"
)

var (
	project = flag.String("project_id", "", "GCE Project ID")
)

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
	topic := client.Topic(test_automation.PUBSUB_TOPIC)
	if exists, err := topic.Exists(ctx); err != nil {
		sklog.Fatal(err)
	} else if !exists {
		topic, err = client.CreateTopic(ctx, test_automation.PUBSUB_TOPIC)
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
		subName := fmt.Sprintf("pubsub_listener_%s", hostname)
		sub, err = client.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
			Topic:       topic,
			AckDeadline: 10 * time.Second,
		})
		if err != nil {
			sklog.Fatal(err)
		}
	}
	for {
		sklog.Infof("Waiting for messages.")
		if err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
			var m test_automation.Message
			if err := json.Unmarshal(msg.Data, &m); err != nil {
				sklog.Fatal(err)
			}
			b, err := json.MarshalIndent(m, "", "  ")
			if err != nil {
				sklog.Fatal(err)
			}
			msg.Ack()
			sklog.Infof("Received message: %s", string(b))
		}); err != nil {
			sklog.Fatal(err)
		}
	}
}
