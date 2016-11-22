package scheduling

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"path"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"

	swarming_api "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/blacklist"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/tryjobs"
)

const (
	// Manually-forced jobs have high priority.
	CANDIDATE_SCORE_FORCE_RUN = 100.0

	// Try jobs have high priority, equal to building at HEAD when we're
	// 5 commits behind.
	CANDIDATE_SCORE_TRY_JOB = 10.0

	NUM_TOP_CANDIDATES = 50
)

var (
	ERR_BLAMELIST_DONE = errors.New("ERR_BLAMELIST_DONE")
)

// TaskScheduler is a struct used for scheduling tasks on bots.
type TaskScheduler struct {
	bl               *blacklist.Blacklist
	busyBots         *busyBots
	db               db.DB
	isolate          *isolate.Client
	jCache           db.JobCache
	lastScheduled    time.Time // protected by queueMtx.
	period           time.Duration
	queue            []*taskCandidate // protected by queueMtx.
	queueMtx         sync.RWMutex
	repos            repograph.Map
	swarming         swarming.ApiClient
	taskCfgCache     *specs.TaskCfgCache
	tCache           db.TaskCache
	timeDecayAmt24Hr float64
	triggerMetrics   *periodicTriggerMetrics
	tryjobs          *tryjobs.TryJobIntegrator
	workdir          string
}

func NewTaskScheduler(d db.DB, period time.Duration, workdir string, repos repograph.Map, isolateClient *isolate.Client, swarmingClient swarming.ApiClient, c *http.Client, timeDecayAmt24Hr float64, buildbucketApiUrl, trybotBucket string, projectRepoMapping map[string]string) (*TaskScheduler, error) {
	bl, err := blacklist.FromFile(path.Join(workdir, "blacklist.json"))
	if err != nil {
		return nil, err
	}

	// Create caches.
	tCache, err := db.NewTaskCache(d, period)
	if err != nil {
		glog.Fatal(err)
	}

	jCache, err := db.NewJobCache(d, period, db.GitRepoGetRevisionTimestamp(repos))
	if err != nil {
		glog.Fatal(err)
	}

	taskCfgCache := specs.NewTaskCfgCache(repos)
	tryjobs, err := tryjobs.NewTryJobIntegrator(buildbucketApiUrl, trybotBucket, c, d, jCache, projectRepoMapping, repos, taskCfgCache)
	if err != nil {
		return nil, err
	}

	pm, err := newPeriodicTriggerMetrics(workdir)
	if err != nil {
		return nil, err
	}

	s := &TaskScheduler{
		bl:               bl,
		busyBots:         newBusyBots(),
		db:               d,
		isolate:          isolateClient,
		jCache:           jCache,
		period:           period,
		queue:            []*taskCandidate{},
		queueMtx:         sync.RWMutex{},
		repos:            repos,
		swarming:         swarmingClient,
		taskCfgCache:     taskCfgCache,
		tCache:           tCache,
		timeDecayAmt24Hr: timeDecayAmt24Hr,
		triggerMetrics:   pm,
		tryjobs:          tryjobs,
		workdir:          workdir,
	}
	return s, nil
}

// Start initiates the TaskScheduler's goroutines for scheduling tasks. beforeMainLoop
// will be run before each scheduling iteration.
func (s *TaskScheduler) Start(ctx context.Context, beforeMainLoop func()) {
	s.tryjobs.Start(ctx)
	lv := metrics2.NewLiveness("last-successful-task-scheduling")
	go util.RepeatCtx(time.Minute, ctx, func() {
		beforeMainLoop()
		if err := s.MainLoop(); err != nil {
			glog.Errorf("Failed to run the task scheduler: %s", err)
		} else {
			lv.Reset()
		}
	})
}

// TaskSchedulerStatus is a struct which provides status information about the
// TaskScheduler.
type TaskSchedulerStatus struct {
	LastScheduled time.Time        `json:"last_scheduled"`
	TopCandidates []*taskCandidate `json:"top_candidates"`
}

// Status returns the current status of the TaskScheduler.
func (s *TaskScheduler) Status() *TaskSchedulerStatus {
	s.queueMtx.RLock()
	defer s.queueMtx.RUnlock()
	candidates := make([]*taskCandidate, 0, NUM_TOP_CANDIDATES)
	n := NUM_TOP_CANDIDATES
	if len(s.queue) < n {
		n = len(s.queue)
	}
	for _, c := range s.queue[:n] {
		candidates = append(candidates, c.Copy())
	}
	return &TaskSchedulerStatus{
		LastScheduled: s.lastScheduled,
		TopCandidates: candidates,
	}
}

// RecentSpecsAndCommits returns the lists of recent JobSpec names, TaskSpec
// names and commit hashes.
func (s *TaskScheduler) RecentSpecsAndCommits() ([]string, []string, []string) {
	return s.taskCfgCache.RecentSpecsAndCommits()
}

