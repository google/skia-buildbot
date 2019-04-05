package pubsub

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/types"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

const (
	// Auth scope.
	AUTH_SCOPE = pubsub.ScopePubSub

	// Sets of topic, based on scheduler instance.
	TOPIC_SET_PRODUCTION = "production"
	TOPIC_SET_INTERNAL   = "internal"
	TOPIC_SET_STAGING    = "staging"

	// Known topic names.
	TOPIC_TASKS                      = "task-scheduler-modified-tasks"
	TOPIC_TASKS_INTERNAL             = "task-scheduler-modified-tasks-internal"
	TOPIC_TASKS_STAGING              = "task-scheduler-modified-tasks-staging"
	TOPIC_JOBS                       = "task-scheduler-modified-jobs"
	TOPIC_JOBS_INTERNAL              = "task-scheduler-modified-jobs-internal"
	TOPIC_JOBS_STAGING               = "task-scheduler-modified-jobs-staging"
	TOPIC_TASK_COMMENTS              = "task-scheduler-modified-task-comments"
	TOPIC_TASK_COMMENTS_INTERNAL     = "task-scheduler-modified-task-comments-internal"
	TOPIC_TASK_COMMENTS_STAGING      = "task-scheduler-modified-task-comments-staging"
	TOPIC_TASKSPEC_COMMENTS          = "task-scheduler-modified-taskspec-comments"
	TOPIC_TASKSPEC_COMMENTS_INTERNAL = "task-scheduler-modified-taskspec-comments-internal"
	TOPIC_TASKSPEC_COMMENTS_STAGING  = "task-scheduler-modified-taskspec-comments-staging"
	TOPIC_COMMIT_COMMENTS            = "task-scheduler-modified-commit-comments"
	TOPIC_COMMIT_COMMENTS_INTERNAL   = "task-scheduler-modified-commit-comments-internal"
	TOPIC_COMMIT_COMMENTS_STAGING    = "task-scheduler-modified-commit-comments-staging"

	// Attributes sent with all pubsub messages.

	// Job or task ID.
	ATTR_ID = "id"
	// Modification or insertion timestamp of the contained data.
	ATTR_TIMESTAMP = "ts"
	// Unique identifier for the sender of the message.
	ATTR_SENDER_ID = "sender"
)

var (
	VALID_TOPIC_SETS = []string{
		TOPIC_SET_PRODUCTION,
		TOPIC_SET_INTERNAL,
		TOPIC_SET_STAGING,
	}

	topics = map[string]topicSet{
		TOPIC_SET_PRODUCTION: topicSet{
			tasks:            TOPIC_TASKS,
			jobs:             TOPIC_JOBS,
			taskComments:     TOPIC_TASK_COMMENTS,
			taskSpecComments: TOPIC_TASKSPEC_COMMENTS,
			commitComments:   TOPIC_COMMIT_COMMENTS,
		},
		TOPIC_SET_INTERNAL: topicSet{
			tasks:            TOPIC_TASKS_INTERNAL,
			jobs:             TOPIC_JOBS_INTERNAL,
			taskComments:     TOPIC_TASK_COMMENTS_INTERNAL,
			taskSpecComments: TOPIC_TASKSPEC_COMMENTS_INTERNAL,
			commitComments:   TOPIC_COMMIT_COMMENTS_INTERNAL,
		},
		TOPIC_SET_STAGING: topicSet{
			tasks:            TOPIC_TASKS_STAGING,
			jobs:             TOPIC_JOBS_STAGING,
			taskComments:     TOPIC_TASK_COMMENTS_STAGING,
			taskSpecComments: TOPIC_TASKSPEC_COMMENTS_STAGING,
			commitComments:   TOPIC_COMMIT_COMMENTS_STAGING,
		},
	}
)

// topicSet is used for organizing sets of pubsub topics.
type topicSet struct {
	tasks            string
	jobs             string
	taskComments     string
	taskSpecComments string
	commitComments   string
}

// publisher sends pubsub messages for modified tasks and jobs.
type publisher struct {
	senderId string
	topic    *pubsub.Topic
	queued   sync.WaitGroup
}

// newPublisher is a helper function for NewTaskPublisher.
func newPublisher(c *pubsub.Client, topic string) (*publisher, error) {
	t := c.Topic(topic)
	exists, err := t.Exists(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Failed to check for topic %q existence: %s", topic, err)
	}
	if !exists {
		if _, err := c.CreateTopic(context.Background(), topic); err != nil {
			return nil, fmt.Errorf("Failed to create topic: %s", err)
		}
	}
	p := &publisher{
		senderId: uuid.New().String(),
		topic:    t,
	}
	cleanup.AtExit(func() {
		sklog.Info("Waiting for pubsub messages to be sent...")
		p.queued.Wait()
		sklog.Info("All pubsub messages have been sent.")
	})
	return p, nil
}

