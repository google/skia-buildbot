package incremental

import (
	"context"
	"sync"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
)

// taskCache is a struct used for tracking modified tasks.
type taskCache struct {
	allTasks         map[string]*types.Task
	db               db.TaskReader
	cbMtx            sync.Mutex // Protects gotTasksCallback.
	gotTasksCallback func()
	mtx              sync.Mutex
	nextUpdateTasks  map[string]*types.Task
	shouldReset      bool
}

// newTaskCache returns a taskCache instance.
func newTaskCache(ctx context.Context, d db.RemoteDB) *taskCache {
	tc := &taskCache{
		allTasks:        map[string]*types.Task{},
		db:              d,
		nextUpdateTasks: map[string]*types.Task{},
		shouldReset:     true,
	}
	go func() {
		for tasks := range d.ModifiedTasksCh(ctx) {
			tc.mtx.Lock()
			for _, task := range tasks {
				if old, ok := tc.allTasks[task.Id]; !ok || task.DbModified.After(old.DbModified) {
					tc.allTasks[task.Id] = task
					tc.nextUpdateTasks[task.Id] = task
				}
			}
			tc.mtx.Unlock()
			tc.cbMtx.Lock()
			cb := tc.gotTasksCallback
			tc.cbMtx.Unlock()
			if cb != nil {
				cb()
			}
		}
	}()
	return tc
}

// mapTasks converts a map of types.Tasks to a map of pared-down Task objects,
// keyed by repo.
func mapTasks(tasks map[string]*types.Task) map[string][]*Task {
	rv := map[string][]*Task{}
	for _, t := range tasks {
		rv[t.Repo] = append(rv[t.Repo], &Task{
			Commits:        t.Commits,
			Name:           t.Name,
			Id:             t.Id,
			Revision:       t.Revision,
			Status:         t.Status,
			SwarmingTaskId: t.SwarmingTaskId,
		})
	}
	if len(rv) == 0 {
		return nil
	}
	return rv
}

// Reset (re)establishes connection to the remote database and returns all tasks
// in the desired range. The boolean return value is the "startOver" indicator
// as returned by Update(), included here for convenience so that Update() can
// just "return c.Reset(...)".
func (c *taskCache) Reset(ctx context.Context, w window.Window) (map[string][]*Task, bool, error) {
	sklog.Infof("Resetting DB connection.")
	tasks, err := db.GetTasksFromWindow(ctx, c.db, w)
	if err != nil {
		return nil, false, err
	}
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.allTasks = make(map[string]*types.Task, len(tasks))
	for _, task := range tasks {
		c.allTasks[task.Id] = task
	}
	for _, task := range c.nextUpdateTasks {
		if old, ok := c.allTasks[task.Id]; !ok || task.DbModified.After(old.DbModified) {
			c.allTasks[task.Id] = task
		}
	}
	c.nextUpdateTasks = map[string]*types.Task{}
	c.shouldReset = false
	return mapTasks(c.allTasks), true, nil
}

// Update returns any new tasks since the last Update() call. In the case of a
// lost connection to the remote database, all tasks from the desired window are
// returned, and the boolean return value is set to true.
func (c *taskCache) Update(ctx context.Context, w window.Window) (map[string][]*Task, bool, error) {
	defer metrics2.FuncTimer().Stop()
	if c.shouldReset {
		return c.Reset(ctx, w)
	}
	c.mtx.Lock()
	defer c.mtx.Unlock()
	// Note that ModifiedTasksCh could technically return tasks we've
	// already seen. In this case, that will cause us to resend some tasks
	// to the client, which is wasteful but won't cause incorrect results.
	newTasks := c.nextUpdateTasks
	c.nextUpdateTasks = map[string]*types.Task{}
	for id, t := range c.allTasks {
		if !w.TestTime(t.Repo, t.Created) {
			delete(c.allTasks, id)
		}
	}
	return mapTasks(newTasks), false, nil
}

// ResetNextTime forces the taskCache to Reset() on the next call to Update().
func (c *taskCache) ResetNextTime() {
	c.shouldReset = true
}

// Set a callback to run whenever tasks are added to the cache. Used for testing.
func (c *taskCache) setTasksCallback(cb func()) {
	c.cbMtx.Lock()
	defer c.cbMtx.Unlock()
	c.gotTasksCallback = cb
}
