package periodic

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

const (
	// Authentication scope required for periodic triggers.
	AUTH_SCOPE = pubsub.ScopePubSub

	// PubSub topic used for periodic triggers. A single topic is used for
	// all triggers, with each message containing an attribute indicating
	// which trigger promted it.
	PUBSUB_TOPIC = "periodic-trigger"

	// Attribute sent with all pubsub messages; the name of the periodic
	// trigger which prompted the message.
	PUBSUB_ATTR_TRIGGER_NAME = "trigger"

	// Attribute sent with all pubsub messages; the unique ID of the call
	// to Trigger() which prompted the message.
	PUBSUB_ATTR_TRIGGER_ID = "id"

	// Acknowledgement deadline for pubsub messages; all TriggerCallbackFns
	// must be faster than this deadline. If this is changed, all
	// subscriptions will need to be deleted and recreated.
	PUBSUB_ACK_DEADLINE = 5 * time.Minute

	// Google Cloud project name used for pubsub.
	PUBSUB_PROJECT = "skia-public"

	// Names of periodic triggers.
	TRIGGER_NIGHTLY = "nightly"
	TRIGGER_WEEKLY  = "weekly"
)

var (
	VALID_TRIGGERS = []string{TRIGGER_NIGHTLY, TRIGGER_WEEKLY}
)

// TriggerCallbackFn is a function called when handling requests for periodic
// triggers. The string parameters are the name of the periodic trigger and the
// unique ID of the call to Trigger() which generated the message. The return
// value determines whether or not the pubsub message should be ACK'd. If the
// TriggerCallbackFn returns false, it may be called again. TriggerCallbackFns
// must finish within the PUBSUB_ACK_DEADLINE.
type TriggerCallbackFn func(context.Context, string, string) bool

// validateTrigger returns an error if the given trigger name is not valid.
func validateTrigger(triggerName string) error {
	if !util.In(triggerName, VALID_TRIGGERS) {
		return fmt.Errorf("Invalid trigger name %q", triggerName)
	}
	return nil
}

// validateId returns an error if the valid trigger ID is not valid.
func validateId(triggerId string) error {
	if triggerId == "" {
		return fmt.Errorf("Invalid trigger ID %q", triggerId)
	}
	return nil
}

// setup is a helper function which returns the pubsub client and topic,
// creating the topic if necessary.
func setup(ctx context.Context, ts oauth2.TokenSource) (*pubsub.Client, *pubsub.Topic, error) {
	c, err := pubsub.NewClient(ctx, PUBSUB_PROJECT, option.WithTokenSource(ts))
	if err != nil {
		return nil, nil, err
	}
	t := c.Topic(PUBSUB_TOPIC)
	exists, err := t.Exists(ctx)
	if err != nil {
		return nil, nil, err
	}
	if !exists {
		if _, err := c.CreateTopic(ctx, PUBSUB_TOPIC); err != nil {
			return nil, nil, err
		}
	}
	return c, t, nil
}

// Listen creates a background goroutine which listens for pubsub messages for
// periodic triggers. The subscriber name is used as part of the pubsub
// subscription ID; if there are multiple instances of a server which all need
// to receive every message, they should use different subscriber names.
func Listen(ctx context.Context, subscriberName string, ts oauth2.TokenSource, cb TriggerCallbackFn) error {
	c, t, err := setup(ctx, ts)
	if err != nil {
		return err
	}
	subId := PUBSUB_TOPIC + "+" + subscriberName
	sub := c.Subscription(subId)
	exists, err := sub.Exists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		if _, err := c.CreateSubscription(ctx, subId, pubsub.SubscriptionConfig{
			Topic:       t,
			AckDeadline: PUBSUB_ACK_DEADLINE + 10*time.Second,
		}); err != nil {
			return err
		}
	}

	// Start the receiving goroutine.
	go func() {
		if err := sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
			triggerName := m.Attributes[PUBSUB_ATTR_TRIGGER_NAME]
			if err := validateTrigger(triggerName); err != nil {
				sklog.Errorf("Received invalid pubsub message: %s", err)
				m.Ack()
			}
			triggerId := m.Attributes[PUBSUB_ATTR_TRIGGER_ID]
			if err := validateId(triggerId); err != nil {
				sklog.Errorf("Received invalid pubsub message: %s", err)
				m.Ack()
			}
			if cb(ctx, triggerName, triggerId) {
				m.Ack()
			} else {
				m.Nack()
			}
		}); err != nil {
			sklog.Errorf("Pubsub subscription receive failed: %s", err)
		}
	}()
	return nil
}

// Send a pubsub message for the given periodic trigger. The triggerId may be
// used for de-duplication on the subscriber side and should therefore be unique
// for each invocation of Trigger.
func Trigger(ctx context.Context, triggerName, triggerId string, ts oauth2.TokenSource) error {
	if err := validateTrigger(triggerName); err != nil {
		return err
	}
	if err := validateId(triggerId); err != nil {
		return err
	}
	_, t, err := setup(ctx, ts)
	if err != nil {
		return err
	}
	_, err = t.Publish(ctx, &pubsub.Message{
		Attributes: map[string]string{
			PUBSUB_ATTR_TRIGGER_NAME: triggerName,
			PUBSUB_ATTR_TRIGGER_ID:   triggerId,
		},
	}).Get(ctx)
	return err
}
