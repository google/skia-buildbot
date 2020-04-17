package sink

import (
	"context"
	"encoding/json"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/machineserver/config"
	"go.skia.org/infra/sk8s/go/bot_config/common"
)

// SinkImpl implements the Sink interface using Google Cloud PubSub.
type SinkImpl struct {
	pubsubClient *pubsub.Client
	topic        *pubsub.Topic
}

// New return a new SinkImpl instance.
func New(ctx context.Context, local bool, instanceConfig config.InstanceConfig) (*SinkImpl, error) {
	pubsubClient, err := common.NewPubSubClient(ctx, local, instanceConfig)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create pubsub client for SinkImpl.")
	}
	return &SinkImpl{
		pubsubClient: pubsubClient,
		topic:        pubsubClient.Topic(instanceConfig.Source.Topic),
	}, nil
}

// Send implements the Sink interface.
func (s *SinkImpl) Send(ctx context.Context, event machine.Event) error {
	b, err := json.Marshal(event)
	if err != nil {
		return skerr.Wrapf(err, "Failed to serialize the event.")
	}
	msg := &pubsub.Message{
		Data: b,
	}
	_, err = s.topic.Publish(ctx, msg).Get(ctx)
	if err != nil {
		return skerr.Wrapf(err, "Failed to send message.")
	}
	return nil
}

// Affirm that *SinkImpl implements the Sink interface.
var _ Sink = (*SinkImpl)(nil)
