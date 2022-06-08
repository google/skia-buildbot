package pubsub

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

const (
	// Auth scope required to use this package.
	AUTH_SCOPE = pubsub.ScopePubSub

	// Template used for building pubsub topic names.
	TOPIC_TMPL = "gitstore-%s-%s-%d"
)

// topicName returns the pubsub topic name for the given BT instance and repo.
func topicName(btInstance, btTable string, repoID int64) string {
	return fmt.Sprintf(TOPIC_TMPL, btInstance, btTable, repoID)
}

// client is a struct used for common setup between Publisher and Subscriber.
type client struct {
	client *pubsub.Client
	topic  *pubsub.Topic
}

// newClient returns a client instance, creating the PubSub topic if requested.
func newClient(ctx context.Context, btConf *bt_gitstore.BTConfig, repoID int64, ts oauth2.TokenSource, createTopic bool) (*client, error) {
	c, err := pubsub.NewClient(ctx, btConf.ProjectID, option.WithTokenSource(ts))
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create PubSub client for project %s", btConf.ProjectID)
	}
	t := c.Topic(topicName(btConf.InstanceID, btConf.TableID, repoID))
	exists, err := t.Exists(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to check existence of PubSub topic %q", t.ID())
	}
	if !exists {
		if !createTopic {
			return nil, skerr.Fmt("PubSub topic %q does not exist; verify that the requested repo is being ingested: %d", t.ID(), repoID)
		}
		if _, err := c.CreateTopic(ctx, t.ID()); err != nil {
			return nil, skerr.Wrapf(err, "failed to create PubSub topic %q for %d", t.ID(), repoID)
		}
	}
	return &client{
		client: c,
		topic:  t,
	}, nil
}

// Publisher is a struct used for publishing pubsub messages for a GitStore.
type Publisher struct {
	*client
	queued sync.WaitGroup
}

// NewPublisher returns a Publisher instance associated with the given GitStore.
func NewPublisher(ctx context.Context, btConf *bt_gitstore.BTConfig, repoID int64, ts oauth2.TokenSource) (*Publisher, error) {
	client, err := newClient(ctx, btConf, repoID, ts, true)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create GitStore PubSub publisher")
	}
	p := &Publisher{
		client: client,
	}
	cleanup.AtExit(func() {
		p.Wait()
	})
	return p, nil
}

// Publish a pubsub message with the given updated branch heads. Typically, only
// the branch heads which have changed should be included. The message is sent
// asynchronously.
func (p *Publisher) Publish(ctx context.Context, branches map[string]string) {
	res := p.topic.Publish(ctx, &pubsub.Message{
		// TODO(borenet): Is it valid to add arbitrary data to this
		// field? The docs do not indicate otherwise, and this is more
		// convenient than having to encode/decode the map ourselves.
		Attributes: branches,
	})
	p.queued.Add(1)
	go func() {
		defer p.queued.Done()
		if _, err := res.Get(ctx); err != nil {
			sklog.Errorf("Failed to send pubsub message: %s", err)
		}
	}()
}

// Wait for all pubsub messages to be sent.
func (p *Publisher) Wait() {
	sklog.Info("Waiting for pubsub messages to be sent...")
	p.queued.Wait()
	sklog.Info("All pubsub messages have been sent.")
}

// NewSubscriber creates a pubsub subscription associated with the given
// GitStore and calls the given function whenever a message is received. The
// parameters to the callback function are the message itself and the branch
// heads as of the time that the message was sent, with names as keys and commit
// hashes as values. The callback function is responsible for calling Ack() or
// Nack() on the message.
func NewSubscriber(ctx context.Context, btConf *bt_gitstore.BTConfig, subscriberID string, repoID int64, ts oauth2.TokenSource, callback func(*pubsub.Message, map[string]string)) error {
	c, err := newClient(ctx, btConf, repoID, ts, false)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create GitStore PubSub subscriber")
	}
	sub := c.client.Subscription(c.topic.ID() + "_" + subscriberID)
	exists, err := sub.Exists(ctx)
	if err != nil {
		return skerr.Wrapf(err, "Failed to check existence of PubSub subscription %q", sub.ID())
	}
	if !exists {
		_, err := c.client.CreateSubscription(ctx, sub.ID(), pubsub.SubscriptionConfig{
			Topic: c.topic,
		})
		if err != nil {
			return skerr.Wrapf(err, "Failed to create PubSub subscription %q", sub.ID())
		}
	}
	go func() {
		for {
			if ctx.Err() != nil {
				sklog.Errorf("Context has error: %s", ctx.Err())
				return
			}
			if err := sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
				select {
				case <-ctx.Done():
					sklog.Warning("Received pubsub message but the context has been canceled.")
					m.Nack()
				default:
					callback(m, m.Attributes)
				}
			}); err != nil {
				sklog.Errorf("Pubsub subscription (ID %q) receive failed: %s", sub.ID(), err)
				time.Sleep(time.Second)
			}
		}
	}()
	return nil
}
