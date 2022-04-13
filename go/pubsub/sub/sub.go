// Package sub creates PubSub subscriptions.
package sub

import (
	"context"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/skerr"
	"golang.org/x/oauth2/google"
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

// BroadcastNameProvider implements SubNameProvider. It prevents messages from
// being load-balanced across multiple subscribers by generating unique,
// per-machine subscription names (based on the machine's hostname). Use this
// provider when you want all machines to receive all messages in a topic.
//
// In production, subscription names are appended a suffix to avoid conflicts
// with local subscriptions created during development.
//
// Note that Kubernetes Deployments and ReplicaSets assign fresh hostnames to
// pods, so applications will leave behind one unused subscription per pod when
// restarted. Unused subscriptions will be garbage-collected after 31 days. See
// https://cloud.google.com/pubsub/docs/admin#pubsub_create_pull_subscription-go.
type BroadcastNameProvider struct {
	local     bool
	topicName string
}

// NewBroadcastNameProvider returns a new BroadcastNameProvider.
func NewBroadcastNameProvider(local bool, topicName string) BroadcastNameProvider {
	return BroadcastNameProvider{
		local:     local,
		topicName: topicName,
	}
}

// SubName implements SubNameProvider.
func (b BroadcastNameProvider) SubName() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to get hostname.")
	}
	subName := fmt.Sprintf("%s-%s", b.topicName, hostname)
	if !b.local {
		subName += subscriptionSuffix
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
	return NewWithSubNameProviderAndExpirationPolicy(ctx, local, project, topicName, NewRoundRobinNameProvider(local, topicName), nil, numGoRoutines)
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
	return NewWithSubNameProviderAndExpirationPolicy(ctx, local, project, topicName, NewConstNameProvider(subName), nil, numGoRoutines)
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
	return NewWithSubNameProviderAndExpirationPolicy(ctx, local, project, topicName, subNameProvider, nil, numGoRoutines)
}

// NewWithSubNameProviderAndExpirationPolicy returns a new *pubsub.Subscription.
//
// project is the Google Cloud project that contains the PubSub topic.
//
// topicName is the PubSub topic to listen to.
//
// subNameProvider generates a subscription name.
//
// expirationPolicy determines the inactivity period before the subscription is
// automatically deleted. The minimum allowed value is 1 day. Defaults to 31
// days if nil.
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
func NewWithSubNameProviderAndExpirationPolicy(ctx context.Context, local bool, project string, topicName string, subNameProvider SubNameProvider, expirationPolicy *time.Duration, numGoRoutines int) (*pubsub.Subscription, error) {
	subName, err := subNameProvider.SubName()
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get subscription name.")
	}

	ts, err := google.DefaultTokenSource(ctx, pubsub.ScopePubSub)
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
		config := pubsub.SubscriptionConfig{Topic: topic}
		// The ExpirationPolicy defaults to 31 days if not specified.
		if expirationPolicy != nil {
			config.ExpirationPolicy = *expirationPolicy
		}
		sub, err = pubsubClient.CreateSubscription(ctx, subName, config)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed creating subscription")
		}
	}

	// How many Go routines should be processing messages.
	sub.ReceiveSettings.MaxOutstandingMessages = numGoRoutines * batchSize
	sub.ReceiveSettings.NumGoroutines = numGoRoutines
	return sub, nil
}
