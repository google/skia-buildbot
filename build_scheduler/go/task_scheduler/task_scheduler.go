package task_scheduler

import (
	"go.skia.org/infra/build_scheduler/go/db"
	"go.skia.org/infra/go/gitinfo"
)

// TaskScheduler is a struct used for scheduling builds on bots.
type TaskScheduler struct {
	cache        *db.TaskCache
	taskCfgCache *taskCfgCache
}

func NewTaskScheduler(cache *db.TaskCache, workdir string) *TaskScheduler {
	repos := gitinfo.NewRepoMap(workdir)
	return &TaskScheduler{
		cache:        cache,
		taskCfgCache: newTaskCfgCache(repos),
	}
}

type taskCandidate struct {
	IsolatedHashes []string
	Name           string
	Repo           string
	Revision       string
	Score          float64
	TaskSpec       *TaskSpec
}

// AllDepsMet determines whether all dependencies for the given task candidate
// have been satisfied, and if so, returns their isolated outputs.
func (s *TaskScheduler) AllDepsMet(c *taskCandidate) (bool, []string, error) {
	isolatedHashes := make([]string, 0, len(c.TaskSpec.Dependencies))
	for _, depName := range c.TaskSpec.Dependencies {
		d, err := s.cache.GetTaskForCommit(depName, c.Revision)
		if err != nil {
			return false, nil, err
		}
		if d == nil {
			return false, nil, nil
		}
		if !d.Finished() || !d.Success() || d.IsolatedOutput == "" {
			return false, nil, nil
		}
		isolatedHashes = append(isolatedHashes, d.IsolatedOutput)
	}
	return true, isolatedHashes, nil
}

func (s *TaskScheduler) FindTaskCandidates(commitsByRepo map[string][]string) ([]*taskCandidate, error) {
	// Obtain all possible tasks.
	specs, err := s.taskCfgCache.GetTaskSpecsForCommits(commitsByRepo)
	if err != nil {
		return nil, err
	}
	candidates := []*taskCandidate{}
	for repo, commits := range specs {
		for commit, tasks := range commits {
			for name, task := range tasks {
				// We shouldn't duplicate pending, in-progress,
				// or successfully completed tasks.
				previous, err := s.cache.GetTaskForCommit(name, commit)
				if err != nil {
					return nil, err
				}
				if previous != nil {
					if previous.TaskResult.State == db.TASK_STATE_PENDING || previous.TaskResult.State == db.TASK_STATE_RUNNING {
						continue
					}
					if previous.Success() {
						continue
					}
				}
				candidates = append(candidates, &taskCandidate{
					IsolatedHashes: nil,
					Name:           name,
					Repo:           repo,
					Revision:       commit,
					Score:          0.0,
					TaskSpec:       task,
				})
			}
		}
	}

	// Filter out candidates whose dependencies have not been met.
	validCandidates := make([]*taskCandidate, 0, len(candidates))
	for _, c := range candidates {
		depsMet, hashes, err := s.AllDepsMet(c)
		if err != nil {
			return nil, err
		}
		if !depsMet {
			continue
		}
		c.IsolatedHashes = hashes
		validCandidates = append(validCandidates, c)
	}

	return validCandidates, nil
}
