package pubsub

import (
	"context"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/louhi"
	"go.skia.org/infra/go/pubsub/sub"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/encoding/prototext"
)

const pubsubTopic = "louhi-notifications"

var protoMarshalOpts = &prototext.MarshalOptions{
	Multiline: true,
}

// ListenPubSub starts listening for pub/sub events and pushing them into the
// given DB. Attempts to create the topic if it does not already exist.
func ListenPubSub(ctx context.Context, db louhi.DB, local bool, project string) error {
	sub, err := sub.New(ctx, local, project, pubsubTopic, 1)
	if err != nil {
		return skerr.Wrapf(err, "failed to create subscription")
	}
	go func() {
		for {
			err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
				var n louhi.Notification
				if err := prototext.Unmarshal(msg.Data, &n); err != nil {
					// We can't handle this message, so Ack() it and we won't try
					// again.
					msg.Ack()
					sklog.Errorf("Failed to decode message as text proto. Message:\n%s\nError: %s", string(msg.Data), err)
					return
				}
				if err := louhi.UpdateFlowFromNotification(ctx, db, &n, msg.PublishTime); err != nil {
					// This might be a transient error, so Nack() the message and
					// we'll try again.
					msg.Nack()
					sklog.Errorf("failed to update flow in DB: %s", err)
					return
				}
				// We successfully handled the message.
				msg.Ack()
			})
			if err != nil {
				sklog.Errorf("Failed receiving pubsub message: %s", err)
			}
		}
	}()
	return nil
}

// PubSubSender is used for sending pub/sub messages.
type PubSubSender struct {
	topic *pubsub.Topic
}

// NewPubSubSender returns a pub/sub notification sender. Does not attempt to
// create the topic.
func NewPubSubSender(ctx context.Context, project string) (*PubSubSender, error) {
	ts, err := google.DefaultTokenSource(ctx, pubsub.ScopePubSub)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create token source.")
	}

	pubsubClient, err := pubsub.NewClient(ctx, project, option.WithTokenSource(ts))
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create PubSub client for project %s", project)
	}
	topic := pubsubClient.Topic(pubsubTopic)
	return &PubSubSender{
		topic: topic,
	}, nil
}

// Send sends a pub/sub notification and blocks until the message is sent.
func (s *PubSubSender) Send(ctx context.Context, n *louhi.Notification) error {
	data, err := protoMarshalOpts.Marshal(n)
	if err != nil {
		return skerr.Wrapf(err, "failed to encode message")
	}
	msg := &pubsub.Message{
		Data: data,
	}
	pr := s.topic.Publish(ctx, msg)
	if _, err := pr.Get(ctx); err != nil {
		return skerr.Wrapf(err, "failed to send message")
	}
	return nil
}
