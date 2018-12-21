package incremental

import (
	"sync"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
)

// taskCache is a struct used for tracking modified tasks.
type taskCache struct {
	db      db.TaskReader
	mtx     sync.Mutex
	queryId string
}

// newTaskCache returns a taskCache instance.
func newTaskCache(d db.TaskReader) *taskCache {
	return &taskCache{
		db: d,
	}
}

// mapTasks converts a slice of types.Tasks to a map of pared-down Task objects,
// keyed by repo.
func mapTasks(tasks []*types.Task) map[string][]*Task {
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
// just "return c.Reset(...)".  Assumes the caller holds a lock.
func (c *taskCache) Reset(w *window.Window) (map[string][]*Task, bool, error) {
	sklog.Infof("Resetting DB connection.")
	if c.queryId != "" {
		c.db.StopTrackingModifiedTasks(c.queryId)
		c.queryId = ""
	}
	queryId, err := c.db.StartTrackingModifiedTasks()
	if err != nil {
		return nil, false, err
	}
	c.queryId = queryId
	tasks, err := db.GetTasksFromWindow(c.db, w, time.Now())
	if err != nil {
		c.db.StopTrackingModifiedTasks(c.queryId)
		c.queryId = ""
		return nil, false, err
	}
	return mapTasks(tasks), true, nil
}

// Update returns any new tasks since the last Update() call. In the case of a
// lost connection to the remote database, all tasks from the desired window are
// returned, and the boolean return value is set to true.
func (c *taskCache) Update(w *window.Window) (map[string][]*Task, bool, error) {
	defer metrics2.FuncTimer().Stop()
	if c.queryId == "" {
		// Initial update, or if we failed to reconnect after a previous
		// lost connection.
		return c.Reset(w)
	}
	newTasks, err := c.db.GetModifiedTasks(c.queryId)
	if err != nil {
		sklog.Errorf("Connection to db lost; re-initializing cache from scratch. Error: %s", err)
		return c.Reset(w)
	}
	return mapTasks(newTasks), false, nil
}

// ResetNextTime forces the taskCache to Reset() on the next call to Update().
func (c *taskCache) ResetNextTime() {
	if c.queryId != "" {
		c.db.StopTrackingModifiedTasks(c.queryId)
		c.queryId = ""
	}
}
