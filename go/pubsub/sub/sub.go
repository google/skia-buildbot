// Package sub creates PubSub subscriptions.
package sub

import (
	"context"
	"fmt"
	"os"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/skerr"
	"google.golang.org/api/option"
)

const (
	// batchSize is the batch size of items to receive per Go routine.
	batchSize = 5

	// subscriptionSuffix is the name we append to a topic name to build a
	// subscription name.
	subscriptionSuffix = "-prod"
)

// SubNameProvider is an interface for how a subscription name gets generated
// for a PubSub topic.
type SubNameProvider interface {
	SubName() (string, error)
}

// RoundRobinNameProvider implements SubNameProvider. Use when running in
// production every instance uses the same subscription name so that they
// load-balance pulling items from the topic, and uses a different subscription
// name when running locally.
type RoundRobinNameProvider struct {
	local     bool
	topicName string
}

// NewRoundRobinNameProvider returns a new RoundRobinNameProvider.
func NewRoundRobinNameProvider(local bool, topicName string) RoundRobinNameProvider {
	return RoundRobinNameProvider{
		local:     local,
		topicName: topicName,
	}
}

// SubName implements SubNameProvider.
func (r RoundRobinNameProvider) SubName() (string, error) {
	subName := r.topicName + subscriptionSuffix
	if r.local {
		// When running locally create a new subscription for every host.
		hostname, err := os.Hostname()
		if err != nil {
			return "", skerr.Wrapf(err, "Failed to get hostname.")
		}
		subName = fmt.Sprintf("%s-%s", r.topicName, hostname)
	}
	return subName, nil
}

// ConstNameProvider implements SubNameProvider that always returns the same
// subscription name.
type ConstNameProvider string

// NewConstNameProvider returns a new ConstNameProvider.
func NewConstNameProvider(subName string) ConstNameProvider {
	return ConstNameProvider(subName)
}

// SubName implements SubNameProvider.
func (c ConstNameProvider) SubName() (string, error) {
	return string(c), nil
}

// New returns a new *pubsub.Subscription.
//
// project is the Google Cloud project that contains the PubSub topic.
//
// topicName is the PubSub topic to listen to.
//
// numGoRoutines is the number of Go routines we want to run.
//
// Note that the returned subscription will have both
// sub.ReceiveSettings.MaxOutstandingMessages and
// sub.ReceiveSettings.NumGoroutines set, but they can be changed in the
// returned subscription.
//
// The name of the returned subscription also takes 'local' into account, to
// avoid conflicting with subscriptions running in production. The topic and
// subscription are created if they don't already exist, which requires the
// "PubSub Admin" role.
func New(ctx context.Context, local bool, project string, topicName string, numGoRoutines int) (*pubsub.Subscription, error) {
	return NewWithSubNameProvider(ctx, local, project, topicName, NewRoundRobinNameProvider(local, topicName), numGoRoutines)
}

// NewWithSubName returns a new *pubsub.Subscription.
//
// project is the Google Cloud project that contains the PubSub topic.
//
// topicName is the PubSub topic to listen to.
//
// subName is the name of the subscription.
//
// numGoRoutines is the number of Go routines we want to run.
//
// Note that the returned subscription will have both
// sub.ReceiveSettings.MaxOutstandingMessages and
// sub.ReceiveSettings.NumGoroutines set, but they can be changed in the
// returned subscription.
//
// The topic and subscription are created if they don't already exist, which
// requires the "PubSub Admin" role.
func NewWithSubName(ctx context.Context, local bool, project string, topicName string, subName string, numGoRoutines int) (*pubsub.Subscription, error) {
	return NewWithSubNameProvider(ctx, local, project, topicName, NewConstNameProvider(subName), numGoRoutines)
}

// NewWithSubNameProvider returns a new *pubsub.Subscription.
//
// project is the Google Cloud project that contains the PubSub topic.
//
// topicName is the PubSub topic to listen to.
//
// subNameProvider generates a subscription name.
//
// numGoRoutines is the number of Go routines we want to run.
//
// Note that the returned subscription will have both
// sub.ReceiveSettings.MaxOutstandingMessages and
// sub.ReceiveSettings.NumGoroutines set, but they can be changed in the
// returned subscription.
//
// The topic and subscription are created if they don't already exist, which
// requires the "PubSub Admin" role.
func NewWithSubNameProvider(ctx context.Context, local bool, project string, topicName string, subNameProvider SubNameProvider, numGoRoutines int) (*pubsub.Subscription, error) {
	subName, err := subNameProvider.SubName()
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get subscription name.")
	}

	ts, err := auth.NewDefaultTokenSource(local, pubsub.ScopePubSub)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create token source.")
	}

	pubsubClient, err := pubsub.NewClient(ctx, project, option.WithTokenSource(ts))
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create PubSub client for project %s", project)
	}
	topic := pubsubClient.Topic(topicName)
	exists, err := topic.Exists(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to check existence of PubSub topic %q", topic.ID())
	}
	if !exists {
		if _, err := pubsubClient.CreateTopic(ctx, topic.ID()); err != nil {
			return nil, skerr.Wrapf(err, "Failed to create PubSub topic %q", topic.ID())
		}
	}

	sub := pubsubClient.Subscription(subName)
	ok, err := sub.Exists(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed checking subscription existence: %q", subName)
	}
	if !ok {
		sub, err = pubsubClient.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
			Topic: topic,
		})
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed creating subscription")
		}
	}

	// How many Go routines should be processing messages.
	sub.ReceiveSettings.MaxOutstandingMessages = numGoRoutines * batchSize
	sub.ReceiveSettings.NumGoroutines = numGoRoutines
	return sub, nil

}