// TriggerJob adds the given Job to the database and returns its ID.
func (s *TaskScheduler) TriggerJob(repo, commit, jobName string) (string, error) {
	j, err := s.taskCfgCache.MakeJob(db.RepoState{
		Repo:     repo,
		Revision: commit,
	}, jobName)
	if err != nil {
		return "", err
	}
	j.IsForce = true
	if err := s.db.PutJob(j); err != nil {
		return "", err
	}
	glog.Infof("Created manually-triggered Job %q", j.Id)
	return j.Id, nil
}

// CancelJob cancels the given Job if it is not already finished.
func (s *TaskScheduler) CancelJob(id string) (*db.Job, error) {
	// TODO(borenet): Prevent concurrent update of the Job.
	j, err := s.jCache.GetJobMaybeExpired(id)
	if err != nil {
		return nil, err
	}
	if j.Done() {
		return nil, fmt.Errorf("Job %s is already finished with status %s", id, j.Status)
	}
	j.Status = db.JOB_STATUS_CANCELED
	if err := s.jobFinished(j); err != nil {
		return nil, err
	}
	if err := s.db.PutJob(j); err != nil {
		return nil, err
	}
	return j, s.jCache.Update()
}

// ComputeBlamelist computes the blamelist for a new task, specified by name,
// repo, and revision. Returns the list of commits covered by the task, and any
// previous task which part or all of the blamelist was "stolen" from (see
// below). There are three cases:
//
// 1. The new task tests commits which have not yet been tested. Trace commit
//    history, accumulating commits until we find commits which have been tested
//    by previous tasks.
//
// 2. The new task runs at the same commit as a previous task. This is a retry,
//    so the entire blamelist of the previous task is "stolen".
//
// 3. The new task runs at a commit which is in a previous task's blamelist, but
//    no task has run at the same commit. This is a bisect. Trace commit
//    history, "stealing" commits from the previous task until we find a commit
//    which was covered by a *different* previous task.
//
// Args:
//   - cache:      TaskCache instance.
//   - repo:       repograph.Graph instance corresponding to the repository of the task.
//   - taskName:   Name of the task.
//   - repoName:   Name of the repository for the task.
//   - revision:   Revision at which the task would run.
//   - commitsBuf: Buffer for use as scratch space.
func ComputeBlamelist(cache db.TaskCache, repo *repograph.Graph, taskName, repoName string, revision *repograph.Commit, commitsBuf []*repograph.Commit) ([]string, *db.Task, error) {
	// If this is the first invocation of a given task spec, don't bother
	// searching for commits. We only want to trigger new bots at branch
	// heads, so if the passed-in revision is a branch head, return it as
	// the blamelist, otherwise return an empty blamelist.
	if !cache.KnownTaskName(repoName, taskName) {
		for _, name := range repo.Branches() {
			if repo.Get(name).Hash == revision.Hash {
				return []string{revision.Hash}, nil, nil
			}
		}
		return []string{}, nil, nil
	}

	commitsBuf = commitsBuf[:0]
	var stealFrom *db.Task

	// Run the helper function to recurse on commit history.
	if err := revision.Recurse(func(commit *repograph.Commit) (bool, error) {
		// Determine whether any task already includes this commit.
		prev, err := cache.GetTaskForCommit(repoName, commit.Hash, taskName)
		if err != nil {
			return false, err
		}

		// Shortcut in case we missed this case before; if this is the
		// first invocation of a given task spec, don't bother searching
		// for commits. We only want to trigger new bots at branch
		// heads, so if the passed-in revision is a branch head, return
		// it as the blamelist, otherwise return an empty blamelist.
		if stealFrom == nil && (len(commitsBuf) > buildbot.MAX_BLAMELIST_COMMITS || (len(commit.Parents) == 0 && prev == nil)) {
			for _, name := range repo.Branches() {
				if repo.Get(name).Hash == revision.Hash {
					commitsBuf = append(commitsBuf[:0], revision)
					glog.Warningf("Found too many commits for %s @ %s; is a branch head.", taskName, revision.Hash)
					return false, ERR_BLAMELIST_DONE
				}
			}
			glog.Warningf("Found too many commits for %s @ %s; not a branch head so returning empty.", taskName, revision.Hash)
			commitsBuf = commitsBuf[:0]
			return false, ERR_BLAMELIST_DONE
		}

		// If we're stealing commits from a previous task but the current
		// commit is not in any task's blamelist, we must have scrolled past
		// the beginning of the tasks. Just return.
		if prev == nil && stealFrom != nil {
			return false, nil
		}

		// If a previous task already included this commit, we have to make a decision.
		if prev != nil {
			// If this Task's Revision is already included in a different
			// Task, then we're either bisecting or retrying a task. We'll
			// "steal" commits from the previous Task's blamelist.
			if len(commitsBuf) == 0 {
				stealFrom = prev

				// Another shortcut: If our Revision is the same as the
				// Revision of the Task we're stealing commits from,
				// ie. both tasks ran at the same commit, then this is a
				// retry. Just steal all of the commits without doing
				// any more work.
				if stealFrom.Revision == revision.Hash {
					commitsBuf = commitsBuf[:0]
					for _, c := range stealFrom.Commits {
						ptr := repo.Get(c)
						if ptr == nil {
							return false, fmt.Errorf("No such commit: %q", c)
						}
						commitsBuf = append(commitsBuf, ptr)
					}
					return false, ERR_BLAMELIST_DONE
				}
			}
			if stealFrom == nil || prev.Id != stealFrom.Id {
				// If we've hit a commit belonging to a different task,
				// we're done.
				return false, nil
			}
		}

		// Add the commit.
		commitsBuf = append(commitsBuf, commit)

		// Recurse on the commit's parents.
		return true, nil

	}); err != nil && err != ERR_BLAMELIST_DONE {
		return nil, nil, err
	}

	rv := make([]string, 0, len(commitsBuf))
	for _, c := range commitsBuf {
		rv = append(rv, c.Hash)
	}
	return rv, stealFrom, nil
}

