package scheduling

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	swarming_api "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/gitrepo"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/blacklist"
	"go.skia.org/infra/task_scheduler/go/db"
)

const (
	NUM_TOP_CANDIDATES = 50
)

// TaskScheduler is a struct used for scheduling tasks on bots.
type TaskScheduler struct {
	bl               *blacklist.Blacklist
	cache            db.TaskCache
	db               db.DB
	isolate          *isolate.Client
	lastScheduled    time.Time // protected by queueMtx.
	period           time.Duration
	queue            []*taskCandidate // protected by queueMtx.
	queueMtx         sync.RWMutex
	recentCommits    []string // protected by recentMtx.
	recentMtx        sync.RWMutex
	recentTaskSpecs  []string // protected by recentMtx.
	repoMap          *gitinfo.RepoMap
	repos            map[string]*gitrepo.Repo
	swarming         swarming.ApiClient
	taskCfgCache     *taskCfgCache
	timeDecayAmt24Hr float64
	workdir          string
}

func NewTaskScheduler(d db.DB, cache db.TaskCache, period time.Duration, workdir string, repoNames []string, isolateClient *isolate.Client, swarmingClient swarming.ApiClient, timeDecayAmt24Hr float64) (*TaskScheduler, error) {
	bl, err := blacklist.FromFile(path.Join(workdir, "blacklist.json"))
	if err != nil {
		return nil, err
	}

	repos := make(map[string]*gitrepo.Repo, len(repoNames))
	rm := gitinfo.NewRepoMap(workdir)
	for _, r := range repoNames {
		repo, err := gitrepo.NewRepo(r, path.Join(workdir, path.Base(r)))
		if err != nil {
			return nil, err
		}
		repos[r] = repo
		if _, err := rm.Repo(r); err != nil {
			return nil, err
		}
	}
	s := &TaskScheduler{
		bl:               bl,
		cache:            cache,
		db:               d,
		isolate:          isolateClient,
		period:           period,
		queue:            []*taskCandidate{},
		queueMtx:         sync.RWMutex{},
		repoMap:          rm,
		repos:            repos,
		swarming:         swarmingClient,
		taskCfgCache:     newTaskCfgCache(rm),
		timeDecayAmt24Hr: timeDecayAmt24Hr,
		workdir:          workdir,
	}
	return s, nil
}

