// Package common has functions useful across its peer modules.
package common

import (
	"context"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/machine/go/machineserver/config"
	"google.golang.org/api/option"
)

// NewPubSubClient creates a new pubsub client from the given config and also
// creates the associated topic specified in the instance config.
func NewPubSubClient(ctx context.Context, local bool, instanceConfig config.InstanceConfig) (*pubsub.Client, error) {
	ts, err := auth.NewDefaultTokenSource(local, pubsub.ScopePubSub)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create token source.")
	}

	pubsubClient, err := pubsub.NewClient(ctx, instanceConfig.Source.Project, option.WithTokenSource(ts))
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create PubSub client for project %s", instanceConfig.Source.Project)
	}
	t := pubsubClient.Topic(instanceConfig.Source.Topic)
	exists, err := t.Exists(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to check existence of PubSub topic %q", t.ID())
	}
	if !exists {
		if _, err := pubsubClient.CreateTopic(ctx, t.ID()); err != nil {
			return nil, skerr.Wrapf(err, "Failed to create PubSub topic %q", t.ID())
		}
	}
	return pubsubClient, nil
}
