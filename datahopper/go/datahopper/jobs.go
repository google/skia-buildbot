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
	"sync"
	"time"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/metrics2/events"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/remote_db"
)

var (
	// Time periods over which to compute metrics. Assumed to be sorted in
	// increasing order.
	TIME_PERIODS = []time.Duration{24 * time.Hour, 7 * 24 * time.Hour}
)

const (
	JOB_STREAM = "job_metrics"
)

// jobTypeString is an enum of the types of Jobs that computeAvgJobDuration will aggregate separately.
// The string value is the same as the job_type tag value returned by computeAvgJobDuration.
type jobTypeString string

const (
	NORMAL jobTypeString = "normal"
	FORCED jobTypeString = "forced"
	TRYJOB jobTypeString = "tryjob"
)

// jobEventDB implements the events.EventDB interface.
type jobEventDB struct {
	cached []*events.Event
	db     db.JobReader
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
	now := time.Now()
	longestPeriod := TIME_PERIODS[len(TIME_PERIODS)-1]
	jobs, err := j.db.GetJobsFromDateRange(now.Add(-longestPeriod), now)
	if err != nil {
		return fmt.Errorf("Failed to load jobs from %s to %s: %s", now.Add(-longestPeriod), now, err)
	}
	sklog.Debugf("jobEventDB.update: Processing %d jobs for time range %s to %s.", len(jobs), now.Add(-longestPeriod), now)
	cached := make([]*events.Event, 0, len(jobs))
	for _, job := range jobs {
		if !job.Done() {
			continue
		}
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(job); err != nil {
			return fmt.Errorf("Failed to encode %#v to GOB: %s", job, err)
		}
		ev := &events.Event{
			Stream:    JOB_STREAM,
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
// The first return value will contain the tags "job_name" (db.Job.Name) and "job_type" (one of
// "normal", "tryjob", "forced"), and the second return value will be the corresponding average Job
// duration. Returns an error if Event.Data can't be GOB-decoded as a db.Job.
func computeAvgJobDuration(ev []*events.Event) ([]map[string]string, []float64, error) {
	if len(ev) > 0 {
		// ev should be ordered by timestamp
		sklog.Debugf("Calculating avg-duration for %d jobs since %s.", len(ev), ev[0].Timestamp)
	}
	type sum struct {
		count int
		total time.Duration
	}
	type jobSums struct {
		normal sum
		tryjob sum
		forced sum
	}
	byJob := map[string]*jobSums{}
	for _, e := range ev {
		var job db.Job
		if err := gob.NewDecoder(bytes.NewReader(e.Data)).Decode(&job); err != nil {
			return nil, nil, err
		}
		if !(job.Status == db.JOB_STATUS_SUCCESS || job.Status == db.JOB_STATUS_FAILURE) {
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
		jobSum.total += job.Finished.Sub(job.Created)
	}

	rvTags := make([]map[string]string, 0, len(byJob)*3)
	rvVals := make([]float64, 0, len(byJob)*3)
	add := func(jobName string, jobType jobTypeString, jobSum sum) {
		if jobSum.count == 0 {
			return
		}
		value := float64(jobSum.total) / float64(jobSum.count)
		rvTags = append(rvTags, map[string]string{
			"job_name": jobName,
			"job_type": string(jobType),
		})
		rvVals = append(rvVals, value)
	}
	for jobName, jobSums := range byJob {
		add(jobName, NORMAL, jobSums.normal)
		add(jobName, TRYJOB, jobSums.tryjob)
		add(jobName, FORCED, jobSums.forced)
	}
	return rvTags, rvVals, nil
}

// computeJobFailureMishapRate is an events.DynamicAggregateFn that returns metrics for Job failure rate and
// mishap rate, given a slice of Events created by jobEventDB.update. The first return value will
// contain the tags "job_name" (db.Job.Name) and "metric" (one of "failure-rate", "mishap-rate"),
// and the second return value will be the corresponding ratio of failed/mishap Jobs to all
// completed Jobs. Returns an error if Event.Data can't be GOB-decoded as a db.Job.
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
		var job db.Job
		if err := gob.NewDecoder(bytes.NewReader(e.Data)).Decode(&job); err != nil {
			return nil, nil, err
		}
		entry, ok := byJob[job.Name]
		if !ok {
			entry = &jobSum{}
			byJob[job.Name] = entry
		}
		entry.count++
		if job.Status == db.JOB_STATUS_FAILURE {
			entry.fails++
		} else if job.Status == db.JOB_STATUS_MISHAP {
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

// addJobAggregates adds aggregation functions for job events to the EventStream.
func addJobAggregates(s *events.EventStream) error {
	for _, period := range TIME_PERIODS {
		if err := s.DynamicMetric(map[string]string{"metric": "avg-duration"}, period, computeAvgJobDuration); err != nil {
			return err
		}

		// Job failure/mishap rate.
		if err := s.DynamicMetric(nil, period, computeJobFailureMishapRate); err != nil {
			return err
		}
	}
	return nil
}

// StartJobMetrics starts a goroutine which ingests metrics data based on Jobs.
func StartJobMetrics(taskSchedulerDbUrl string, ctx context.Context) error {
	db, err := remote_db.NewClient(taskSchedulerDbUrl, httputils.NewTimeoutClient())
	if err != nil {
		return err
	}
	edb := &jobEventDB{
		cached: []*events.Event{},
		db:     db,
	}
	em, err := events.NewEventMetrics(edb, "job_metrics")
	if err != nil {
		return err
	}
	edb.em = em

	s := em.GetEventStream(JOB_STREAM)
	if err := addJobAggregates(s); err != nil {
		return err
	}

	lv := metrics2.NewLiveness("last_successful_job_metrics_update")
	go util.RepeatCtx(5*time.Minute, ctx, func() {
		if err := edb.update(); err != nil {
			sklog.Errorf("Failed to update job data: %s", err)
		} else {
			lv.Reset()
		}
	})
	em.Start(ctx)
	return nil
}
