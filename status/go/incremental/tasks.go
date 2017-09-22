package incremental

import (
	"sync"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/db"
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

// mapTasks converts a slice of db.Tasks to a map of pared-down Task objects,
// keyed by repo.
func mapTasks(tasks []*db.Task) map[string][]*Task {
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

// reset (re)establishes connection to the remote database and returns all tasks
// in the desired range. The boolean return value is the "startOver" indicator
// as returned by Update(), included here for convenience so that Update() can
// just "return c.reset(...)".  Assumes the caller holds a lock.
func (c *taskCache) reset(w *window.Window) (map[string][]*Task, bool, error) {
	sklog.Infof("Resetting DB connection.")
	c.queryId = ""
	queryId, err := c.db.StartTrackingModifiedTasks()
	if err != nil {
		return nil, false, err
	}
	c.queryId = queryId
	tasks, err := c.db.GetTasksFromDateRange(w.EarliestStart(), time.Now())
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
	if c.queryId == "" {
		// Initial update, or if we failed to reconnect after a previous
		// lost connection.
		return c.reset(w)
	}
	newTasks, err := c.db.GetModifiedTasks(c.queryId)
	if db.IsUnknownId(err) {
		sklog.Warningf("Connection to db lost; re-initializing cache from scratch.")
		return c.reset(w)
	} else if err != nil {
		return nil, false, err
	}
	return mapTasks(newTasks), false, nil
}
