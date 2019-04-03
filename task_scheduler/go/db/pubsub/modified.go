package pubsub

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/types"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

// NewModifiedData returns a db.ModifiedData instance which uses pubsub.
func NewModifiedData(projectId, topicSet, label string, ts oauth2.TokenSource) (db.ModifiedData, error) {
	topicSetObj, ok := topics[topicSet]
	if !ok {
		return nil, fmt.Errorf("Topic must be one of %v, not %q", VALID_TOPIC_SETS, topicSet)
	}
	t, err := NewModifiedTasks(projectId, topicSetObj.tasks, label, ts)
	if err != nil {
		return nil, err
	}
	j, err := NewModifiedJobs(projectId, topicSetObj.jobs, label, ts)
	if err != nil {
		return nil, err
	}
	c, err := NewModifiedComments(projectId, topicSetObj.taskComments, topicSetObj.taskSpecComments, topicSetObj.commitComments, label, ts)
	if err != nil {
		return nil, err
	}
	return db.NewModifiedData(t, j, c), nil
}

type entry struct {
	ts   time.Time
	data []byte
}

// modifiedClient contains common elements to support ModifiedTasks and
// ModifiedJobs.
type modifiedClient struct {
	client    *pubsub.Client
	publisher *publisher
	topic     string
	label     string

	// Protects subscribers and modified data.
	mtx         sync.Mutex
	modified    map[string]map[string]*entry
	subscribers map[string]context.CancelFunc
	senderId    map[string]string
	errors      map[string]error
}

// newModifiedClient returns a modifiedClient instance.
func newModifiedClient(c *pubsub.Client, topic, subscriberLabel string) (*modifiedClient, error) {
	publisher, err := newPublisher(c, topic)
	if err != nil {
		return nil, err
	}
	return &modifiedClient{
		client:      c,
		publisher:   publisher,
		topic:       topic,
		label:       subscriberLabel,
		modified:    map[string]map[string]*entry{},
		subscribers: map[string]context.CancelFunc{},
		senderId:    map[string]string{},
		errors:      map[string]error{},
	}, nil
}

// getModifiedData is a helper function for GetModifiedTasks and
// GetModifiedJobs.
func (c *modifiedClient) getModifiedData(id string) (map[string][]byte, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.errors[id] != nil {
		return nil, c.errors[id]
	}
	mod, ok := c.modified[id]
	if !ok {
		return nil, db.ErrUnknownId
	}
	rv := map[string][]byte{}
	for k, v := range mod {
		if v.data != nil {
			rv[k] = v.data
			v.data = nil
		}
	}
	return rv, nil
}

// startTrackingModifiedData is a helper function for StartTrackingModifiedTasks
// and StartTrackingModifiedJobs.
func (c *modifiedClient) startTrackingModifiedData() (string, error) {
	var id string
	s, err := newSubscriber(c.client, c.topic, c.label, func(m *pubsub.Message) error {
		dataId, ok := m.Attributes[ATTR_ID]
		if !ok {
			sklog.Errorf("Message contains no %q attribute! Ignoring.", ATTR_ID)
			return nil
		}
		senderId, ok := m.Attributes[ATTR_SENDER_ID]
		if !ok {
			sklog.Errorf("Message contains no %q attribute! Ignoring.", ATTR_SENDER_ID)
			return nil
		}
		dbModifiedStr, ok := m.Attributes[ATTR_TIMESTAMP]
		if !ok {
			sklog.Errorf("Message contains no %q attribute! Ignoring.", ATTR_TIMESTAMP)
			return nil
		}
		dbModified, err := time.Parse(util.RFC3339NanoZeroPad, dbModifiedStr)
		if err != nil {
			sklog.Errorf("Failed to parse message timestamp; ignoring: %s", err)
			return nil
		}
		c.mtx.Lock()
		defer c.mtx.Unlock()
		if _, ok := c.modified[id]; !ok {
			sklog.Warningf("No modified data entry for id %s; ignoring message. Did we call stopTrackingModifiedData?", id)
			return nil
		}
		if err := c.errors[id]; err != nil {
			sklog.Warningf("modifiedClient is in error state; ignoring all messages.")
			return nil
		}
		// If the sender has changed, refuse the new messages and store
		// an error. The error will be returned on the next call to
		// getModifiedData, and the expectation is that the caller will
		// call stopTrackingModifiedData, then startTrackingModifiedData
		// and reload from scratch.
		if c.senderId[id] == "" {
			c.senderId[id] = senderId
		} else if senderId != c.senderId[id] {
			err := fmt.Errorf("Message has unknown sender %s (expected %s); not ack'ing.", senderId, c.senderId[id])
			c.errors[id] = err
			return nil
		}
		prev, ok := c.modified[id][dataId]
		if !ok || prev.ts.Before(dbModified) {
			c.modified[id][dataId] = &entry{
				ts:   dbModified,
				data: m.Data,
			}
		} else {
			sklog.Debugf("Received duplicate or outdated message (%s vs %s) for %s", prev.ts, dbModified, dataId)
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	// Initialize the storage for this subscriber.
	id = s.SubscriberID()
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.modified[id] = map[string]*entry{}

	// Start receiving pubsub messages.
	cancel, err := s.start()
	if err != nil {
		return "", err
	}

	// Delete old entries.
	doneCh := make(chan bool)
	cleanup := func() {
		cancel()
		delete(c.modified, id)
		delete(c.subscribers, id)
		delete(c.senderId, id)
		delete(c.errors, id)
	}
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-doneCh:
				return
			case <-ticker.C:
				c.mtx.Lock()
				now := time.Now()
				entries := c.modified[id]
				for entryId, entry := range entries {
					if now.Sub(entry.ts) > time.Hour {
						if entry.data == nil {
							delete(entries, entryId)
						} else {
							// If the client hasn't called GetModified in over an hour,
							// assume that something is wrong and delete the subscriber.
							// The next call to GetModified will return an error.
							cleanup()
						}
					}
				}
				c.mtx.Unlock()
			}
		}
	}()
	c.subscribers[id] = func() {
		doneCh <- true
		cleanup()
	}
	return id, nil
}

