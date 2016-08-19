package task_scheduler

import "go.skia.org/infra/build_scheduler/go/db"

// taskCandidate is a struct used for determining which tasks to schedule.
type taskCandidate struct {
	Commits        []string
	IsolatedHashes []string
	Name           string
	Repo           string
	Revision       string
	Score          float64
	StealingFrom   *db.Task
	TaskSpec       *TaskSpec
}

// MakeTask instantiates a db.Task from the taskCandidate.
func (c *taskCandidate) MakeTask() *db.Task {
	commits := make([]string, 0, len(c.Commits))
	copy(commits, c.Commits)
	return &db.Task{
		Commits:  commits,
		Id:       "", // Filled in when the task is inserted into the DB.
		Name:     c.Name,
		Repo:     c.Repo,
		Revision: c.Revision,
	}
}

// allDepsMet determines whether all dependencies for the given task candidate
// have been satisfied, and if so, returns their isolated outputs.
func (c *taskCandidate) allDepsMet(cache *db.TaskCache) (bool, []string, error) {
	isolatedHashes := make([]string, 0, len(c.TaskSpec.Dependencies))
	for _, depName := range c.TaskSpec.Dependencies {
		d, err := cache.GetTaskForCommit(depName, c.Revision)
		if err != nil {
			return false, nil, err
		}
		if d == nil {
			return false, nil, nil
		}
		if !d.Done() || !d.Success() || d.IsolatedOutput == "" {
			return false, nil, nil
		}
		isolatedHashes = append(isolatedHashes, d.IsolatedOutput)
	}
	return true, isolatedHashes, nil
}

// taskCandidateSlice is an alias used for sorting a slice of taskCandidates.
type taskCandidateSlice []*taskCandidate

func (s taskCandidateSlice) Len() int { return len(s) }
func (s taskCandidateSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s taskCandidateSlice) Less(i, j int) bool {
	return s[i].Score > s[j].Score // candidates sort in decreasing order.
}