// Start initiates the TaskScheduler's goroutines for scheduling tasks.
func (s *TaskScheduler) Start() {
	go func() {
		lv := metrics2.NewLiveness("last-successful-task-scheduling")
		for _ = range time.Tick(time.Minute) {
			if err := s.MainLoop(); err != nil {
				glog.Errorf("Failed to run the task scheduler: %s", err)
			} else {
				lv.Reset()
			}
		}
	}()
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

// RecentTaskSpecsAndCommits returns the lists of recent TaskSpec names and
// commit hashes.
func (s *TaskScheduler) RecentTaskSpecsAndCommits() ([]string, []string) {
	s.recentMtx.RLock()
	defer s.recentMtx.RUnlock()
	c := make([]string, len(s.recentCommits))
	copy(c, s.recentCommits)
	t := make([]string, len(s.recentTaskSpecs))
	copy(t, s.recentTaskSpecs)
	return t, c
}

// Trigger adds the given task request to the queue.
func (s *TaskScheduler) Trigger(repo, commit, taskSpec string) error {
	return fmt.Errorf("TaskScheduler.Trigger not implemented.")
}

// computeBlamelistRecursive traces through commit history, adding to
// the commits map until the blamelist for the task is complete.
//
// Args:
//  - repoName:  Name of the repository.
//  - commit:    Current commit as we're recursing through history.
//  - taskName:  Name of the task.
//  - revision:  Revision at which the task would run.
//  - commits:   Buffer in which to place the blamelist commits as they accumulate.
//  - cache:     TaskCache, for finding previous tasks.
//  - repo:      gitrepo.Repo corresponding to the repository.
//  - stealFrom: Existing Task from which this Task will steal commits, if one exists.
func computeBlamelistRecursive(repoName string, commit *gitrepo.Commit, taskName string, revision *gitrepo.Commit, commits []*gitrepo.Commit, cache db.TaskCache, repo *gitrepo.Repo, stealFrom *db.Task) ([]*gitrepo.Commit, *db.Task, error) {
	// Shortcut in case we missed this case before; if this is the first
	// task for this task spec which has a valid Revision, the blamelist will
	// be the entire Git history. If we find too many commits, assume we've
	// hit this case and just return the Revision as the blamelist.
	if len(commits) > buildbot.MAX_BLAMELIST_COMMITS && stealFrom == nil {
		commits = append(commits[:0], commit)
		return commits, nil, nil
	}

	// Determine whether any task already includes this commit.
	prev, err := cache.GetTaskForCommit(repoName, commit.Hash, taskName)
	if err != nil {
		return nil, nil, err
	}

	// If we're stealing commits from a previous task but the current
	// commit is not in any task's blamelist, we must have scrolled past
	// the beginning of the tasks. Just return.
	if prev == nil && stealFrom != nil {
		return commits, stealFrom, nil
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
			if stealFrom.Revision == revision.Hash {
				commits = commits[:0]
				for _, c := range stealFrom.Commits {
					ptr := repo.Get(c)
					if ptr == nil {
						return nil, nil, fmt.Errorf("No such commit: %q", c)
					}
					commits = append(commits, ptr)
				}
				return commits, stealFrom, nil
			}
		}
		if stealFrom == nil || prev != stealFrom {
			// If we've hit a commit belonging to a different task,
			// we're done.
			return commits, stealFrom, nil
		}
	}

	// Add the commit.
	commits = append(commits, commit)

	// Recurse on the commit's parents.
	for _, p := range commit.Parents {
		var err error
		commits, stealFrom, err = computeBlamelistRecursive(repoName, p, taskName, revision, commits, cache, repo, stealFrom)
		if err != nil {
			return nil, nil, err
		}
	}
	return commits, stealFrom, nil
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
//   - repo:       gitrepo.Repo instance corresponding to the repository of the task.
//   - name:       Name of the task.
//   - repoName:   Name of the repository for the task.
//   - revision:   Revision at which the task would run.
//   - commitsBuf: Buffer for use as scratch space.
func ComputeBlamelist(cache db.TaskCache, repo *gitrepo.Repo, name, repoName, revision string, commitsBuf []*gitrepo.Commit) ([]string, *db.Task, error) {
	// TODO(borenet): If this is a try job, don't compute the blamelist.

	// If this is the first invocation of a given task spec, save time by
	// setting the blamelist to only be c.Revision.
	if !cache.KnownTaskName(repoName, name) {
		return []string{revision}, nil, nil
	}

	commit := repo.Get(revision)
	if commit == nil {
		return nil, nil, fmt.Errorf("No such commit: %q", revision)
	}

	commitsBuf = commitsBuf[:0]

	// Run the helper function to recurse on commit history.
	commits, stealFrom, err := computeBlamelistRecursive(repoName, commit, name, commit, commitsBuf, cache, repo, nil)
	if err != nil {
		return nil, nil, err
	}

	// De-duplicate the commits list. Duplicates are rare but will occur
	// in the case of a task which runs after a short-lived branch is merged
	// so that the blamelist includes both the branch point and the merge
	// commit. In this case, any commits just before the branch point will
	// be duplicated.
	// TODO(borenet): This has never happened in the Skia repo, but the
	// below 8 lines of code account for ~10% of the execution time of this
	// function, which is the critical path for the scheduler. Consider
	// either ignoring this case or come up with an alternate solution which
	// moves this logic out of the critical path.
	rv := make([]string, 0, len(commits))
	visited := make(map[*gitrepo.Commit]bool, len(commits))
	for _, c := range commits {
		if !visited[c] {
			rv = append(rv, c.Hash)
			visited[c] = true
		}
	}
	return rv, stealFrom, nil
}

// findTaskCandidates goes through the given commits-by-repos, loads task specs
// from each repo/commit pair and passes them onto the out channel, filtering
// candidates which we don't want to run. The out channel will be closed when
// all task specs have been considered, or when an error occurs. If an error
// occurs, it will be passed onto the errs channel, but that channel will not
// be closed.
func (s *TaskScheduler) findTaskCandidates(commitsByRepo map[string][]string) (map[string][]*taskCandidate, error) {
	defer timer.New("TaskScheduler.findTaskCandidates").Stop()
	// Obtain all possible tasks.
	specs, err := s.taskCfgCache.GetTaskSpecsForCommits(commitsByRepo)
	if err != nil {
		return nil, err
	}
	bySpec := map[string][]*taskCandidate{}
	total := 0
	recentCommits := []string{}
	recentTaskSpecsMap := map[string]bool{}
	for repo, commits := range specs {
		for commit, tasks := range commits {
			recentCommits = append(recentCommits, commit)
			for name, task := range tasks {
				recentTaskSpecsMap[name] = true
				if rule := s.bl.MatchRule(name, commit); rule != "" {
					glog.Warningf("Skipping blacklisted task candidate: %s @ %s due to rule %q", name, commit, rule)
					continue
				}
				c := &taskCandidate{
					IsolatedHashes: nil,
					Name:           name,
					Repo:           repo,
					Revision:       commit,
					Score:          0.0,
					TaskSpec:       task,
				}
				// We shouldn't duplicate pending, in-progress,
				// or successfully completed tasks.
				previous, err := s.cache.GetTaskForCommit(c.Repo, c.Revision, c.Name)
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
					// Only retry a task once.
					if previous.RetryOf != "" {
						continue
					}
					c.RetryOf = previous.Id
				}

				// Don't consider candidates whose dependencies are not met.
				depsMet, idsToHashes, err := c.allDepsMet(s.cache)
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

				key := fmt.Sprintf("%s|%s", c.Repo, c.Name)
				candidates, ok := bySpec[key]
				if !ok {
					candidates = make([]*taskCandidate, 0, len(commits))
				}
				bySpec[key] = append(candidates, c)
				total++
			}
		}
	}
	glog.Infof("Found %d candidates in %d specs", total, len(bySpec))
	recentTaskSpecs := make([]string, 0, len(recentTaskSpecsMap))
	for spec, _ := range recentTaskSpecsMap {
		recentTaskSpecs = append(recentTaskSpecs, spec)
	}
	s.recentMtx.Lock()
	defer s.recentMtx.Unlock()
	s.recentCommits = recentCommits
	s.recentTaskSpecs = recentTaskSpecs
	return bySpec, nil
}

