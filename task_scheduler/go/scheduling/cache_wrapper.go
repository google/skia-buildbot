package scheduling

import (
	"fmt"

	"go.skia.org/infra/task_scheduler/go/db"
)

// cacheWrapper is an implementation of db.TaskCache which allows insertion of
// fake Tasks. Use one per task spec.
type cacheWrapper struct {
	byCommit map[string]*db.Task
	byId     map[string]*db.Task
	c        db.TaskCache
	known    bool
}

func newCacheWrapper(c db.TaskCache) *cacheWrapper {
	return &cacheWrapper{
		byCommit: map[string]*db.Task{},
		byId:     map[string]*db.Task{},
		c:        c,
		known:    false,
	}
}

// See documentation for TaskCache interface.
func (c *cacheWrapper) GetTask(id string) (*db.Task, error) {
	if t, ok := c.byId[id]; ok {
		return t, nil
	}
	return c.c.GetTask(id)
}

// See documentation for TaskCache interface.
func (c *cacheWrapper) GetTaskForCommit(repo, commit, name string) (*db.Task, error) {
	if t, ok := c.byCommit[commit]; ok {
		return t, nil
	}
	return c.c.GetTaskForCommit(repo, commit, name)
}

// See documentation for TaskCache interface.
func (c *cacheWrapper) GetTasksForCommits(string, []string) (map[string]map[string]*db.Task, error) {
	return nil, fmt.Errorf("cacheWrapper.GetTasksForCommits not implemented.")
}

// See documentation for TaskCache interface.
func (c *cacheWrapper) KnownTaskName(repo, name string) bool {
	if c.known {
		return true
	}
	return c.c.KnownTaskName(repo, name)
}

// See documentation for TaskCache interface.
func (c *cacheWrapper) UnfinishedTasks() ([]*db.Task, error) {
	return nil, fmt.Errorf("cacheWrapper.UnfinishedTasks not implemented.")
}

// See documentation for TaskCache interface.
func (c *cacheWrapper) Update() error {
	return fmt.Errorf("cacheWrapper.Update not implemented.")
}

// insert adds a task to the cacheWrapper's fake layer so that it will be
// included in query results but not actually inserted into the DB.
func (c *cacheWrapper) insert(t *db.Task) {
	c.byId[t.Id] = t
	for _, commit := range t.Commits {
		c.byCommit[commit] = t
	}
	c.known = true
}
