package scheduling

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"path"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"go.opencensus.io/trace"

	multierror "github.com/hashicorp/go-multierror"
	"go.skia.org/infra/go/cas"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/skip_tasks"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
	"golang.org/x/oauth2"
)

const (
	// Manually-forced jobs have high priority.
	CANDIDATE_SCORE_FORCE_RUN = 100.0

	// Try jobs have high priority, equal to building at HEAD when we're
	// 5 commits behind.
	CANDIDATE_SCORE_TRY_JOB = 10.0

	// When retrying a try job task that has failed, prioritize the retry
	// lower than tryjob tasks that haven't run yet.
	CANDIDATE_SCORE_TRY_JOB_RETRY_MULTIPLIER = 0.75

	// When bisecting or retrying a task that failed or had a mishap, add a bonus
	// to the raw score.
	//
	// A value of 0.75 means that a retry scores higher than a bisecting a
	// successful task with a blamelist of 2 commits, but lower than testing new
	// commits or bisecting successful tasks with blamelist of 3 or
	// more. Bisecting a failure with a blamelist of 2 commits scores the same as
	// bisecting a successful task with a blamelist of 4 commits.
	CANDIDATE_SCORE_FAILURE_OR_MISHAP_BONUS = 0.75

	// MAX_BLAMELIST_COMMITS is the maximum number of commits which are
	// allowed in a task blamelist before we stop tracing commit history.
	MAX_BLAMELIST_COMMITS = 500

	// Measurement name for task candidate counts by dimension set.
	MEASUREMENT_TASK_CANDIDATE_COUNT = "task_candidate_count"

	NUM_TOP_CANDIDATES = 50

	// To avoid errors resulting from DB transaction size limits, we
	// restrict the number of tasks triggered per TaskSpec (we insert tasks
	// into the DB in chunks by TaskSpec) to half of the DB transaction size
	// limit (since we may need to update an existing whose blamelist was
	// split by the new task).
	SCHEDULING_LIMIT_PER_TASK_SPEC = firestore.MAX_TRANSACTION_DOCS / 2

	GCS_MAIN_LOOP_DIAGNOSTICS_DIR = "MainLoop"
	GCS_DIAGNOSTICS_WRITE_TIMEOUT = 60 * time.Second
)

var (
	ERR_BLAMELIST_DONE = errors.New("ERR_BLAMELIST_DONE")
)

// TaskScheduler is a struct used for scheduling tasks on bots.
type TaskScheduler struct {
	busyBots            *busyBots
	candidateMetrics    map[string]metrics2.Int64Metric
	candidateMetricsMtx sync.Mutex
	cdPool              string
	db                  db.DB
	diagClient          gcs.GCSClient
	diagInstance        string
	rbeCas              cas.CAS
	rbeCasInstance      string
	jCache              cache.JobCache
	lastScheduled       time.Time // protected by queueMtx.

	pendingInsert    map[string]bool
	pendingInsertMtx sync.RWMutex

	pools        []string
	pubsubCount  metrics2.Counter
	pubsubTopic  string
	queue        []*TaskCandidate // protected by queueMtx.
	queueMtx     sync.RWMutex
	repos        repograph.Map
	skipTasks    *skip_tasks.DB
	taskExecutor types.TaskExecutor
	taskCfgCache task_cfg_cache.TaskCfgCache
	tCache       cache.TaskCache
	// testWaitGroup keeps track of any goroutines the TaskScheduler methods
	// create so that tests can ensure all goroutines finish before asserting.
	testWaitGroup         sync.WaitGroup
	timeDecayAmt24Hr      float64
	triggeredCount        metrics2.Counter
	updateUnfinishedCount metrics2.Counter
	window                window.Window
}

func NewTaskScheduler(ctx context.Context, d db.DB, bl *skip_tasks.DB, period time.Duration, numCommits int, repos repograph.Map, rbeCas cas.CAS, rbeCasInstance string, taskExecutor types.TaskExecutor, c *http.Client, timeDecayAmt24Hr float64, pools []string, cdPool, pubsubTopic string, taskCfgCache task_cfg_cache.TaskCfgCache, ts oauth2.TokenSource, diagClient gcs.GCSClient, diagInstance string) (*TaskScheduler, error) {
	// Repos must be updated before window is initialized; otherwise the repos may be uninitialized,
	// resulting in the window being too short, causing the caches to be loaded with incomplete data.
	for _, r := range repos {
		if err := r.Update(ctx); err != nil {
			return nil, fmt.Errorf("Failed initial repo sync: %s", err)
		}
	}
	w, err := window.New(ctx, period, numCommits, repos)
	if err != nil {
		return nil, fmt.Errorf("Failed to create window: %s", err)
	}

	// Create caches.
	tCache, err := cache.NewTaskCache(ctx, d, w, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to create TaskCache: %s", err)
	}

	jCache, err := cache.NewJobCache(ctx, d, w, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to create JobCache: %s", err)
	}

	// Add the CD pool to the list of pools if it isn't there already. We'll
	// verify that only CD tasks get assigned to that pool.
	if cdPool != "" && !util.In(cdPool, pools) {
		pools = append(pools, cdPool)
	}

	s := &TaskScheduler{
		skipTasks:             bl,
		busyBots:              newBusyBots(),
		candidateMetrics:      map[string]metrics2.Int64Metric{},
		cdPool:                cdPool,
		db:                    d,
		diagClient:            diagClient,
		diagInstance:          diagInstance,
		jCache:                jCache,
		pendingInsert:         map[string]bool{},
		pools:                 pools,
		pubsubCount:           metrics2.GetCounter("task_scheduler_pubsub_handler"),
		pubsubTopic:           pubsubTopic,
		queue:                 []*TaskCandidate{},
		queueMtx:              sync.RWMutex{},
		rbeCas:                rbeCas,
		rbeCasInstance:        rbeCasInstance,
		repos:                 repos,
		taskExecutor:          taskExecutor,
		taskCfgCache:          taskCfgCache,
		tCache:                tCache,
		timeDecayAmt24Hr:      timeDecayAmt24Hr,
		triggeredCount:        metrics2.GetCounter("task_scheduler_triggered_count"),
		updateUnfinishedCount: metrics2.GetCounter("task_scheduler_update_unfinished_tasks_count"),
		window:                w,
	}
	return s, nil
}

// Close cleans up resources used by the TaskScheduler.
func (s *TaskScheduler) Close() error {
	err1 := s.taskCfgCache.Close()
	err2 := s.rbeCas.Close()
	if err1 != nil {
		return skerr.Wrap(err1)
	}
	if err2 != nil {
		return skerr.Wrap(err2)
	}
	return nil
}

// Start initiates the TaskScheduler's goroutines for scheduling tasks. beforeMainLoop
// will be run before each scheduling iteration.
func (s *TaskScheduler) Start(ctx context.Context) {
	lvScheduling := metrics2.NewLiveness("last_successful_task_scheduling")
	cleanup.Repeat(5*time.Second, func(_ context.Context) {
		// Explicitly ignore the passed-in context; this allows us to
		// finish the current scheduling cycle even if the context is
		// canceled, which helps prevent "orphaned" tasks which were
		// triggered on Swarming but were not inserted into the DB.
		ctx, span := trace.StartSpan(context.Background(), "taskscheduler_Start_MainLoop", trace.WithSampler(trace.AlwaysSample()))
		defer span.End()
		if err := s.MainLoop(ctx); err != nil {
			sklog.Errorf("Failed to run the task scheduler: %s", err)
		} else {
			lvScheduling.Reset()
		}
	}, nil)
	lvUpdateUnfinishedTasks := metrics2.NewLiveness("last_successful_tasks_update")
	go util.RepeatCtx(ctx, 5*time.Minute, func(ctx context.Context) {
		ctx, span := trace.StartSpan(ctx, "taskscheduler_Start_UpdateUnfinishedTasks", trace.WithSampler(trace.AlwaysSample()))
		defer span.End()
		if err := s.updateUnfinishedTasks(ctx); err != nil {
			sklog.Errorf("Failed to run periodic tasks update: %s", err)
		} else {
			lvUpdateUnfinishedTasks.Reset()
		}
	})
}

// putTask is a wrapper around DB.PutTask which adds the task to the cache.
func (s *TaskScheduler) putTask(ctx context.Context, t *types.Task) error {
	if err := s.db.PutTask(ctx, t); err != nil {
		return err
	}
	s.tCache.AddTasks([]*types.Task{t})
	return nil
}

// putTasks is a wrapper around DB.PutTasks which adds the tasks to the cache.
func (s *TaskScheduler) putTasks(ctx context.Context, t []*types.Task) error {
	if err := s.db.PutTasks(ctx, t); err != nil {
		return err
	}
	s.tCache.AddTasks(t)
	return nil
}

// putTasksInChunks is a wrapper around DB.PutTasksInChunks which adds the tasks
// to the cache.
func (s *TaskScheduler) putTasksInChunks(ctx context.Context, t []*types.Task) error {
	if err := s.db.PutTasksInChunks(ctx, t); err != nil {
		return err
	}
	s.tCache.AddTasks(t)
	return nil
}

