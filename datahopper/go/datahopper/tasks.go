package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"sort"
	"sync"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/metrics2/events"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/flakes"
	"go.skia.org/infra/task_scheduler/go/types"
)

const (
	TASK_STREAM = "task_metrics"
)

// taskEventDB implements the events.EventDB interface.
type taskEventDB struct {
	cached []*events.Event
	tCache cache.TaskCache
	em     *events.EventMetrics
	// Do not lock mtx when calling methods on em. Otherwise deadlock can occur.
	// E.g. (now fixed):
	// 1. Thread 1: EventManager.updateMetrics locks EventMetrics.mtx
	// 2. Thread 2: taskEventDB.update locks taskEventDB.mtx
	// 3. Thread 1: EventManager.updateMetrics calls taskEventDB.Range, which waits
	//    to lock taskEventDB.mtx
	// 4. Thread 2: taskEventDB.update calls EventMetrics.AggregateMetric (by way
	//    of addTaskAggregates and EventStream.AggregateMetric), which waits to lock
	//    EventMetrics.mtx
	mtx sync.Mutex
}

// See docs for events.EventDB interface.
func (t *taskEventDB) Append(string, []byte) error {
	return fmt.Errorf("taskEventDB is read-only!")
}

// See docs for events.EventDB interface.
func (t *taskEventDB) Close() error {
	return nil
}

// See docs for events.EventDB interface.
func (t *taskEventDB) Insert(*events.Event) error {
	return fmt.Errorf("taskEventDB is read-only!")
}

// See docs for events.EventDB interface.
func (t *taskEventDB) Range(stream string, start, end time.Time) ([]*events.Event, error) {
	t.mtx.Lock()
	defer t.mtx.Unlock()

	n := len(t.cached)
	if n == 0 {
		return []*events.Event{}, nil
	}

	first := sort.Search(n, func(i int) bool {
		return !t.cached[i].Timestamp.Before(start)
	})

	last := first + sort.Search(n-first, func(i int) bool {
		return !t.cached[i+first].Timestamp.Before(end)
	})

	rv := make([]*events.Event, last-first, last-first)
	copy(rv[:], t.cached[first:last])
	return rv, nil
}

// update updates the cached tasks in the taskEventDB. Only a single thread may
// call this method, but it can be called concurrently with other methods.
func (t *taskEventDB) update() error {
	defer metrics2.FuncTimer().Stop()
	if err := t.tCache.Update(context.TODO()); err != nil {
		return skerr.Wrapf(err, "Failed to update cache")
	}
	now := time.Now()
	longestPeriod := TIME_PERIODS[len(TIME_PERIODS)-1]
	tasks, err := t.tCache.GetTasksFromDateRange(now.Add(-longestPeriod), now)
	if err != nil {
		return skerr.Wrapf(err, "Failed to load tasks from %s to %s", now.Add(-longestPeriod), now)
	}
	sklog.Debugf("taskEventDB.update: Processing %d tasks for time range %s to %s.", len(tasks), now.Add(-longestPeriod), now)
	cached := make([]*events.Event, 0, len(tasks))
	for _, task := range tasks {
		if !task.Done() {
			continue
		}
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(task); err != nil {
			return skerr.Wrapf(err, "Failed to encode %#v to GOB", task)
		}
		ev := &events.Event{
			Stream:    TASK_STREAM,
			Timestamp: task.Created,
			Data:      buf.Bytes(),
		}
		cached = append(cached, ev)
	}
	t.mtx.Lock()
	defer t.mtx.Unlock()
	t.cached = cached
	return nil
}

// computeTaskFlakeRate is an events.DynamicAggregateFn that returns metrics for Task flake rate, given
// a slice of Events created by taskEventDB.update. The first return value will contain the tags
// "task_name" (types.Task.Name) and "metric" (one of "failure-rate", "mishap-rate"),
// and the second return value will be the corresponding ratio of failed/mishap Task to all
// completed Tasks. Returns an error if Event.Data can't be GOB-decoded as a types.Task.
func computeTaskFlakeRate(ev []*events.Event) ([]map[string]string, []float64, error) {
	if len(ev) > 0 {
		// ev should be ordered by timestamp
		sklog.Debugf("Calculating flake-rate for %d tasks since %s.", len(ev), ev[0].Timestamp)
	}
	type taskSum struct {
		flakes int
		count  int
	}
	byTask := map[string]*taskSum{}
	tasks := make([]*types.Task, 0, len(ev))
	for _, e := range ev {
		var task types.Task
		if err := gob.NewDecoder(bytes.NewReader(e.Data)).Decode(&task); err != nil {
			return nil, nil, err
		}
		tasks = append(tasks, &task)
		entry, ok := byTask[task.Name]
		if !ok {
			entry = &taskSum{}
			byTask[task.Name] = entry
		}
		entry.count++
	}
	flaky := flakes.FindFlakes(tasks)
	for _, task := range flaky {
		byTask[task.Name].flakes++
	}
	rvTags := make([]map[string]string, 0, len(byTask)*2)
	rvVals := make([]float64, 0, len(byTask)*2)
	add := func(taskName, metric string, value float64) {
		rvTags = append(rvTags, map[string]string{
			"task_name": taskName,
			"metric":    metric,
		})
		rvVals = append(rvVals, value)
	}
	for taskName, taskSum := range byTask {
		if taskSum.count == 0 {
			continue
		}
		add(taskName, "flake-rate", float64(taskSum.flakes)/float64(taskSum.count))
	}
	return rvTags, rvVals, nil
}

// addTaskAggregates adds aggregation functions for job events to the EventStream.
func addTaskAggregates(s *events.EventStream, instance string) error {
	for _, period := range TIME_PERIODS {
		// Flake rate.
		if err := s.DynamicMetric(map[string]string{"instance": instance}, period, computeTaskFlakeRate); err != nil {
			return err
		}
	}
	return nil
}

// StartTaskMetrics starts a goroutine which ingests metrics data based on Tasks.
func StartTaskMetrics(ctx context.Context, tCache cache.TaskCache, instance string) error {
	edb := &taskEventDB{
		cached: []*events.Event{},
		tCache: tCache,
	}
	em, err := events.NewEventMetrics(edb, "task_metrics")
	if err != nil {
		return err
	}
	edb.em = em

	s := em.GetEventStream(TASK_STREAM)
	if err := addTaskAggregates(s, instance); err != nil {
		return err
	}

	lv := metrics2.NewLiveness("last_successful_task_metrics_update")
	go util.RepeatCtx(ctx, 5*time.Minute, func(ctx context.Context) {
		if err := edb.update(); err != nil {
			sklog.Errorf("Failed to update task data: %s", err)
		} else {
			lv.Reset()
		}
	})
	em.Start(ctx)
	return nil
}