// findTaskCandidatesForJobs returns the set of all taskCandidates needed by all
// currently-unfinished jobs.
func (s *TaskScheduler) findTaskCandidatesForJobs(unfinishedJobs []*db.Job) (map[db.TaskKey]*taskCandidate, error) {
	defer metrics2.FuncTimer().Stop()

	// Get the repo+commit+taskspecs for each job.
	candidates := map[db.TaskKey]*taskCandidate{}
	for _, j := range unfinishedJobs {
		for tsName, _ := range j.Dependencies {
			key := j.MakeTaskKey(tsName)
			spec, err := s.taskCfgCache.GetTaskSpec(j.RepoState, tsName)
			if err != nil {
				return nil, err
			}
			c := &taskCandidate{
				JobCreated: j.Created,
				TaskKey:    key,
				TaskSpec:   spec,
			}
			candidates[key] = c
		}
	}
	glog.Infof("Found %d task candidates for %d unfinished jobs.", len(candidates), len(unfinishedJobs))
	return candidates, nil
}

// filterTaskCandidates reduces the set of taskCandidates to the ones we might
// actually want to run and organizes them by repo and TaskSpec name.
func (s *TaskScheduler) filterTaskCandidates(preFilterCandidates map[db.TaskKey]*taskCandidate) (map[string]map[string][]*taskCandidate, error) {
	defer metrics2.FuncTimer().Stop()

	candidatesBySpec := map[string]map[string][]*taskCandidate{}
	total := 0
	for _, c := range preFilterCandidates {
		if rule := s.bl.MatchRule(c.Name, c.Revision); rule != "" {
			glog.Warningf("Skipping blacklisted task candidate: %s @ %s due to rule %q", c.Name, c.Revision, rule)
			continue
		}

		// We shouldn't duplicate pending, in-progress,
		// or successfully completed tasks.
		prevTasks, err := s.tCache.GetTasksByKey(&c.TaskKey)
		if err != nil {
			return nil, err
		}
		var previous *db.Task
		if len(prevTasks) > 0 {
			// Just choose the last (most recently created) previous
			// Task.
			previous = prevTasks[len(prevTasks)-1]
		}
		if previous != nil {
			if previous.Status == db.TASK_STATUS_PENDING || previous.Status == db.TASK_STATUS_RUNNING {
				continue
			}
			if previous.Success() {
				continue
			}
			// Only retry a task once.
			if previous.RetryOf != "" {
				continue
			}
			c.RetryOf = previous.Id
		}

		// Don't consider candidates whose dependencies are not met.
		depsMet, idsToHashes, err := c.allDepsMet(s.tCache)
		if err != nil {
			return nil, err
		}
		if !depsMet {
			continue
		}
		hashes := make([]string, 0, len(idsToHashes))
		parentTaskIds := make([]string, 0, len(idsToHashes))
		for id, hash := range idsToHashes {
			hashes = append(hashes, hash)
			parentTaskIds = append(parentTaskIds, id)
		}
		c.IsolatedHashes = hashes
		sort.Strings(parentTaskIds)
		c.ParentTaskIds = parentTaskIds

		candidates, ok := candidatesBySpec[c.Repo]
		if !ok {
			candidates = map[string][]*taskCandidate{}
			candidatesBySpec[c.Repo] = candidates
		}
		candidates[c.Name] = append(candidates[c.Name], c)
		total++
	}
	glog.Infof("Filtered to %d candidates in %d spec categories.", total, len(candidatesBySpec))
	return candidatesBySpec, nil
}