// putJob is a wrapper around DB.PutJob which adds the job to the cache.
func (s *TaskScheduler) putJob(ctx context.Context, j *types.Job) error {
	if err := s.db.PutJob(ctx, j); err != nil {
		return err
	}
	s.jCache.AddJobs([]*types.Job{j})
	return nil
}

// putJobsInChunks is a wrapper around DB.PutJobsInChunks which adds the jobs
// to the cache.
func (s *TaskScheduler) putJobsInChunks(ctx context.Context, j []*types.Job) error {
	if err := s.db.PutJobsInChunks(ctx, j); err != nil {
		return err
	}
	s.jCache.AddJobs(j)
	return nil
}

// TaskSchedulerStatus is a struct which provides status information about the
// TaskScheduler.
type TaskSchedulerStatus struct {
	LastScheduled time.Time        `json:"last_scheduled"`
	TopCandidates []*TaskCandidate `json:"top_candidates"`
}

// Status returns the current status of the TaskScheduler.
func (s *TaskScheduler) Status() *TaskSchedulerStatus {
	defer metrics2.FuncTimer().Stop()
	s.queueMtx.RLock()
	defer s.queueMtx.RUnlock()
	n := NUM_TOP_CANDIDATES
	if len(s.queue) < n {
		n = len(s.queue)
	}
	return &TaskSchedulerStatus{
		LastScheduled: s.lastScheduled,
		TopCandidates: s.queue[:n],
	}
}

// TaskCandidateSearchTerms includes fields used for searching task candidates.
type TaskCandidateSearchTerms struct {
	types.TaskKey
	Dimensions []string `json:"dimensions"`
}

// SearchQueue returns all task candidates in the queue which match the given
// TaskKey. Any blank fields are considered to be wildcards.
func (s *TaskScheduler) SearchQueue(q *TaskCandidateSearchTerms) []*TaskCandidate {
	s.queueMtx.RLock()
	defer s.queueMtx.RUnlock()
	rv := []*TaskCandidate{}
	for _, c := range s.queue {
		// TODO(borenet): I wish there was a better way to do this.
		if q.ForcedJobId != "" && c.ForcedJobId != q.ForcedJobId {
			continue
		}
		if q.Name != "" && c.Name != q.Name {
			continue
		}
		if q.Repo != "" && c.Repo != q.Repo {
			continue
		}
		if q.Revision != "" && c.Revision != q.Revision {
			continue
		}
		if q.Issue != "" && c.Issue != q.Issue {
			continue
		}
		if q.PatchRepo != "" && c.PatchRepo != q.PatchRepo {
			continue
		}
		if q.Patchset != "" && c.Patchset != q.Patchset {
			continue
		}
		if q.Server != "" && c.Server != q.Server {
			continue
		}
		if len(q.Dimensions) > 0 {
			ok := true
			for _, d := range q.Dimensions {
				if !util.In(d, c.TaskSpec.Dimensions) {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
		}
		rv = append(rv, c)
	}
	return rv
}

type commitTester interface {
	TestCommit(repo string, c *repograph.Commit) bool
}

type tasksCfgGetter interface {
	Get(ctx context.Context, rs types.RepoState) (*specs.TasksCfg, error, error)
}

type commitGetter interface {
	Get(ref string) *repograph.Commit
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
func ComputeBlamelist(ctx context.Context, cache cache.TaskCache, repo commitGetter, taskName, repoName string, revision *repograph.Commit, commitsBuf []*repograph.Commit, tcg tasksCfgGetter, ct commitTester) ([]string, *types.Task, error) {
	ctx, span := trace.StartSpan(ctx, "taskscheduler_ComputeBlamelist")
	defer span.End()
	commitsBuf = commitsBuf[:0]
	var stealFrom *types.Task

	// Run the helper function to recurse on commit history.
	if err := revision.Recurse(func(commit *repograph.Commit) error {
		// If this commit is outside the scheduling window, we won't
		// have tasks in the cache for it, and thus we won't be able
		// to compute the correct blamelist. Stop here.
		if !ct.TestCommit(repoName, commit) {
			return repograph.ErrStopRecursing
		}

		// If the task spec is not defined at this commit, it can't be
		// part of the blamelist.
		rs := types.RepoState{
			Repo:     repoName,
			Revision: commit.Hash,
		}
		cfg, cachedErr, err := tcg.Get(ctx, rs)
		if cachedErr != nil {
			sklog.Warningf("Stopping blamelist recursion at %s; TaskCfgCache has error: %s", commit.Hash, err)
			return repograph.ErrStopRecursing
		}
		if err != nil {
			if err == task_cfg_cache.ErrNoSuchEntry {
				sklog.Warningf("Computing blamelist for %s in %s @ %s, no cached TasksCfg at %s; stopping blamelist calculation.", taskName, repoName, revision.Hash, commit.Hash)
				return repograph.ErrStopRecursing
			}
			return skerr.Wrap(err)
		}
		if _, ok := cfg.Tasks[taskName]; !ok {
			sklog.Infof("Computing blamelist for %s in %s @ %s, Task Spec not defined in %s (have %d tasks); stopping blamelist calculation.", taskName, repoName, revision.Hash, commit.Hash, len(cfg.Tasks))
			return repograph.ErrStopRecursing
		}

		// Determine whether any task already includes this commit.
		prev, err := cache.GetTaskForCommit(repoName, commit.Hash, taskName)
		if err != nil {
			return err
		}

		// If the blamelist is too large, just use a single commit.
		if len(commitsBuf) > MAX_BLAMELIST_COMMITS {
			commitsBuf = append(commitsBuf[:0], revision)
			//sklog.Warningf("Found too many commits for %s @ %s; using single-commit blamelist.", taskName, revision.Hash)
			return ERR_BLAMELIST_DONE
		}

		// If we're stealing commits from a previous task but the current
		// commit is not in any task's blamelist, we must have scrolled past
		// the beginning of the tasks. Just return.
		if prev == nil && stealFrom != nil {
			return repograph.ErrStopRecursing
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
							return skerr.Fmt("No such commit: %q", c)
						}
						commitsBuf = append(commitsBuf, ptr)
					}
					return ERR_BLAMELIST_DONE
				}
			}
			if stealFrom == nil || prev.Id != stealFrom.Id {
				// If we've hit a commit belonging to a different task,
				// we're done.
				return repograph.ErrStopRecursing
			}
		}

		// Add the commit.
		commitsBuf = append(commitsBuf, commit)

		// Recurse on the commit's parents.
		return nil

	}); err != nil && err != ERR_BLAMELIST_DONE {
		return nil, nil, err
	}

	rv := make([]string, 0, len(commitsBuf))
	for _, c := range commitsBuf {
		rv = append(rv, c.Hash)
	}
	return rv, stealFrom, nil
}

type taskSchedulerMainLoopDiagnostics struct {
	StartTime  time.Time        `json:"startTime"`
	EndTime    time.Time        `json:"endTime"`
	Error      string           `json:"error,omitEmpty"`
	Candidates []*TaskCandidate `json:"candidates"`
	FreeBots   []*types.Machine `json:"freeBots"`
}

// writeMainLoopDiagnosticsToGCS writes JSON containing allCandidates and
// freeBots to GCS. If called in a goroutine, the arguments may not be modified.
func writeMainLoopDiagnosticsToGCS(ctx context.Context, start time.Time, end time.Time, diagClient gcs.GCSClient, diagInstance string, allCandidates map[types.TaskKey]*TaskCandidate, freeBots []*types.Machine, scheduleErr error) error {
	ctx, span := trace.StartSpan(ctx, "writeMainLoopDiagnosticsToGCS")
	defer span.End()
	defer metrics2.FuncTimer().Stop()
	candidateSlice := make([]*TaskCandidate, 0, len(allCandidates))
	for _, c := range allCandidates {
		candidateSlice = append(candidateSlice, c)
	}
	sort.Sort(taskCandidateSlice(candidateSlice))
	content := taskSchedulerMainLoopDiagnostics{
		StartTime:  start.UTC(),
		EndTime:    end.UTC(),
		Candidates: candidateSlice,
		FreeBots:   freeBots,
	}
	if scheduleErr != nil {
		content.Error = scheduleErr.Error()
	}
	filenameBase := start.UTC().Format("20060102T150405.000000000Z")
	path := path.Join(diagInstance, GCS_MAIN_LOOP_DIAGNOSTICS_DIR, filenameBase+".json")
	ctx, cancel := context.WithTimeout(ctx, GCS_DIAGNOSTICS_WRITE_TIMEOUT)
	defer cancel()
	return gcs.WithWriteFileGzip(diagClient, ctx, path, func(w io.Writer) error {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(&content)
	})
}

