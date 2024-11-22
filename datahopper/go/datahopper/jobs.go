package main

/*
	Jobs metrics.
*/

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/metrics2/events"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
)

var (
	// Time periods over which to compute metrics. Assumed to be sorted in
	// increasing order.
	timePeriods = []time.Duration{24 * time.Hour, 7 * 24 * time.Hour}
)

const (
	// Measurement name for overdue job specs. Records the age of the oldest commit for which the
	// job has not completed (nor for any later commit), for each job spec and for each repo.
	measurementOverdueJobSpecs = "overdue_job_specs_s"

	// Measurement indicating the age of the most-recently created job, by
	// name, in seconds.
	measurementLatestJobAge = "latest_job_age_s"

	// Measurement indicating the lag time between a job being requested
	// (eg. a commit landing or tryjob requested) and a job being added.
	measurementJobCreationLag = "job_creation_lag_s"

	// Measurement indicating the lag time between a job being created and the
	// job being started.
	measurementJobStartedLag = "job_started_lag_s"

	// Measurement indicating the lag time between a job being started and its
	// first task being triggered.
	measurementFirstTaskTriggeredLag = "first_task_lag_s"

	jobStream = "job_metrics"

	overdueJobMetricsPeriod     = 8 * 24 * time.Hour
	overdueJobMetricsNumCommits = 5
)

// jobTypeString is an enum of the types of Jobs that computeAvgJobDuration will aggregate separately.
// The string value is the same as the job_type tag value returned by computeAvgJobDuration.
type jobTypeString string

const (
	jobType_Normal jobTypeString = "normal"
	jobType_Forced jobTypeString = "forced"
	jobType_TryJob jobTypeString = "tryjob"
)

// jobEventDB implements the events.EventDB interface.
type jobEventDB struct {
	cached []*events.Event
	jCache cache.JobCache
	em     *events.EventMetrics
	// Do not lock mtx when calling methods on em. Otherwise deadlock can occur.
	// E.g. (now fixed):
	// 1. Thread 1: EventManager.updateMetrics locks EventMetrics.mtx
	// 2. Thread 2: jobEventDB.update locks jobEventDB.mtx
	// 3. Thread 1: EventManager.updateMetrics calls jobEventDB.Range, which waits
	//    to lock jobEventDB.mtx
	// 4. Thread 2: jobEventDB.update calls EventMetrics.AggregateMetric (by way
	//    of addJobAggregates and EventStream.AggregateMetric), which waits to lock
	//    EventMetrics.mtx
	mtx sync.Mutex
}

// See docs for events.EventDB interface.
func (j *jobEventDB) Append(string, []byte) error {
	return fmt.Errorf("jobEventDB is read-only!")
}

// See docs for events.EventDB interface.
func (j *jobEventDB) Close() error {
	return nil
}

// See docs for events.EventDB interface.
func (j *jobEventDB) Insert(*events.Event) error {
	return fmt.Errorf("jobEventDB is read-only!")
}

// See docs for events.EventDB interface.
func (j *jobEventDB) Range(stream string, start, end time.Time) ([]*events.Event, error) {
	j.mtx.Lock()
	defer j.mtx.Unlock()

	n := len(j.cached)
	if n == 0 {
		return []*events.Event{}, nil
	}

	first := sort.Search(n, func(i int) bool {
		return !j.cached[i].Timestamp.Before(start)
	})

	last := first + sort.Search(n-first, func(i int) bool {
		return !j.cached[i+first].Timestamp.Before(end)
	})

	rv := make([]*events.Event, last-first, last-first)
	copy(rv[:], j.cached[first:last])
	return rv, nil
}

