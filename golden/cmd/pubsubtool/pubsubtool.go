// The pubsubtool executable is a convenent way to create PubSub topics and subscriptions.
// It also allows for manual injection of messages to test systems end-to-end.
package main

import (
	"context"
	"flag"
	"io/ioutil"
	"strings"

	"cloud.google.com/go/pubsub"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

func main() {
	projectID := flag.String("project_id", "skia-public", "The project for PubSub events")
	topicName := flag.String("topic_name", "", "The topic to create if it does not exist")
	subscriptionName := flag.String("subscription_name", "", "The subscription to create if it does not exist")
	jsonMessageFile := flag.String("json_message_file", "", "A file that contains the JSON contents to send as the body of a pubsub message.")

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
	} else if task == "publish" {
		if err := publishMessage(ctx, psc, *topicName, *jsonMessageFile); err != nil {
			sklog.Fatalf("Sending contents of %s to topic %s: %S", *jsonMessageFile)
		}
	} else {
		sklog.Fatalf(`Invalid command: %q. Try "create".`, task)
	}
}

func publishMessage(ctx context.Context, psc *pubsub.Client, topic, jsonMessageFile string) error {
	if topic == "" || jsonMessageFile == "" {
		return skerr.Fmt("Can't have empty topic or message file")
	}
	body, err := ioutil.ReadFile(jsonMessageFile)
	if err != nil {
		return skerr.Wrapf(err, "reading %s", jsonMessageFile)
	}
	pr := psc.Topic(topic).Publish(ctx, &pubsub.Message{
		Data: body,
	})
	// Blocks until message actual sent
	_, err = pr.Get(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	sklog.Infof("Sent")
	return nil
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