// findTaskCandidatesForJobs returns the set of all taskCandidates needed by all
// currently-unfinished jobs.
func (s *TaskScheduler) findTaskCandidatesForJobs(ctx context.Context, unfinishedJobs []*types.Job) (map[types.TaskKey]*TaskCandidate, error) {
	ctx, span := trace.StartSpan(ctx, "findTaskCandidatesForJobs")
	defer span.End()
	defer metrics2.FuncTimer().Stop()

	// Get the repo+commit+taskspecs for each job.
	candidates := map[types.TaskKey]*TaskCandidate{}
	for _, j := range unfinishedJobs {
		if !s.window.TestTime(j.Repo, j.Created) {
			continue
		}

		// If git history was changed, we should avoid running jobs at
		// orphaned commits.
		if s.repos[j.Repo].Get(j.Revision) == nil {
			// TODO(borenet): Cancel the job.
			continue
		}

		// Add task candidates for this job.
		for tsName := range j.Dependencies {
			key := j.MakeTaskKey(tsName)
			c, ok := candidates[key]
			if !ok {
				taskCfg, cachedErr, err := s.taskCfgCache.Get(ctx, j.RepoState)
				if cachedErr != nil {
					continue
				}
				if err != nil {
					return nil, skerr.Wrap(err)
				}
				spec, ok := taskCfg.Tasks[tsName]
				if !ok {
					// TODO(borenet): This should have already been caught when
					// we validated the TasksCfg before inserting the job.
					sklog.Errorf("Job %s wants task %s which is not defined in %+v", j.Name, tsName, j.RepoState)
					continue
				}
				casSpec, ok := taskCfg.CasSpecs[spec.CasSpec]
				if !ok {
					sklog.Errorf("Task %s at %+v depends on non-existent CasSpec %q; wanted by job %s", tsName, j.RepoState, spec.CasSpec, j.Id)
					continue
				}
				jobSpec, ok := taskCfg.Jobs[j.Name]
				if !ok {
					// This shouldn't happen, because we couldn't have created
					// the job if it wasn't present in the TasksCfg.
					sklog.Errorf("Unable to find JobSpec for %s at %+v", j.Name, j.RepoState)
					continue
				}
				c = &TaskCandidate{
					// NB: Because multiple Jobs may share a Task, the BuildbucketBuildId
					// could be inherited from any matching Job. Therefore, this should be
					// used for non-critical, informational purposes only.
					BuildbucketBuildId: j.BuildbucketBuildId,
					CasDigests:         []string{casSpec.Digest},
					IsCD:               jobSpec.IsCD,
					Jobs:               nil,
					TaskKey:            key,
					TaskSpec:           spec,
				}
				candidates[key] = c
			}
			c.AddJob(j)
		}
	}
	sklog.Infof("Found %d task candidates for %d unfinished jobs.", len(candidates), len(unfinishedJobs))
	return candidates, nil
}

// filterTaskCandidates reduces the set of taskCandidates to the ones we might
// actually want to run and organizes them by repo and TaskSpec name.
func (s *TaskScheduler) filterTaskCandidates(ctx context.Context, preFilterCandidates map[types.TaskKey]*TaskCandidate) (map[string]map[string][]*TaskCandidate, error) {
	ctx, span := trace.StartSpan(ctx, "filterTaskCandidates")
	defer span.End()
	defer metrics2.FuncTimer().Stop()

	candidatesBySpec := map[string]map[string][]*TaskCandidate{}
	total := 0
	skipped := map[string]int{}
	for _, c := range preFilterCandidates {
		// Reject skipped tasks.
		if rule := s.skipTasks.MatchRule(c.Name, c.Revision); rule != "" {
			skipped[rule]++
			c.GetDiagnostics().Filtering = &taskCandidateFilteringDiagnostics{SkippedByRule: rule}
			continue
		}

		// Reject tasks for too-old commits, as long as they aren't try jobs.
		if !c.IsTryJob() {
			if in, err := s.window.TestCommitHash(c.Repo, c.Revision); err != nil {
				return nil, skerr.Wrap(err)
			} else if !in {
				c.GetDiagnostics().Filtering = &taskCandidateFilteringDiagnostics{RevisionTooOld: true}
				continue
			}
		}
		// We shouldn't duplicate pending, in-progress,
		// or successfully completed tasks.
		prevTasks, err := s.tCache.GetTasksByKey(c.TaskKey)
		if err != nil {
			return nil, err
		}
		var previous *types.Task
		if len(prevTasks) > 0 {
			// Just choose the last (most recently created) previous
			// Task.
			previous = prevTasks[len(prevTasks)-1]
		}
		if previous != nil {
			if previous.Status == types.TASK_STATUS_PENDING || previous.Status == types.TASK_STATUS_RUNNING {
				c.GetDiagnostics().Filtering = &taskCandidateFilteringDiagnostics{SupersededByTask: previous.Id}
				continue
			}
			if previous.Success() {
				c.GetDiagnostics().Filtering = &taskCandidateFilteringDiagnostics{SupersededByTask: previous.Id}
				continue
			}
			// The attempt counts are only valid if the previous
			// attempt we're looking at is the last attempt for this
			// TaskSpec. Fortunately, TaskCache.GetTasksByKey sorts
			// by creation time, and we've selected the last of the
			// results.
			maxAttempts := c.TaskSpec.MaxAttempts
			if maxAttempts == 0 {
				maxAttempts = specs.DEFAULT_TASK_SPEC_MAX_ATTEMPTS
			}
			// Special case for tasks created before arbitrary
			// numbers of attempts were possible.
			previousAttempt := previous.Attempt
			if previousAttempt == 0 && previous.RetryOf != "" {
				previousAttempt = 1
			}
			if previousAttempt >= maxAttempts-1 {
				previousIds := make([]string, 0, len(prevTasks))
				for _, t := range prevTasks {
					previousIds = append(previousIds, t.Id)
				}
				c.GetDiagnostics().Filtering = &taskCandidateFilteringDiagnostics{PreviousAttempts: previousIds}
				continue
			}
			c.Attempt = previousAttempt + 1
			c.RetryOf = previous.Id
		}

		// Don't consider candidates whose dependencies are not met.
		depsMet, idsToHashes, err := c.allDepsMet(s.tCache)
		if err != nil {
			return nil, err
		}
		if !depsMet {
			// c.Filtering set in allDepsMet.
			continue
		}

		// Ensure that only CD tasks get assigned to the CD pool.
		// TODO(borenet): It'd be great to perform this check earlier in the
		// flow (ie. during gen_tasks.go), so that we have a better chance of
		// the error message reaching the user directly, rather than relying on
		// error-rate alerts. Unfortunately, I'm not sure how to do that unless
		// we make the CD pool name constant, eg. "SkiaCD".
		if s.cdPool != "" {
			cdPoolDimension := fmt.Sprintf("pool:%s", s.cdPool)
			if util.In(cdPoolDimension, c.TaskSpec.Dimensions) && !c.IsCD {
				// Log an error; this is a mistake in the task configuration which
				// needs to be corrected.
				sklog.Errorf("Non-CD task %s at %s is requesting use of CD pool %q; rejecting the task.", c.Name, c.Revision, s.cdPool)
				c.GetDiagnostics().Filtering = &taskCandidateFilteringDiagnostics{ForbiddenPool: s.cdPool}
				continue
			}
			if !util.In(cdPoolDimension, c.TaskSpec.Dimensions) && c.IsCD {
				// Log an error; this is a mistake in the task configuration which
				// needs to be corrected.
				// TODO(borenet): In this case, could we just automatically add the
				// correct pool, unless another is specifically requested?
				pool := "(no pool specified)"
				for _, dim := range c.TaskSpec.Dimensions {
					if strings.HasPrefix(dim, "pool:") {
						pool = strings.TrimPrefix(dim, "pool:")
						break
					}
				}

				sklog.Errorf("CD task %s at %s is not configured to use CD pool %q and instead uses %q; rejecting the task.", c.Name, c.Revision, s.cdPool, pool)
				c.GetDiagnostics().Filtering = &taskCandidateFilteringDiagnostics{ForbiddenPool: pool}
				continue
			}
		}

		// Add the CasDigests and ParentTaskIds which feed into this candidate.
		hashes := make([]string, 0, len(idsToHashes))
		parentTaskIds := make([]string, 0, len(idsToHashes))
		for id, hash := range idsToHashes {
			hashes = append(hashes, hash)
			parentTaskIds = append(parentTaskIds, id)
		}
		c.CasDigests = append(c.CasDigests, hashes...)
		sort.Strings(parentTaskIds)
		c.ParentTaskIds = parentTaskIds

		candidates, ok := candidatesBySpec[c.Repo]
		if !ok {
			candidates = map[string][]*TaskCandidate{}
			candidatesBySpec[c.Repo] = candidates
		}
		candidates[c.Name] = append(candidates[c.Name], c)
		total++
	}
	for rule, numSkipped := range skipped {
		diagLink := fmt.Sprintf("https://console.cloud.google.com/storage/browser/skia-task-scheduler-diagnostics/%s?project=google.com:skia-corp", path.Join(s.diagInstance, GCS_MAIN_LOOP_DIAGNOSTICS_DIR))
		sklog.Infof("Skipped %d candidates due to skip_tasks rule %q. See details in diagnostics at %s.", numSkipped, rule, diagLink)
	}
	sklog.Infof("Filtered to %d candidates in %d spec categories.", total, len(candidatesBySpec))
	return candidatesBySpec, nil
}