// update updates the cached jobs in the jobEventDB. Only a single thread may
// call this method, but it can be called concurrently with other methods.
func (j *jobEventDB) update() error {
	defer metrics2.FuncTimer().Stop()
	if err := j.jCache.Update(context.TODO()); err != nil {
		return err
	}
	now := time.Now()
	longestPeriod := timePeriods[len(timePeriods)-1]
	jobs, err := j.jCache.GetJobsFromDateRange(now.Add(-longestPeriod), now)
	if err != nil {
		return skerr.Wrapf(err, "Failed to load jobs from %s to %s", now.Add(-longestPeriod), now)
	}
	sklog.Debugf("jobEventDB.update: Processing %d jobs for time range %s to %s.", len(jobs), now.Add(-longestPeriod), now)
	cached := make([]*events.Event, 0, len(jobs))
	for _, job := range jobs {
		if !job.Done() {
			continue
		}
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(job); err != nil {
			return skerr.Wrapf(err, "Failed to encode %#v to GOB", job)
		}
		ev := &events.Event{
			Stream:    jobStream,
			Timestamp: job.Created,
			Data:      buf.Bytes(),
		}
		cached = append(cached, ev)
	}
	j.mtx.Lock()
	defer j.mtx.Unlock()
	j.cached = cached
	return nil
}

// computeAvgJobDuration is an events.DynamicAggregateFn that returns metrics for average Job duration
// for Jobs with status SUCCESS or FAILURE, given a slice of Events created by jobEventDB.update.
// The first return value will contain the tags "job_name" (types.Job.Name) and "job_type" (one of
// "normal", "tryjob", "forced"), and the second return value will be the corresponding average Job
// duration. Returns an error if Event.Data can't be GOB-decoded as a types.Job.
func computeAvgJobDuration(ev []*events.Event) ([]map[string]string, []float64, error) {
	if len(ev) > 0 {
		// ev should be ordered by timestamp
		sklog.Debugf("Calculating avg-duration for %d jobs since %s.", len(ev), ev[0].Timestamp)
	}
	type sum struct {
		count int
		total float64
	}
	type jobSums struct {
		normal sum
		tryjob sum
		forced sum
	}
	byJob := map[string]*jobSums{}
	for _, e := range ev {
		var job types.Job
		if err := gob.NewDecoder(bytes.NewReader(e.Data)).Decode(&job); err != nil {
			return nil, nil, err
		}
		if !(job.Status == types.JOB_STATUS_SUCCESS || job.Status == types.JOB_STATUS_FAILURE) {
			continue
		}
		entry, ok := byJob[job.Name]
		if !ok {
			entry = &jobSums{}
			byJob[job.Name] = entry
		}
		var jobSum *sum
		if job.IsForce {
			jobSum = &entry.forced
		} else if job.IsTryJob() {
			jobSum = &entry.tryjob
		} else {
			jobSum = &entry.normal
		}
		jobSum.count++
		jobSum.total += job.Finished.Sub(job.Created).Seconds()
	}

	rvTags := make([]map[string]string, 0, len(byJob)*3)
	rvVals := make([]float64, 0, len(byJob)*3)
	add := func(jobName string, jobType jobTypeString, jobSum sum) {
		if jobSum.count == 0 {
			return
		}
		value := jobSum.total / float64(jobSum.count)
		rvTags = append(rvTags, map[string]string{
			"job_name": jobName,
			"job_type": string(jobType),
		})
		rvVals = append(rvVals, value)
	}
	for jobName, jobSums := range byJob {
		add(jobName, jobType_Normal, jobSums.normal)
		add(jobName, jobType_TryJob, jobSums.tryjob)
		add(jobName, jobType_Forced, jobSums.forced)
	}
	return rvTags, rvVals, nil
}

