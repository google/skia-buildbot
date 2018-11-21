package pubsub

import (
	"bytes"
	"encoding/gob"
	"sort"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/sklog"
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
	subscribers map[string]*subscriber
}

// newModifiedClient returns a modifiedClient instance.
func newModifiedClient(c *pubsub.Client, topic, label string) (*modifiedClient, error) {
	publisher, err := newPublisher(c, topic)
	if err != nil {
		return nil, err
	}
	return &modifiedClient{
		client:      c,
		publisher:   publisher,
		topic:       topic,
		label:       label,
		subscribers: map[string]*subscriber{},
		modified:    map[string]map[string]entry{},
	}, nil
}

// getModifiedData is a helper function for GetModifiedTasks and
// GetModifiedJobs.
func (c *modifiedClient) getModifiedData(id string) (map[string][]byte, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
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
		c.mtx.Lock()
		defer c.mtx.Unlock()
		dataId, ok := m.Attributes[ATTR_ID]
		if !ok {
			sklog.Errorf("Message contains no %q attribute! Ignoring.", ATTR_ID)
			return nil
		}
		prev, ok := c.modified[id][dataId]
		if !ok || prev.ts.Before(m.PublishTime) {
			c.modified[id][dataId] = entry{
				ts:   m.PublishTime,
				data: m.Data,
			}
		} else {
			sklog.Warningf("Received duplicate or outdated message for %s", dataId)
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	// Initialize the storage for this watcher.
	id = s.SubscriberID()
	c.mtx.Lock()
	c.modified[id] = map[string]entry{}
	c.subscribers[id] = s
	c.mtx.Unlock()

	// Start receiving pubsub messages.
	if err := s.start(); err != nil {
		return "", err
	}
	return id, nil
}

// stopTrackingModifiedData is a helper function for StopTrackingModifiedTasks
// and StopTrackingModifiedJobs.
func (c *modifiedClient) stopTrackingModifiedData(id string) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	s, ok := c.subscribers[id]
	if !ok {
		sklog.Errorf("Unknown subscriber ID: %s", id)
		return
	}
	if err := s.Stop(); err != nil {
		sklog.Error(err)
		return
	}
	delete(c.modified, id)
	delete(c.subscribers, id)
}

// taskClient implements db.ModifiedTasks using pubsub.
type taskClient struct {
	*modifiedClient
}

// NewModifiedTasks returns a db.ModifiedTasks which uses pubsub. The given
// label should be descriptive and unique to this process, or if the process
// uses multiple instances of ModifiedTasks, unique to each instance.
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
func (c *taskClient) TrackModifiedTask(t *db.Task) error {
	return c.publisher.publish(t.Id, t)
}

// See documentation for db.ModifiedTasks interface.
func (c *taskClient) TrackModifiedTasksGOB(tasksById map[string][]byte) error {
	return c.publisher.publishGOB(tasksById)
}

// jobClient implements db.ModifiedTasks using pubsub.
type jobClient struct {
	*modifiedClient
}

// NewModifiedJobs returns a db.ModifiedJobs which uses pubsub.
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
func (c *jobClient) TrackModifiedJob(j *db.Job) error {
	return c.publisher.publish(j.Id, j)
}

// See documentation for db.ModifiedJobs interface.
func (c *jobClient) TrackModifiedJobsGOB(jobsById map[string][]byte) error {
	return c.publisher.publishGOB(jobsById)
}