// scoreCandidate sets the Score field on the given Task Candidate. Also records
// diagnostic information on TaskCandidate.Diagnostics.Scoring.
func (s *TaskScheduler) scoreCandidate(ctx context.Context, c *TaskCandidate, cycleStart, commitTime time.Time, stealingFrom *types.Task) {
	ctx, span := trace.StartSpan(ctx, "scoreTaskCandidate")
	defer span.End()
	if len(c.Jobs) == 0 {
		// Log an error and return to allow scheduling other tasks.
		sklog.Errorf("taskCandidate has no Jobs: %#v", c)
		c.Score = 0
		return
	}

	// Record diagnostic information; this will be uploaded to GCS for forensics
	// in case we need to determine why a candidate was or was not triggered.
	diag := &taskCandidateScoringDiagnostics{}
	c.GetDiagnostics().Scoring = diag

	// Formula for priority is 1 - (1-<job1 priority>)(1-<job2 priority>)...(1-<jobN priority>).
	inversePriorityProduct := 1.0
	for _, j := range c.Jobs {
		jobPriority := specs.DEFAULT_JOB_SPEC_PRIORITY
		if j.Priority <= 1 && j.Priority > 0 {
			jobPriority = j.Priority
		}
		inversePriorityProduct *= 1 - jobPriority
	}
	priority := 1 - inversePriorityProduct
	diag.Priority = priority

	// Use the earliest Job's Created time, which will maximize priority for older forced/try jobs.
	earliestJob := c.Jobs[0]
	diag.JobCreatedHours = cycleStart.Sub(earliestJob.Created).Hours()

	if c.IsTryJob() {
		c.Score = CANDIDATE_SCORE_TRY_JOB + cycleStart.Sub(earliestJob.Created).Hours()
		// Prioritize each subsequent attempt lower than the previous attempt.
		for i := 0; i < c.Attempt; i++ {
			c.Score *= CANDIDATE_SCORE_TRY_JOB_RETRY_MULTIPLIER
		}
		c.Score *= priority
		return
	}

	if c.IsForceRun() {
		c.Score = CANDIDATE_SCORE_FORCE_RUN + cycleStart.Sub(earliestJob.Created).Hours()
		c.Score *= priority
		return
	}

	// Score the candidate.
	// The score for a candidate is based on the "testedness" increase
	// provided by running the task.
	stoleFromCommits := 0
	stoleFromStatus := types.TASK_STATUS_SUCCESS
	if stealingFrom != nil {
		stoleFromCommits = len(stealingFrom.Commits)
		stoleFromStatus = stealingFrom.Status
	}
	diag.StoleFromCommits = stoleFromCommits
	score := testednessIncrease(len(c.Commits), stoleFromCommits)
	diag.TestednessIncrease = score

	// Add a bonus when retrying or backfilling failures and mishaps.
	if stoleFromStatus == types.TASK_STATUS_FAILURE || stoleFromStatus == types.TASK_STATUS_MISHAP {
		score += CANDIDATE_SCORE_FAILURE_OR_MISHAP_BONUS
	}

	// Scale the score by other factors, eg. time decay.
	decay := s.timeDecayForCommit(cycleStart, commitTime)
	diag.TimeDecay = decay
	score *= decay
	score *= priority

	c.Score = score
}

// Process task candidates within a single task spec.
func (s *TaskScheduler) processTaskCandidatesSingleTaskSpec(ctx context.Context, currentTime time.Time, repoUrl, name string, candidatesWithTryJobs []*TaskCandidate) ([]*TaskCandidate, error) {
	ctx, span := trace.StartSpan(ctx, "processTaskCandidatesSingleTaskSpec")
	defer span.End()
	defer metrics2.FuncTimer().Stop()

	// commitsBuf is used in blamelist computation to prevent needing repeated
	// allocation of large blocks of commits.
	commitsBuf := make([]*repograph.Commit, 0, MAX_BLAMELIST_COMMITS)
	repo := s.repos[repoUrl]
	finished := make([]*TaskCandidate, 0, len(candidatesWithTryJobs))

	// 1. Handle try jobs. Tasks for try jobs don't have blamelists,
	//    so we can go ahead and score them and remove them from
	//    consideration below.
	candidates := make([]*TaskCandidate, 0, len(candidatesWithTryJobs))
	for _, candidate := range candidatesWithTryJobs {
		if candidate.IsTryJob() {
			// We already reject tryjobs for CD jobs elsewhere, so this
			// shouldn't be necessary, but there's no harm in being extra
			// careful.  Just throw away any try job candidates for CD jobs.
			if !candidate.IsCD {
				s.scoreCandidate(ctx, candidate, currentTime, repo.Get(candidate.Revision).Timestamp, nil)
				finished = append(finished, candidate)
			}
		} else {
			candidates = append(candidates, candidate)
		}
	}

	// 2. Compute blamelists and scores for all other candidates.
	//    The scores are just the initial scores, in the absence of
	//    all the other candidates.  In reality, the candidates are
	//    not independent, since their blamelists will interact, so
	//    as we repeatedly choose the most important candidate, we
	//    need to update the others' blamelists and scores.
	stealingFromTasks := map[string]*types.Task{}
	for _, candidate := range candidates {
		revision := repo.Get(candidate.Revision)
		commits, stealingFrom, err := ComputeBlamelist(ctx, s.tCache, repo, name, repoUrl, revision, commitsBuf, s.taskCfgCache, s.window)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		candidate.Commits = commits
		if stealingFrom != nil {
			candidate.StealingFromId = stealingFrom.Id
			stealingFromTasks[stealingFrom.Id] = stealingFrom
		}
		s.scoreCandidate(ctx, candidate, currentTime, revision.Timestamp, stealingFrom)
	}

	// 3. Throw away all backfill candidates for CD jobs.
	// If one candidate for this TaskSpec is a CD candidate, they all are.
	if len(candidates) > 0 && candidates[0].IsCD {
		// Find the candidate with the largest blamelist; that will be the
		// newest commit.
		var best *TaskCandidate
		for _, candidate := range candidates {
			// If a candidate steals commits, it's a backfill; only consider
			// those which don't steal commits.
			if candidate.StealingFromId == "" {
				if best == nil || len(candidate.Commits) > len(best.Commits) {
					best = candidate
				}
			}
		}
		if best != nil {
			candidates = []*TaskCandidate{best}
		} else {
			candidates = []*TaskCandidate{}
		}
	}

	// 4. Repeat until all candidates have been ranked:
	for len(candidates) > 0 {
		// a. Choose the highest-scoring candidate and add to the queue.
		sort.Sort(taskCandidateSlice(candidates))
		bestOrig := candidates[0]
		finished = append(finished, bestOrig)
		candidates = candidates[1:]
		// Copy, since we might mangle the blamelist later.
		best := bestOrig.CopyNoDiagnostics()
		bestId := best.MakeId()
		bestFakeTask := best.MakeTask()
		stealingFromTasks[bestId] = bestFakeTask
		// Update the blamelist of the task we stole commits from if applicable.
		if best.StealingFromId != "" {
			stealingFrom := stealingFromTasks[best.StealingFromId]
			stealingFrom.Commits = util.NewStringSet(stealingFrom.Commits).Complement(util.NewStringSet(best.Commits)).Keys()
		}

		// b. Update candidates which were affected by the choice of the best
		//    candidate; their blamelist, StealingFrom field, and score may need
		//    to be updated.
		for _, candidate := range candidates {
			// TODO(borenet): This is still O(n^2), we should be able to get
			// down to O(n lg n) with a blamelist map.
			updated := false
			// b1. This candidate has the best candidate's revision in its
			//     blamelist.
			if util.In(best.Revision, candidate.Commits) {
				// Only transfer commits if the new candidate runs at a
				// different revision from the best candidate. If they run at
				// the same revision, this new candidate is a retry of the best
				// candidate and therefore has the same blamelist.
				if best.Revision != candidate.Revision {
					// candidate.StealingFrom doesn't change, but the best
					// candidate effectively steals commits from this candidate.
					candidate.Commits = util.NewStringSet(candidate.Commits).Complement(util.NewStringSet(best.Commits)).Keys()
					updated = true
				}
			}
			// b2. The best candidate has this candidate's revision in its
			//     blamelist.
			if util.In(candidate.Revision, best.Commits) {
				// This candidate is now stealing commits from the best
				// candidate, but its blamelist doesn't change.
				candidate.StealingFromId = bestId
				updated = true
			}
			if updated {
				s.scoreCandidate(ctx, candidate, currentTime, repo.Get(candidate.Revision).Timestamp, stealingFromTasks[candidate.StealingFromId])
			}
		}
	}
	return finished, nil
}

