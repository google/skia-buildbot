package task_scheduler

import (
	"fmt"
	"math"
	"path"
	"sort"
	"sync"
	"time"

	swarming_api "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/build_scheduler/go/db"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
)

// TaskScheduler is a struct used for scheduling tasks on bots.
type TaskScheduler struct {
	cache            *db.TaskCache
	db               db.DB
	isolate          *isolate.Client
	period           time.Duration
	queue            []*taskCandidate
	queueMtx         sync.RWMutex
	repos            *gitinfo.RepoMap
	swarming         swarming.ApiClient
	taskCfgCache     *taskCfgCache
	timeDecayAmt24Hr float64
	workdir          string
}

func NewTaskScheduler(d db.DB, cache *db.TaskCache, period time.Duration, workdir string, repoNames []string, isolateClient *isolate.Client, swarmingClient swarming.ApiClient) (*TaskScheduler, error) {
	repos := gitinfo.NewRepoMap(workdir)
	for _, r := range repoNames {
		if _, err := repos.Repo(r); err != nil {
			return nil, err
		}
	}
	s := &TaskScheduler{
		cache:            cache,
		db:               d,
		isolate:          isolateClient,
		period:           period,
		queue:            []*taskCandidate{},
		queueMtx:         sync.RWMutex{},
		repos:            repos,
		swarming:         swarmingClient,
		taskCfgCache:     newTaskCfgCache(repos),
		timeDecayAmt24Hr: 1.0,
		workdir:          workdir,
	}
	return s, nil
}

