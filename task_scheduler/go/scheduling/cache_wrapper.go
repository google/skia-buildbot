package scheduling

import (
	"context"
	"fmt"
	"time"

	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/types"
)

// cacheWrapper is an implementation of types.TaskCache which allows insertion of
// fake Tasks. Use one per task spec.
type cacheWrapper struct {
	byCommit map[string]*types.Task
	byId     map[string]*types.Task
	c        cache.TaskCache
	known    bool
}

func newCacheWrapper(c cache.TaskCache) *cacheWrapper {
	return &cacheWrapper{
		byCommit: map[string]*types.Task{},
		byId:     map[string]*types.Task{},
		c:        c,
		known:    false,
	}
}

// See documentation for TaskCache interface.
func (c *cacheWrapper) GetTask(id string) (*types.Task, error) {
	if t, ok := c.byId[id]; ok {
		return t, nil
	}
	return c.c.GetTask(id)
}

// See documentation for TaskCache interface.
func (c *cacheWrapper) GetTaskMaybeExpired(string) (*types.Task, error) {
	return nil, fmt.Errorf("cacheWrapper.GetTaskMaybeExpired not implemented.")
}

// See documentation for TaskCache interface.
func (c *cacheWrapper) GetTaskForCommit(repo, commit, name string) (*types.Task, error) {
	if t, ok := c.byCommit[commit]; ok {
		return t, nil
	}
	return c.c.GetTaskForCommit(repo, commit, name)
}

// See documentation for TaskCache interface.
func (c *cacheWrapper) GetTasksByKey(*types.TaskKey) ([]*types.Task, error) {
	return nil, fmt.Errorf("cacheWrapper.GetTasksByKey not implemented.")
}

// See documentation for TaskCache interface.
func (c *cacheWrapper) GetTasksForCommits(string, []string) (map[string]map[string]*types.Task, error) {
	return nil, fmt.Errorf("cacheWrapper.GetTasksForCommits not implemented.")
}

// See documentation for TaskCache interface.
func (c *cacheWrapper) GetTasksFromDateRange(time.Time, time.Time) ([]*types.Task, error) {
	return nil, fmt.Errorf("cacheWrapper.GetTasksFromDateRange not implemented.")
}

// See documentation for TaskCache interface.
func (c *cacheWrapper) KnownTaskName(repo, name string) bool {
	if c.known {
		return true
	}
	return c.c.KnownTaskName(repo, name)
}

// See documentation for TaskCache interface.
func (c *cacheWrapper) UnfinishedTasks() ([]*types.Task, error) {
	return nil, fmt.Errorf("cacheWrapper.UnfinishedTasks not implemented.")
}

// See documentation for TaskCache interface.
func (c *cacheWrapper) Update(context.Context) error {
	return fmt.Errorf("cacheWrapper.Update not implemented.")
}

// insert adds a task to the cacheWrapper's fake layer so that it will be
// included in query results but not actually inserted into the DB.
func (c *cacheWrapper) insert(t *types.Task) {
	c.byId[t.Id] = t
	for _, commit := range t.Commits {
		c.byCommit[commit] = t
	}
	if !t.IsForceRun() && !t.IsTryJob() {
		c.known = true
	}
}

// See documentation for TaskCache interface.
func (c *cacheWrapper) AddTasks(tasks []*types.Task) {
	for _, task := range tasks {
		c.insert(task)
	}
}

// See documentation for TaskCache interface.
func (c *cacheWrapper) OnModifiedTasks(func()) {}
