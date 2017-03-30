package find_breaks

import (
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/task_scheduler/go/db"
)

// failure is a struct which represents a failure of a given task spec,
// potentially merged from several failed tasks.
type failure struct {
	// id is provided by the creator as a handle for the failure.
	id string

	// brokeIn is the slice of commits which may have caused the failure.
	// Should be empty if the failure might have been caused before the
	// current commit window.
	brokeIn slice

	// failing is the slice of commits affected by the failure. Must overlap
	// all of brokeIn. Should not extend outside the current commit window.
	failing slice

	// fixedIn is the slice of commits which may have fixed the failure. May
	// be empty if the failure has not yet been fixed.
	fixedIn slice
}

// findFailuresForSpec creates a slice of failure instances from the given slice
// of Tasks within the given slice of commits. Assumes all tasks are of the same
// TaskSpec.
func findFailuresForSpec(tasks []*db.Task, commits []*repograph.Commit) []*failure {
	rv := []*failure{}
	byCommit := map[string]*db.Task{}
	for _, t := range tasks {
		for _, c := range t.Commits {
			byCommit[c] = t
		}
	}

	var f *failure
	firstSuccess := -1
	firstFailed := -1
	lastFailed := -1
	for idx, c := range commits {
		t := byCommit[c.Hash]
		if t != nil {
			if t.Status == db.TASK_STATUS_FAILURE {
				if f == nil {
					brokeIn := newSlice(-1, -1)
					if firstSuccess != -1 {
						brokeIn = makeSlice(t.Commits, commits)
					}
					f = &failure{
						id:      t.Id,
						brokeIn: brokeIn,
						fixedIn: newSlice(-1, -1),
					}
				}
				if firstFailed == -1 {
					firstFailed = idx
				}
				lastFailed = idx + 1
			} else if t.Status == db.TASK_STATUS_SUCCESS {
				if f != nil {
					f.fixedIn = makeSlice(t.Commits, commits)
					if f.brokeIn.Empty() {
						f.failing = slice{
							start: 0,
							end:   f.fixedIn.start,
						}
					} else {
						f.failing = slice{
							start: f.brokeIn.start,
							end:   f.fixedIn.start,
						}
					}
					rv = append(rv, f)
					f = nil
				}
				firstSuccess = idx
				firstFailed = -1
				lastFailed = -1
			} else {
				// TODO(borenet): What do we do with mishaps?
			}
		}
	}
	if f != nil {
		f.failing = newSlice(firstFailed, lastFailed)
		f.fixedIn = newSlice(-1, -1)
		rv = append(rv, f)
	}
	return rv
}

// findFailures creates a slice of failure instances from the given slice of
// Tasks within the given slice of commits.
func findFailures(tasks []*db.Task, commits []*repograph.Commit) []*failure {
	rv := []*failure{}
	bySpec := map[string][]*db.Task{}
	for _, t := range tasks {
		if !t.Done() {
			continue
		}
		bySpec[t.Name] = append(bySpec[t.Name], t)
	}
	for _, t := range bySpec {
		rv = append(rv, findFailuresForSpec(t, commits)...)
	}
	return rv
}
