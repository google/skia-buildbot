package job_creation

import (
	"context"
	"net/http"
	"strings"
	"time"

	"go.skia.org/infra/go/cas"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/progress"
	"go.skia.org/infra/go/pubsub"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/cacher"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/syncer"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	"go.skia.org/infra/task_scheduler/go/tryjobs"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
	"golang.org/x/sync/errgroup"
)

const periodicSyncInterval = 5 * time.Minute

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
	cacher              cacher.Cacher
	db                  db.DB
	gatherNewJobsQueues map[string]chan func(error)
	jCache              cache.JobCache
	lvUpdateRepos       metrics2.Liveness
	repos               repograph.Map
	syncer              *syncer.Syncer
	taskCfgCache        task_cfg_cache.TaskCfgCache
	tryjobs             *tryjobs.TryJobIntegrator
	window              window.Window
}

// NewJobCreator returns a JobCreator instance.
func NewJobCreator(ctx context.Context, d db.DB, period time.Duration, numCommits int, workdir, host string, repos repograph.Map, rbe cas.CAS, c *http.Client, buildbucketProject, buildbucketTarget, buildbucketBucket string, projectRepoMapping map[string]string, depotTools string, gerrit gerrit.GerritInterface, taskCfgCache task_cfg_cache.TaskCfgCache, pubsubClient pubsub.Client, numSyncWorkers int, useGitCache bool) (*JobCreator, error) {
	jc, err := newJobCreatorWithoutInit(ctx, d, period, numCommits, workdir, host, repos, rbe, c, buildbucketProject, buildbucketTarget, buildbucketBucket, projectRepoMapping, depotTools, gerrit, taskCfgCache, pubsubClient, numSyncWorkers, useGitCache)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return jc, jc.initCaches(ctx)
}

func newJobCreatorWithoutInit(ctx context.Context, d db.DB, period time.Duration, numCommits int, workdir, host string, repos repograph.Map, rbe cas.CAS, c *http.Client, buildbucketProject, buildbucketTarget, buildbucketBucket string, projectRepoMapping map[string]string, depotTools string, gerrit gerrit.GerritInterface, taskCfgCache task_cfg_cache.TaskCfgCache, pubsubClient pubsub.Client, numSyncWorkers int, useGitCache bool) (*JobCreator, error) {
	// Repos must be updated before window is initialized; otherwise the repos may be uninitialized,
	// resulting in the window being too short, causing the caches to be loaded with incomplete data.
	for _, r := range repos {
		if err := r.Update(ctx); err != nil {
			return nil, skerr.Wrapf(err, "failed initial repo sync")
		}
	}
	w, err := window.New(ctx, period, numCommits, repos)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create window")
	}

	// Create caches.
	jCache, err := cache.NewJobCache(ctx, d, w, nil)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create JobCache")
	}

	sc, err := syncer.New(ctx, repos, depotTools, workdir, numSyncWorkers, useGitCache)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create Syncer")
	}

	chr := cacher.New(sc, taskCfgCache, rbe, gerrit)

	tryjobs, err := tryjobs.NewTryJobIntegrator(ctx, buildbucketProject, buildbucketTarget, buildbucketBucket, host, c, d, jCache, projectRepoMapping, repos, taskCfgCache, chr, gerrit, pubsubClient)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create TryJobIntegrator")
	}
	gatherNewJobsQueues := map[string]chan func(error){}
	for repoUrl := range repos {
		gatherNewJobsQueues[repoUrl] = make(chan func(error))
	}
	jc := &JobCreator{
		cacher:              chr,
		db:                  d,
		gatherNewJobsQueues: gatherNewJobsQueues,
		jCache:              jCache,
		lvUpdateRepos:       metrics2.NewLiveness("last_successful_repo_update"),
		repos:               repos,
		syncer:              sc,
		taskCfgCache:        taskCfgCache,
		tryjobs:             tryjobs,
		window:              w,
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
	return nil
}

func (jc *JobCreator) triggerRepoUpdate(repoUrl string, callback func(error)) {
	ch, ok := jc.gatherNewJobsQueues[repoUrl]
	if !ok {
		sklog.Errorf("Received request to update unknown repo %q", repoUrl)
		return
	}
	ch <- callback
}

// Start initeates the JobCreator's goroutines for creating jobs.
func (jc *JobCreator) Start(ctx context.Context, enableTryjobs bool) {
	if enableTryjobs {
		jc.tryjobs.Start(ctx)
	}
	for repoUrl, repo := range jc.repos {
		repoUrl := repoUrl
		repo := repo
		go func() {
			ch := jc.gatherNewJobsQueues[repoUrl]
			for {
				select {
				case <-ctx.Done():
					return
				case callback := <-ch:
					err := jc.insertNewJobsFromRepo(ctx, repoUrl, repo)
					if err != nil {
						sklog.Errorf("Failed inserting new jobs for %s; will try again in %s: %s", repoUrl, periodicSyncInterval, err)
					}
					callback(err)
				}
			}
		}()
	}
	go util.RepeatCtx(ctx, periodicSyncInterval, func(ctx context.Context) {
		for repoUrl := range jc.repos {
			jc.triggerRepoUpdate(repoUrl, func(error) {})
		}
	})
}

