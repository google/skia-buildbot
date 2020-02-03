package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"cloud.google.com/go/pubsub"

	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/niagara/go/machine_manager"
)

const (
	pubsubTopic = "niagara-machines-skia"
)

func main() {
	flag.Parse()
	ifirestore.EnsureNotEmulator()
	fmt.Println("hello machine manager")
	ctx := context.Background()

	// Auth note: the underlying ifirestore.NewClient looks at the
	// GOOGLE_APPLICATION_CREDENTIALS env variable, so we don't need to supply
	// a token source.
	fsClient, err := ifirestore.NewClient(ctx, "skia-firestore", "niagara", "testing", nil)
	if err != nil {
		sklog.Fatalf("Unable to configure Firestore: %s", err)
	}
	sklog.Infof("Firestore good %v\n", fsClient)

	psClient, err := pubsub.NewClient(ctx, "skia-public")
	if err != nil {
		sklog.Fatalf("Unable to configure Pubsub: %s", err)
	}
	sub, err := setupTopicSubscription(ctx, psClient, pubsubTopic, "machine-manager-01")
	if err != nil {
		sklog.Fatalf("Unable to setups subscription: %s", err)
	}

	sklog.Infof("PubSub topic/subscription set up %v\n", sub)
	m := machine_manager.New(fsClient)

	sklog.Fatalf("error while receiving pubsub notifications: %s", sub.Receive(ctx, m.ProcessPubsubMessage))
}

func setupTopicSubscription(ctx context.Context, psClient *pubsub.Client, topicName, subscriberName string) (*pubsub.Subscription, error) {
	// Create the topic if it doesn't exist yet.
	topic := psClient.Topic(topicName)
	if exists, err := topic.Exists(ctx); err != nil {
		return nil, skerr.Wrapf(err, "checking whether topic %s exists", topicName)
	} else if !exists {
		if topic, err = psClient.CreateTopic(ctx, topicName); err != nil {
			return nil, skerr.Wrapf(err, "creating pubsub topic '%s'", topicName)
		}
	}

	// Create the subscription if it doesn't exist.
	subName := fmt.Sprintf("%s+%s", subscriberName, topicName)
	sub := psClient.Subscription(subName)
	if exists, err := sub.Exists(ctx); err != nil {
		return nil, skerr.Wrapf(err, "checking existence of pubsub subscription '%s'", subName)
	} else if !exists {
		sub, err = psClient.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
			Topic:             topic,
			AckDeadline:       10 * time.Second,
			RetentionDuration: time.Hour,
			ExpirationPolicy:  time.Duration(0),
		})
		if err != nil {
			return nil, skerr.Wrapf(err, "creating pubsub subscription '%s'", subName)
		}
	}
	sub.ReceiveSettings.MaxOutstandingMessages = 100
	sub.ReceiveSettings.NumGoroutines = 10
	return sub, nil
}
