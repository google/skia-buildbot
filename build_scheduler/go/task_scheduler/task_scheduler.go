package task_scheduler

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/build_scheduler/go/db"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/metrics2"
)

// TaskScheduler is a struct used for scheduling builds on bots.
type TaskScheduler struct {
	cache            *db.TaskCache
	period           time.Duration
	queue            []*taskCandidate
	queueMtx         sync.RWMutex
	repos            *gitinfo.RepoMap
	taskCfgCache     *taskCfgCache
	timeDecayAmt24Hr float64
}

func NewTaskScheduler(cache *db.TaskCache, period time.Duration, repos *gitinfo.RepoMap) *TaskScheduler {
	s := &TaskScheduler{
		cache:            cache,
		period:           period,
		queue:            []*taskCandidate{},
		queueMtx:         sync.RWMutex{},
		repos:            repos,
		taskCfgCache:     newTaskCfgCache(repos),
		timeDecayAmt24Hr: 1.0,
	}
	return s
}

// Start initiates the TaskScheduler's goroutines for scheduling tasks.
func (s *TaskScheduler) Start() {
	go func() {
		lv := metrics2.NewLiveness("last-successful-queue-regeneration")
		for _ = range time.Tick(time.Minute) {
			if err := s.regenerateTaskQueue(); err != nil {
				glog.Errorf("Failed to regenerate task queue: %s", err)
			} else {
				lv.Reset()
			}
		}
	}()
}

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

type taskCandidateSlice []*taskCandidate

func (s taskCandidateSlice) Len() int { return len(s) }
func (s taskCandidateSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s taskCandidateSlice) Less(i, j int) bool {
	return s[i].Score > s[j].Score // candidates sort in decreasing order.
}

// MakeTask instantiates a db.Task from the taskCandidate.
func (c *taskCandidate) MakeTask() *db.Task {
	commits := make([]string, 0, len(c.Commits))
	copy(commits, c.Commits)
	return &db.Task{
		SwarmingRpcsTaskRequestMetadata: nil,
		Commits:        commits,
		Id:             "", // Filled in when the task is inserted into the DB.
		IsolatedOutput: "", // Filled in when the task finishes, if successful.
		Name:           c.Name,
		Repo:           c.Repo,
		Revision:       c.Revision,
	}
}