// processTaskCandidate computes the remaining information about the task
// candidate, eg. blamelists and scoring.
func (s *TaskScheduler) processTaskCandidate(c *taskCandidate, now time.Time, cache *cacheWrapper, commitsBuf []*gitrepo.Commit) error {
	// Compute blamelist.
	repo, ok := s.repos[c.Repo]
	if !ok {
		return fmt.Errorf("No such repo: %s", c.Repo)
	}
	commits, stealingFrom, err := ComputeBlamelist(cache, repo, c.Name, c.Repo, c.Revision, commitsBuf)
	if err != nil {
		return err
	}
	c.Commits = commits
	if stealingFrom != nil {
		c.StealingFromId = stealingFrom.Id
	}

	// Score the candidates.
	// The score for a candidate is based on the "testedness" increase
	// provided by running the task.
	stoleFromCommits := 0
	if stealingFrom != nil {
		stoleFromCommits = len(stealingFrom.Commits)
	}
	score := testednessIncrease(len(c.Commits), stoleFromCommits)

	// Scale the score by other factors, eg. time decay.
	decay, err := s.timeDecayForCommit(now, c.Repo, c.Revision)
	if err != nil {
		return err
	}
	score *= decay

	c.Score = score
	return nil
}

// regenerateTaskQueue obtains the set of all eligible task candidates, scores
// them, and prepares them to be triggered.
func (s *TaskScheduler) regenerateTaskQueue() error {
	defer timer.New("TaskScheduler.regenerateTaskQueue").Stop()

	// Find the recent commits to use.
	for _, repoName := range s.repoMap.Repos() {
		r, err := s.repoMap.Repo(repoName)
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
	if err := s.repoMap.Update(); err != nil {
		return err
	}
	from := time.Now().Add(-s.period)
	commits := map[string][]string{}
	for _, repoName := range s.repoMap.Repos() {
		repo, err := s.repoMap.Repo(repoName)
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
	defer timer.New("process task candidates").Stop()
	now := time.Now()
	processed := make(chan *taskCandidate)
	errs := make(chan error)
	wg := sync.WaitGroup{}
	for _, c := range candidates {
		wg.Add(1)
		go func(candidates []*taskCandidate) {
			defer wg.Done()
			cache := newCacheWrapper(s.cache)
			commitsBuf := make([]*gitrepo.Commit, 0, buildbot.MAX_BLAMELIST_COMMITS)
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
		return rvErrs[0]
	}
	sort.Sort(taskCandidateSlice(rvCandidates))

	s.queueMtx.Lock()
	defer s.queueMtx.Unlock()
	s.lastScheduled = time.Now()
	s.queue = rvCandidates
	return nil
}

// getCandidatesToSchedule matches the list of free Swarming bots to task
// candidates in the queue and returns the candidates which should be run.
// Assumes that the tasks are sorted in decreasing order by score.
func getCandidatesToSchedule(bots []*swarming_api.SwarmingRpcsBotInfo, tasks []*taskCandidate) []*taskCandidate {
	defer timer.New("scheduling.getCandidatesToSchedule").Stop()
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
	sort.Sort(taskCandidateSlice(rv))
	return rv
}

// tempGitRepo creates a git repository in a temporary directory and returns its
// location.
func (s *TaskScheduler) tempGitRepo(repoUrl, subdir string) (string, error) {
	tmpRepoDir := path.Join(s.workdir, "tmp_git_repos")
	if err := os.Mkdir(tmpRepoDir, os.ModePerm); err != nil && !os.IsExist(err) {
		return "", err
	}
	tmp, err := ioutil.TempDir(tmpRepoDir, fmt.Sprintf("tmp_%s", path.Base(repoUrl)))
	if err != nil {
		return "", err
	}
	d := path.Join(tmp, subdir)
	if err := os.MkdirAll(d, os.ModePerm); err != nil {
		if !os.IsExist(err) {
			return "", err
		}
	}
	if _, err := exec.RunCwd(d, "git", "clone", repoUrl, "."); err != nil {
		return "", err
	}
	if _, err := exec.RunCwd(d, "git", "checkout", "master"); err != nil {
		return "", err
	}
	// Write a dummy .gclient file in the parent of the checkout.
	if err := ioutil.WriteFile(path.Join(d, "..", ".gclient"), []byte(""), os.ModePerm); err != nil {
		return "", err
	}
	return d, nil
}

// scheduleTasks queries for free Swarming bots and triggers tasks according
// to relative priorities in the queue.
func (s *TaskScheduler) scheduleTasks() error {
	defer timer.New("TaskScheduler.scheduleTasks").Stop()
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

	// Cleanup.
	defer s.cleanupRepos()

	// Isolate the tasks by commit.
	for repoName, commits := range byRepoCommit {
		repoDir, err := s.tempGitRepo(path.Join(s.workdir, path.Base(repoName)), strings.TrimSuffix(path.Base(repoName), ".git"))
		if err != nil {
			return err
		}
		defer util.RemoveAll(repoDir)
		infraBotsDir := path.Join(repoDir, "infra", "bots")
		for commit, candidates := range commits {
			if _, err := exec.RunCwd(repoDir, "git", "checkout", commit); err != nil {
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
	byCandidateId := make(map[string]*db.Task, len(schedule))
	tasksToInsert := make(map[string]*db.Task, len(schedule)*2)
	for _, candidate := range schedule {
		t := candidate.MakeTask()
		if err := s.db.AssignId(t); err != nil {
			return err
		}
		req := candidate.MakeTaskRequest(t.Id)
		j, err := json.MarshalIndent(req, "", "    ")
		if err != nil {
			glog.Errorf("Failed to marshal JSON: %s", err)
		} else {
			glog.Infof("Requesting Swarming Task:\n\n%s\n\n", string(j))
		}
		resp, err := s.swarming.TriggerTask(req)
		if err != nil {
			return err
		}
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
			if _, _, _, err := parseId(candidate.StealingFromId); err == nil {
				stealingFrom = byCandidateId[candidate.StealingFromId]
				if stealingFrom == nil {
					return fmt.Errorf("Attempting to backfill a just-triggered candidate but can't find it: %q", candidate.StealingFromId)
				}
			} else {
				var ok bool
				stealingFrom, ok = tasksToInsert[candidate.StealingFromId]
				if !ok {
					stealingFrom, err = s.cache.GetTask(candidate.StealingFromId)
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

// MainLoop runs a single end-to-end task scheduling loop.
func (s *TaskScheduler) MainLoop() error {
	defer timer.New("TaskSchedulder.MainLoop").Stop()

	glog.Infof("Task Scheduler updating...")
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
		if err := updateUnfinishedTasks(s.cache, s.db, s.swarming); err != nil {
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

	// Update the task cache.
	if err := s.cache.Update(); err != nil {
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

	// Update the cache again to include the newly-inserted tasks.
	return s.cache.Update()
}

// cleanupRepos cleans up the scheduler's repos. It logs errors rather than
// returning them.
func (s *TaskScheduler) cleanupRepos() {
	for _, repoName := range s.repoMap.Repos() {
		repo, err := s.repoMap.Repo(repoName)
		if err != nil {
			glog.Errorf("Failed to cleanup repo: %s", err)
			continue
		}
		if err := repo.Reset("HEAD"); err != nil {
			glog.Errorf("Failed to cleanup repo: %s", err)
			continue
		}
		// TODO(borenet): With trybots, we probably need to "git clean -d -f".
		if err := repo.Checkout("master"); err != nil {
			glog.Errorf("Failed to cleanup repo: %s", err)
			continue
		}

	}
}

// updateRepos syncs the scheduler's repos.
func (s *TaskScheduler) updateRepos() error {
	s.cleanupRepos()
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
func (s *TaskScheduler) timeDecayForCommit(now time.Time, repoName, commit string) (float64, error) {
	if s.timeDecayAmt24Hr == 1.0 {
		// Shortcut for special case.
		return 1.0, nil
	}
	repo, err := s.repoMap.Repo(repoName)
	if err != nil {
		return 0.0, err
	}
	d, err := repo.Details(commit, false)
	if err != nil {
		return 0.0, err
	}
	return timeDecay24Hr(s.timeDecayAmt24Hr, now.Sub(d.Timestamp)), nil
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

// updateUnfinishedTasks queries Swarming for all unfinished tasks and updates
// their status in the DB.
func updateUnfinishedTasks(cache db.TaskCache, d db.DB, s swarming.ApiClient) error {
	tasks, err := cache.UnfinishedTasks()
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
			swarmTask, err := s.GetTask(t.SwarmingTaskId)
			if err != nil {
				errs[idx] = fmt.Errorf("Failed to update unfinished task; failed to get updated task from swarming: %s", err)
				return
			}
			if err := db.UpdateDBFromSwarmingTask(d, swarmTask); err != nil {
				errs[idx] = fmt.Errorf("Failed to update unfinished task: %s", err)
				return
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
