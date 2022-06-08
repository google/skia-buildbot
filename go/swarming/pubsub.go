package swarming

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
)

const (
	PUBSUB_FULLY_QUALIFIED_TOPIC_TMPL    = "projects/%s/topics/%s"
	PUBSUB_TOPIC_SWARMING_TASKS          = "swarming-tasks"
	PUBSUB_TOPIC_SWARMING_TASKS_INTERNAL = "swarming-tasks-internal"
)

// InitPubSub ensures that the pub/sub topic and subscription exist and starts
// receiving messages, calling the given callback function for each one. The
// callback returns a bool indicating whether or not to ACK the message.
func InitPubSub(topicName, subscriberName string, callback func(*PubSubTaskMessage) bool) error {
	ctx := context.Background()

	// Create a client.
	client, err := pubsub.NewClient(ctx, common.PROJECT_ID)
	if err != nil {
		return err
	}

	// Create topic and subscription if necessary.

	// Topic.
	topic := client.Topic(topicName)
	exists, err := topic.Exists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		if _, err := client.CreateTopic(ctx, topicName); err != nil {
			return err
		}
	}

	// Subscription.
	subName := fmt.Sprintf("%s+%s", subscriberName, topicName)
	sub := client.Subscription(subName)
	exists, err = sub.Exists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		if _, err := client.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
			Topic:       topic,
			AckDeadline: 3 * time.Minute,
		}); err != nil {
			return err
		}
	}
	go func() {
		for {
			if ctx.Err() != nil {
				sklog.Errorf("Context has error: %s", ctx.Err())
				return
			}
			if err := sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
				var taskMsg PubSubTaskMessage
				if err := json.Unmarshal(m.Data, &taskMsg); err != nil {
					sklog.Errorf("Failed to decode pubsub message body: %s", err)
					m.Ack() // We'll never be able to handle this message.
				}
				if callback(&taskMsg) {
					m.Ack()
				} else {
					m.Nack()
				}
			}); err != nil {
				sklog.Errorf("Failed to receive pubsub messages: %s", err)
				time.Sleep(time.Second)
			}
		}
	}()
	return nil
}

// PubSubTaskMessage is a message received from Swarming via pub/sub about a
// Task.
type PubSubTaskMessage struct {
	SwarmingTaskId string `json:"task_id"`
	UserData       string `json:"userdata"`
}