// Start initiates the TaskScheduler's goroutines for scheduling tasks.
func (s *TaskScheduler) Start() {
	go func() {
		lv := metrics2.NewLiveness("last-successful-task-scheduling")
		for _ = range time.Tick(time.Minute) {
			if err := s.mainLoop(); err != nil {
				glog.Errorf("Failed to run the task scheduler: %s", err)
			} else {
				lv.Reset()
			}
		}
	}()
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

	// Filter out candidates we've already run or whose dependencies have not been met.
	validCandidates := make([]*taskCandidate, 0, len(candidates))
	for _, c := range candidates {
		// We shouldn't duplicate pending, in-progress,
		// or successfully completed tasks.
		previous, err := s.cache.GetTaskForCommit(c.Name, c.Revision)
		if err != nil {
			return nil, err
		}
		if previous != nil && previous.Revision == c.Revision {
			if previous.Status == db.TASK_STATUS_PENDING || previous.Status == db.TASK_STATUS_RUNNING {
				continue
			}
			if previous.Success() {
				continue
			}
		}

		// Don't consider candidates whose dependencies are not met.
		depsMet, hashes, err := c.allDepsMet(s.cache)
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
	stoleFrom := make([]*db.Task, len(candidates))
	for idx, c := range candidates {
		commits, stealingFrom, err := ComputeBlamelist(s.cache, s.repos, c)
		if err != nil {
			return err
		}
		c.Commits = commits
		if stealingFrom != nil {
			c.StealingFromId = stealingFrom.Id
			stoleFrom[idx] = stealingFrom
		}
	}

	// Score the candidates.
	now := time.Now()
	for idx, c := range candidates {
		// The score for a candidate is based on the "testedness" increase
		// provided by running the task.
		stoleFromCommits := 0
		if stoleFrom[idx] != nil {
			stoleFromCommits = len(stoleFrom[idx].Commits)
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
	for _, repoName := range s.repos.Repos() {
		r, err := s.repos.Repo(repoName)
		if err != nil {
			return err
		}
		if err := r.Reset("HEAD"); err != nil {
			return err
		}
		if err := r.Checkout("master"); err != nil {
			return err
		}
	}
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

// getCandidatesToSchedule matches the list of free Swarming bots to task
// candidates in the queue and returns the candidates which should be run.
// Assumes that the tasks are sorted in decreasing order by score.
func getCandidatesToSchedule(bots []*swarming_api.SwarmingRpcsBotInfo, tasks []*taskCandidate) []*taskCandidate {
	// Create a bots-by-swarming-dimension mapping.
	botsByDim := map[string]util.StringSet{}
	for _, b := range bots {
		for _, dim := range b.Dimensions {
			for _, val := range dim.Value {
				d := fmt.Sprintf("%s:%s", dim.Key, val)
				if _, ok := botsByDim[d]; !ok {
					botsByDim[d] = util.StringSet{}
				}
				botsByDim[d][b.BotId] = true
			}
		}
	}

	// Match bots to tasks.
	// TODO(borenet): Some tasks require a more specialized bot. We should
	// match so that less-specialized tasks don't "steal" more-specialized
	// bots which they don't actually need.
	rv := make([]*taskCandidate, 0, len(bots))
	for _, c := range tasks {
		// For each dimension of the task, find the set of bots which matches.
		matches := util.StringSet{}
		for i, d := range c.TaskSpec.Dimensions {
			if i == 0 {
				matches = matches.Union(botsByDim[d])
			} else {
				matches = matches.Intersect(botsByDim[d])
			}
		}
		if len(matches) > 0 {
			// We're going to run this task. Choose a bot. Sort the
			// bots by ID so that the choice is deterministic.
			choices := make([]string, 0, len(matches))
			for botId, _ := range matches {
				choices = append(choices, botId)
			}
			sort.Strings(choices)
			bot := choices[0]

			// Remove the bot from consideration.
			for dim, subset := range botsByDim {
				delete(subset, bot)
				if len(subset) == 0 {
					delete(botsByDim, dim)
				}
			}

			// Force the candidate to run on this bot.
			c.TaskSpec.Dimensions = append(c.TaskSpec.Dimensions, fmt.Sprintf("id:%s", bot))

			// Add the task to the scheduling list.
			rv = append(rv, c)

			// If we've exhausted the bot list, stop here.
			if len(botsByDim) == 0 {
				break
			}
		}
	}
	return rv
}

// scheduleTasks queries for free Swarming bots and triggers tasks according
// to relative priorities in the queue.
func (s *TaskScheduler) scheduleTasks() error {
	// Find free bots, match them with tasks.
	bots, err := getFreeSwarmingBots(s.swarming)
	if err != nil {
		return err
	}
	s.queueMtx.Lock()
	defer s.queueMtx.Unlock()
	schedule := getCandidatesToSchedule(bots, s.queue)

	// First, group by commit hash since we have to isolate the code at
	// a particular revision for each task.
	byRepoCommit := map[string]map[string][]*taskCandidate{}
	for _, c := range schedule {
		if mRepo, ok := byRepoCommit[c.Repo]; !ok {
			byRepoCommit[c.Repo] = map[string][]*taskCandidate{c.Revision: []*taskCandidate{c}}
		} else {
			mRepo[c.Revision] = append(mRepo[c.Revision], c)
		}
	}

	// Isolate the tasks by commit.
	for repoName, commits := range byRepoCommit {
		infraBotsDir := path.Join(s.workdir, repoName, "infra", "bots")
		for commit, candidates := range commits {
			repo, err := s.repos.Repo(repoName)
			if err != nil {
				return err
			}
			if err := repo.Checkout(commit); err != nil {
				return err
			}
			tasks := make([]*isolate.Task, 0, len(candidates))
			for _, c := range candidates {
				tasks = append(tasks, c.MakeIsolateTask(infraBotsDir, s.workdir))
			}
			hashes, err := s.isolate.IsolateTasks(tasks)
			if err != nil {
				return err
			}
			if len(hashes) != len(candidates) {
				return fmt.Errorf("IsolateTasks returned incorrect number of hashes.")
			}
			for i, c := range candidates {
				c.IsolatedInput = hashes[i]
			}
		}
	}

	// Trigger tasks.
	tasks := make([]*db.Task, 0, len(schedule)*2)
	for _, candidate := range schedule {
		t := candidate.MakeTask()
		if err := s.db.AssignId(t); err != nil {
			return err
		}
		req := candidate.MakeTaskRequest(t.Id)
		resp, err := s.swarming.TriggerTask(req)
		if err != nil {
			return err
		}
		if _, err := t.UpdateFromSwarming(resp); err != nil {
			return err
		}
		tasks = append(tasks, t)
		// If we're stealing commits from another task, find it and adjust
		// its blamelist.
		// TODO(borenet): We're retrieving a cached task which may have been
		// changed since the cache was last updated. We need to handle that.
		if candidate.StealingFromId != "" {
			stealingFrom, err := s.cache.GetTask(candidate.StealingFromId)
			if err != nil {
				return err
			}
			oldCommits := util.NewStringSet(stealingFrom.Commits)
			stealing := util.NewStringSet(t.Commits)
			stealingFrom.Commits = oldCommits.Complement(stealing).Keys()
			tasks = append(tasks, stealingFrom)
		}
	}

	// Insert the tasks into the database.
	if err := s.db.PutTasks(tasks); err != nil {
		return err
	}

	// Update the TaskCache.
	if err := s.cache.Update(); err != nil {
		return err
	}

	// Remove the tasks from the queue.
	newQueue := make([]*taskCandidate, 0, len(s.queue)-len(schedule))
	for i, j := 0, 0; i < len(s.queue); {
		if j >= len(schedule) {
			newQueue = append(newQueue, s.queue[i:]...)
			break
		}
		if s.queue[i] == schedule[j] {
			j++
		} else {
			newQueue = append(newQueue, s.queue[i])
		}
		i++
	}
	s.queue = newQueue
	return nil
}

// mainLoop runs a single end-to-end task scheduling loop.
func (s *TaskScheduler) mainLoop() error {
	if err := s.regenerateTaskQueue(); err != nil {
		return err
	}
	return s.scheduleTasks()
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

// getFreeSwarmingBots returns a slice of free swarming bots.
func getFreeSwarmingBots(s swarming.ApiClient) ([]*swarming_api.SwarmingRpcsBotInfo, error) {
	bots, err := s.ListSkiaBots()
	if err != nil {
		return nil, err
	}
	rv := make([]*swarming_api.SwarmingRpcsBotInfo, 0, len(bots))
	for _, bot := range bots {
		if bot.IsDead {
			continue
		}
		if bot.Quarantined {
			continue
		}
		if bot.TaskId != "" {
			continue
		}
		rv = append(rv, bot)
	}
	return rv, nil
}