// Process the task candidates.
func (s *TaskScheduler) processTaskCandidates(ctx context.Context, candidates map[string]map[string][]*TaskCandidate) ([]*TaskCandidate, error) {
	ctx, span := trace.StartSpan(ctx, "processTaskCandidates")
	defer span.End()
	defer metrics2.FuncTimer().Stop()

	currentTime := now.Now(ctx)
	processed := make(chan *TaskCandidate)
	errs := make(chan error)
	wg := sync.WaitGroup{}
	for repo, cs := range candidates {
		for name, c := range cs {
			wg.Add(1)
			go func(repoUrl, name string, candidatesWithTryJobs []*TaskCandidate) {
				defer wg.Done()
				c, err := s.processTaskCandidatesSingleTaskSpec(ctx, currentTime, repoUrl, name, candidatesWithTryJobs)
				if err != nil {
					errs <- err
				} else {
					for _, candidate := range c {
						processed <- candidate
					}
				}
			}(repo, name, c)
		}
	}
	go func() {
		wg.Wait()
		close(processed)
		close(errs)
	}()
	rvCandidates := []*TaskCandidate{}
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

// flatten all the dimensions in 'dims' into a single valued map.
func flatten(dims map[string]string) map[string]string {
	keys := make([]string, 0, len(dims))
	for key := range dims {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ret := make([]string, 0, 2*len(dims))
	for _, key := range keys {
		ret = append(ret, key, dims[key])
	}
	return map[string]string{"dimensions": strings.Join(ret, " ")}
}

// recordCandidateMetrics generates metrics for candidates by dimension sets.
func (s *TaskScheduler) recordCandidateMetrics(ctx context.Context, candidates map[string]map[string][]*TaskCandidate) {
	ctx, span := trace.StartSpan(ctx, "recordCandidateMetrics")
	defer span.End()
	defer metrics2.FuncTimer().Stop()

	// Generate counts. These maps are keyed by the MD5 hash of the
	// candidate's TaskSpec's dimensions.
	counts := map[string]int64{}
	dimensions := map[string]map[string]string{}
	for _, byRepo := range candidates {
		for _, bySpec := range byRepo {
			for _, c := range bySpec {
				parseDims, err := swarming.ParseDimensions(c.TaskSpec.Dimensions)
				if err != nil {
					sklog.Errorf("Failed to parse dimensions: %s", err)
					continue
				}
				dims := make(map[string]string, len(parseDims))
				for k, v := range parseDims {
					// Just take the first value for each dimension.
					dims[k] = v[0]
				}
				k, err := util.MD5Sum(dims)
				if err != nil {
					sklog.Errorf("Failed to create metrics key: %s", err)
					continue
				}
				dimensions[k] = dims
				counts[k]++
			}
		}
	}
	// Report the data.
	s.candidateMetricsMtx.Lock()
	defer s.candidateMetricsMtx.Unlock()
	for k, count := range counts {
		metric, ok := s.candidateMetrics[k]
		if !ok {
			metric = metrics2.GetInt64Metric(MEASUREMENT_TASK_CANDIDATE_COUNT, flatten(dimensions[k]))
			s.candidateMetrics[k] = metric
		}
		metric.Update(count)
	}
	for k, metric := range s.candidateMetrics {
		_, ok := counts[k]
		if !ok {
			metric.Update(0)
			delete(s.candidateMetrics, k)
		}
	}
}

// regenerateTaskQueue obtains the set of all eligible task candidates, scores
// them, and prepares them to be triggered. The second return value contains
// all candidates.
func (s *TaskScheduler) regenerateTaskQueue(ctx context.Context) ([]*TaskCandidate, map[types.TaskKey]*TaskCandidate, error) {
	ctx, span := trace.StartSpan(ctx, "regenerateTaskQueue")
	defer span.End()
	defer metrics2.FuncTimer().Stop()

	// Find the unfinished Jobs.
	unfinishedJobs, err := s.jCache.UnfinishedJobs()
	if err != nil {
		return nil, nil, err
	}

	// Find TaskSpecs for all unfinished Jobs.
	preFilterCandidates, err := s.findTaskCandidatesForJobs(ctx, unfinishedJobs)
	if err != nil {
		return nil, nil, err
	}

	// Filter task candidates.
	candidates, err := s.filterTaskCandidates(ctx, preFilterCandidates)
	if err != nil {
		return nil, nil, err
	}

	// Record the number of task candidates per dimension set.
	s.recordCandidateMetrics(ctx, candidates)

	// Process the remaining task candidates.
	queue, err := s.processTaskCandidates(ctx, candidates)
	if err != nil {
		return nil, nil, err
	}

	return queue, preFilterCandidates, nil
}

// getCandidatesToSchedule matches the list of free Swarming bots to task
// candidates in the queue and returns the candidates which should be run.
// Assumes that the tasks are sorted in decreasing order by score.
func getCandidatesToSchedule(ctx context.Context, bots []*types.Machine, tasks []*TaskCandidate) []*TaskCandidate {
	ctx, span := trace.StartSpan(ctx, "getCandidatesToSchedule")
	defer span.End()
	defer metrics2.FuncTimer().Stop()
	// Create a bots-by-swarming-dimension mapping.
	botsByDim := map[string]util.StringSet{}
	for _, b := range bots {
		for _, dim := range b.Dimensions {
			if _, ok := botsByDim[dim]; !ok {
				botsByDim[dim] = util.StringSet{}
			}
			botsByDim[dim][b.ID] = true
		}
	}
	// BotIds that have been used by previous candidates.
	usedBots := util.StringSet{}
	// Map BotId to the candidates that could have used that bot. In the
	// case that no bots are available for a candidate, map concatenated
	// dimensions to candidates.
	botToCandidates := map[string][]*TaskCandidate{}

	// Match bots to tasks.
	// TODO(borenet): Some tasks require a more specialized bot. We should
	// match so that less-specialized tasks don't "steal" more-specialized
	// bots which they don't actually need.
	rv := make([]*TaskCandidate, 0, len(bots))
	countByTaskSpec := make(map[string]int, len(bots))
	for _, c := range tasks {
		diag := &taskCandidateSchedulingDiagnostics{}
		c.GetDiagnostics().Scheduling = diag
		// Don't exceed SCHEDULING_LIMIT_PER_TASK_SPEC.
		if countByTaskSpec[c.Name] == SCHEDULING_LIMIT_PER_TASK_SPEC {
			sklog.Warningf("Too many tasks to schedule for %s; not scheduling more than %d", c.Name, SCHEDULING_LIMIT_PER_TASK_SPEC)
			diag.OverSchedulingLimitPerTaskSpec = true
			continue
		}
		// TODO(borenet): Make this threshold configurable.
		if c.Score <= 0.0 {
			// This normally shouldn't happen, but it can happen if there is both a
			// forced task and an unused retry for the same repo state.
			sklog.Debugf("candidate %s @ %s has a score of %2f; skipping (%d commits). %+v", c.Name, c.Revision, c.Score, len(c.Commits), c.Diagnostics.Scoring)
			diag.ScoreBelowThreshold = true
			continue
		}

		// For each dimension of the task, find the set of bots which matches.
		matches := util.StringSet{}
		for i, d := range c.TaskSpec.Dimensions {
			if i == 0 {
				matches = matches.Union(botsByDim[d])
			} else {
				matches = matches.Intersect(botsByDim[d])
			}
		}

		// Set of candidates that could have used the same bots.
		similarCandidates := map[*TaskCandidate]struct{}{}
		var lowestScoreSimilarCandidate *TaskCandidate
		addCandidates := func(key string) {
			candidates := botToCandidates[key]
			for _, candidate := range candidates {
				similarCandidates[candidate] = struct{}{}
			}
			if len(candidates) > 0 {
				lastCandidate := candidates[len(candidates)-1]
				if lowestScoreSimilarCandidate == nil || lowestScoreSimilarCandidate.Score > lastCandidate.Score {
					lowestScoreSimilarCandidate = lastCandidate
				}
			}
			botToCandidates[key] = append(candidates, c)
		}

		// Choose a particular bot to mark as used. Sort by ID so that the choice is deterministic.
		var chosenBot string
		if len(matches) > 0 {
			diag.MatchingBots = matches.Keys()
			sort.Strings(diag.MatchingBots)
			for botId := range matches {
				if (chosenBot == "" || botId < chosenBot) && !usedBots[botId] {
					chosenBot = botId
				}
				addCandidates(botId)
			}
		} else {
			diag.NoBotsAvailable = true
			diag.MatchingBots = nil
			// Use sorted concatenated dimensions instead of botId as the key.
			dims := util.CopyStringSlice(c.TaskSpec.Dimensions)
			sort.Strings(dims)
			addCandidates(strings.Join(dims, ","))
		}
		diag.NumHigherScoreSimilarCandidates = len(similarCandidates)
		if lowestScoreSimilarCandidate != nil {
			diag.LastSimilarCandidate = &lowestScoreSimilarCandidate.TaskKey
		}

		if chosenBot != "" {
			// We're going to run this task.
			diag.Selected = true
			usedBots[chosenBot] = true

			// Add the task to the scheduling list.
			rv = append(rv, c)
			countByTaskSpec[c.Name]++
		}
	}
	sort.Sort(taskCandidateSlice(rv))
	return rv
}

// mergeCASInputs uploads inputs for the taskCandidates to content-addressed
// storage. Returns the list of candidates which were successfully merged, with
// their CasInput set, and any error which occurred. Note that the successful
// candidates AND an error may both be returned if some were successfully merged
// but others failed.
func (s *TaskScheduler) mergeCASInputs(ctx context.Context, candidates []*TaskCandidate) ([]*TaskCandidate, error) {
	ctx, span := trace.StartSpan(ctx, "mergeCASInputs")
	defer span.End()
	defer metrics2.FuncTimer().Stop()

	mergedCandidates := make([]*TaskCandidate, 0, len(candidates))
	var errs *multierror.Error
	for _, c := range candidates {
		digest, err := s.rbeCas.Merge(ctx, c.CasDigests)
		if err != nil {
			errStr := err.Error()
			c.GetDiagnostics().Triggering = &taskCandidateTriggeringDiagnostics{IsolateError: errStr}
			errs = multierror.Append(errs, fmt.Errorf("Failed to merge CAS inputs: %s", errStr))
			continue
		}
		c.CasInput = digest
		mergedCandidates = append(mergedCandidates, c)
	}

	return mergedCandidates, errs.ErrorOrNil()
}

// triggerTasks triggers the given slice of tasks to run on Swarming and returns
// a channel of the successfully-triggered tasks which is closed after all tasks
// have been triggered or failed. Each failure is sent to errCh.
func (s *TaskScheduler) triggerTasks(ctx context.Context, candidates []*TaskCandidate, errCh chan<- error) <-chan *types.Task {
	ctx, span := trace.StartSpan(ctx, "triggerTasks")
	defer span.End()
	defer metrics2.FuncTimer().Stop()
	triggered := make(chan *types.Task)
	var wg sync.WaitGroup
	for _, candidate := range candidates {
		wg.Add(1)
		go func(candidate *TaskCandidate) {
			defer wg.Done()
			t := candidate.MakeTask()
			diag := &taskCandidateTriggeringDiagnostics{}
			candidate.GetDiagnostics().Triggering = diag
			recordErr := func(context string, err error) {
				err = fmt.Errorf("%s: %s", context, err)
				diag.TriggerError = err.Error()
				errCh <- err
			}
			if err := s.db.AssignId(ctx, t); err != nil {
				recordErr("Failed to assign id", err)
				return
			}
			diag.TaskId = t.Id
			req, err := candidate.MakeTaskRequest(t.Id, s.rbeCasInstance, s.pubsubTopic)
			if err != nil {
				recordErr("Failed to create task request", err)
				return
			}
			s.pendingInsertMtx.Lock()
			s.pendingInsert[t.Id] = true
			s.pendingInsertMtx.Unlock()
			resp, err := s.taskExecutor.TriggerTask(ctx, req)
			if err != nil {
				s.pendingInsertMtx.Lock()
				delete(s.pendingInsert, t.Id)
				s.pendingInsertMtx.Unlock()
				jobIds := make([]string, 0, len(candidate.Jobs))
				for _, job := range candidate.Jobs {
					jobIds = append(jobIds, job.Id)
				}
				recordErr("Failed to trigger task", skerr.Wrapf(err, "%q needed for jobs: %+v", candidate.Name, jobIds))
				return
			}
			t.Created = resp.Created
			t.Started = resp.Started
			t.Finished = resp.Finished
			t.SwarmingTaskId = resp.ID
			// The task may have been de-duplicated.
			if resp.Status == types.TASK_STATUS_SUCCESS {
				if _, err := t.UpdateFromTaskResult(resp); err != nil {
					recordErr("Failed to update de-duplicated task", err)
					return
				}
			}
			triggered <- t
		}(candidate)
	}
	go func() {
		wg.Wait()
		close(triggered)
	}()
	return triggered
}

// scheduleTasks queries for free Swarming bots and triggers tasks according
// to relative priorities in the queue.
func (s *TaskScheduler) scheduleTasks(ctx context.Context, bots []*types.Machine, queue []*TaskCandidate) error {
	ctx, span := trace.StartSpan(ctx, "scheduleTasks")
	defer span.End()
	defer metrics2.FuncTimer().Stop()
	// Match free bots with tasks.
	candidates := getCandidatesToSchedule(ctx, bots, queue)

	// Merge CAS inputs for the tasks.
	merged, mergeErr := s.mergeCASInputs(ctx, candidates)
	if mergeErr != nil && len(merged) == 0 {
		return mergeErr
	}

	// Setup the error channel.
	errs := []error{}
	if mergeErr != nil {
		errs = append(errs, mergeErr)
	}
	errCh := make(chan error)
	var errWg sync.WaitGroup
	errWg.Add(1)
	go func() {
		defer errWg.Done()
		for err := range errCh {
			errs = append(errs, err)
		}
	}()

	// Trigger Swarming tasks.
	triggered := s.triggerTasks(ctx, merged, errCh)

	// Collect the tasks we triggered.
	numTriggered := 0
	insert := map[string]map[string][]*types.Task{}
	for t := range triggered {
		byRepo, ok := insert[t.Repo]
		if !ok {
			byRepo = map[string][]*types.Task{}
			insert[t.Repo] = byRepo
		}
		byRepo[t.Name] = append(byRepo[t.Name], t)
		numTriggered++
	}
	close(errCh)
	errWg.Wait()

	if len(insert) > 0 {
		// Insert the newly-triggered tasks into the DB.
		err := s.addTasks(ctx, insert)

		// Remove the tasks from the pending map, regardless of whether
		// we successfully inserted into the DB.
		s.pendingInsertMtx.Lock()
		for _, byRepo := range insert {
			for _, byName := range byRepo {
				for _, t := range byName {
					delete(s.pendingInsert, t.Id)
				}
			}
		}
		s.pendingInsertMtx.Unlock()

		// Handle failure/success.
		if err != nil {
			errs = append(errs, fmt.Errorf("Triggered tasks but failed to insert into DB: %s", err))
		} else {
			// Organize the triggered task by TaskKey.
			remove := make(map[types.TaskKey]*types.Task, numTriggered)
			for _, byRepo := range insert {
				for _, byName := range byRepo {
					for _, t := range byName {
						remove[t.TaskKey] = t
					}
				}
			}
			if len(remove) != numTriggered {
				return fmt.Errorf("Number of tasks to remove from the queue (%d) differs from the number of tasks triggered (%d)", len(remove), numTriggered)
			}

			// Remove the tasks from the queue.
			newQueue := make([]*TaskCandidate, 0, len(queue)-numTriggered)
			for _, c := range queue {
				if _, ok := remove[c.TaskKey]; !ok {
					newQueue = append(newQueue, c)
				}
			}

			// Note; if regenerateQueue and scheduleTasks are ever decoupled so that
			// the queue is reused by multiple runs of scheduleTasks, we'll need to
			// address the fact that some candidates may still have their
			// StoleFromId pointing to candidates which have been triggered and
			// removed from the queue. In that case, we should just need to write a
			// loop which updates those candidates to use the IDs of the newly-
			// inserted Tasks in the database rather than the candidate ID.

			sklog.Infof("Triggered %d tasks on %d bots (%d in queue).", numTriggered, len(bots), len(queue))
			queue = newQueue
		}
	} else {
		sklog.Infof("Triggered no tasks (%d in queue, %d bots available)", len(queue), len(bots))
	}
	s.triggeredCount.Inc(int64(numTriggered))
	s.queueMtx.Lock()
	defer s.queueMtx.Unlock()
	s.queue = queue
	s.lastScheduled = now.Now(ctx)

	if len(errs) > 0 {
		rvErr := "Got failures: "
		for _, e := range errs {
			rvErr += fmt.Sprintf("\n%s\n", e)
		}
		return fmt.Errorf(rvErr)
	}
	return nil
}

// MainLoop runs a single end-to-end task scheduling loop.
func (s *TaskScheduler) MainLoop(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "taskscheduler_MainLoop")
	defer span.End()
	defer metrics2.FuncTimer().Stop()

	sklog.Infof("Task Scheduler MainLoop starting...")
	diagStart := now.Now(ctx)

	var wg sync.WaitGroup

	var bots []*types.Machine
	var getSwarmingBotsErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		bots, getSwarmingBotsErr = getFreeMachines(ctx, s.taskExecutor, s.busyBots, s.pools)
	}()

	if err := s.tCache.Update(ctx); err != nil {
		return fmt.Errorf("Failed to update task cache: %s", err)
	}

	if err := s.jCache.Update(ctx); err != nil {
		return fmt.Errorf("Failed to update job cache: %s", err)
	}

	if err := s.updateUnfinishedJobs(ctx); err != nil {
		return fmt.Errorf("Failed to update unfinished jobs: %s", err)
	}

	if err := s.skipTasks.Update(ctx); err != nil {
		return fmt.Errorf("Failed to update skip_tasks: %s", err)
	}

	// Regenerate the queue.
	sklog.Infof("Task Scheduler regenerating the queue...")
	queue, allCandidates, err := s.regenerateTaskQueue(ctx)
	if err != nil {
		return fmt.Errorf("Failed to regenerate task queue: %s", err)
	}

	wg.Wait()
	if getSwarmingBotsErr != nil {
		return fmt.Errorf("Failed to retrieve free Swarming bots: %s", getSwarmingBotsErr)
	}

	sklog.Infof("Task Scheduler scheduling tasks...")
	err = s.scheduleTasks(ctx, bots, queue)

	// An error from scheduleTasks can indicate a partial error; write diagnostics
	// in either case.
	if s.diagClient != nil {
		diagEnd := now.Now(ctx)
		s.testWaitGroup.Add(1)
		go func() {
			defer s.testWaitGroup.Done()
			util.LogErr(writeMainLoopDiagnosticsToGCS(ctx, diagStart, diagEnd, s.diagClient, s.diagInstance, allCandidates, bots, err))
		}()
	}

	if err != nil {
		return fmt.Errorf("Failed to schedule tasks: %s", err)
	}

	sklog.Infof("Task Scheduler MainLoop finished.")
	return nil
}

