package main

/*
	Jobs metrics.
*/

import (
	"bytes"
	"encoding/gob"
	"path"
	"time"

	"golang.org/x/net/context"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/metrics2/events"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/remote_db"
)

const (
	// The job event streams are stored in this DB file.
	DB_FILE = "job_events.bdb"
)

// jobMetrics is a struct used for collecting metrics based on jobs.
type jobMetrics struct {
	db         db.JobReader
	edb        events.EventDB
	em         *events.EventMetrics
	lastRan    time.Time
	jobStreams map[string]*events.EventStream
	queryId    string
}

// addAggregates adds aggregation functions for job events to the EventStream.
func addAggregates(s *events.EventStream) {
	for _, period := range []time.Duration{24 * time.Hour, 7 * 24 * time.Hour} {
		// Average Job duration.
		s.AggregateMetric(map[string]string{"metric": "avg-duration"}, period, func(ev []*events.Event) (float64, error) {
			count := 0
			total := time.Duration(0)
			for _, e := range ev {
				var job db.Job
				if err := gob.NewDecoder(bytes.NewBuffer(e.Data)).Decode(&job); err != nil {
					return 0.0, err
				}
				if !job.Done() {
					continue
				}
				count++
				total += job.Finished.Sub(job.Created)
			}
			if count == 0 {
				return 0.0, nil
			}
			return float64(total) / float64(count), nil
		})

		// Job failure rate.
		s.AggregateMetric(map[string]string{"metric": "failure-rate"}, period, func(ev []*events.Event) (float64, error) {
			count := 0
			fails := 0
			for _, e := range ev {
				var job db.Job
				if err := gob.NewDecoder(bytes.NewBuffer(e.Data)).Decode(&job); err != nil {
					return 0.0, err
				}
				if !job.Done() {
					continue
				}
				count++
				if job.Status != db.JOB_STATUS_SUCCESS {
					fails++
				}
			}
			if count == 0 {
				return 0.0, nil
			}
			return float64(fails) / float64(count), nil
		})
	}
}

// insertEvents inserts events for each of the given jobs into the EventDB.
func (m *jobMetrics) insertEvents(jobs []*db.Job, now time.Time) error {
	for _, job := range jobs {
		s, ok := m.jobStreams[job.Name]
		if !ok {
			s = m.em.GetEventStream(job.Name)
			m.jobStreams[job.Name] = s
			addAggregates(s)
		}
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(job); err != nil {
			return err
		}
		if err := s.Insert(&events.Event{
			Stream:    job.Name,
			Timestamp: job.Created,
			Data:      buf.Bytes(),
		}); err != nil {
			return err
		}
	}
	m.lastRan = now
	return nil
}

// reset resets the connection to the remote job database, retrieves any
// missed jobs, and inserts events into the EventDB.
func (m *jobMetrics) reset() error {
	if m.queryId != "" {
		m.db.StopTrackingModifiedJobs(m.queryId)
	}
	queryId, err := m.db.StartTrackingModifiedJobs()
	if err != nil {
		return err
	}

	// Get jobs since the last time we ran.
	now := time.Now()
	jobs, err := m.db.GetJobsFromDateRange(m.lastRan, now)
	if err != nil {
		m.db.StopTrackingModifiedJobs(queryId)
		return err
	}
	m.queryId = queryId
	return m.insertEvents(jobs, now)
}

// update loads new jobs from the DB and inserts events into the EventDB.
func (m *jobMetrics) update() error {
	now := time.Now()
	newJobs, err := m.db.GetModifiedJobs(m.queryId)
	if db.IsUnknownId(err) {
		if err := m.reset(); err != nil {
			return err
		}
		return nil
	} else if err != nil {
		return err
	}
	return m.insertEvents(newJobs, now)
}

// StartJobMetrics starts a goroutine which ingests metrics data based on Jobs.
func StartJobMetrics(workdir, taskSchedulerDbUrl string) error {
	db, err := remote_db.NewClient(taskSchedulerDbUrl)
	if err != nil {
		return err
	}
	edb, err := events.NewEventDB(path.Join(workdir, DB_FILE))
	if err != nil {
		return err
	}
	em, err := events.NewEventMetrics(edb)
	if err != nil {
		return err
	}

	m := &jobMetrics{
		db:         db,
		edb:        edb,
		em:         em,
		jobStreams: map[string]*events.EventStream{},
		lastRan:    time.Now().Add(-7 * 24 * time.Hour),
		queryId:    "",
	}
	if err := m.reset(); err != nil {
		return err
	}
	lv := metrics2.NewLiveness("time-since-last-job-metrics")
	go util.RepeatCtx(10*time.Minute, context.Background(), func() {
		if err := m.update(); err != nil {
			sklog.Errorf("Failed to update job metrics: %s", err)
		} else {
			lv.Reset()
		}
	})
	return nil
}
