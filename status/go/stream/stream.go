package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"go.skia.org/infra/go/eventsource"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/task_scheduler/go/db/pubsub"
	"go.skia.org/infra/task_scheduler/go/types"
	"golang.org/x/oauth2"
)

// Cache contains data to be sent to clients on initial load.
type Cache struct {
	data        map[string]*initialData
	es          map[string]*eventsource.EventSource
	cancelTasks context.CancelFunc
}

// New returns a Cache instance and starts streaming.
func New(ctx context.Context, repos []git.Repo, host, tasksTopic, jobsTopic, commentsTopic string, ts oauth2.TokenSource) (*Cache, error) {
	data := map[string]*initialData{}
	es := map[string]*eventsource.EventSource{}
	for _, repo := range repos {
		d := initialData{}
		data[repo] = d
		e := eventsource.New()
		es[repo] = e
	}
	cancelTasks, err := pubsub.NewTaskSubscriber(tasksTopic, fmt.Sprintf("stream_%s", host), ts, func(t *types.Task) error {
		e, ok := es[t.Repo]
		if !ok {
			// Returning nil causes the message to be ACK'd, which
			// is what we want in this case, since we can't do
			// anything with tasks from repos we don't know about.
			sklog.Errorf("Got task %s from unknown repo %s!", t.Id, t.Repo)
			return nil
		}
		data[t.Repo].updateTask(t)
		b, err := json.Marshal(t)
		if err != nil {
			// If we can't encode the task, we probably will never
			// be able to encode it. Returning nil causes the
			// message to be ACK'd, which is what we want in this
			// case.
			sklog.Errorf("Failed to encode task: %s", err)
			return nil
		}
		e.Send("task_"+t.Id, "task", b)
	})
	if err != nil {
		return nil, err
	}
	rv := &Cache{
		data:        data,
		es:          es,
		cancelTasks: cancelTasks,
	}
	if err := rv.Update(ctx); err != nil {
		return nil, err
	}
	return rv, nil
}

// Close cleans up the Cache.
func (c *Cache) Close() error {
	c.cancelTasks()
	return nil
}

// Get returns the cached data for the given repo.
func (c *Cache) Get(repo string) ([]byte, error) {
	d, ok := c.data[repo]
	if !ok {
		return nil, fmt.Errorf("Unknown repo.")
	}
	return d.Bytes(), nil
}

// Update updates the cached data for all repos.
func (c *Cache) Update(ctx context.Context) error {
	for _, d := range c.data {
		if err := d.Update(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Handler returns an HTTP handler for the given repo.
func (c *Cache) Handler(repo string) (http.HandlerFunc, error) {
	es, ok := c.es[repo]
	if !ok {
		return nil, fmt.Errorf("Unknown repo.")
	}
	return es.Handler(), nil
}

// initialData contains data to be sent to clients on initial load.
type initialData struct {
	// Public fields, used for JSON encoding.
	BranchHeads      []*gitinfo.GitBranch                       `json:"branch_heads,omitempty"`
	CommitComments   map[string][]*types.CommitComment          `json:"commit_comments,omitempty"`
	Commits          []*vcsinfo.LongCommit                      `json:"commits,omitempty"`
	SwarmingUrl      string                                     `json:"swarming_url,omitempty"`
	TaskComments     map[string]map[string][]*types.TaskComment `json:"task_comments,omitempty"`
	Tasks            []*types.Task                              `json:"tasks,omitempty"`
	TaskSchedulerUrl string                                     `json:"task_scheduler_url,omitempty"`
	TaskSpecComments map[string][]*types.TaskSpecComment        `json:"task_spec_comments,omitempty"`

	mtx   sync.Mutex
	bytes []byte
}

// Bytes returns the JSON-encoded stored data.
func (d *initialData) Bytes() []byte {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	return d.bytes
}

// updateBytes updates the encoded initialData. Assumes the caller holds d.mtx.
func (d *initialData) updateBytes() error {
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	d.bytes = b
	return nil
}

// Update updates the initialData.
func (d *initialData) Update() error {
	// TODO(borenet): Actually update.
	return d.updateBytes()
}

// updateTask updates the given task in the cache.
func (d *initialData) updateTask(t *types.Task) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	found := false
	for idx, old := range d.Tasks {
		if old.Id == t.Id && old.DbModified.Before(t.DbModified) {
			d.Tasks[idx] = t
			found = true
			break
		}
	}
	if !found {
		d.Tasks = append(d.Tasks, t)
	}
	return d.updateBytes()
}