// publish publishes a pubsub message for the given data.
func (p *publisher) publish(id string, ts time.Time, data interface{}) {
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(data); err != nil {
		sklog.Fatal(err)
	}
	p.publishGOB(ts, map[string][]byte{
		id: buf.Bytes(),
	})
}

// publishGOB publishes a pubsub message for the given each of gob-encoded data.
func (p *publisher) publishGOB(ts time.Time, byId map[string][]byte) {
	ctx := context.Background()
	res := make([]*pubsub.PublishResult, 0, len(byId))
	for id, data := range byId {
		res = append(res, p.topic.Publish(ctx, &pubsub.Message{
			Data: data,
			Attributes: map[string]string{
				ATTR_ID:        id,
				ATTR_TIMESTAMP: ts.Format(util.RFC3339NanoZeroPad),
				ATTR_SENDER_ID: p.senderId,
			},
		}))
	}
	for _, result := range res {
		p.queued.Add(1)
		go func(result *pubsub.PublishResult) {
			defer p.queued.Done()
			if _, err := result.Get(ctx); err != nil {
				sklog.Errorf("Failed to send pubsub message: %s", err)
			}
		}(result)
	}
}

// TaskPublisher sends pubsub messages for modified tasks.
type TaskPublisher struct {
	*publisher
}

