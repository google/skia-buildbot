package job_creation

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.skia.org/infra/go/cas"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/cacher"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/isolate_cache"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/syncer"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	"go.skia.org/infra/task_scheduler/go/tryjobs"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"
)

var (
	// ignoreBranches indicates that we shouldn't schedule on these branches.
	// WARNING: Any commit reachable from any of these branches will be
	// skipped. So, for example, if you fork a branch from head of main
	// and immediately ignore it, no tasks will be scheduled for any
	// commits on main up to the branch point.
	// TODO(borenet): An alternative would be to only follow the first
	// parent for merge commits. That way, we could remove the checks which
	// cause this issue but still ignore the branch as expected. The
	// downside is that we'll miss commits in the case where we fork a
	// branch, merge it back, and delete the new branch head.
	ignoreBranches = map[string][]string{
		common.REPO_SKIA_INTERNAL: {
			"skia-master",
		},
	}
)

// JobCreator is a struct used for creating Jobs based on new commits, tryjobs,
// and timed triggers.
type JobCreator struct {
	cacher        *cacher.Cacher
	db            db.DB
	isolateCache  *isolate_cache.Cache
	jCache        cache.JobCache
	lvUpdateRepos metrics2.Liveness
	repos         repograph.Map
	syncer        *syncer.Syncer
	taskCfgCache  *task_cfg_cache.TaskCfgCache
	tryjobs       *tryjobs.TryJobIntegrator
	window        *window.Window
}

func NewJobCreator(ctx context.Context, d db.DB, period time.Duration, numCommits int, workdir, host string, repos repograph.Map, isolateClient *isolate.Client, rbe cas.CAS, c *http.Client, buildbucketApiUrl, trybotBucket string, projectRepoMapping map[string]string, depotTools string, gerrit gerrit.GerritInterface, taskCfgCache *task_cfg_cache.TaskCfgCache, isolateCache *isolate_cache.Cache, ts oauth2.TokenSource) (*JobCreator, error) {
	// Repos must be updated before window is initialized; otherwise the repos may be uninitialized,
	// resulting in the window being too short, causing the caches to be loaded with incomplete data.
	for _, r := range repos {
		if err := r.Update(ctx); err != nil {
			return nil, fmt.Errorf("Failed initial repo sync: %s", err)
		}
	}
	w, err := window.New(period, numCommits, repos)
	if err != nil {
		return nil, fmt.Errorf("Failed to create window: %s", err)
	}

	// Create caches.
	jCache, err := cache.NewJobCache(ctx, d, w, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to create JobCache: %s", err)
	}

	sc := syncer.New(ctx, repos, depotTools, workdir, syncer.DEFAULT_NUM_WORKERS)
	chr := cacher.New(sc, taskCfgCache, isolateClient, isolateCache, rbe)

	tryjobs, err := tryjobs.NewTryJobIntegrator(buildbucketApiUrl, trybotBucket, host, c, d, jCache, projectRepoMapping, repos, taskCfgCache, chr, gerrit)
	if err != nil {
		return nil, fmt.Errorf("Failed to create TryJobIntegrator: %s", err)
	}
	jc := &JobCreator{
		cacher:        chr,
		db:            d,
		isolateCache:  isolateCache,
		jCache:        jCache,
		lvUpdateRepos: metrics2.NewLiveness("last_successful_repo_update"),
		repos:         repos,
		syncer:        sc,
		taskCfgCache:  taskCfgCache,
		tryjobs:       tryjobs,
		window:        w,
	}
	if err := jc.initCaches(ctx); err != nil {
		return nil, err
	}
	return jc, nil
}

// Close cleans up resources used by the JobCreator.
func (jc *JobCreator) Close() error {
	if err := jc.syncer.Close(); err != nil {
		return err
	}
	if err := jc.taskCfgCache.Close(); err != nil {
		return err
	}
	return jc.isolateCache.Close()
}

// Start initeates the JobCreator's goroutines for creating jobs.
func (jc *JobCreator) Start(ctx context.Context, enableTryjobs bool) {
	if enableTryjobs {
		jc.tryjobs.Start(ctx)
	}
}