// computeJobFailureMishapRate is an events.DynamicAggregateFn that returns metrics for Job failure rate and
// mishap rate, given a slice of Events created by jobEventDB.update. The first return value will
// contain the tags "job_name" (types.Job.Name) and "metric" (one of "failure-rate", "mishap-rate"),
// and the second return value will be the corresponding ratio of failed/mishap Jobs to all
// completed Jobs. Returns an error if Event.Data can't be GOB-decoded as a types.Job.
func computeJobFailureMishapRate(ev []*events.Event) ([]map[string]string, []float64, error) {
	if len(ev) > 0 {
		// ev should be ordered by timestamp
		sklog.Debugf("Calculating failure-rate for %d jobs since %s.", len(ev), ev[0].Timestamp)
	}
	type jobSum struct {
		fails   int
		mishaps int
		count   int
	}
	byJob := map[string]*jobSum{}
	for _, e := range ev {
		var job types.Job
		if err := gob.NewDecoder(bytes.NewReader(e.Data)).Decode(&job); err != nil {
			return nil, nil, err
		}
		entry, ok := byJob[job.Name]
		if !ok {
			entry = &jobSum{}
			byJob[job.Name] = entry
		}
		entry.count++
		if job.Status == types.JOB_STATUS_FAILURE {
			entry.fails++
		} else if job.Status == types.JOB_STATUS_MISHAP {
			entry.mishaps++
		}
	}

	rvTags := make([]map[string]string, 0, len(byJob)*2)
	rvVals := make([]float64, 0, len(byJob)*2)
	add := func(jobName, metric string, value float64) {
		rvTags = append(rvTags, map[string]string{
			"job_name": jobName,
			"job_type": "",
			"metric":   metric,
		})
		rvVals = append(rvVals, value)
	}
	for jobName, jobSum := range byJob {
		if jobSum.count == 0 {
			continue
		}
		add(jobName, "failure-rate", float64(jobSum.fails)/float64(jobSum.count))
		add(jobName, "mishap-rate", float64(jobSum.mishaps)/float64(jobSum.count))
	}
	return rvTags, rvVals, nil
}

// isPeriodic returns true if the job runs periodically, as opposed to at every
// commit.
// TODO(borenet): We could add a Job.Trigger JobSpec.Trigger field which is
// copied from JobSpec.Trigger and not need to make inferences from job names...
func isPeriodic(job *types.Job) bool {
	if strings.Contains(job.Name, "Nightly") {
		return true
	}
	if strings.Contains(job.Name, "OnDemand") {
		return true
	}
	if strings.Contains(job.Name, "Weekly") {
		return true
	}
	return false
}

// makeObservationsFn takes the given function, which applies to either try jobs
// or normal jobs, and returns an events.ObservationsFn.
func makeObservationsFn(fn func([]*events.Event, bool) ([]float64, error), tryjobs bool) func([]*events.Event) ([]float64, error) {
	return func(ev []*events.Event) ([]float64, error) {
		return fn(ev, tryjobs)
	}
}

// computeJobCreationLatencies is an events.ObservationsFn that computes the
// latency between the Requested timestamp (time at which the commit landed) and
// the Creation timestamp.
func computeJobCreationLatencies(ev []*events.Event, tryjobs bool) ([]float64, error) {
	if len(ev) == 0 {
		return []float64{}, nil
	}
	var observations []float64
	forcedOrPeriodic := 0
	notTryjobMatch := 0
	emptyCreatedTs := 0
	emptyRequestedTs := 0
	createdBeforeRequested := 0
	for _, e := range ev {
		var job types.Job
		if err := gob.NewDecoder(bytes.NewReader(e.Data)).Decode(&job); err != nil {
			return nil, skerr.Wrapf(err, "Failed to decode job")
		}
		// Forced jobs and periodic jobs will incorrectly skew the average.
		if job.IsForce || isPeriodic(&job) {
			forcedOrPeriodic++
			continue
		}
		// Filter out try jobs or normal jobs as appropriate.
		if tryjobs != job.IsTryJob() {
			notTryjobMatch++
			continue
		}
		// Guard against missing or incorrect Created or Requested timestamps.
		if util.TimeIsZero(job.Created) {
			sklog.Warningf("Job %s has an empty Created timestamp", job.Id)
			emptyCreatedTs++
			continue
		}
		if util.TimeIsZero(job.Requested) {
			sklog.Warningf("Job %s has an empty Requested timestamp", job.Id)
			emptyRequestedTs++
			continue
		}
		if job.Created.Before(job.Requested) {
			sklog.Warningf("Job %s has a Created timestamp before its Requested timestamp", job.Id)
			createdBeforeRequested++
			continue
		}
		observations = append(observations, job.Created.Sub(job.Requested).Seconds())
	}
	tryjobsStr := ""
	if tryjobs {
		tryjobsStr = "not "
	}
	sklog.Infof("Computed %d job creation latencies from %d jobs (%d forced or periodic, %d %stryjobs, %d empty created ts, %d empty requested ts, %d created before requested)", len(observations), len(ev), forcedOrPeriodic, notTryjobMatch, tryjobsStr, emptyCreatedTs, emptyRequestedTs, createdBeforeRequested)
	return observations, nil
}