// QueueLen returns the length of the queue.
func (s *TaskScheduler) QueueLen() int {
	s.queueMtx.RLock()
	defer s.queueMtx.RUnlock()
	return len(s.queue)
}

// CloneQueue returns a full copy of the queue.
func (s *TaskScheduler) CloneQueue() []*TaskCandidate {
	s.queueMtx.RLock()
	defer s.queueMtx.RUnlock()
	rv := make([]*TaskCandidate, 0, len(s.queue))
	for _, c := range s.queue {
		rv = append(rv, c.CopyNoDiagnostics())
	}
	return rv
}

// timeDecay24Hr computes a linear time decay amount for the given duration,
// given the requested decay amount at 24 hours.
func timeDecay24Hr(decayAmt24Hr float64, elapsed time.Duration) float64 {
	return math.Max(1.0-(1.0-decayAmt24Hr)*(float64(elapsed)/float64(24*time.Hour)), 0.0)
}

// timeDecayForCommit computes a multiplier for a task candidate score based
// on how long ago the given commit landed. This allows us to prioritize more
// recent commits.
func (s *TaskScheduler) timeDecayForCommit(currentTime, commitTime time.Time) float64 {
	if s.timeDecayAmt24Hr == 1.0 {
		// Shortcut for special case.
		return 1.0
	}
	rv := timeDecay24Hr(s.timeDecayAmt24Hr, commitTime.Sub(commitTime))
	// TODO(benjaminwagner): Change to an exponential decay to prevent
	// zero/negative scores.
	//if rv == 0.0 {
	//	sklog.Warningf("timeDecayForCommit is zero. Now: %s, Commit: %s ts %s, TimeDecay: %2f\nDetails: %v", now, commit.Hash, commit.Timestamp, s.timeDecayAmt24Hr, commit)
	//}
	return rv
}

