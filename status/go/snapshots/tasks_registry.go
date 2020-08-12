package snapshots

import (
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/types"
)

// TaskRegistry is a struct which centralizes the decoding of QuerySnapshots
// into Tasks to prevent duplicate work and excessive memory usage.
type TaskRegistry struct {
	tasks   map[string]types.Task
	updated map[string]time.Time
	mtx     sync.Mutex
}

// NewTaskRegistry returns a TaskRegistry instance.
func NewTaskRegistry() *TaskRegistry {
	return &TaskRegistry{
		tasks:   map[string]*types.Task{},
		updated: map[string]*time.Time{},
	}
}

// FromSnapshot retrieves the tasks from the given snapshot or the cache,
// updating modified tasks in the cache as needed.
func (r *TaskRegistry) FromSnapshot(snap *firestore.QuerySnapshot) map[string]*types.Task {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	rv := make(map[string]*types.Task, len(snap.Changes))
	for _, ch := range snap.Changes {
		// If we already have this task in the cache and it's up to
		// date, just return the cached version and don't bother
		// decoding the new one.
		id := ch.Doc.Ref.ID
		if updated, ok := r.updated[id]; !ok || updated.Before(ch.Doc.Updated) {
			var t types.Task
			if err := ch.Doc.DataTo(&t); err != nil {
				// TODO(borenet): This shouldn't happen, but we
				// should probably handle it anyway.
				sklog.Fatal(err)
			}
			r.tasks[id] = &t
			r.updated = ch.Doc.Updated
		}
		rv[id] = r.tasks[id]
	}
	return rv
}
