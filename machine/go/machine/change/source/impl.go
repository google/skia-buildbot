package source

import (
	"context"
	"fmt"
	"os"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/machine/go/machine/change"
	"go.skia.org/infra/machine/go/machineserver/config"
)

// changeSource implements Source.
type changeSource struct {
	sub       *pubsub.Subscription
	machineID string
}

// New returns a new changeSource.
func New(ctx context.Context, local bool, config config.DescriptionChangeSource, machineID string) (*changeSource, error) {
	client, topic, err := change.ClientFromConfig(ctx, local, config)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create pubsub client: %q.", config.Project)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get hostname.")
	}
	subName := fmt.Sprintf("%s-%s-%s", config.Topic, machineID, hostname)
	if !local {
		subName += "-prod"
	}

	// Filter the subscription so that we only see messages for this machine.
	cfg := pubsub.SubscriptionConfig{
		Topic:  topic,
		Filter: fmt.Sprintf("attributes.%s = %q", change.Attribute, machineID),
	}
	sub := client.Subscription(subName)
	exists, err := sub.Exists(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to check subscription existence.")
	}

	if !exists {
		sub, err = client.CreateSubscription(ctx, subName, cfg)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to create subscription.")
		}
	}

	return &changeSource{
		sub:       sub,
		machineID: machineID,
	}, nil
}

// Start implement change.Source.
func (s *changeSource) Start(ctx context.Context) (<-chan interface{}, error) {

	ch := make(chan interface{})

	go func() {
		if err := s.sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
			if m.Attributes[change.Attribute] != s.machineID {
				// We are effectively implementing filtering again here because
				// the PubSub emulator doesn't implement filtering.
				return
			}
			ch <- nil
			m.Ack()
		}); err != nil {
			sklog.Errorf("Pubsub subscription receive failed: %s", err)
			close(ch)
		}
	}()

	return ch, nil
}

// Assert that *changeSource implements the Source interface.
var _ Source = (*changeSource)(nil)