// putJobsInChunks is a wrapper around DB.PutJobsInChunks which adds the jobs
// to the cache.
func (jc *JobCreator) putJobsInChunks(ctx context.Context, j []*types.Job) error {
	if err := jc.db.PutJobsInChunks(ctx, j); err != nil {
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
			if cacher.IsCachedError(err) {
				// If we return an error here, we'll never
				// recover from bad commits.
				sklog.Warningf("Repo %s @ %s has a permanent cached error which prevents jobs from being scheduled at this commit. Skipping.", rs.Repo, rs.Revision)
				errSplit := strings.Split(err.Error(), "\n")
				if len(errSplit) > 50 {
					errSplit = errSplit[len(errSplit)-50:]
				}
				sklog.Debugf("Last 50 lines of error: %s", strings.Join(errSplit, "\n"))
				return nil
			}
			return skerr.Wrap(err)
		}
		alreadyScheduledAllJobs := true
		for name, spec := range cfg.Jobs {
			shouldRun := false
			if !util.In(spec.Trigger, specs.PERIODIC_TRIGGERS) {
				if spec.Trigger == specs.TRIGGER_MASTER_ONLY || spec.Trigger == specs.TRIGGER_MAIN_ONLY {
					mainBranch := git.MainBranch
					if r.Get(mainBranch) == nil {
						mainBranch = git.MasterBranch
					}
					if r.Get(mainBranch) == nil {
						// No known main branch in this repo, so we'll trigger.
						shouldRun = true
					} else {
						isAncestor, err := r.IsAncestor(c.Hash, mainBranch)
						if err != nil {
							return skerr.Wrap(err)
						} else if isAncestor {
							shouldRun = true
						}
					}
				} else if spec.Trigger == specs.TRIGGER_ANY_BRANCH {
					shouldRun = true
				}
			}
			if shouldRun {
				prevJobs, err := jc.jCache.GetJobsByRepoState(name, rs)
				if err != nil {
					return skerr.Wrap(err)
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
					j, err := task_cfg_cache.MakeJob(ctx, jc.taskCfgCache, rs, name)
					if err != nil {
						// We shouldn't get ErrNoSuchEntry due to the
						// call to jc.cacher.GetOrCacheRepoState above,
						// but we check the error and don't propagate
						// it, just in case.
						if err == task_cfg_cache.ErrNoSuchEntry {
							sklog.Errorf("Got ErrNoSuchEntry after a successful call to GetOrCacheRepoState! Job %s; RepoState: %+v", name, rs)
							continue
						}
						return skerr.Wrap(err)
					}
					j.Requested = firestore.FixTimestamp(c.Timestamp)
					j.Created = firestore.FixTimestamp(j.Created)
					if !j.Requested.Before(j.Created) {
						sklog.Errorf("Job created time %s is before requested time %s! Setting equal.", j.Created, j.Requested)
						j.Requested = j.Created.Add(-firestore.TS_RESOLUTION)
					}
					j.Started = j.Created
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
		return nil, skerr.Wrap(err)
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
func (jc *JobCreator) HandleRepoUpdate(ctx context.Context, repoUrl string, g *repograph.Graph, ack, _ func()) error {
	// Don't wait for the sync to start to ack the message.
	ack()
	jc.triggerRepoUpdate(repoUrl, func(error) {})
	return nil
}

func (jc JobCreator) insertNewJobsFromRepo(ctx context.Context, repoUrl string, g *repograph.Graph) error {
	newJobs, err := jc.gatherNewJobs(ctx, repoUrl, g)
	if err != nil {
		return skerr.Wrapf(err, "gatherNewJobs returned transient error")
	}
	if err := jc.putJobsInChunks(ctx, newJobs); err != nil {
		return skerr.Wrapf(err, "Failed to insert new jobs into the DB")
	}
	if err := jc.window.Update(ctx); err != nil {
		return skerr.Wrapf(err, "failed to update window")
	}
	if err := jc.taskCfgCache.Cleanup(ctx, now.Now(ctx).Sub(jc.window.EarliestStart())); err != nil {
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
	if err := jc.jCache.Update(ctx); err != nil {
		return skerr.Wrapf(err, "failed to update job cache")
	}
	unfinishedJobs, err := jc.jCache.InProgressJobs()
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
	pt := progress.New(int64(len(repoStatesToCache)))
	pt.AtInterval(ctx, 5*time.Minute, func(count, total int64) {
		sklog.Debugf("Cached %d of %d RepoStates", count, total)
	})
	var g errgroup.Group
	for rs := range repoStatesToCache {
		rs := rs // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			defer pt.Inc(1)
			if _, err := jc.cacher.GetOrCacheRepoState(ctx, rs); err != nil {
				if cacher.IsCachedError(err) {
					// Returning an error here would cause the app to repeatedly
					// fail to start, and since the error is permanent, retries
					// wouldn't help us.  Note that there was an error in the
					// log, but don't log the error itself, which is typically
					// very long.
					sklog.Errorf("Have cached error for RepoState %s", rs.RowKey())
				} else {
					return skerr.Wrapf(err, "failed to cache RepoState")
				}
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
	end := now.Now(ctx)
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
		main := repo.Get(git.MasterBranch)
		if main == nil {
			main = repo.Get(git.MainBranch)
		}
		if main == nil {
			return skerr.Fmt("failed to retrieve branch %q or %q for %s", git.MasterBranch, git.MainBranch, repoUrl)
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
			return skerr.Wrapf(err, "failed to retrieve TaskCfg from %s", repoUrl)
		}
		for name, js := range cfg.Jobs {
			if js.Trigger == triggerName {
				job, err := task_cfg_cache.MakeJob(ctx, jc.taskCfgCache, rs, name)
				if err != nil {
					return skerr.Wrapf(err, "failed to create job")
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
	if err := jc.putJobsInChunks(ctx, jobsToInsert); err != nil {
		return skerr.Wrapf(err, "failed to add periodic jobs")
	}
	sklog.Infof("Created %d periodic jobs for trigger %q", len(jobs), triggerName)
	return nil
}