// NewTaskPublisher creates a TaskPublisher instance. It creates the given topic
// if it does not already exist.
func NewTaskPublisher(projectId, topic string, ts oauth2.TokenSource) (*TaskPublisher, error) {
	c, err := pubsub.NewClient(context.Background(), projectId, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	pub, err := newPublisher(c, topic)
	if err != nil {
		return nil, err
	}
	return &TaskPublisher{pub}, nil
}

// Publish publishes a pubsub message for the given task.
func (p *TaskPublisher) Publish(t *types.Task) {
	p.publish(t.Id, t.DbModified, t)
}

// JobPublisher sends pubsub messages for modified jobs.
type JobPublisher struct {
	*publisher
}

// NewJobPublisher creates a JobPublisher instance. It creates the given topic
// if it does not already exist.
func NewJobPublisher(projectId, topic string, ts oauth2.TokenSource) (*JobPublisher, error) {
	c, err := pubsub.NewClient(context.Background(), projectId, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	pub, err := newPublisher(c, topic)
	if err != nil {
		return nil, err
	}
	return &JobPublisher{pub}, nil
}

// Publish publishes a pubsub message for the given job.
func (p *JobPublisher) Publish(j *types.Job) {
	p.publish(j.Id, j.DbModified, j)
}

// TaskSpecCommentPublisher sends pubsub messages for comments.
type TaskSpecCommentPublisher struct {
	*publisher
}

// NewTaskSpecCommentPublisher creates a TaskSpecCommentPublisher instance. It
// creates the given topic if it does not already exist.
func NewTaskSpecCommentPublisher(projectId, topic string, ts oauth2.TokenSource) (*TaskSpecCommentPublisher, error) {
	c, err := pubsub.NewClient(context.Background(), projectId, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	pub, err := newPublisher(c, topic)
	if err != nil {
		return nil, err
	}
	return &TaskSpecCommentPublisher{pub}, nil
}

// Publish publishes a pubsub message for the given comment.
func (p *TaskSpecCommentPublisher) Publish(t *types.TaskSpecComment) {
	p.publish(t.Id(), t.Timestamp, t)
}

// TaskCommentPublisher sends pubsub messages for comments.
type TaskCommentPublisher struct {
	*publisher
}

// NewTaskCommentPublisher creates a TaskCommentPublisher instance. It creates the given
// topic if it does not already exist.
func NewTaskCommentPublisher(projectId, topic string, ts oauth2.TokenSource) (*TaskCommentPublisher, error) {
	c, err := pubsub.NewClient(context.Background(), projectId, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	pub, err := newPublisher(c, topic)
	if err != nil {
		return nil, err
	}
	return &TaskCommentPublisher{pub}, nil
}

// Publish publishes a pubsub message for the given comment.
func (p *TaskCommentPublisher) Publish(t *types.TaskComment) {
	p.publish(t.Id(), t.Timestamp, t)
}

// CommitCommentPublisher sends pubsub messages for comments.
type CommitCommentPublisher struct {
	*publisher
}

// NewCommitCommentPublisher creates a CommitCommentPublisher instance. It creates the given
// topic if it does not already exist.
func NewCommitCommentPublisher(projectId, topic string, ts oauth2.TokenSource) (*CommitCommentPublisher, error) {
	c, err := pubsub.NewClient(context.Background(), projectId, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	pub, err := newPublisher(c, topic)
	if err != nil {
		return nil, err
	}
	return &CommitCommentPublisher{pub}, nil
}

// Publish publishes a pubsub message for the given comment.
func (p *CommitCommentPublisher) Publish(t *types.CommitComment) {
	p.publish(t.Id(), t.Timestamp, t)
}

// subscriber uses pubsub to watch for modified data.
type subscriber struct {
	client   *pubsub.Client
	callback func(*pubsub.Message) error
	id       string
	topic    string
	sub      *pubsub.Subscription
}

// newSubscriber creates a subscriber instance which calls the given callback
// function for every pubsub message. The topic should be one of the TOPIC_*
// constants defined in this package. The subscriberLabel is included in the
// subscription ID, along with a timestamp; this should help to debug zombie
// subscriptions. The callback function should not Ack/Nack the message; this
// is done automatically based on the return value of the callback: if the
// callback returns an error, the message is Nack'd and will be re-sent at a
// later time, otherwise the message is Ack'd and will not be re-sent.
// Therefore, if the message data is not valid or otherwise cannot ever be
// processed, the callback should return nil to prevent the message from being
// re-sent.
func newSubscriber(c *pubsub.Client, topic, subscriberLabel string, callback func(*pubsub.Message) error) (*subscriber, error) {
	// Create a pubsub subscription. This will return an error if we somehow
	// reused an ID.
	id := topic + "+" + subscriberLabel + "_" + time.Now().Format(util.SAFE_TIMESTAMP_FORMAT)
	return &subscriber{
		client:   c,
		callback: callback,
		topic:    topic,
		id:       id,
		sub:      c.Subscription(id),
	}, nil
}

// SubscriberID returns the ID of the pubsub subscription.
func (s *subscriber) SubscriberID() string {
	return s.id
}

// start causes the subscriber to start watching for modified data.
func (s *subscriber) start() (context.CancelFunc, error) {
	var cancelFn context.CancelFunc
	errCh := make(chan error)
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		sub, err := s.client.CreateSubscription(ctx, s.id, pubsub.SubscriptionConfig{
			Topic: s.client.Topic(s.topic),
		})
		errCh <- err
		if err != nil {
			return
		}
		cancelFn = cancel
		if err := sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
			select {
			case <-ctx.Done():
				sklog.Warning("Received pubsub message but the context has been canceled.")
				m.Nack()
			default:
				if err := s.callback(m); err != nil {
					sklog.Warningf("Callback failed for pubsub message: %s", err)
					m.Nack()
				} else {
					m.Ack()
				}
			}
		}); err != nil {
			sklog.Errorf("Pubsub subscription receive failed: %s", err)
		}
	}()
	err := <-errCh
	if err != nil {
		return nil, fmt.Errorf("Failed to create subscription: %s", err)
	}
	return func() {
		cancelFn()
		if err := s.client.Subscription(s.id).Delete(context.Background()); err != nil {
			sklog.Errorf("Failed to delete pubsub subscription: %s", err)
		}
	}, nil
}

// NewTaskSubscriber creates a subscriber which calls the given callback
// function for every pubsub message. The topic should be one of the TOPIC_*
// constants defined in this package. The subscriberLabel is included in the
// subscription ID, along with a timestamp; this should help to debug zombie
// subscriptions. Acknowledgement of the message is done automatically based on
// the return value of the callback: if the callback returns an error, the
// message is Nack'd and will be re-sent at a later time, otherwise the message
// is Ack'd and will not be re-sent. Therefore, if the task is not valid or
// otherwise cannot ever be processed, the callback should return nil to prevent
// the message from being re-sent.
func NewTaskSubscriber(projectId, topic, subscriberLabel string, ts oauth2.TokenSource, callback func(*types.Task) error) (context.CancelFunc, error) {
	c, err := pubsub.NewClient(context.Background(), projectId, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	s, err := newSubscriber(c, topic, subscriberLabel, func(m *pubsub.Message) error {
		var t types.Task
		if err := gob.NewDecoder(bytes.NewReader(m.Data)).Decode(&t); err != nil {
			sklog.Errorf("Failed to decode task from pubsub message: %s", err)
			return nil // We will never be able to process this message.
		}
		return callback(&t)
	})
	if err != nil {
		return nil, err
	}
	return s.start()
}

// NewJobSubscriber creates a subscriber which calls the given callback
// function for every pubsub message. The topic should be one of the TOPIC_*
// constants defined in this package. The subscriberLabel is included in the
// subscription ID, along with a timestamp; this should help to debug zombie
// subscriptions. Acknowledgement of the message is done automatically based on
// the return value of the callback: if the callback returns an error, the
// message is Nack'd and will be re-sent at a later time, otherwise the message
// is Ack'd and will not be re-sent. Therefore, if the job is not valid or
// otherwise cannot ever be processed, the callback should return nil to prevent
// the message from being re-sent.
func NewJobSubscriber(projectId, topic, subscriberLabel string, ts oauth2.TokenSource, callback func(*types.Job) error) (context.CancelFunc, error) {
	c, err := pubsub.NewClient(context.Background(), projectId, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	s, err := newSubscriber(c, topic, subscriberLabel, func(m *pubsub.Message) error {
		var j types.Job
		if err := gob.NewDecoder(bytes.NewReader(m.Data)).Decode(&j); err != nil {
			sklog.Errorf("Failed to decode job from pubsub message: %s", err)
			return nil // We will never be able to process this message.
		}
		return callback(&j)
	})
	if err != nil {
		return nil, err
	}
	return s.start()
}

// NewTaskCommentSubscriber creates a subscriber which calls the given callback
// function for every pubsub message. The topic should be one of the TOPIC_*
// constants defined in this package. The subscriberLabel is included in the
// subscription ID, along with a timestamp; this should help to debug zombie
// subscriptions. Acknowledgement of the message is done automatically based on
// the return value of the callback: if the callback returns an error, the
// message is Nack'd and will be re-sent at a later time, otherwise the message
// is Ack'd and will not be re-sent. Therefore, if the comment is not valid or
// otherwise cannot ever be processed, the callback should return nil to prevent
// the message from being re-sent.
func NewTaskCommentSubscriber(projectId, topic, subscriberLabel string, ts oauth2.TokenSource, callback func(*types.TaskComment) error) (context.CancelFunc, error) {
	c, err := pubsub.NewClient(context.Background(), projectId, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	s, err := newSubscriber(c, topic, subscriberLabel, func(m *pubsub.Message) error {
		var c types.TaskComment
		if err := gob.NewDecoder(bytes.NewReader(m.Data)).Decode(&c); err != nil {
			sklog.Errorf("Failed to decode TaskComment from pubsub message: %s", err)
			return nil // We will never be able to process this message.
		}
		return callback(&c)
	})
	if err != nil {
		return nil, err
	}
	return s.start()
}

// NewTaskSpecCommentSubscriber creates a subscriber which calls the given
// callback function for every pubsub message. The topic should be one of the
// TOPIC_* constants defined in this package. The subscriberLabel is included in
// the subscription ID, along with a timestamp; this should help to debug zombie
// subscriptions. Acknowledgement of the message is done automatically based on
// the return value of the callback: if the callback returns an error, the
// message is Nack'd and will be re-sent at a later time, otherwise the message
// is Ack'd and will not be re-sent. Therefore, if the comment is not valid or
// otherwise cannot ever be processed, the callback should return nil to prevent
// the message from being re-sent.
func NewTaskSpecCommentSubscriber(projectId, topic, subscriberLabel string, ts oauth2.TokenSource, callback func(*types.TaskSpecComment) error) (context.CancelFunc, error) {
	c, err := pubsub.NewClient(context.Background(), projectId, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	s, err := newSubscriber(c, topic, subscriberLabel, func(m *pubsub.Message) error {
		var c types.TaskSpecComment
		if err := gob.NewDecoder(bytes.NewReader(m.Data)).Decode(&c); err != nil {
			sklog.Errorf("Failed to decode TaskSpecComment from pubsub message: %s", err)
			return nil // We will never be able to process this message.
		}
		return callback(&c)
	})
	if err != nil {
		return nil, err
	}
	return s.start()
}

// NewCommitCommentSubscriber creates a subscriber which calls the given
// callback function for every pubsub message. The topic should be one of the
// TOPIC_* constants defined in this package. The subscriberLabel is included in
// the subscription ID, along with a timestamp; this should help to debug zombie
// subscriptions. Acknowledgement of the message is done automatically based on
// the return value of the callback: if the callback returns an error, the
// message is Nack'd and will be re-sent at a later time, otherwise the message
// is Ack'd and will not be re-sent. Therefore, if the comment is not valid or
// otherwise cannot ever be processed, the callback should return nil to prevent
// the message from being re-sent.
func NewCommitCommentSubscriber(projectId, topic, subscriberLabel string, ts oauth2.TokenSource, callback func(*types.CommitComment) error) (context.CancelFunc, error) {
	c, err := pubsub.NewClient(context.Background(), projectId, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	s, err := newSubscriber(c, topic, subscriberLabel, func(m *pubsub.Message) error {
		var c types.CommitComment
		if err := gob.NewDecoder(bytes.NewReader(m.Data)).Decode(&c); err != nil {
			sklog.Errorf("Failed to decode CommitComment from pubsub message: %s", err)
			return nil // We will never be able to process this message.
		}
		return callback(&c)
	})
	if err != nil {
		return nil, err
	}
	return s.start()
}