// processTaskCandidate computes the remaining information about the task
// candidate, eg. blamelists and scoring.
func (s *TaskScheduler) processTaskCandidate(c *taskCandidate, now time.Time, cache *cacheWrapper, commitsBuf []*repograph.Commit) error {
	if c.IsTryJob() {
		c.Score = CANDIDATE_SCORE_TRY_JOB + now.Sub(c.JobCreated).Hours()
		return nil
	}

	// Compute blamelist.
	repo, ok := s.repos[c.Repo]
	if !ok {
		return fmt.Errorf("No such repo: %s", c.Repo)
	}
	revision := repo.Get(c.Revision)
	if revision == nil {
		return fmt.Errorf("No such commit %s in %s.", c.Revision, c.Repo)
	}
	commits, stealingFrom, err := ComputeBlamelist(cache, repo, c.Name, c.Repo, revision, commitsBuf)
	if err != nil {
		return err
	}
	c.Commits = commits
	if stealingFrom != nil {
		c.StealingFromId = stealingFrom.Id
	}
	if len(c.Commits) > 0 && !util.In(c.Revision, c.Commits) {
		glog.Errorf("task candidate %s @ %s doesn't have its own revision in its blamelist: %v", c.Name, c.Revision, c.Commits)
	}

	if c.IsForceRun() {
		c.Score = CANDIDATE_SCORE_FORCE_RUN + now.Sub(c.JobCreated).Hours()
		return nil
	}

	// Score the candidate.
	// The score for a candidate is based on the "testedness" increase
	// provided by running the task.
	stoleFromCommits := 0
	if stealingFrom != nil {
		// Treat retries as if they're new; don't use stealingFrom.Commits.
		if c.RetryOf != "" {
			if stealingFrom.Id != c.RetryOf {
				glog.Errorf("Candidate %v is a retry of %s but is stealing commits from %s!", c.TaskKey, c.RetryOf, stealingFrom.Id)
			}
		} else {
			stoleFromCommits = len(stealingFrom.Commits)
		}
	}
	score := testednessIncrease(len(c.Commits), stoleFromCommits)

	// Scale the score by other factors, eg. time decay.
	decay, err := s.timeDecayForCommit(now, revision)
	if err != nil {
		return err
	}
	score *= decay

	c.Score = score
	return nil
}

// Process the task candidates.
func (s *TaskScheduler) processTaskCandidates(candidates map[string]map[string][]*taskCandidate) ([]*taskCandidate, error) {
	defer metrics2.FuncTimer().Stop()
	now := time.Now()
	processed := make(chan *taskCandidate)
	errs := make(chan error)
	wg := sync.WaitGroup{}
	for _, cs := range candidates {
		for _, c := range cs {
			wg.Add(1)
			go func(candidates []*taskCandidate) {
				defer wg.Done()
				cache := newCacheWrapper(s.tCache)
				commitsBuf := make([]*repograph.Commit, 0, buildbot.MAX_BLAMELIST_COMMITS)
				for {
					// Find the best candidate.
					idx := -1
					var best *taskCandidate
					for i, candidate := range candidates {
						c := candidate.Copy()
						if err := s.processTaskCandidate(c, now, cache, commitsBuf); err != nil {
							errs <- err
							return
						}
						if best == nil || c.Score > best.Score {
							best = c
							idx = i
						}
					}
					if best == nil {
						return
					}
					processed <- best
					t := best.MakeTask()
					t.Id = best.MakeId()
					cache.insert(t)
					if best.StealingFromId != "" {
						stoleFrom, err := cache.GetTask(best.StealingFromId)
						if err != nil {
							errs <- err
							return
						}
						stole := util.NewStringSet(best.Commits)
						oldC := util.NewStringSet(stoleFrom.Commits)
						newC := oldC.Complement(stole)
						commits := make([]string, 0, len(newC))
						for c, _ := range newC {
							commits = append(commits, c)
						}
						stoleFrom.Commits = commits
						cache.insert(stoleFrom)
					}
					candidates = append(candidates[:idx], candidates[idx+1:]...)
				}
			}(c)
		}
	}
	go func() {
		wg.Wait()
		close(processed)
		close(errs)
	}()
	rvCandidates := []*taskCandidate{}
	rvErrs := []error{}
	for {
		select {
		case c, ok := <-processed:
			if ok {
				rvCandidates = append(rvCandidates, c)
			} else {
				processed = nil
			}
		case err, ok := <-errs:
			if ok {
				rvErrs = append(rvErrs, err)
			} else {
				errs = nil
			}
		}
		if processed == nil && errs == nil {
			break
		}
	}

	if len(rvErrs) != 0 {
		return nil, rvErrs[0]
	}
	sort.Sort(taskCandidateSlice(rvCandidates))
	return rvCandidates, nil
}

