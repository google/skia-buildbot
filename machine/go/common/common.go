// Package common has functions useful across its peer modules.
package common

import (
	"context"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/machine/go/machineserver/config"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

// NewPubSubClient creates a new pubsub client from the given config and also
// creates the associated topic specified in the instance config.
func NewPubSubClient(ctx context.Context, local bool, instanceConfig config.InstanceConfig) (*pubsub.Client, *pubsub.Topic, error) {
	ts, err := google.DefaultTokenSource(ctx, pubsub.ScopePubSub)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "Failed to create token source.")
	}

	pubsubClient, err := pubsub.NewClient(ctx, instanceConfig.Source.Project, option.WithTokenSource(ts))
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "Failed to create PubSub client for project %s", instanceConfig.Source.Project)
	}
	topic := pubsubClient.Topic(instanceConfig.Source.Topic)
	exists, err := topic.Exists(ctx)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "Failed to check existence of PubSub topic %q", topic.ID())
	}
	if !exists {
		if _, err := pubsubClient.CreateTopic(ctx, topic.ID()); err != nil {
			return nil, nil, skerr.Wrapf(err, "Failed to create PubSub topic %q", topic.ID())
		}
	}
	return pubsubClient, topic, nil
}
