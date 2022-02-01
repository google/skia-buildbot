package sink

import (
	"context"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/machine/go/machine/change"
	"go.skia.org/infra/machine/go/machineserver/config"
)

type changeSink struct {
	topic *pubsub.Topic
}

// New returns a new *changeSink.
func New(ctx context.Context, local bool, config config.DescriptionChangeSource) (*changeSink, error) {
	_, topic, err := change.ClientFromConfig(ctx, local, config)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create pubsub client: %q.", config.Project)
	}

	return &changeSink{topic}, nil
}

// Send implements change.Sink.
func (s *changeSink) Send(ctx context.Context, machineID string) error {
	msg := &pubsub.Message{
		Attributes: map[string]string{change.Attribute: machineID},
	}
	pubResult := s.topic.Publish(ctx, msg)
	_, err := pubResult.Get(ctx)
	return err
}

// Assert that *changeSink implements Sink.
var _ Sink = (*changeSink)(nil)
