package candidate_stats

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"sort"
	"strings"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/deploy"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	db_pubsub "go.skia.org/infra/task_scheduler/go/db/pubsub"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

const (
	projectId       = db_pubsub.PROJECT_ID
	pubsubTopicTmpl = "task-candidate-stats-%s"
)

// CandidateStats is a struct used for communicating the number of not-yet-
// scheduled task candidates by dimension set.
type CandidateStats struct {
	Dimensions []string `json:"dimensions"`
	Count      int      `json:"count"`
}

// Client wraps a pubsub.Client to Publish and/or Receive Pub/Sub messages
// about task candidate statistics.
type Client struct {
	c     *pubsub.Client
	topic *pubsub.Topic
}

// NewClient returns a Client.
func NewClient(ctx context.Context, ts oauth2.TokenSource, deployment deploy.Deployment) (*Client, error) {
	topicName := fmt.Sprintf(pubsubTopicTmpl, deployment)
	c, err := pubsub.NewClient(ctx, projectId, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("Failed to create pubsub client: %s", err)
	}
	topic := c.Topic(topicName)
	if exists, err := topic.Exists(ctx); err != nil {
		return nil, fmt.Errorf("Failed to check for pubsub topic existence: %s", err)
	} else if !exists {
		if _, err := c.CreateTopic(ctx, topicName); err != nil {
			return nil, fmt.Errorf("Failed to create pubsub topic: %s", err)
		}
	}
	return &Client{
		c:     c,
		topic: topic,
	}, nil
}

// Publish the CandidateStats.
func (c *Client) Publish(ctx context.Context, stats []*CandidateStats) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(stats); err != nil {
		return fmt.Errorf("Failed to encode messge: %s", err)
	}
	_, err := c.topic.Publish(ctx, &pubsub.Message{
		Data: buf.Bytes(),
	}).Get(ctx)
	return err
}

// Call f when new CandidateStats are received.
func (c *Client) Receive(ctx context.Context, subscriberName string, f func(context.Context, []*CandidateStats)) error {
	subName := fmt.Sprintf("%s+%s", c.topic.ID(), subscriberName)
	sub := c.c.Subscription(subName)
	if exists, err := sub.Exists(ctx); err != nil {
		return fmt.Errorf("Failed to check for pubsub subscription existence: %s", err)
	} else if !exists {
		if _, err := c.c.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
			Topic: c.topic,
		}); err != nil {
			return fmt.Errorf("Failed to create pubsub subscription: %s", err)
		}
	}
	return sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		var stats []*CandidateStats
		if err := gob.NewDecoder(bytes.NewReader(msg.Data)).Decode(&stats); err != nil {
			sklog.Errorf("Failed to decode message: %s", err)
		} else {
			f(ctx, stats)
		}
		msg.Ack()
	})
}

// Counter helps to count candidates by dimension set. Not thread-safe.
type Counter struct {
	client *Client
	stats  map[string]*CandidateStats
}

// NewCounter returns a Counter instance.
func (c *Client) NewCounter() *Counter {
	return &Counter{
		client: c,
		stats:  map[string]*CandidateStats{},
	}
}

// key returns a string key for the given dimension set.
func (c *Counter) key(dims []string) string {
	cpy := util.CopyStringSlice(dims)
	sort.Strings(cpy)
	return strings.Join(cpy, "\n")
}

// Add the given dimensions to the counter.
func (c *Counter) Add(dims []string) {
	key := c.key(dims)
	s, ok := c.stats[key]
	if !ok {
		s = &CandidateStats{
			Dimensions: dims,
		}
		c.stats[key] = s
	}
	s.Count++
}

// Return the CandidateStats.
func (c *Counter) Get() []*CandidateStats {
	rv := make([]*CandidateStats, 0, len(c.stats))
	for _, s := range c.stats {
		rv = append(rv, s)
	}
	return rv
}

// Publish the CandidateStats.
func (c *Counter) Publish(ctx context.Context) error {
	return c.client.Publish(ctx, c.Get())
}