// computeCreatedToStartedLatencies is an events.ObservationsFn that computes
// the average latency between a job's Created and Started timestamps.
func computeCreatedToStartedLatencies(ev []*events.Event, tryjobs bool) ([]float64, error) {
	if len(ev) == 0 {
		return []float64{}, nil
	}
	var observations []float64
	for _, e := range ev {
		var job types.Job
		if err := gob.NewDecoder(bytes.NewReader(e.Data)).Decode(&job); err != nil {
			return nil, skerr.Wrapf(err, "Failed to decode job")
		}
		// Forced jobs and periodic jobs will incorrectly skew the average.
		if job.IsForce || isPeriodic(&job) {
			continue
		}
		// Filter out try jobs or normal jobs as appropriate.
		if tryjobs != job.IsTryJob() {
			continue
		}
		// Guard against missing or incorrect Created or Started timestamps.
		if util.TimeIsZero(job.Created) {
			sklog.Warningf("Job %s has an empty Created timestamp", job.Id)
			continue
		}
		if util.TimeIsZero(job.Started) {
			// We may have Jobs in the DB that aren't Started yet; this is
			// normal.
			continue
		}
		if job.Started.Before(job.Created) {
			sklog.Warningf("Job %s has a Started timestamp before its Created timestamp", job.Id)
			continue
		}
		observations = append(observations, job.Started.Sub(job.Created).Seconds())
	}
	return observations, nil
}

// computeStartedToFirstTaskLatencies is an events.ObservationsFn that computes
// the average time between a job's Started timestamp and the time that its
// first task was triggered.
func computeStartedToFirstTaskLatencies(ev []*events.Event) ([]float64, error) {
	if len(ev) == 0 {
		return []float64{}, nil
	}
	var observations []float64
	for _, e := range ev {
		var job types.Job
		if err := gob.NewDecoder(bytes.NewReader(e.Data)).Decode(&job); err != nil {
			return nil, skerr.Wrapf(err, "Failed to decode job")
		}
		// Forced jobs and periodic jobs will incorrectly skew the average.
		if job.IsForce || isPeriodic(&job) {
			continue
		}
		if util.TimeIsZero(job.Started) {
			// We may have Jobs in the DB that aren't Started yet; this is
			// normal.
			continue
		}
		firstTaskTriggered := time.Time{}
		for _, tasks := range job.Tasks {
			for _, task := range tasks {
				if util.TimeIsZero(firstTaskTriggered) || task.Created.Before(firstTaskTriggered) {
					firstTaskTriggered = task.Created
				}
			}
		}
		if util.TimeIsZero(firstTaskTriggered) {
			// We may have Jobs in the DB whose first task hasn't been triggered
			// yet; this is normal.
			continue
		}
		if firstTaskTriggered.Before(job.Started) {
			sklog.Warningf("Job %s has a first-task-triggered timestamp before its Started timestamp", job.Id)
			continue
		}
		observations = append(observations, firstTaskTriggered.Sub(job.Started).Seconds())
	}
	return observations, nil
}

