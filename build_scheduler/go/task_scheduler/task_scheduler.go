package task_scheduler

import (
	"github.com/skia-dev/glog"
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

// testedness computes the total "testedness" of a set of commits covered by a
// task whose blamelist included N commits. The "testedness" of a task spec at a
// given commit is defined as follows:
//
// -1.0    if no task has ever included this commit for this task spec.
// 1.0     if a task was run for this task spec AT this commit.
// 1.0 / N if a task for this task spec has included this commit, where N is
//         the number of commits included in the task.
//
// This function gives the sum of the testedness for a blamelist of N commits.
func testedness(n int) float64 {
	if n < 0 {
		// This should never happen.
		glog.Errorf("Task score function got a blamelist with %d commits", n)
		return -1.0
	} else if n == 0 {
		// Zero commits have zero testedness.
		return 0.0
	} else if n == 1 {
		return 1.0
	} else {
		return 1.0 + float64(n-1)/float64(n)
	}
}

// testednessIncrease computes the increase in "testedness" obtained by running
// a task with the given blamelist length which may have "stolen" commits from
// a previous task with a different blamelist length. To do so, we compute the
// "testedness" for every commit affected by the task,  before and after the
// task would run. We subtract the "before" score from the "after" score to
// obtain the "testedness" increase at each commit, then sum them to find the
// total increase in "testedness" obtained by running the task.
func testednessIncrease(blamelistLength, stoleFromBlamelistLength int) float64 {
	// Invalid inputs.
	if blamelistLength <= 0 || stoleFromBlamelistLength < 0 {
		return -1.0
	}

	if stoleFromBlamelistLength == 0 {
		// This task covers previously-untested commits. Previous testedness
		// is -1.0 for each commit in the blamelist.
		beforeTestedness := float64(-blamelistLength)
		afterTestedness := testedness(blamelistLength)
		return afterTestedness - beforeTestedness
	} else if blamelistLength == stoleFromBlamelistLength {
		// This is a retry. It provides no testedness increase, so shortcut here
		// rather than perform the math to obtain the same answer.
		return 0.0
	} else {
		// This is a bisect/backfill.
		beforeTestedness := testedness(stoleFromBlamelistLength)
		afterTestedness := testedness(blamelistLength) + testedness(stoleFromBlamelistLength-blamelistLength)
		return afterTestedness - beforeTestedness
	}
}
