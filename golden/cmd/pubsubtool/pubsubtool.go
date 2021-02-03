package main

import (
	"context"
	"flag"
	"strings"

	"cloud.google.com/go/pubsub"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

func main() {
	projectID := flag.String("project_id", "skia-public", "The project for PubSub events")
	topicName := flag.String("topic_name", "", "The topic to create if it does not exist")
	subscriptionName := flag.String("subscription_name", "", "The subscription to create if it does not exist")

	flag.Parse()
	task := strings.ToLower(flag.Arg(0))

	ctx := context.Background()
	psc, err := pubsub.NewClient(ctx, *projectID)
	if err != nil {
		sklog.Fatalf("initializing pubsub client for project %s: %s", *projectID, err)
	}

	if task == "create" {
		if err := createTopicAndSubscription(ctx, psc, *topicName, *subscriptionName); err != nil {
			sklog.Fatalf("Making topic %s and subscription %s: %s", *topicName, *subscriptionName, err)
		}
	} else {
		sklog.Fatalf(`Invalid command: %q. Try "create".`)
	}
}

func createTopicAndSubscription(ctx context.Context, psc *pubsub.Client, topic, sub string) error {
	if topic == "" || sub == "" {
		return skerr.Fmt("Can't have empty topic or subscription")
	}
	// Create the topic if it doesn't exist yet.
	t := psc.Topic(topic)
	if exists, err := t.Exists(ctx); err != nil {
		return skerr.Fmt("Error checking whether topic exits: %s", err)
	} else if !exists {
		if t, err = psc.CreateTopic(ctx, topic); err != nil {
			return skerr.Fmt("Error creating pubsub topic '%s': %s", topic, err)
		}
	}

	// Create the subscription if it doesn't exist.
	s := psc.Subscription(sub)
	if exists, err := s.Exists(ctx); err != nil {
		return skerr.Fmt("Error checking existence of pubsub subscription '%s': %s", sub, err)
	} else if !exists {
		_, err = psc.CreateSubscription(ctx, sub, pubsub.SubscriptionConfig{
			Topic: t,
		})
		if err != nil {
			return skerr.Fmt("Error creating pubsub subscription '%s': %s", sub, err)
		}
	}
	sklog.Infof("Topic %s and Subscription %s exist if they didn't before", topic, sub)
	return nil
}