// regenerateTaskQueue obtains the set of all eligible task candidates, scores
// them, and prepares them to be triggered.
func (s *TaskScheduler) regenerateTaskQueue() error {
	defer metrics2.FuncTimer().Stop()

	// Find the unfinished Jobs.
	unfinishedJobs, err := s.jCache.UnfinishedJobs()
	if err != nil {
		return err
	}

	// Find TaskSpecs for all unfinished Jobs.
	preFilterCandidates, err := s.findTaskCandidatesForJobs(unfinishedJobs)
	if err != nil {
		return err
	}

	// Filter task candidates.
	candidates, err := s.filterTaskCandidates(preFilterCandidates)
	if err != nil {
		return err
	}

	// Process the remaining task candidates.
	queue, err := s.processTaskCandidates(candidates)
	if err != nil {
		return err
	}

	// Save the queue.
	s.queueMtx.Lock()
	defer s.queueMtx.Unlock()
	s.lastScheduled = time.Now()
	s.queue = queue
	return nil
}

// getCandidatesToSchedule matches the list of free Swarming bots to task
// candidates in the queue and returns the candidates which should be run.
// Assumes that the tasks are sorted in decreasing order by score.
func getCandidatesToSchedule(bots []*swarming_api.SwarmingRpcsBotInfo, tasks []*taskCandidate) []*taskCandidate {
	defer metrics2.FuncTimer().Stop()
	// Create a bots-by-swarming-dimension mapping.
	botsByDim := map[string]util.StringSet{}
	// TODO(benjaminwagner): Remove debug logging.
	botsByDeviceType := map[string]util.StringSet{}
	for _, b := range bots {
		for _, dim := range b.Dimensions {
			for _, val := range dim.Value {
				d := fmt.Sprintf("%s:%s", dim.Key, val)
				if _, ok := botsByDim[d]; !ok {
					botsByDim[d] = util.StringSet{}
				}
				botsByDim[d][b.BotId] = true
				if dim.Key == "device_type" {
					if _, ok := botsByDeviceType[val]; !ok {
						botsByDeviceType[val] = util.StringSet{}
					}
					botsByDeviceType[val][b.BotId] = true
				}
			}
		}
	}

	// TODO(benjaminwagner): Remove debug logging.
	glog.Infof("DEBUG: free android bots: %s", botsByDeviceType)

	// Match bots to tasks.
	// TODO(borenet): Some tasks require a more specialized bot. We should
	// match so that less-specialized tasks don't "steal" more-specialized
	// bots which they don't actually need.
	rv := make([]*taskCandidate, 0, len(bots))
	for _, c := range tasks {
		// TODO(borenet): Make this threshold configurable.
		if c.Score <= 0.0 {
			glog.Warningf("candidate %s @ %s has a score of %2f; skipping (%d commits).", c.Name, c.Revision, c.Score, len(c.Commits))
			continue
		}

		// For each dimension of the task, find the set of bots which matches.
		matches := util.StringSet{}
		// TODO(benjaminwagner): Remove debug logging.
		deviceType := ""
		for i, d := range c.TaskSpec.Dimensions {
			if i == 0 {
				matches = matches.Union(botsByDim[d])
			} else {
				matches = matches.Intersect(botsByDim[d])
			}
			if strings.HasPrefix(d, "device_type:") {
				deviceType = d[len("device_type:"):]
			}
		}
		if len(matches) > 0 {
			// TODO(benjaminwagner): Remove debug logging.
			if deviceType != "" {
				glog.Infof("DEBUG: %s task %s matches %s", deviceType, c.TaskKey, matches)
			}
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
			c.BotId = bot
			c.TaskSpec.Dimensions = append(c.TaskSpec.Dimensions, fmt.Sprintf("id:%s", bot))

			// Add the task to the scheduling list.
			rv = append(rv, c)

			// TODO(benjaminwagner): Remove debug logging.
			if deviceType != "" {
				glog.Infof("DEBUG: chose bot %s for %s task %s", bot, deviceType, c.TaskKey)
			}

			// If we've exhausted the bot list, stop here.
			if len(botsByDim) == 0 {
				break
			}
		}
	}
	sort.Sort(taskCandidateSlice(rv))
	return rv
}

