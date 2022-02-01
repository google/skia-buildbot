package change

import (
	"context"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/machine/go/machineserver/config"
	"google.golang.org/api/option"
)

// Attribute is the attribute key used on PubSub messages that contains the
// hostname of the target machine, used for filtering PubSub subscriptions.
const Attribute = "hostname"

// ClientFromConfig returns a pubsub client and topic for the given
// configuration.
func ClientFromConfig(ctx context.Context, local bool, config config.DescriptionChangeSource) (*pubsub.Client, *pubsub.Topic, error) {
	ts, err := auth.NewDefaultTokenSource(local, pubsub.ScopePubSub)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "Failed to create TokenSource.")
	}

	client, err := pubsub.NewClient(ctx, config.Project, option.WithTokenSource(ts))
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "Failed to create pubsub client: %q.", config.Project)
	}

	topic := client.Topic(config.Topic)
	exists, err := topic.Exists(ctx)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "Failed to check existence of PubSub topic %q %q", config.Project, topic.ID())
	}
	if !exists {
		if _, err := client.CreateTopic(ctx, topic.ID()); err != nil {
			return nil, nil, skerr.Wrapf(err, "Failed to create PubSub topic %q %q", config.Project, topic.ID())
		}
	}

	return client, topic, nil
}
