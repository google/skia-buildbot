package main

/*
	Jobs metrics.
*/

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"sync"
	"time"

	"golang.org/x/net/context"

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

// jobEventDB implements the events.EventDB interface.
type jobEventDB struct {
	cached  map[string][]*events.Event
	db      db.JobReader
	em      *events.EventMetrics
	metrics map[string]bool
	mtx     sync.Mutex
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
	rv := make([]*events.Event, 0, len(j.cached[stream]))
	for _, ev := range j.cached[stream] {
		if !start.After(ev.Timestamp) && ev.Timestamp.Before(end) {
			rv = append(rv, ev)
		}
	}
	return rv, nil
}

// update updates the cached jobs in the jobEventDB.
func (j *jobEventDB) update() error {
	j.mtx.Lock()
	defer j.mtx.Unlock()
	now := time.Now()
	longestPeriod := TIME_PERIODS[len(TIME_PERIODS)-1]
	jobs, err := j.db.GetJobsFromDateRange(now.Add(-longestPeriod), now)
	if err != nil {
		return err
	}
	cached := map[string][]*events.Event{}
	for _, job := range jobs {
		if !job.Done() {
			continue
		}
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(job); err != nil {
			return err
		}
		ev := &events.Event{
			Stream:    job.Name,
			Timestamp: job.Created,
			Data:      buf.Bytes(),
		}
		cached[job.Name] = append(cached[job.Name], ev)

		// TODO(borenet): Need to think about what happens when jobs are
		// removed or renamed. As written, we'll continue to report
		// metrics on defunct jobs until datahopper is restarted. There
		// isn't currently a way to remove metrics from EventMetrics. If
		// we added that, we could diff the jobs before and after and
		// remove metrics for those we don't see in the window.
		if !j.metrics[job.Name] {
			s := j.em.GetEventStream(job.Name)
			if err := addAggregates(s); err != nil {
				return err
			}
			j.metrics[job.Name] = true
		}
	}
	j.cached = cached
	return nil
}

// addAggregates adds aggregation functions for job events to the EventStream.
func addAggregates(s *events.EventStream) error {
	for _, period := range TIME_PERIODS {
		// Average Job duration.
		if err := s.AggregateMetric(map[string]string{"metric": "avg-duration"}, period, func(ev []*events.Event) (float64, error) {
			count := 0
			total := time.Duration(0)
			for _, e := range ev {
				var job db.Job
				if err := gob.NewDecoder(bytes.NewBuffer(e.Data)).Decode(&job); err != nil {
					return 0.0, err
				}
				if !(job.Status == db.JOB_STATUS_SUCCESS || job.Status == db.JOB_STATUS_FAILURE) {
					continue
				}
				count++
				total += job.Finished.Sub(job.Created)
			}
			if count == 0 {
				return 0.0, nil
			}
			return float64(total) / float64(count), nil
		}); err != nil {
			return err
		}

		// Job failure rate.
		if err := s.AggregateMetric(map[string]string{"metric": "failure-rate"}, period, func(ev []*events.Event) (float64, error) {
			count := 0
			fails := 0
			for _, e := range ev {
				var job db.Job
				if err := gob.NewDecoder(bytes.NewBuffer(e.Data)).Decode(&job); err != nil {
					return 0.0, err
				}
				count++
				if job.Status == db.JOB_STATUS_FAILURE {
					fails++
				}
			}
			if count == 0 {
				return 0.0, nil
			}
			return float64(fails) / float64(count), nil
		}); err != nil {
			return err
		}

		// Job mishap rate.
		if err := s.AggregateMetric(map[string]string{"metric": "mishap-rate"}, period, func(ev []*events.Event) (float64, error) {
			count := 0
			mishap := 0
			for _, e := range ev {
				var job db.Job
				if err := gob.NewDecoder(bytes.NewBuffer(e.Data)).Decode(&job); err != nil {
					return 0.0, err
				}
				count++
				if job.Status == db.JOB_STATUS_MISHAP {
					mishap++
				}
			}
			if count == 0 {
				return 0.0, nil
			}
			return float64(mishap) / float64(count), nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// StartJobMetrics starts a goroutine which ingests metrics data based on Jobs.
func StartJobMetrics(taskSchedulerDbUrl string, ctx context.Context) error {
	db, err := remote_db.NewClient(taskSchedulerDbUrl)
	if err != nil {
		return err
	}
	edb := &jobEventDB{
		cached:  map[string][]*events.Event{},
		db:      db,
		metrics: map[string]bool{},
	}
	em, err := events.NewEventMetrics(edb)
	if err != nil {
		return err
	}
	edb.em = em
	lv := metrics2.NewLiveness("last-successful-job-metrics-update")
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
