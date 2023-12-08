// Package pubsub contains utilities for working with Cloud PubSub.
package pubsub

import (
	"context"
	"os"
	"time"

	"cloud.google.com/go/iam"
	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/emulators"
	"go.skia.org/infra/go/skerr"
	"google.golang.org/api/option"
)

// ScopePubSub aliases the same constant from the pubsub package to prevent
// callers from needing to import both packages.
const ScopePubSub = pubsub.ScopePubSub

// EnsureNotEmulator panics if the PubSub emulator environment variable is set.
func EnsureNotEmulator() {
	if os.Getenv(string(emulators.PubSub)) != "" {
		panic("PubSub Emulator detected. Be sure to unset the following environment variable: " + emulators.GetEmulatorHostEnvVarName(emulators.PubSub))
	}
}

// Client represents a pubsub.Client in an interface which can be mocked for
// testing.
type Client interface {
	Close() error
	CreateSubscription(ctx context.Context, id string, cfg pubsub.SubscriptionConfig) (Subscription, error)
	CreateTopic(ctx context.Context, topicID string) (Topic, error)
	CreateTopicWithConfig(ctx context.Context, topicID string, tc *pubsub.TopicConfig) (Topic, error)
	DetachSubscription(ctx context.Context, sub string) (*pubsub.DetachSubscriptionResult, error)
	Project() string

	Snapshot(id string) Snapshot
	Snapshots(ctx context.Context) *pubsub.SnapshotConfigIterator

	Subscription(id string) Subscription
	SubscriptionInProject(id, projectID string) Subscription
	Subscriptions(ctx context.Context) *pubsub.SubscriptionIterator

	Topic(id string) Topic
	TopicInProject(id, projectID string) Topic
	Topics(ctx context.Context) *pubsub.TopicIterator
}

// Snapshot represents a pubsub.Snapshot in an interface which can be mocked for
// testing.
type Snapshot interface {
	Delete(ctx context.Context) error
	ID() string
	SetLabels(ctx context.Context, label map[string]string) (*pubsub.SnapshotConfig, error)
}

// Subscription represents a pubsub.Subscription in an interface which can be
// mocked for testing.
type Subscription interface {
	Config(ctx context.Context) (pubsub.SubscriptionConfig, error)
	CreateSnapshot(ctx context.Context, name string) (*pubsub.SnapshotConfig, error)
	Delete(ctx context.Context) error
	Exists(ctx context.Context) (bool, error)
	IAM() *iam.Handle
	ID() string
	Receive(ctx context.Context, f func(context.Context, *pubsub.Message)) error
	SeekToSnapshot(ctx context.Context, snap Snapshot) error
	SeekToTime(ctx context.Context, t time.Time) error
	String() string
	Update(ctx context.Context, cfg pubsub.SubscriptionConfigToUpdate) (pubsub.SubscriptionConfig, error)
}

// Topic represents a pubsub.Topic in an interface which can be mocked for
// testing.
type Topic interface {
	Config(ctx context.Context) (pubsub.TopicConfig, error)
	Delete(ctx context.Context) error
	Exists(ctx context.Context) (bool, error)
	Flush()
	IAM() *iam.Handle
	ID() string
	Publish(ctx context.Context, msg *pubsub.Message) PublishResult
	ResumePublish(orderingKey string)
	Stop()
	String() string
	Subscriptions(ctx context.Context) *pubsub.SubscriptionIterator
	Update(ctx context.Context, cfg pubsub.TopicConfigToUpdate) (pubsub.TopicConfig, error)
}

// PublishResult represents a pubsub.PublishResult in an interface which can be
// mocked for testing.
type PublishResult interface {
	Get(ctx context.Context) (serverID string, err error)
	Ready() <-chan struct{}
}

// WrappedClient implements Client by wrapping a pubsub.Client.
type WrappedClient struct {
	*pubsub.Client
}

// NewClient creates a new WrappedClient.
func NewClient(ctx context.Context, projectID string, opts ...option.ClientOption) (*WrappedClient, error) {
	client, err := pubsub.NewClient(ctx, projectID, opts...)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &WrappedClient{client}, nil
}

// NewClientWithConfig creates a new WrappedClient with the given config..
func NewClientWithConfig(ctx context.Context, projectID string, config *pubsub.ClientConfig, opts ...option.ClientOption) (*WrappedClient, error) {
	client, err := pubsub.NewClientWithConfig(ctx, projectID, config, opts...)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &WrappedClient{client}, nil
}

// CreateSubscription implements Client.
func (w *WrappedClient) CreateSubscription(ctx context.Context, id string, cfg pubsub.SubscriptionConfig) (Subscription, error) {
	sub, err := w.Client.CreateSubscription(ctx, id, cfg)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &WrappedSubscription{sub}, nil
}

// CreateTopic implements Client.
func (w *WrappedClient) CreateTopic(ctx context.Context, topicID string) (Topic, error) {
	topic, err := w.Client.CreateTopic(ctx, topicID)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &WrappedTopic{topic}, nil
}

// CreateTopicWithConfig implements Client.
func (w *WrappedClient) CreateTopicWithConfig(ctx context.Context, topicID string, tc *pubsub.TopicConfig) (Topic, error) {
	topic, err := w.Client.CreateTopicWithConfig(ctx, topicID, tc)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &WrappedTopic{topic}, nil
}

// Snapshot implements Client.
func (w *WrappedClient) Snapshot(id string) Snapshot {
	return w.Client.Snapshot(id)
}

// Subscription implements Client.
func (w *WrappedClient) Subscription(id string) Subscription {
	return &WrappedSubscription{w.Client.Subscription(id)}
}

// SubscriptionInProject implements Client.
func (w *WrappedClient) SubscriptionInProject(id, projectID string) Subscription {
	return &WrappedSubscription{w.Client.SubscriptionInProject(id, projectID)}
}

// Topic implements Client.
func (w *WrappedClient) Topic(id string) Topic {
	return &WrappedTopic{w.Client.Topic(id)}
}

// TopicInProject implements Client.
func (w *WrappedClient) TopicInProject(id, projectID string) Topic {
	return &WrappedTopic{w.Client.TopicInProject(id, projectID)}
}

// WrappedSubscription implements Subscription by wrapping a
// pubsub.Subscription.
type WrappedSubscription struct {
	*pubsub.Subscription
}

// SeekToSnapshot implements Client.
func (w *WrappedSubscription) SeekToSnapshot(ctx context.Context, snap Snapshot) error {
	wrapped, ok := snap.(*pubsub.Snapshot)
	if !ok {
		return skerr.Fmt("expected a pubsub.Snapshot")
	}
	return w.Subscription.SeekToSnapshot(ctx, wrapped)
}

// WrappedTopic implements Topic by wrapping a pubsub.Topic.
type WrappedTopic struct {
	*pubsub.Topic
}

// Publish implements Client.
func (w *WrappedTopic) Publish(ctx context.Context, msg *pubsub.Message) PublishResult {
	return w.Topic.Publish(ctx, msg)
}

// Assert that we implement the interfaces.
var _ Client = &WrappedClient{}
var _ Subscription = &WrappedSubscription{}
var _ Topic = &WrappedTopic{}