// addJobAggregates adds aggregation functions for job events to the EventStream.
func addJobAggregates(s *events.EventStream, instance string) error {
	for _, period := range timePeriods {
		// Average job duration.
		if err := s.DynamicMetric(map[string]string{"metric": "avg-duration", "instance": instance}, period, computeAvgJobDuration); err != nil {
			return err
		}

		// Job failure/mishap rate.
		if err := s.DynamicMetric(map[string]string{"instance": instance}, period, computeJobFailureMishapRate); err != nil {
			return err
		}

		// Lag time between commit landing and job creation.
		if err := s.ComputeStatsMetric(map[string]string{
			"metric":   measurementJobCreationLag,
			"instance": instance,
			// The metrics package crashes if we don't use the same
			// set of tags for every Aggregate/Dynamic metric...
			"job_name": "",
			"job_type": string(jobType_Normal),
		}, period, makeObservationsFn(computeJobCreationLatencies, false)); err != nil {
			return err
		}

		// Lag time between try request and job creation.
		if err := s.ComputeStatsMetric(map[string]string{
			"metric":   measurementJobCreationLag,
			"instance": instance,
			// The metrics package crashes if we don't use the same
			// set of tags for every Aggregate/Dynamic metric...
			"job_name": "",
			"job_type": string(jobType_TryJob),
		}, period, makeObservationsFn(computeJobCreationLatencies, true)); err != nil {
			return err
		}

		// Lag time between job creation and job starting.
		// Normal jobs.
		if err := s.ComputeStatsMetric(map[string]string{
			"metric":   measurementJobStartedLag,
			"instance": instance,
			// The metrics package crashes if we don't use the same
			// set of tags for every Aggregate/Dynamic metric...
			"job_name": "",
			"job_type": string(jobType_Normal),
		}, period, makeObservationsFn(computeCreatedToStartedLatencies, false)); err != nil {
			return err
		}
		// Try jobs.
		if err := s.ComputeStatsMetric(map[string]string{
			"metric":   measurementJobStartedLag,
			"instance": instance,
			// The metrics package crashes if we don't use the same
			// set of tags for every Aggregate/Dynamic metric...
			"job_name": "",
			"job_type": string(jobType_TryJob),
		}, period, makeObservationsFn(computeCreatedToStartedLatencies, true)); err != nil {
			return err
		}

		// Lag time between job starting and first task triggered.
		if err := s.ComputeStatsMetric(map[string]string{
			"metric":   measurementFirstTaskTriggeredLag,
			"instance": instance,
			// The metrics package crashes if we don't use the same
			// set of tags for every Aggregate/Dynamic metric...
			"job_name": "",
			"job_type": "",
		}, period, computeStartedToFirstTaskLatencies); err != nil {
			return err
		}
	}
	return nil
}

// StartJobMetrics starts a goroutine which ingests metrics data based on Jobs.
// The caller is responsible for updating the passed-in repos and TaskCfgCache.
func StartJobMetrics(ctx context.Context, jCache cache.JobCache, w window.Window, instance string, repos repograph.Map, tcc *task_cfg_cache.TaskCfgCacheImpl) error {
	edb := &jobEventDB{
		cached: []*events.Event{},
		jCache: jCache,
	}
	em, err := events.NewEventMetrics(edb, "job_metrics")
	if err != nil {
		return err
	}
	edb.em = em

	s := em.GetEventStream(jobStream)
	if err := addJobAggregates(s, instance); err != nil {
		return err
	}

	om, err := newOverdueJobMetrics(jCache, repos, tcc, w)
	if err != nil {
		return err
	}
	om.start(ctx)

	lv := metrics2.NewLiveness("last_successful_job_metrics_update")
	go util.RepeatCtx(ctx, 5*time.Minute, func(ctx context.Context) {
		if err := edb.update(); err != nil {
			sklog.Errorf("Failed to update job data: %s", err)
		} else {
			lv.Reset()
		}
	})
	em.Start(ctx)
	return nil
}

// jobSpecMetricKey is a map key for overdueJobMetrics.overdueMetrics. The tags added to the
// metric are reflected here so that we delete/recreate the metric when the tags change.
type jobSpecMetricKey struct {
	// Repo URL.
	Repo string
	// Job name.
	Job string
	// Job trigger.
	Trigger string
}

type overdueJobMetrics struct {
	// Metric for age of commit with no completed job, in seconds.
	overdueMetrics map[jobSpecMetricKey]metrics2.Int64Metric

	// Metric for age of last-created job by name, in seconds.
	prevLatestJobAge map[jobSpecMetricKey]metrics2.Int64Metric

	jCache       cache.JobCache
	repos        repograph.Map
	taskCfgCache *task_cfg_cache.TaskCfgCacheImpl
	window       window.Window
}

