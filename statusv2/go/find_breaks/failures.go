package find_breaks

import (
	"fmt"

	"go.skia.org/infra/task_scheduler/go/types"
)

// failure is a struct which represents a failure of a given task spec,
// potentially merged from several failed tasks.
type failure struct {
	// id is provided by the creator as a handle for the failure.
	id string

	// brokeIn is the slice of commits which may have caused the failure.
	// Should not extend outside the current commit window, even if the real
	// cause of the failure might be outside.
	brokeIn slice

	// failing is the slice of commits affected by the failure. Must overlap
	// all of brokeIn. Should not extend outside the current commit window.
	failing slice

	// fixedIn is the slice of commits which may have fixed the failure. May
	// be empty if the failure has not yet been fixed.
	fixedIn slice
}

// valid returns an error if the failure is not valid.
func (f *failure) valid() error {
	// All of the component slices must be valid.
	if !f.brokeIn.Valid() {
		return fmt.Errorf("failure: brokeIn is invalid: %v", f.brokeIn)
	}
	if !f.failing.Valid() {
		return fmt.Errorf("failure: failing is invalid: %v", f.failing)
	}
	if !f.fixedIn.Valid() {
		return fmt.Errorf("failure: fixedIn is invalid: %v", f.fixedIn)
	}
	// brokeIn must be a subslice of failing.
	if f.brokeIn.Overlap(f.failing).Len() != f.brokeIn.Len() {
		return fmt.Errorf("failure: brokeIn is not a subslice of failing: %v and %v", f.brokeIn, f.failing)
	}
	// brokeIn and fixedIn can't overlap.
	if f.brokeIn.Overlap(f.fixedIn).Len() != 0 {
		return fmt.Errorf("failure: brokeIn overlaps fixedIn: %v and %v", f.brokeIn, f.fixedIn)
	}
	// failing and fixedIn can't overlap.
	if f.failing.Overlap(f.fixedIn).Len() != 0 {
		return fmt.Errorf("failure: failing overlaps fixedIn: %v and %v", f.failing, f.fixedIn)
	}
	return nil
}

// findFailuresForSpec creates a slice of failure instances from the given slice
// of Tasks within the given slice of commits. Assumes all tasks are of the same
// TaskSpec, and that the given slice of commits are in chronological order, ie.
// oldest first.
func findFailuresForSpec(tasks []*types.Task, commits []string) ([]*failure, error) {
	rv := []*failure{}
	byCommit := map[string]*types.Task{}
	for _, t := range tasks {
		for _, c := range t.Commits {
			byCommit[c] = t
		}
	}

	var f *failure
	firstFailed := -1
	lastFailed := -1
	wildCardStart := -1
	for idx, c := range commits {
		t := byCommit[c]
		if t != nil && t.Status == types.TASK_STATUS_FAILURE {
			if f == nil {
				brokeIn := makeSlice(t.Commits, commits)
				if wildCardStart != -1 {
					brokeIn.start = wildCardStart
				}
				f = &failure{
					id:      t.Id,
					brokeIn: brokeIn,
					fixedIn: newSlice(-1, -1),
				}
			}
			if firstFailed == -1 {
				firstFailed = idx
				if wildCardStart != -1 {
					firstFailed = wildCardStart
				}
			}
			lastFailed = idx + 1
			wildCardStart = -1
		} else if t != nil && t.Status == types.TASK_STATUS_SUCCESS {
			if f != nil {
				f.fixedIn = makeSlice(t.Commits, commits)
				if wildCardStart != -1 {
					f.fixedIn.start = wildCardStart
				}
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
				if err := f.valid(); err != nil {
					return nil, err
				}
				rv = append(rv, f)
				f = nil
			}
			firstFailed = -1
			lastFailed = -1
			wildCardStart = -1
		} else {
			// The task encountered a mishap, or is not
			// finished, or we somehow have a blamelist gap. We
			// essentially have to treat it as a wild card; if the
			// previous task succeeded and the subsequent task
			// succeeded, then we assume it's a success. If the
			// previous task failed and the subsequent task failed,
			// then we assume it's a failure. If the previous
			// task failed and the subsequent task
			// succeeded, then the commits for both this
			// task and the next must be part of fixedIn. If
			// the previous task succeeded and the
			// subsequent task failed, the commits for this
			// task and the next must be part of brokeIn.
			if wildCardStart == -1 {
				wildCardStart = idx
			}
		}
	}
	if f != nil {
		f.failing = newSlice(firstFailed, lastFailed)
		if !f.brokeIn.Empty() && f.brokeIn.start < f.failing.start {
			f.failing.start = f.brokeIn.start
		}
		f.fixedIn = newSlice(-1, -1)
		if err := f.valid(); err != nil {
			return nil, err
		}
		rv = append(rv, f)
	}
	return rv, nil
}

// findFailures creates a slice of failure instances from the given slice of
// Tasks within the given slice of commits.
func findFailures(tasks []*types.Task, commits []string) ([]*failure, error) {
	rv := []*failure{}
	bySpec := map[string][]*types.Task{}
	for _, t := range tasks {
		if !t.Done() {
			continue
		}
		bySpec[t.Name] = append(bySpec[t.Name], t)
	}
	for _, t := range bySpec {
		fails, err := findFailuresForSpec(t, commits)
		if err != nil {
			return nil, err
		}
		rv = append(rv, fails...)
	}
	return rv, nil
}