// putJobsInChunks is a wrapper around DB.PutJobsInChunks which adds the jobs
// to the cache.
func (jc *JobCreator) putJobsInChunks(j []*types.Job) error {
	if err := jc.db.PutJobsInChunks(j); err != nil {
		return err
	}
	jc.jCache.AddJobs(j)
	return nil
}

// recurseAllBranches runs the given func on every commit on all branches, with
// some Task Scheduler-specific exceptions.
func (jc *JobCreator) recurseAllBranches(ctx context.Context, repoUrl string, repo *repograph.Graph, fn func(string, *repograph.Graph, *repograph.Commit) error) error {
	skipBranches := ignoreBranches[repoUrl]
	skipCommits := make(map[*repograph.Commit]string, len(skipBranches))
	for _, b := range skipBranches {
		c := repo.Get(b)
		if c != nil {
			skipCommits[c] = b
		}
	}
	if err := repo.RecurseAllBranches(func(c *repograph.Commit) error {
		if skippedBranch, ok := skipCommits[c]; ok {
			sklog.Infof("Skipping ignored branch %q", skippedBranch)
			return repograph.ErrStopRecursing
		}
		for head, skippedBranch := range skipCommits {
			isAncestor, err := repo.IsAncestor(c.Hash, head.Hash)
			if err != nil {
				return err
			} else if isAncestor {
				sklog.Infof("Skipping ignored branch %q (--is-ancestor)", skippedBranch)
				return repograph.ErrStopRecursing
			}
		}
		if !jc.window.TestCommit(repoUrl, c) {
			return repograph.ErrStopRecursing
		}
		return fn(repoUrl, repo, c)
	}); err != nil {
		return err
	}
	return nil
}