// Return an overdueJobMetrics instance. The caller is responsible for updating
// the passed-in repos and TaskCfgCache.
func newOverdueJobMetrics(jCache cache.JobCache, repos repograph.Map, tcc *task_cfg_cache.TaskCfgCacheImpl, w window.Window) (*overdueJobMetrics, error) {
	return &overdueJobMetrics{
		overdueMetrics: map[jobSpecMetricKey]metrics2.Int64Metric{},
		jCache:         jCache,
		repos:          repos,
		taskCfgCache:   tcc,
		window:         w,
	}, nil
}

func (m *overdueJobMetrics) start(ctx context.Context) {
	lvOverdueMetrics := metrics2.NewLiveness("last_successful_overdue_metrics_update")
	go util.RepeatCtx(ctx, 5*time.Second, func(ctx context.Context) {
		if err := m.updateOverdueJobSpecMetrics(ctx, time.Now()); err != nil {
			sklog.Errorf("Failed to update metrics for overdue job specs: %s", err)
		} else {
			lvOverdueMetrics.Reset()
		}
	})
}

// getMostRecentCachedRev returns the Commit and TasksCfg for the most recent
// commit which has an entry in the TaskCfgCache.
func getMostRecentCachedRev(ctx context.Context, tcc *task_cfg_cache.TaskCfgCacheImpl, repoUrl string, repo *repograph.Graph) (*repograph.Commit, *specs.TasksCfg, error) {
	head := repo.Get(git.MainBranch)
	if head == nil {
		head = repo.Get(git.MainBranch)
	}
	if head == nil {
		return nil, nil, skerr.Fmt("Can't resolve %q or %q in %q.", git.MainBranch, git.MainBranch, repoUrl)
	}
	var commit *repograph.Commit
	var cfg *specs.TasksCfg
	if err := head.Recurse(func(c *repograph.Commit) error {
		tasksCfg, cachedErr, err := tcc.Get(ctx, types.RepoState{
			Repo:     repoUrl,
			Revision: c.Hash,
		})
		if err == task_cfg_cache.ErrNoSuchEntry || cachedErr != nil {
			return nil
		} else if err != nil {
			return err
		}
		cfg = tasksCfg
		commit = c
		return repograph.ErrStopRecursing
	}); err != nil {
		return nil, nil, err
	}
	return commit, cfg, nil
}

