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

func mapTasks(tasks []*db.Task) map[string][]*Task {
	rv := map[string][]*Task{}
	for _, t := range tasks {
		rv[t.Repo] = append(rv[t.Repo], &Task{
			Commits:  t.Commits,
			Name:     t.Name,
			Id:       t.Id,
			Revision: t.Revision,
			Status:   t.Status,
		})
	}
	return rv
}

// reset (re)establishes connection to the remote database and returns all tasks
// in the desired range. Assumes the caller holds a lock.
func (c *taskCache) reset(w *window.Window, now time.Time) (map[string][]*Task, bool, error) {
	sklog.Infof("Resetting DB connection.")
	c.queryId = ""
	queryId, err := c.db.StartTrackingModifiedTasks()
	if err != nil {
		return nil, false, err
	}
	c.queryId = queryId
	tasks, err := c.db.GetTasksFromDateRange(w.EarliestStart(), now)
	if err != nil {
		return nil, false, err
	}
	// This works around the race condition due to the fact that the "now"
	// variable was set before we called StartTrackingModifiedTasks, which
	// might cause us to miss some things.
	newTasks, err := c.db.GetModifiedTasks(c.queryId)
	if err != nil {
		c.db.StopTrackingModifiedTasks(c.queryId)
		c.queryId = ""
		return nil, false, err
	}
	tasks = append(tasks, newTasks...)
	return mapTasks(tasks), true, nil
}

// Update returns any new tasks since the last Update() call. In the case of a
// lost connection to the remote database, all tasks from the desired window are
// returned, and the boolean return value is set to true.
func (c *taskCache) Update(w *window.Window, now time.Time) (map[string][]*Task, bool, error) {
	if c.queryId == "" {
		// Initial update, or if we failed to reconnect after a previous
		// lost connection.
		return c.reset(w, now)
	}
	newTasks, err := c.db.GetModifiedTasks(c.queryId)
	if db.IsUnknownId(err) {
		sklog.Warningf("Connection to db lost; re-initializing cache from scratch.")
		return c.reset(w, now)
	} else if err != nil {
		return nil, false, err
	}
	return mapTasks(newTasks), false, nil
}