// gatherNewJobs finds and returns Jobs for all new commits, keyed by RepoState.
func (jc *JobCreator) gatherNewJobs(ctx context.Context, repoUrl string, repo *repograph.Graph) ([]*types.Job, error) {
	defer metrics2.FuncTimer().Stop()

	// Find all new Jobs for all new commits.
	newJobs := []*types.Job{}
	if err := jc.recurseAllBranches(ctx, repoUrl, repo, func(repoUrl string, r *repograph.Graph, c *repograph.Commit) error {
		// If this commit isn't in scheduling range, stop recursing.
		if !jc.window.TestCommit(repoUrl, c) {
			return repograph.ErrStopRecursing
		}

		rs := types.RepoState{
			Repo:     repoUrl,
			Revision: c.Hash,
		}
		cfg, err := jc.cacher.GetOrCacheRepoState(ctx, rs)
		if err != nil {
			if specs.ErrorIsPermanent(err) {
				// If we return an error here, we'll never
				// recover from bad commits.
				// TODO(borenet): We should consider canceling the jobs
				// (how?) since we can never fulfill them.
				sklog.Errorf("Failed to obtain new jobs due to permanent error: %s", err)
				return nil
			}
			return err
		}
		alreadyScheduledAllJobs := true
		for name, spec := range cfg.Jobs {
			shouldRun := false
			if !util.In(spec.Trigger, specs.PERIODIC_TRIGGERS) {
				if spec.Trigger == specs.TRIGGER_ANY_BRANCH {
					shouldRun = true
				} else if spec.Trigger == specs.TRIGGER_MASTER_ONLY {
					isAncestor, err := r.IsAncestor(c.Hash, git.DefaultBranch)
					if err != nil {
						return err
					} else if isAncestor {
						shouldRun = true
					}
				}
			}
			if shouldRun {
				prevJobs, err := jc.jCache.GetJobsByRepoState(name, rs)
				if err != nil {
					return err
				}
				alreadyScheduled := false
				for _, prev := range prevJobs {
					// We don't need to check whether it's a
					// try job because a try job wouldn't
					// match the RepoState passed into
					// GetJobsByRepoState.
					if !prev.IsForce {
						alreadyScheduled = true
					}
				}
				if !alreadyScheduled {
					alreadyScheduledAllJobs = false
					j, err := jc.taskCfgCache.MakeJob(ctx, rs, name)
					if err != nil {
						// We shouldn't get ErrNoSuchEntry due to the
						// call to jc.cacher.GetOrCacheRepoState above,
						// but we check the error and don't propagate
						// it, just in case.
						if err == task_cfg_cache.ErrNoSuchEntry {
							sklog.Errorf("Got ErrNoSuchEntry after a successful call to GetOrCacheRepoState! Job %s; RepoState: %+v", name, rs)
							continue
						}
						return err
					}
					j.Requested = firestore.FixTimestamp(c.Timestamp)
					j.Created = firestore.FixTimestamp(j.Created)
					if !j.Requested.Before(j.Created) {
						sklog.Errorf("Job created time %s is before requested time %s! Setting equal.", j.Created, j.Requested)
						j.Requested = j.Created.Add(-firestore.TS_RESOLUTION)
					}
					newJobs = append(newJobs, j)
				}
			}
		}
		// If we'd already scheduled all of the jobs for this commit,
		// stop recursing, under the assumption that we've already
		// scheduled all of the jobs for the ones before it.
		if alreadyScheduledAllJobs {
			return repograph.ErrStopRecursing
		}
		if c.Hash == "50537e46e4f0999df0a4707b227000cfa8c800ff" {
			// Stop recursing here, since Jobs were added
			// in this commit and previous commits won't be
			// valid.
			return repograph.ErrStopRecursing
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// Reverse the new jobs list, so that if we fail to insert all of the
	// jobs (eg. because the process is interrupted), the algorithm above
	// will find the missing jobs and we'll pick up where we left off.
	for a, b := 0, len(newJobs)-1; a < b; a, b = a+1, b-1 {
		newJobs[a], newJobs[b] = newJobs[b], newJobs[a]
	}
	return newJobs, nil
}

// HandleRepoUpdate is a pubsub.AutoUpdateMapCallback which is called when any
// of the repos is updated.
func (jc *JobCreator) HandleRepoUpdate(ctx context.Context, repoUrl string, g *repograph.Graph, ack, nack func()) error {
	newJobs, err := jc.gatherNewJobs(ctx, repoUrl, g)
	if err != nil {
		// gatherNewJobs does not return an error if the
		// commit is invalid; so the error indicates
		// something transient that should be retried.
		nack()
		return skerr.Wrapf(err, "gatherNewJobs returned transient error")
	}
	if err := jc.putJobsInChunks(newJobs); err != nil {
		// nack the pubsub message so that we'll have
		// another chance to add these jobs.
		nack()
		return skerr.Wrapf(err, "Failed to insert new jobs into the DB")
	}
	// Now we've inserted jobs for the new commits. We don't
	// want to go through and do it again, so ack the pubsub
	// message without waiting to see if the cache refreshes
	// below succeed.
	ack()
	if err := jc.window.Update(); err != nil {
		return skerr.Wrapf(err, "failed to update window")
	}
	if err := jc.taskCfgCache.Cleanup(ctx, time.Now().Sub(jc.window.EarliestStart())); err != nil {
		return skerr.Wrapf(err, "failed to Cleanup TaskCfgCache")
	}
	jc.lvUpdateRepos.Reset()
	return nil
}

// initCaches ensures that all of the RepoStates we care about are present
// in the various caches.
func (jc *JobCreator) initCaches(ctx context.Context) error {
	defer metrics2.FuncTimer().Stop()

	sklog.Infof("Initializing caches...")
	defer sklog.Infof("Done initializing caches.")

	// Some existing jobs may not have been cached by Cacher already, eg.
	// because of poorly-timed process restarts. Go through the unfinished
	// jobs and cache them if necessary.
	if err := jc.jCache.Update(); err != nil {
		return fmt.Errorf("Failed to update job cache: %s", err)
	}
	unfinishedJobs, err := jc.jCache.UnfinishedJobs()
	if err != nil {
		return err
	}
	repoStatesToCache := map[types.RepoState]bool{}
	for _, job := range unfinishedJobs {
		repoStatesToCache[job.RepoState] = true
	}

	// Also cache the repo states for all commits in range.
	for repoUrl, repo := range jc.repos {
		if err := jc.recurseAllBranches(ctx, repoUrl, repo, func(_ string, _ *repograph.Graph, c *repograph.Commit) error {
			repoStatesToCache[types.RepoState{
				Repo:     repoUrl,
				Revision: c.Hash,
			}] = true
			return nil
		}); err != nil {
			return err
		}
	}

	// Actually cache the RepoStates.
	var g errgroup.Group
	for rs := range repoStatesToCache {
		rs := rs // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			if _, err := jc.cacher.GetOrCacheRepoState(ctx, rs); err != nil {
				return fmt.Errorf("Failed to cache RepoState: %s", err)
			}
			return nil
		})
	}
	return g.Wait()
}