// isolateTasks sets up the given RepoState and isolates the given
// taskCandidates.
func (s *TaskScheduler) isolateTasks(rs db.RepoState, candidates []*taskCandidate) error {
	// Create and check out a temporary repo.
	repo, ok := s.repos[rs.Repo]
	if !ok {
		return fmt.Errorf("Unknown repo: %q", rs.Repo)
	}
	c, err := specs.TempGitRepo(repo.Repo(), rs)
	if err != nil {
		return err
	}
	defer c.Delete()

	// Isolate the tasks.
	infraBotsDir := path.Join(c.Dir(), "infra", "bots")
	baseDir := path.Dir(c.Dir())
	tasks := make([]*isolate.Task, 0, len(candidates))
	for _, c := range candidates {
		tasks = append(tasks, c.MakeIsolateTask(infraBotsDir, baseDir))
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
	return nil
}

func hasDim(bot *swarming_api.SwarmingRpcsBotInfo, key, value string) bool {
	for _, dim := range bot.Dimensions {
		if key == dim.Key {
			if value == "*" {
				return true
			}
			for _, val := range dim.Value {
				if value == val {
					return true
				}
			}
			return false
		}
	}
	return false
}

// scheduleTasks queries for free Swarming bots and triggers tasks according
// to relative priorities in the queue.
func (s *TaskScheduler) scheduleTasks() error {
	defer metrics2.FuncTimer().Stop()
	// Find free bots, match them with tasks.
	bots, err := getFreeSwarmingBots(s.swarming)
	if err != nil {
		return err
	}
	bots = s.busyBots.Filter(bots)

	s.queueMtx.Lock()
	defer s.queueMtx.Unlock()
	schedule := getCandidatesToSchedule(bots, s.queue)

	// First, group by commit hash since we have to isolate the code at
	// a particular revision for each task.
	byRepoState := map[db.RepoState][]*taskCandidate{}
	for _, c := range schedule {
		byRepoState[c.RepoState] = append(byRepoState[c.RepoState], c)
	}

	// Isolate the tasks by commit.
	for rs, candidates := range byRepoState {
		if err := s.isolateTasks(rs, candidates); err != nil {
			return err
		}
	}

	// Trigger tasks.
	byCandidateId := make(map[string]*db.Task, len(schedule))
	tasksToInsert := make(map[string]*db.Task, len(schedule)*2)
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
		s.busyBots.Reserve(candidate.BotId, resp.TaskId)
		created, err := swarming.ParseTimestamp(resp.Request.CreatedTs)
		if err != nil {
			return fmt.Errorf("Failed to parse timestamp of created task: %s", err)
		}
		t.Created = created
		t.SwarmingTaskId = resp.TaskId
		byCandidateId[candidate.MakeId()] = t
		tasksToInsert[t.Id] = t
		// If we're stealing commits from another task, find it and adjust
		// its blamelist.
		// TODO(borenet): We're retrieving a cached task which may have been
		// changed since the cache was last updated. We need to handle that.
		if candidate.StealingFromId != "" {
			var stealingFrom *db.Task
			if _, err := parseId(candidate.StealingFromId); err == nil {
				stealingFrom = byCandidateId[candidate.StealingFromId]
				if stealingFrom == nil {
					return fmt.Errorf("Attempting to backfill a just-triggered candidate but can't find it: %q", candidate.StealingFromId)
				}
			} else {
				var ok bool
				stealingFrom, ok = tasksToInsert[candidate.StealingFromId]
				if !ok {
					stealingFrom, err = s.tCache.GetTask(candidate.StealingFromId)
					if err != nil {
						return err
					}
				}
			}
			oldCommits := util.NewStringSet(stealingFrom.Commits)
			stealing := util.NewStringSet(t.Commits)
			stealingFrom.Commits = oldCommits.Complement(stealing).Keys()
			tasksToInsert[stealingFrom.Id] = stealingFrom
		}
	}
	tasks := make([]*db.Task, 0, len(tasksToInsert))
	for _, t := range tasksToInsert {
		tasks = append(tasks, t)
	}

	// Insert the tasks into the database.
	if err := s.db.PutTasks(tasks); err != nil {
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

	// Note; if regenerateQueue and scheduleTasks are ever decoupled so that
	// the queue is reused by multiple runs of scheduleTasks, we'll need to
	// address the fact that some candidates may still have their
	// StoleFromId pointing to candidates which have been triggered and
	// removed from the queue. In that case, we should just need to write a
	// loop which updates those candidates to use the IDs of the newly-
	// inserted Tasks in the database rather than the candidate ID.

	glog.Infof("Triggered %d tasks on %d bots.", len(schedule), len(bots))
	return nil
}

// gatherNewJobs finds and inserts Jobs for all new commits.
func (s *TaskScheduler) gatherNewJobs() error {
	defer metrics2.FuncTimer().Stop()

	// Find all new Jobs for all new commits.
	now := time.Now()
	newJobs := []*db.Job{}
	for repoUrl, r := range s.repos {
		if err := r.RecurseAllBranches(func(c *repograph.Commit) (bool, error) {
			if now.Add(-s.period).After(c.Timestamp) {
				return false, nil
			}
			scheduled, err := s.jCache.ScheduledJobsForCommit(repoUrl, c.Hash)
			if err != nil {
				return false, err
			}
			if scheduled {
				return false, nil
			}
			rs := db.RepoState{
				Repo:     repoUrl,
				Revision: c.Hash,
			}
			cfg, err := s.taskCfgCache.ReadTasksCfg(rs)
			if err != nil {
				return false, err
			}
			for name, spec := range cfg.Jobs {
				if spec.Trigger == "" {
					j, err := s.taskCfgCache.MakeJob(rs, name)
					if err != nil {
						return false, err
					}
					newJobs = append(newJobs, j)
				}
			}
			if c.Hash == "50537e46e4f0999df0a4707b227000cfa8c800ff" {
				// Stop recursing here, since Jobs were added
				// in this commit and previous commits won't be
				// valid.
				return false, nil
			}
			return true, nil
		}); err != nil {
			return err
		}
	}

	if err := s.db.PutJobs(newJobs); err != nil {
		return err
	}

	// Also trigger any available periodic jobs.
	if err := s.triggerPeriodicJobs(); err != nil {
		return err
	}

	return s.jCache.Update()
}

// MainLoop runs a single end-to-end task scheduling loop.
func (s *TaskScheduler) MainLoop() error {
	defer metrics2.FuncTimer().Stop()

	glog.Infof("Task Scheduler updating...")

	// TODO(borenet): This is only needed for the perftest because it no
	// longer has access to the TaskCache used by TaskScheduler. Since it
	// pushes tasks into the DB between executions of MainLoop, we need to
	// update the cache here so that we see those changes.
	if err := s.tCache.Update(); err != nil {
		return err
	}

	var e1, e2 error
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.updateRepos(); err != nil {
			e1 = err
		}
	}()

	// TODO(borenet): Do we have to fail out of scheduling if we fail to
	// updateUnfinishedTasks? Maybe we can just add a liveness metric and
	// alert if we go too long without updating successfully.
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.updateUnfinishedTasks(); err != nil {
			e2 = err
		}
	}()
	wg.Wait()

	if e1 != nil {
		return e1
	}
	if e2 != nil {
		return e2
	}

	// Update the caches.
	if err := s.tCache.Update(); err != nil {
		return err
	}

	if err := s.jCache.Update(); err != nil {
		return err
	}

	if err := s.updateUnfinishedJobs(); err != nil {
		return err
	}

	// Add Jobs for new commits.
	if err := s.gatherNewJobs(); err != nil {
		return err
	}

	// Regenerate the queue, schedule tasks.
	// TODO(borenet): Query for free Swarming bots while we're regenerating
	// the queue.
	glog.Infof("Task Scheduler regenerating the queue...")
	if err := s.regenerateTaskQueue(); err != nil {
		return err
	}

	glog.Infof("Task Scheduler scheduling tasks...")
	if err := s.scheduleTasks(); err != nil {
		return err
	}
	return nil
}