func (s *TaskScheduler) GetSkipTasks() *skip_tasks.DB {
	return s.skipTasks
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
		sklog.Errorf("Task score function got a blamelist with %d commits", n)
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

// getFreeMachines returns a slice of free machines.
func getFreeMachines(ctx context.Context, taskExec types.TaskExecutor, busy *busyBots, pools []string) ([]*types.Machine, error) {
	ctx, span := trace.StartSpan(ctx, "getFreeMachines")
	defer span.End()
	defer metrics2.FuncTimer().Stop()

	// Query for free machines and pending tasks in all pools.
	var wg sync.WaitGroup
	machines := []*types.Machine{}
	pending := []*types.TaskResult{}
	errs := []error{}
	var mtx sync.Mutex
	for _, pool := range pools {
		// Free bots.
		wg.Add(1)
		go func(pool string) {
			defer wg.Done()
			b, err := taskExec.GetFreeMachines(ctx, pool)
			mtx.Lock()
			defer mtx.Unlock()
			if err != nil {
				errs = append(errs, err)
			} else {
				machines = append(machines, b...)
			}
		}(pool)

		// Pending tasks.
		wg.Add(1)
		go func(pool string) {
			defer wg.Done()
			pendingTasks, err := taskExec.GetPendingTasks(ctx, pool)
			mtx.Lock()
			defer mtx.Unlock()
			if err != nil {
				errs = append(errs, err)
			} else {
				pending = append(pending, pendingTasks...)
			}
		}(pool)
	}

	wg.Wait()
	if len(errs) > 0 {
		return nil, fmt.Errorf("Got errors loading bots and tasks from Swarming: %v", errs)
	}

	rv := make([]*types.Machine, 0, len(machines))
	for _, machine := range machines {
		if machine.IsDead {
			continue
		}
		if machine.IsQuarantined {
			continue
		}
		if machine.CurrentTaskID != "" {
			continue
		}
		rv = append(rv, machine)
	}
	busy.RefreshTasks(pending)
	return busy.Filter(rv), nil
}

// updateUnfinishedTasks queries Swarming for all unfinished tasks and updates
// their status in the DB.
func (s *TaskScheduler) updateUnfinishedTasks(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "updateUnfinishedTasks")
	defer span.End()
	defer metrics2.FuncTimer().Stop()
	// Update the TaskCache.
	if err := s.tCache.Update(ctx); err != nil {
		return err
	}

	tasks, err := s.tCache.UnfinishedTasks()
	if err != nil {
		return err
	}
	sort.Sort(types.TaskSlice(tasks))

	// Query Swarming for all unfinished tasks.
	sklog.Infof("Querying states of %d unfinished tasks.", len(tasks))
	ids := make([]string, 0, len(tasks))
	for _, t := range tasks {
		ids = append(ids, t.SwarmingTaskId)
	}
	finishedStates, err := s.taskExecutor.GetTaskCompletionStatuses(ctx, ids)
	if err != nil {
		return err
	}
	finished := make([]*types.Task, 0, len(finishedStates))
	for idx, task := range tasks {
		if finishedStates[idx] {
			finished = append(finished, task)
		}
	}

	// Update any newly-finished tasks.
	if len(finished) > 0 {
		sklog.Infof("Updating %d newly-finished tasks.", len(finished))
		var wg sync.WaitGroup
		errs := make([]error, len(tasks))
		for i, t := range finished {
			wg.Add(1)
			go func(idx int, t *types.Task) {
				defer wg.Done()
				taskResult, err := s.taskExecutor.GetTaskResult(ctx, t.SwarmingTaskId)
				if err != nil {
					errs[idx] = fmt.Errorf("Failed to update unfinished task; failed to get updated task from swarming: %s", err)
					return
				}
				modified, err := db.UpdateDBFromTaskResult(ctx, s.db, taskResult)
				if err != nil {
					errs[idx] = fmt.Errorf("Failed to update unfinished task: %s", err)
					return
				} else if modified {
					s.updateUnfinishedCount.Inc(1)
				}
			}(i, t)
		}
		wg.Wait()
		for _, err := range errs {
			if err != nil {
				return err
			}
		}
	}

	return s.tCache.Update(ctx)
}

// updateUnfinishedJobs updates all not-yet-finished Jobs to determine if their
// state has changed.
func (s *TaskScheduler) updateUnfinishedJobs(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "updateUnfinishedJobs")
	defer span.End()
	defer metrics2.FuncTimer().Stop()
	jobs, err := s.jCache.UnfinishedJobs()
	if err != nil {
		return err
	}

	modifiedJobs := make([]*types.Job, 0, len(jobs))
	modifiedTasks := make(map[string]*types.Task, len(jobs))
	for _, j := range jobs {
		tasks, err := s.getTasksForJob(j)
		if err != nil {
			return err
		}
		summaries := make(map[string][]*types.TaskSummary, len(tasks))
		for k, v := range tasks {
			cpy := make([]*types.TaskSummary, 0, len(v))
			for _, t := range v {
				if existing := modifiedTasks[t.Id]; existing != nil {
					t = existing
				}
				cpy = append(cpy, t.MakeTaskSummary())
				// The Jobs list is always sorted.
				oldLen := len(t.Jobs)
				t.Jobs = util.InsertStringSorted(t.Jobs, j.Id)
				if len(t.Jobs) > oldLen {
					modifiedTasks[t.Id] = t
				}
			}
			summaries[k] = cpy
		}
		if !reflect.DeepEqual(summaries, j.Tasks) {
			j.Tasks = summaries
			j.Status = j.DeriveStatus()
			if j.Done() {
				j.Finished = now.Now(ctx)
			}
			modifiedJobs = append(modifiedJobs, j)
		}
	}
	if len(modifiedTasks) > 0 {
		tasks := make([]*types.Task, 0, len(modifiedTasks))
		for _, t := range modifiedTasks {
			tasks = append(tasks, t)
		}
		if err := s.putTasksInChunks(ctx, tasks); err != nil {
			return err
		}
	}
	if len(modifiedJobs) > 0 {
		if err := s.putJobsInChunks(ctx, modifiedJobs); err != nil {
			return err
		}
	}
	return nil
}