// updateOverdueJobSpecMetrics updates metrics for MEASUREMENT_OVERDUE_JOB_SPECS.
func (m *overdueJobMetrics) updateOverdueJobSpecMetrics(ctx context.Context, now time.Time) error {
	defer metrics2.FuncTimer().Stop()

	// Update the window and cache.
	if err := m.window.Update(ctx); err != nil {
		return err
	}
	if err := m.jCache.Update(ctx); err != nil {
		return err
	}

	latestJobAge := make(map[jobSpecMetricKey]metrics2.Int64Metric, len(m.prevLatestJobAge))
	// Process each repo individually.
	for repoUrl, repo := range m.repos {
		// Include only the jobs at current tip (or most recently cached
		// commit). We don't report on JobSpecs that have been removed.
		head, headTaskCfg, err := getMostRecentCachedRev(ctx, m.taskCfgCache, repoUrl, repo)
		if err != nil {
			return err
		}
		// Set of JobSpec names left to process.
		todo := util.StringSet{}
		// Maps JobSpec name to time of oldest untested commit; initialized to 'now' and updated each
		// time we see an untested commit.
		times := map[string]time.Time{}
		for name := range headTaskCfg.Jobs {
			todo[name] = true
			times[name] = now
		}

		// Iterate backwards to find the most-recently tested commit. We're not going to worry about
		// merges -- if a job was run on both branches, we'll use the first commit we come across.
		if err := head.Recurse(func(c *repograph.Commit) error {
			// Stop if this commit is outside the scheduling window.
			if in, err := m.window.TestCommitHash(repoUrl, c.Hash); err != nil {
				return skerr.Fmt("TestCommitHash: %s", err)
			} else if !in {
				return repograph.ErrStopRecursing
			}
			rs := types.RepoState{
				Repo:     repoUrl,
				Revision: c.Hash,
			}
			// Look in the cache for each remaining JobSpec at this commit.
			for name := range todo {
				jobs, err := m.jCache.GetJobsByRepoState(name, rs)
				if err != nil {
					return skerr.Fmt("GetJobsByRepoState: %s", err)
				}
				for _, j := range jobs {
					if j.Done() {
						delete(todo, name)
						break
					}
				}
			}
			if len(todo) == 0 {
				return repograph.ErrStopRecursing
			}
			// Check that the remaining JobSpecs are still valid at this commit.
			taskCfg, cachedErr, err := m.taskCfgCache.Get(ctx, rs)
			if cachedErr != nil {
				// The TasksCfg is invalid at this revision. Stop recursing.
				for name := range todo {
					delete(todo, name)
				}
			} else if err != nil {
				return skerr.Fmt("Error reading TaskCfg for %q at %q: %s", repoUrl, c.Hash, err)
			}
			for name := range todo {
				if _, ok := taskCfg.Jobs[name]; !ok {
					delete(todo, name)
				} else {
					// Set times, since this job still exists and we haven't seen a completed job.
					times[name] = c.Timestamp
				}
			}
			if len(todo) > 0 {
				return nil
			}
			return repograph.ErrStopRecursing
		}); err != nil {
			return err
		}
		// Delete metrics for jobs that have been removed or whose tags have changed.
		for key, metric := range m.overdueMetrics {
			if key.Repo != repoUrl {
				continue
			}
			if jobCfg, ok := headTaskCfg.Jobs[key.Job]; !ok || jobCfg.Trigger != key.Trigger {
				if err := metric.Delete(); err != nil {
					sklog.Errorf("Failed to delete metric: %s", err)
					// Set to 0; we'll attempt to remove on the next cycle.
					metric.Update(0)
				} else {
					delete(m.overdueMetrics, key)
				}
			}
		}
		// Update metrics or add metrics for any new jobs (or other tag changes).
		for name, ts := range times {
			key := jobSpecMetricKey{
				Repo:    repoUrl,
				Job:     name,
				Trigger: headTaskCfg.Jobs[name].Trigger,
			}
			metric, ok := m.overdueMetrics[key]
			if !ok {
				metric = metrics2.GetInt64Metric(measurementOverdueJobSpecs, map[string]string{
					"repo":        key.Repo,
					"job_name":    key.Job,
					"job_trigger": key.Trigger,
				})
				m.overdueMetrics[key] = metric
			}
			metric.Update(int64(now.Sub(ts).Seconds()))
		}

		// Record the age of the most-recently created job for each
		// JobSpec.
		names := []string{}
		for name, jobSpec := range headTaskCfg.Jobs {
			// We're only interested in periodic jobs for this metric.
			if util.In(jobSpec.Trigger, specs.PERIODIC_TRIGGERS) {
				names = append(names, name)
			}
		}
		jobsByName, err := m.jCache.GetMatchingJobsFromDateRange(names, m.window.Start(repoUrl), now)
		if err != nil {
			return err
		}
		for name, jobs := range jobsByName {
			key := jobSpecMetricKey{
				Repo:    repoUrl,
				Job:     name,
				Trigger: headTaskCfg.Jobs[name].Trigger,
			}
			latest := time.Time{}
			for _, job := range jobs {
				if !job.IsTryJob() && !job.IsForce && job.Created.After(latest) {
					latest = job.Created
				}
			}
			metric := metrics2.GetInt64Metric(measurementLatestJobAge, map[string]string{
				"repo":        key.Repo,
				"job_name":    key.Job,
				"job_trigger": key.Trigger,
			})
			metric.Update(int64(now.Sub(latest).Seconds()))
			latestJobAge[key] = metric
		}
	}
	for key, metric := range m.prevLatestJobAge {
		if _, ok := latestJobAge[key]; !ok {
			if err := metric.Delete(); err != nil {
				sklog.Errorf("Failed to delete metric: %s", err)
				// Set to 0 and attempt to remove on the next cycle.
				metric.Update(0)
				latestJobAge[key] = metric
			}
		}
	}
	m.prevLatestJobAge = latestJobAge
	return nil
}