// updateRepos syncs the scheduler's repos.
func (s *TaskScheduler) updateRepos() error {
	for _, r := range s.repos {
		if err := r.Update(); err != nil {
			return err
		}
	}
	return nil
}

// QueueLen returns the length of the queue.
func (s *TaskScheduler) QueueLen() int {
	s.queueMtx.RLock()
	defer s.queueMtx.RUnlock()
	return len(s.queue)
}

// timeDecay24Hr computes a linear time decay amount for the given duration,
// given the requested decay amount at 24 hours.
func timeDecay24Hr(decayAmt24Hr float64, elapsed time.Duration) float64 {
	return math.Max(1.0-(1.0-decayAmt24Hr)*(float64(elapsed)/float64(24*time.Hour)), 0.0)
}

// timeDecayForCommit computes a multiplier for a task candidate score based
// on how long ago the given commit landed. This allows us to prioritize more
// recent commits.
func (s *TaskScheduler) timeDecayForCommit(now time.Time, commit *repograph.Commit) (float64, error) {
	if s.timeDecayAmt24Hr == 1.0 {
		// Shortcut for special case.
		return 1.0, nil
	}
	rv := timeDecay24Hr(s.timeDecayAmt24Hr, now.Sub(commit.Timestamp))
	if rv == 0.0 {
		glog.Warningf("timeDecayForCommit is zero. Now: %s, Commit: %s ts %s, TimeDecay: %2f\nDetails: %v", now, commit.Hash, commit.Timestamp, s.timeDecayAmt24Hr, commit)
	}
	return rv, nil
}

