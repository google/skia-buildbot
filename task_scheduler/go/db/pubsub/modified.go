package pubsub

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"sort"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
)

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
	modified    map[string]map[string]entry
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
		modified:    map[string]map[string]entry{},
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
	c.modified[id] = map[string]entry{}
	rv := make(map[string][]byte, len(mod))
	for k, v := range mod {
		rv[k] = v.data
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
			return err
		}
		// If the sender has changed, refuse the new messages and store
		// an error. The error will be returned on the next call to
		// getModifiedData, and the expectation is that the caller will
		// call stopTrackingModifiedData, then startTrackingModifiedData
		// and reload from scratch.
		if c.senderId[id] == "" {
			c.senderId[id] = senderId
		} else if senderId != c.senderId[id] {
			err := fmt.Errorf("Message has unknown sender %s (expected %s); not ack'ing.", senderId, c.senderId)
			c.errors[id] = err
			return err
		}
		prev, ok := c.modified[id][dataId]
		if !ok || prev.ts.Before(dbModified) {
			c.modified[id][dataId] = entry{
				ts:   m.PublishTime,
				data: m.Data,
			}
		} else {
			sklog.Debugf("Received duplicate or outdated message for %s", dataId)
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
	c.modified[id] = map[string]entry{}

	// Start receiving pubsub messages.
	cancel, err := s.start()
	if err != nil {
		return "", err
	}
	c.subscribers[id] = cancel
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
	delete(c.modified, id)
	delete(c.subscribers, id)
	delete(c.senderId, id)
	delete(c.errors, id)
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
// ModifiedTasks, unique to each instance. It is important to note that, since
// pubsub messages may arrive out of order, it is possible for GetModifiedTasks
// to return an older version of a task than was returned in a previous call.
func NewModifiedTasks(c *pubsub.Client, topic, label string) (db.ModifiedTasks, error) {
	mc, err := newModifiedClient(c, topic, label)
	if err != nil {
		return nil, err
	}
	return &taskClient{mc}, nil
}

// See documentation for db.ModifiedTasks interface.
func (c *taskClient) GetModifiedTasks(id string) ([]*db.Task, error) {
	gobs, err := c.GetModifiedTasksGOB(id)
	if err != nil {
		return nil, err
	}
	rv := make([]*db.Task, 0, len(gobs))
	for _, g := range gobs {
		var t db.Task
		if err := gob.NewDecoder(bytes.NewReader(g)).Decode(&t); err != nil {
			// We didn't attempt to decode the blob in the pubsub
			// message when we received it. Ignore this task.
			sklog.Errorf("Failed to decode task from pubsub message: %s", err)
		} else {
			rv = append(rv, &t)
		}
	}
	sort.Sort(db.TaskSlice(rv))
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
func (c *taskClient) TrackModifiedTask(t *db.Task) {
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
// ModifiedJobs, unique to each instance. It is important to note that, since
// pubsub messages may arrive out of order, it is possible for GetModifiedJobs
// to return an older version of a job than was returned in a previous call.
func NewModifiedJobs(c *pubsub.Client, topic, label string) (db.ModifiedJobs, error) {
	mc, err := newModifiedClient(c, topic, label)
	if err != nil {
		return nil, err
	}
	return &jobClient{mc}, nil
}

// See documentation for db.ModifiedJobs interface.
func (c *jobClient) GetModifiedJobs(id string) ([]*db.Job, error) {
	gobs, err := c.GetModifiedJobsGOB(id)
	if err != nil {
		return nil, err
	}
	rv := make([]*db.Job, 0, len(gobs))
	for _, g := range gobs {
		var j db.Job
		if err := gob.NewDecoder(bytes.NewReader(g)).Decode(&j); err != nil {
			// We didn't attempt to decode the blob in the pubsub
			// message when we received it. Ignore this job.
			sklog.Errorf("Failed to decode job from pubsub message: %s", err)
		} else {
			rv = append(rv, &j)
		}
	}
	sort.Sort(db.JobSlice(rv))
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
func (c *jobClient) TrackModifiedJob(j *db.Job) {
	c.publisher.publish(j.Id, j.DbModified, j)
}

// See documentation for db.ModifiedJobs interface.
func (c *jobClient) TrackModifiedJobsGOB(ts time.Time, jobsById map[string][]byte) {
	c.publisher.publishGOB(ts, jobsById)
}