// MaybeTriggerPeriodicJobs triggers all periodic jobs with the given trigger
// name, if those jobs haven't already been triggered.
func (jc *JobCreator) MaybeTriggerPeriodicJobs(ctx context.Context, triggerName string) error {
	// We'll search the jobs we've already triggered to ensure that we don't
	// trigger the same jobs multiple times in a day/week/whatever. Search a
	// window that is not quite the size of the trigger interval, to allow
	// for lag time.
	end := time.Now()
	var start time.Time
	if triggerName == specs.TRIGGER_NIGHTLY {
		start = end.Add(-23 * time.Hour)
	} else if triggerName == specs.TRIGGER_WEEKLY {
		// Note that if the cache window is less than a week, this start
		// time isn't going to work as expected. However, we only really
		// expect to need to debounce periodic triggers for a short
		// window, so anything longer than a few minutes would probably
		// be enough, and the ~4 days we normally keep in the cache
		// should be more than sufficient.
		start = end.Add(-6 * 24 * time.Hour)
	} else {
		sklog.Warningf("Ignoring unknown periodic trigger %q", triggerName)
		return nil
	}

	// Find the job specs matching the trigger and create Job instances.
	jobs := []*types.Job{}
	for repoUrl, repo := range jc.repos {
		main := repo.Get(git.DefaultBranch)
		if main == nil {
			return fmt.Errorf("Failed to retrieve branch %q for %s", git.DefaultBranch, repoUrl)
		}
		rs := types.RepoState{
			Repo:     repoUrl,
			Revision: main.Hash,
		}
		cfg, cachedErr, err := jc.taskCfgCache.Get(ctx, rs)
		if cachedErr != nil {
			err = cachedErr
		}
		if err != nil {
			return fmt.Errorf("Failed to retrieve TaskCfg from %s: %s", repoUrl, err)
		}
		for name, js := range cfg.Jobs {
			if js.Trigger == triggerName {
				job, err := jc.taskCfgCache.MakeJob(ctx, rs, name)
				if err != nil {
					return fmt.Errorf("Failed to create job: %s", err)
				}
				job.Requested = job.Created
				jobs = append(jobs, job)
			}
		}
	}
	if len(jobs) == 0 {
		return nil
	}

	// Filter out any jobs which we've already triggered. Generally, we'd
	// expect to have triggered all of the jobs or none of them, but there
	// might be circumstances which caused us to trigger a partial set.
	names := make([]string, 0, len(jobs))
	for _, job := range jobs {
		names = append(names, job.Name)
	}
	existing, err := jc.jCache.GetMatchingJobsFromDateRange(names, start, end)
	if err != nil {
		return err
	}
	jobsToInsert := make([]*types.Job, 0, len(jobs))
	for _, job := range jobs {
		var prev *types.Job = nil
		for _, existingJob := range existing[job.Name] {
			if !existingJob.IsTryJob() && !existingJob.IsForce {
				// Pick an arbitrary pre-existing job for logging.
				prev = existingJob
				break
			}
		}
		if prev == nil {
			jobsToInsert = append(jobsToInsert, job)
		} else {
			sklog.Warningf("Already triggered a job for %s (eg. id %s at %s); not triggering again.", job.Name, prev.Id, prev.Created)
		}
	}
	if len(jobsToInsert) == 0 {
		return nil
	}

	// Insert the new jobs into the DB.
	if err := jc.putJobsInChunks(jobsToInsert); err != nil {
		return fmt.Errorf("Failed to add periodic jobs: %s", err)
	}
	sklog.Infof("Created %d periodic jobs for trigger %q", len(jobs), triggerName)
	return nil
}