func (ts *TaskScheduler) GetBlacklist() *blacklist.Blacklist {
	return ts.bl
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
	defer metrics2.FuncTimer().Stop()
	bots, err := s.ListSkiaBots()
	if err != nil {
		return nil, err
	}
	// TODO(benjaminwagner): Remove debug logging.
	androidbots := map[string]string{}
	rv := make([]*swarming_api.SwarmingRpcsBotInfo, 0, len(bots))
	for _, bot := range bots {
		// TODO(benjaminwagner): Remove debug logging.
		isandroid := hasDim(bot, "device_type", "*")
		if bot.IsDead {
			if isandroid {
				androidbots[bot.BotId] = "IsDead"
			}
			continue
		}
		if bot.Quarantined {
			if isandroid {
				androidbots[bot.BotId] = "Quarantined"
			}
			continue
		}
		if bot.TaskId != "" {
			if isandroid {
				androidbots[bot.BotId] = "has TaskId " + bot.TaskId
			}
			continue
		}
		if isandroid {
			androidbots[bot.BotId] = "available"
		}
		rv = append(rv, bot)
	}
	glog.Infof("DEBUG: android bots: %s", androidbots)
	return rv, nil
}

// updateUnfinishedTasks queries Swarming for all unfinished tasks and updates
// their status in the DB.
func (s *TaskScheduler) updateUnfinishedTasks() error {
	tasks, err := s.tCache.UnfinishedTasks()
	if err != nil {
		return err
	}
	sort.Sort(db.TaskSlice(tasks))

	// TODO(borenet): This would be faster if Swarming had a
	// get-multiple-tasks-by-ID endpoint.
	var wg sync.WaitGroup
	errs := make([]error, len(tasks))
	for i, t := range tasks {
		wg.Add(1)
		go func(idx int, t *db.Task) {
			defer wg.Done()
			swarmTask, err := s.swarming.GetTask(t.SwarmingTaskId)
			if err != nil {
				errs[idx] = fmt.Errorf("Failed to update unfinished task; failed to get updated task from swarming: %s", err)
				return
			}
			if err := db.UpdateDBFromSwarmingTask(s.db, swarmTask); err != nil {
				errs[idx] = fmt.Errorf("Failed to update unfinished task: %s", err)
				return
			}
			// If the task is finished, free its bot.
			if swarmTask.State != db.SWARMING_STATE_PENDING {
				s.busyBots.Release(swarmTask.BotId, t.SwarmingTaskId)
			}
		}(i, t)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// jobFinished marks the Job as finished and reports its result to Buildbucket
// if necessary.
func (s *TaskScheduler) jobFinished(j *db.Job) error {
	if !j.Done() {
		return fmt.Errorf("jobFinished called on Job with status %q", j.Status)
	}
	j.Finished = time.Now()
	if j.IsTryJob() {
		// Report the try job status to Buildbucket. If
		// we fail to do so, don't insert the updated
		// Job into the DB - we'll come back to it on
		// the next loop.
		if err := s.tryjobs.JobFinished(j); err != nil {
			return fmt.Errorf("Failed to send update for try job: %s", err)
		}
	}
	return nil
}

// updateUnfinishedJobs updates all not-yet-finished Jobs to determine if their
// state has changed.
func (s *TaskScheduler) updateUnfinishedJobs() error {
	jobs, err := s.jCache.UnfinishedJobs()
	if err != nil {
		return err
	}

	modified := make([]*db.Job, 0, len(jobs))
	errs := []error{}
	for _, j := range jobs {
		tasks, err := s.getTasksForJob(j)
		if err != nil {
			return err
		}
		summaries := make(map[string][]*db.TaskSummary, len(tasks))
		for k, v := range tasks {
			cpy := make([]*db.TaskSummary, 0, len(v))
			for _, t := range v {
				cpy = append(cpy, t.MakeTaskSummary())
			}
			summaries[k] = cpy
		}
		if !reflect.DeepEqual(summaries, j.Tasks) {
			j.Tasks = summaries
			j.Status = j.DeriveStatus()
			if j.Done() {
				if err := s.jobFinished(j); err != nil {
					errs = append(errs, err)
				}
			}

			modified = append(modified, j)
		}
	}
	if len(modified) > 0 {
		if err := s.db.PutJobs(modified); err != nil {
			errs = append(errs, err)
		} else if err := s.jCache.Update(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("Got errors updating unfinished jobs: %v", errs)
	}
	return nil
}

// getTasksForJob finds all Tasks for the given Job. It returns the Tasks
// in a map keyed by name.
func (s *TaskScheduler) getTasksForJob(j *db.Job) (map[string][]*db.Task, error) {
	tasks := map[string][]*db.Task{}
	for d, _ := range j.Dependencies {
		key := j.MakeTaskKey(d)
		gotTasks, err := s.tCache.GetTasksByKey(&key)
		if err != nil {
			return nil, err
		}
		tasks[d] = gotTasks
	}
	return tasks, nil
}

// GetJob returns the given Job.
func (s *TaskScheduler) GetJob(id string) (*db.Job, error) {
	return s.jCache.GetJobMaybeExpired(id)
}
