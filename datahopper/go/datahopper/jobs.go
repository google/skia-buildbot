package main

/*
	Jobs metrics.
*/

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"time"

	"golang.org/x/net/context"

	"go.skia.org/infra/go/metrics2/events"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/remote_db"
)

const (
	// STREAM_JOBS is the name of the jobs stream.
	STREAM_JOBS = "jobs"
)

// jobEventDB implements the events.EventDB interface.
type jobEventDB struct {
	db.JobReader
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
	if stream != STREAM_JOBS {
		return []*events.Event{}, nil
	}

	jobs, err := j.GetJobsFromDateRange(start, end)
	if err != nil {
		return nil, err
	}
	rv := make([]*events.Event, 0, len(jobs))
	for _, job := range jobs {
		if !job.Done() {
			continue
		}
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(job); err != nil {
			return nil, err
		}
		rv = append(rv, &events.Event{
			Stream:    STREAM_JOBS,
			Timestamp: job.Created,
			Data:      buf.Bytes(),
		})
	}
	return rv, nil
}

// addAggregates adds aggregation functions for job events to the EventStream.
func addAggregates(s *events.EventStream) error {
	for _, period := range []time.Duration{24 * time.Hour, 7 * 24 * time.Hour} {
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
		db,
	}
	em, err := events.NewEventMetrics(edb)
	if err != nil {
		return err
	}
	s := em.GetEventStream(STREAM_JOBS)
	if err := addAggregates(s); err != nil {
		return err
	}
	em.Start(ctx)
	return nil
}