// stopTrackingModifiedData is a helper function for StopTrackingModifiedTasks
// and StopTrackingModifiedJobs.
func (c *modifiedClient) stopTrackingModifiedData(id string) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	cancel, ok := c.subscribers[id]
	if !ok {
		sklog.Errorf("Unknown subscriber ID: %s", id)
		return
	}
	cancel()
}

// taskClient implements db.ModifiedTasks using pubsub.
type taskClient struct {
	*modifiedClient
}

// NewModifiedTasks returns a db.ModifiedTasks which uses pubsub. The topic
// should be one of the TOPIC_* constants defined in this package. The
// subscriberLabel is included in the subscription ID, along with a timestamp;
// this should help to debug zombie subscriptions. It should be descriptive and
// unique to this process, or if the process uses multiple instances of
// ModifiedTasks, unique to each instance.
func NewModifiedTasks(projectId, topic, label string, ts oauth2.TokenSource) (db.ModifiedTasks, error) {
	c, err := pubsub.NewClient(context.Background(), projectId, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	mc, err := newModifiedClient(c, topic, label)
	if err != nil {
		return nil, err
	}
	return &taskClient{mc}, nil
}

// See documentation for db.ModifiedTasks interface.
func (c *taskClient) GetModifiedTasks(id string) ([]*types.Task, error) {
	gobs, err := c.GetModifiedTasksGOB(id)
	if err != nil {
		return nil, err
	}
	rv := make([]*types.Task, 0, len(gobs))
	for _, g := range gobs {
		var t types.Task
		if err := gob.NewDecoder(bytes.NewReader(g)).Decode(&t); err != nil {
			// We didn't attempt to decode the blob in the pubsub
			// message when we received it. Ignore this task.
			sklog.Errorf("Failed to decode task from pubsub message: %s", err)
		} else {
			rv = append(rv, &t)
		}
	}
	sort.Sort(types.TaskSlice(rv))
	return rv, nil
}

// See documentation for db.ModifiedTasks interface.
func (c *taskClient) GetModifiedTasksGOB(id string) (map[string][]byte, error) {
	return c.getModifiedData(id)
}

// See documentation for db.ModifiedTasks interface.
func (c *taskClient) StartTrackingModifiedTasks() (string, error) {
	return c.startTrackingModifiedData()
}

// See documentation for db.ModifiedTasks interface.
func (c *taskClient) StopTrackingModifiedTasks(id string) {
	c.stopTrackingModifiedData(id)
}

// See documentation for db.ModifiedTasks interface.
func (c *taskClient) TrackModifiedTask(t *types.Task) {
	c.publisher.publish(t.Id, t.DbModified, t)
}

// See documentation for db.ModifiedTasks interface.
func (c *taskClient) TrackModifiedTasksGOB(ts time.Time, tasksById map[string][]byte) {
	c.publisher.publishGOB(ts, tasksById)
}

// jobClient implements db.ModifiedTasks using pubsub.
type jobClient struct {
	*modifiedClient
}

// NewModifiedJobs returns a db.ModifiedJobs which uses pubsub. The topic
// should be one of the TOPIC_* constants defined in this package. The
// subscriberLabel is included in the subscription ID, along with a timestamp;
// this should help to debug zombie subscriptions. It should be descriptive and
// unique to this process, or if the process uses multiple instances of
// ModifiedJobs, unique to each instance.
func NewModifiedJobs(projectId, topic, label string, ts oauth2.TokenSource) (db.ModifiedJobs, error) {
	c, err := pubsub.NewClient(context.Background(), projectId, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	mc, err := newModifiedClient(c, topic, label)
	if err != nil {
		return nil, err
	}
	return &jobClient{mc}, nil
}

// See documentation for db.ModifiedJobs interface.
func (c *jobClient) GetModifiedJobs(id string) ([]*types.Job, error) {
	gobs, err := c.GetModifiedJobsGOB(id)
	if err != nil {
		return nil, err
	}
	rv := make([]*types.Job, 0, len(gobs))
	for _, g := range gobs {
		var j types.Job
		if err := gob.NewDecoder(bytes.NewReader(g)).Decode(&j); err != nil {
			// We didn't attempt to decode the blob in the pubsub
			// message when we received it. Ignore this job.
			sklog.Errorf("Failed to decode job from pubsub message: %s", err)
		} else {
			rv = append(rv, &j)
		}
	}
	sort.Sort(types.JobSlice(rv))
	return rv, nil
}

// See documentation for db.ModifiedJobs interface.
func (c *jobClient) GetModifiedJobsGOB(id string) (map[string][]byte, error) {
	return c.getModifiedData(id)
}

// See documentation for db.ModifiedJobs interface.
func (c *jobClient) StartTrackingModifiedJobs() (string, error) {
	return c.startTrackingModifiedData()
}

// See documentation for db.ModifiedJobs interface.
func (c *jobClient) StopTrackingModifiedJobs(id string) {
	c.stopTrackingModifiedData(id)
}

// See documentation for db.ModifiedJobs interface.
func (c *jobClient) TrackModifiedJob(j *types.Job) {
	c.publisher.publish(j.Id, j.DbModified, j)
}

// See documentation for db.ModifiedJobs interface.
func (c *jobClient) TrackModifiedJobsGOB(ts time.Time, jobsById map[string][]byte) {
	c.publisher.publishGOB(ts, jobsById)
}

// commentClient implements db.ModifiedComments using pubsub.
type commentClient struct {
	tasks     *modifiedClient
	taskSpecs *modifiedClient
	commits   *modifiedClient
}

// NewModifiedComments returns a db.ModifiedComments which uses pubsub. The
// topics should be one of the sets of TOPIC_* constants defined in this
// package. The subscriberLabel is included in the subscription ID, along with a
// timestamp; this should help to debug zombie subscriptions. It should be
// descriptive and unique to this process, or if the process uses multiple
// instances of ModifiedJobs, unique to each instance.
func NewModifiedComments(projectId, taskCommentsTopic, taskSpecCommentsTopic, commitCommentsTopic string, label string, ts oauth2.TokenSource) (db.ModifiedComments, error) {
	c, err := pubsub.NewClient(context.Background(), projectId, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	tasks, err := newModifiedClient(c, taskCommentsTopic, label)
	if err != nil {
		return nil, err
	}
	taskSpecs, err := newModifiedClient(c, taskSpecCommentsTopic, label)
	if err != nil {
		return nil, err
	}
	commits, err := newModifiedClient(c, commitCommentsTopic, label)
	if err != nil {
		return nil, err
	}
	return &commentClient{
		tasks:     tasks,
		taskSpecs: taskSpecs,
		commits:   commits,
	}, nil
}

// See documentation for db.ModifiedComments interface.
func (c *commentClient) GetModifiedComments(id string) ([]*types.TaskComment, []*types.TaskSpecComment, []*types.CommitComment, error) {
	ids := strings.Split(id, "#")
	if len(ids) != 3 {
		return nil, nil, nil, db.ErrUnknownId
	}
	gobs, err := c.tasks.getModifiedData(ids[0])
	if err != nil {
		return nil, nil, nil, err
	}
	rv1 := make([]*types.TaskComment, 0, len(gobs))
	for _, g := range gobs {
		var c types.TaskComment
		if err := gob.NewDecoder(bytes.NewReader(g)).Decode(&c); err != nil {
			// We didn't attempt to decode the blob in the pubsub
			// message when we received it. Ignore this job.
			sklog.Errorf("Failed to decode job from pubsub message: %s", err)
		} else {
			rv1 = append(rv1, &c)
		}
	}
	sort.Sort(types.TaskCommentSlice(rv1))

	gobs, err = c.taskSpecs.getModifiedData(ids[1])
	if err != nil {
		return nil, nil, nil, err
	}
	rv2 := make([]*types.TaskSpecComment, 0, len(gobs))
	for _, g := range gobs {
		var c types.TaskSpecComment
		if err := gob.NewDecoder(bytes.NewReader(g)).Decode(&c); err != nil {
			// We didn't attempt to decode the blob in the pubsub
			// message when we received it. Ignore this job.
			sklog.Errorf("Failed to decode job from pubsub message: %s", err)
		} else {
			rv2 = append(rv2, &c)
		}
	}
	sort.Sort(types.TaskSpecCommentSlice(rv2))

	gobs, err = c.commits.getModifiedData(ids[2])
	if err != nil {
		return nil, nil, nil, err
	}
	rv3 := make([]*types.CommitComment, 0, len(gobs))
	for _, g := range gobs {
		var c types.CommitComment
		if err := gob.NewDecoder(bytes.NewReader(g)).Decode(&c); err != nil {
			// We didn't attempt to decode the blob in the pubsub
			// message when we received it. Ignore this job.
			sklog.Errorf("Failed to decode job from pubsub message: %s", err)
		} else {
			rv3 = append(rv3, &c)
		}
	}
	sort.Sort(types.CommitCommentSlice(rv3))
	return rv1, rv2, rv3, nil
}

// See documentation for db.ModifiedComments interface.
func (c *commentClient) StartTrackingModifiedComments() (string, error) {
	id1, err := c.tasks.startTrackingModifiedData()
	if err != nil {
		return "", err
	}
	id2, err := c.taskSpecs.startTrackingModifiedData()
	if err != nil {
		return "", err
	}
	id3, err := c.commits.startTrackingModifiedData()
	if err != nil {
		return "", err
	}
	return id1 + "#" + id2 + "#" + id3, nil
}

// See documentation for db.ModifiedComments interface.
func (c *commentClient) StopTrackingModifiedComments(id string) {
	ids := strings.Split(id, "#")
	if len(ids) != 3 {
		sklog.Errorf("Invalid ID %q", id)
		return
	}
	c.tasks.stopTrackingModifiedData(ids[0])
	c.taskSpecs.stopTrackingModifiedData(ids[1])
	c.commits.stopTrackingModifiedData(ids[2])
}

// See documentation for db.ModifiedComments interface.
func (c *commentClient) TrackModifiedTaskComment(tc *types.TaskComment) {
	// Hack: since the timestamp is part of the ID, we can't change it. But,
	// we have to provide a different timestamp from the one we sent when
	// the comment was created, or else it'll get de-duplicated.
	ts := tc.Timestamp
	if tc.Deleted != nil && *tc.Deleted {
		ts = time.Now()
	}
	c.tasks.publisher.publish(tc.Id(), ts, tc)
}

// See documentation for db.ModifiedComments interface.
func (c *commentClient) TrackModifiedTaskSpecComment(tc *types.TaskSpecComment) {
	// Hack: since the timestamp is part of the ID, we can't change it. But,
	// we have to provide a different timestamp from the one we sent when
	// the comment was created, or else it'll get de-duplicated.
	ts := tc.Timestamp
	if tc.Deleted != nil && *tc.Deleted {
		ts = time.Now()
	}
	c.taskSpecs.publisher.publish(tc.Id(), ts, tc)
}

// See documentation for db.ModifiedComments interface.
func (c *commentClient) TrackModifiedCommitComment(cc *types.CommitComment) {
	// Hack: since the timestamp is part of the ID, we can't change it. But,
	// we have to provide a different timestamp from the one we sent when
	// the comment was created, or else it'll get de-duplicated.
	ts := cc.Timestamp
	if cc.Deleted != nil && *cc.Deleted {
		ts = time.Now()
	}
	c.commits.publisher.publish(cc.Id(), ts, cc)
}