// getTasksForJob finds all Tasks for the given Job. It returns the Tasks
// in a map keyed by name.
func (s *TaskScheduler) getTasksForJob(j *types.Job) (map[string][]*types.Task, error) {
	tasks := map[string][]*types.Task{}
	for d := range j.Dependencies {
		key := j.MakeTaskKey(d)
		gotTasks, err := s.tCache.GetTasksByKey(key)
		if err != nil {
			return nil, err
		}
		tasks[d] = gotTasks
	}
	return tasks, nil
}

// addTasksSingleTaskSpec computes the blamelist for each task in tasks, all of
// which must have the same Repo and Name fields, and inserts/updates them in
// the TaskDB. Also adjusts blamelists of existing tasks.
func (s *TaskScheduler) addTasksSingleTaskSpec(ctx context.Context, tasks []*types.Task) error {
	sort.Sort(types.TaskSlice(tasks))
	// TODO(borenet): This is the only user of cacheWrapper; can we remove it?
	cache := newCacheWrapper(s.tCache)
	repoName := tasks[0].Repo
	taskName := tasks[0].Name
	repo, ok := s.repos[repoName]
	if !ok {
		return fmt.Errorf("No such repo: %s", repoName)
	}

	commitsBuf := make([]*repograph.Commit, 0, MAX_BLAMELIST_COMMITS)
	updatedTasks := map[string]*types.Task{}
	for _, task := range tasks {
		if task.Repo != repoName || task.Name != taskName {
			return fmt.Errorf("Mismatched Repo or Name: %v", tasks)
		}
		if task.Id == "" {
			if err := s.db.AssignId(ctx, task); err != nil {
				return err
			}
		}
		if task.IsTryJob() {
			updatedTasks[task.Id] = task
			continue
		}
		// Compute blamelist.
		revision := repo.Get(task.Revision)
		if revision == nil {
			return fmt.Errorf("No such commit %s in %s.", task.Revision, task.Repo)
		}
		if !s.window.TestTime(task.Repo, revision.Timestamp) {
			return fmt.Errorf("Can not add task %s with revision %s (at %s) before window start.", task.Id, task.Revision, revision.Timestamp)
		}
		commits, stealingFrom, err := ComputeBlamelist(ctx, cache, repo, task.Name, task.Repo, revision, commitsBuf, s.taskCfgCache, s.window)
		if err != nil {
			return err
		}
		task.Commits = commits
		if len(task.Commits) > 0 && !util.In(task.Revision, task.Commits) {
			sklog.Errorf("task %s (%s @ %s) doesn't have its own revision in its blamelist: %v", task.Id, task.Name, task.Revision, task.Commits)
		}
		updatedTasks[task.Id] = task
		cache.insert(task)
		if stealingFrom != nil && stealingFrom.Id != task.Id {
			stole := util.NewStringSet(commits)
			oldC := util.NewStringSet(stealingFrom.Commits)
			newC := oldC.Complement(stole)
			if len(newC) == 0 {
				stealingFrom.Commits = nil
			} else {
				newCommits := make([]string, 0, len(newC))
				for c := range newC {
					newCommits = append(newCommits, c)
				}
				stealingFrom.Commits = newCommits
			}
			updatedTasks[stealingFrom.Id] = stealingFrom
			cache.insert(stealingFrom)
		}
	}
	putTasks := make([]*types.Task, 0, len(updatedTasks))
	for _, task := range updatedTasks {
		putTasks = append(putTasks, task)
	}
	if err := s.putTasks(ctx, putTasks); err != nil {
		return err
	}
	return nil
}

// addTasks inserts the given tasks into the TaskDB, updating blamelists. The
// provided Tasks should have all fields initialized except for Commits, which
// will be overwritten, and optionally Id, which will be assigned if necessary.
// addTasks updates existing Tasks' blamelists, if needed. The provided map
// groups Tasks by repo and TaskSpec name. May return error on partial success.
// May modify Commits and Id of argument tasks on error.
func (s *TaskScheduler) addTasks(ctx context.Context, taskMap map[string]map[string][]*types.Task) error {
	ctx, span := trace.StartSpan(ctx, "addTasks")
	defer span.End()
	type queueItem struct {
		Repo string
		Name string
	}
	queue := map[queueItem]bool{}
	for repo, byName := range taskMap {
		for name, tasks := range byName {
			if len(tasks) == 0 {
				continue
			}
			queue[queueItem{
				Repo: repo,
				Name: name,
			}] = true
		}
	}

	for i := 0; i < db.NUM_RETRIES; i++ {
		if len(queue) == 0 {
			return nil
		}
		if err := s.tCache.Update(ctx); err != nil {
			return err
		}

		done := make(chan queueItem)
		errs := make(chan error, len(queue))
		wg := sync.WaitGroup{}
		for item := range queue {
			wg.Add(1)
			go func(item queueItem, tasks []*types.Task) {
				defer wg.Done()
				if err := s.addTasksSingleTaskSpec(ctx, tasks); err != nil {
					if !db.IsConcurrentUpdate(err) {
						errs <- fmt.Errorf("Error adding tasks for %s (in repo %s): %s", item.Name, item.Repo, err)
					}
				} else {
					done <- item
				}
			}(item, taskMap[item.Repo][item.Name])
		}
		go func() {
			wg.Wait()
			close(done)
			close(errs)
		}()
		for item := range done {
			delete(queue, item)
		}
		rvErrs := []error{}
		for err := range errs {
			sklog.Error(err)
			rvErrs = append(rvErrs, err)
		}
		if len(rvErrs) != 0 {
			return rvErrs[0]
		}
	}

	if len(queue) > 0 {
		return fmt.Errorf("addTasks: %d consecutive ErrConcurrentUpdate; %d of %d TaskSpecs failed. %#v", db.NUM_RETRIES, len(queue), len(taskMap), queue)
	}
	return nil
}

// HandleSwarmingPubSub loads the given Swarming task ID from Swarming and
// updates the associated types.Task in the database. Returns a bool indicating
// whether the pubsub message should be acknowledged.
func (s *TaskScheduler) HandleSwarmingPubSub(msg *swarming.PubSubTaskMessage) bool {
	ctx, span := trace.StartSpan(context.Background(), "taskscheduler_HandleSwarmingPubSub")
	defer span.End()
	s.pubsubCount.Inc(1)
	if msg.UserData == "" {
		// This message is invalid. ACK it to make it go away.
		return true
	}

	// If the task has been triggered but not yet inserted into the DB, NACK
	// the message so that we'll receive it later.
	s.pendingInsertMtx.RLock()
	isPending := s.pendingInsert[msg.UserData]
	s.pendingInsertMtx.RUnlock()
	if isPending {
		sklog.Debugf("Received pub/sub message for task which hasn't yet been inserted into the db: %s (%s); not ack'ing message; will try again later.", msg.SwarmingTaskId, msg.UserData)
		return false
	}

	// Obtain the Swarming task data.
	res, err := s.taskExecutor.GetTaskResult(ctx, msg.SwarmingTaskId)
	if err != nil {
		sklog.Errorf("pubsub: Failed to retrieve task from Swarming: %s", err)
		return true
	}
	// Skip unfinished tasks.
	if util.TimeIsZero(res.Finished) {
		return true
	}
	// Update the task in the DB.
	if _, err := db.UpdateDBFromTaskResult(ctx, s.db, res); err != nil {
		// TODO(borenet): Some of these cases should never be hit, after all tasks
		// start supplying the ID in msg.UserData. We should be able to remove the logic.
		id := "<MISSING ID TAG>"
		if err == db.ErrNotFound {
			ids, ok := res.Tags[types.SWARMING_TAG_ID]
			if ok {
				id = ids[0]
			}
			if now.Now(ctx).Sub(res.Created) < 2*time.Minute {
				sklog.Infof("Failed to update task %q: No such task ID: %q. Less than two minutes old; try again later.", msg.SwarmingTaskId, id)
				return false
			}
			sklog.Errorf("Failed to update task %q: No such task ID: %q", msg.SwarmingTaskId, id)
			return true
		} else if err == db.ErrUnknownId {
			expectedSwarmingTaskId := "<unknown>"
			ids, ok := res.Tags[types.SWARMING_TAG_ID]
			if ok {
				id = ids[0]
				t, err := s.db.GetTaskById(ctx, id)
				if err != nil {
					sklog.Errorf("Failed to update task %q; mismatched ID and failed to retrieve task from DB: %s", msg.SwarmingTaskId, err)
					return true
				} else {
					expectedSwarmingTaskId = t.SwarmingTaskId
				}
			}
			sklog.Errorf("Failed to update task %q: Task %s has a different Swarming task ID associated with it: %s", msg.SwarmingTaskId, id, expectedSwarmingTaskId)
			return true
		} else {
			sklog.Errorf("Failed to update task %q: %s", msg.SwarmingTaskId, err)
			return true
		}
	}
	return true
}