// allDepsMet determines whether all dependencies for the given task candidate
// have been satisfied, and if so, returns their isolated outputs.
func (s *TaskScheduler) allDepsMet(c *taskCandidate) (bool, []string, error) {
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

// ComputeBlamelist computes the blamelist for the given taskCandidate. Returns
// the list of commits covered by the task, and any previous task which part or
// all of the blamelist was "stolen" from (see below). There are three cases:
//
// 1. This taskCandidate tests commits which have not yet been tested. Trace
//    commit history, accumulating commits until we find commits which have
//    been tested by previous tasks.
//
// 2. This taskCandidate runs at the same commit as a previous task. This is a
//    retry, so the entire blamelist of the previous task is "stolen".
//
// 3. This taskCandidate runs at a commit which is in a previous task's
//    blamelist, but no task has run at the same commit. This is a bisect. Trace
//    commit history, "stealing" commits from the previous task until we find a
//    commit which was covered by a *different* previous task.
func ComputeBlamelist(cache *db.TaskCache, repos *gitinfo.RepoMap, c *taskCandidate) ([]string, *db.Task, error) {
	commits := map[string]bool{}
	var stealFrom *db.Task

	// TODO(borenet): If this is a try job, don't compute the blamelist.

	// If this is the first invocation of a given task spec, save time by
	// setting the blamelist to only be c.Revision.
	if !cache.KnownTaskName(c.Name) {
		return []string{c.Revision}, nil, nil
	}

	repo, err := repos.Repo(c.Repo)
	if err != nil {
		return nil, nil, fmt.Errorf("Could not compute blamelist for task candidate: %s", err)
	}

	// computeBlamelistRecursive traces through commit history, adding to
	// the commits map until the blamelist for the task is complete.
	var computeBlamelistRecursive func(string) error
	computeBlamelistRecursive = func(hash string) error {
		// Shortcut for empty hashes. This can happen when a commit has
		// no parents.
		if hash == "" {
			return nil
		}

		// Shortcut in case we missed this case before; if this is the first
		// task for this task spec which has a valid Revision, the blamelist will
		// be the entire Git history. If we find too many commits, assume we've
		// hit this case and just return the Revision as the blamelist.
		if len(commits) > buildbot.MAX_BLAMELIST_COMMITS && stealFrom == nil {
			commits = map[string]bool{
				c.Revision: true,
			}
			return nil
		}

		// Determine whether any task already includes this commit.
		prev, err := cache.GetTaskForCommit(c.Name, hash)
		if err != nil {
			return fmt.Errorf("Could not find task %q for commit %q: %s", c.Name, hash, err)
		}

		// If we're stealing commits from a previous task but the current
		// commit is not in any task's blamelist, we must have scrolled past
		// the beginning of the tasks. Just return.
		if prev == nil && stealFrom != nil {
			return nil
		}

		// If a previous task already included this commit, we have to make a decision.
		if prev != nil {
			// If this Task's Revision is already included in a different
			// Task, then we're either bisecting or retrying a task. We'll
			// "steal" commits from the previous Task's blamelist.
			if len(commits) == 0 {
				stealFrom = prev

				// Another shortcut: If our Revision is the same as the
				// Revision of the Task we're stealing commits from,
				// ie. both tasks ran at the same commit, then this is a
				// retry. Just steal all of the commits without doing
				// any more work.
				if stealFrom.Revision == c.Revision {
					commits = map[string]bool{}
					for _, c := range stealFrom.Commits {
						commits[c] = true
					}
					return nil
				}
			}
			if stealFrom == nil || prev.Id != stealFrom.Id {
				// If we've hit a commit belonging to a different task,
				// we're done.
				return nil
			}
		}

		// Add the commit.
		commits[hash] = true

		// Recurse on the commit's parents.
		details, err := repo.Details(hash, false)
		if err != nil {
			return err
		}
		for _, p := range details.Parents {
			// If we've already seen this parent commit, don't revisit it.
			if _, ok := commits[p]; ok {
				continue
			}
			if err := computeBlamelistRecursive(p); err != nil {
				return err
			}
		}
		return nil
	}

	// Run the helper function to recurse on commit history.
	if err := computeBlamelistRecursive(c.Revision); err != nil {
		return nil, nil, err
	}

	rv := make([]string, 0, len(commits))
	for c, _ := range commits {
		rv = append(rv, c)
	}
	sort.Strings(rv)
	return rv, stealFrom, nil
}

func (s *TaskScheduler) findTaskCandidates(commitsByRepo map[string][]string) ([]*taskCandidate, error) {
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
				if previous != nil && previous.Revision == commit {
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
		depsMet, hashes, err := s.allDepsMet(c)
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

// processTaskCandidates computes the remaining information about each task
// candidate, eg. blamelists and scoring.
func (s *TaskScheduler) processTaskCandidates(candidates []*taskCandidate) error {
	// Compute blamelists.
	for _, c := range candidates {
		commits, stealingFrom, err := ComputeBlamelist(s.cache, s.repos, c)
		if err != nil {
			return err
		}
		c.Commits = commits
		c.StealingFrom = stealingFrom
	}

	// Score the candidates.
	now := time.Now()
	for _, c := range candidates {
		// The score for a candidate is based on the "testedness" increase
		// provided by running the task.
		stoleFromCommits := 0
		if c.StealingFrom != nil {
			stoleFromCommits = len(c.StealingFrom.Commits)
		}
		score := testednessIncrease(len(c.Commits), stoleFromCommits)

		// Scale the score by other factors, eg. time decay.
		decay, err := s.timeDecayForCommit(now, c.Repo, c.Revision)
		if err != nil {
			return err
		}
		score *= decay

		c.Score = score
	}
	sort.Sort(taskCandidateSlice(candidates))
	return nil
}

// regenerateTaskQueue obtains the set of all eligible task candidates, scores
// them, and prepares them to be triggered.
func (s *TaskScheduler) regenerateTaskQueue() error {
	// Update the task cache.
	if err := s.cache.Update(); err != nil {
		return nil
	}

	// Find the recent commits to use.
	if err := s.repos.Update(); err != nil {
		return err
	}
	from := time.Now().Add(-s.period)
	commits := map[string][]string{}
	for _, repoName := range s.repos.Repos() {
		repo, err := s.repos.Repo(repoName)
		if err != nil {
			return err
		}
		commits[repoName] = repo.From(from)
	}

	// Find and process task candidates.
	candidates, err := s.findTaskCandidates(commits)
	if err != nil {
		return err
	}
	if err := s.processTaskCandidates(candidates); err != nil {
		return err
	}

	s.queueMtx.Lock()
	defer s.queueMtx.Unlock()
	// TODO(borenet): Find a faster data structure for matching candidates
	// to free Swarming bots so that we don't have to scan the whole queue
	// every time.
	s.queue = candidates
	return nil
}

// timeDecay24Hr computes a linear time decay amount for the given duration,
// given the requested decay amount at 24 hours.
func timeDecay24Hr(decayAmt24Hr float64, elapsed time.Duration) float64 {
	return math.Max(1.0-(1.0-decayAmt24Hr)*(float64(elapsed)/float64(24*time.Hour)), 0.0)
}

// timeDecayForCommit computes a multiplier for a task candidate score based
// on how long ago the given commit landed. This allows us to prioritize more
// recent commits.
func (s *TaskScheduler) timeDecayForCommit(now time.Time, repoName, commit string) (float64, error) {
	if s.timeDecayAmt24Hr == 1.0 {
		// Shortcut for special case.
		return 1.0, nil
	}
	repo, err := s.repos.Repo(repoName)
	if err != nil {
		return 0.0, err
	}
	d, err := repo.Details(commit, false)
	if err != nil {
		return 0.0, err
	}
	return timeDecay24Hr(s.timeDecayAmt24Hr, now.Sub(d.Timestamp)), nil
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
