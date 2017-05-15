package swarming_metrics

/*
	Package swarming_metrics generates metrics from Swarming data.
*/

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"path"
	"time"

	swarming_api "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/metrics2/events"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
)

const (
	MEASUREMENT_SWARMING_TASKS = "swarming-tasks"
	STREAM_SWARMING_TASKS      = "swarming-tasks"
)

var (
	errNoValue = fmt.Errorf("no value")
)

// loadSwarmingTasks loads the Swarming tasks which were created within the
// given time range, plus any tasks we're explicitly told to load. Inserts all
// completed tasks into the EventDB and returns any unfinished tasks so that
// they can be revisited later.
func loadSwarmingTasks(s swarming.ApiClient, edb events.EventDB, lastLoad, now time.Time, revisit []string) ([]string, error) {
	sklog.Info("Loading swarming tasks.")

	// TODO(borenet): Load tasks for all pools we care about, including
	// internal.
	tasks, err := s.ListSkiaTasks(lastLoad, now)
	if err != nil {
		return nil, err
	}
	for _, id := range revisit {
		task, err := s.GetTaskMetadata(id)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	revisitLater := []string{}
	loaded := 0
	for _, t := range tasks {
		// Don't include de-duped tasks, as they'll skew the metrics down.
		if t.TaskResult.DedupedFrom != "" {
			continue
		}
		// Only include finished tasks.
		if t.TaskResult.State != swarming.TASK_STATE_COMPLETED {
			revisitLater = append(revisitLater, t.TaskId)
			continue
		}
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(t); err != nil {
			return nil, fmt.Errorf("Failed to serialize Swarming task: %s", err)
		}
		created, err := swarming.Created(t)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse Created time: %s", err)
		}
		if err := edb.Insert(&events.Event{
			Stream:    STREAM_SWARMING_TASKS,
			Timestamp: created,
			Data:      buf.Bytes(),
		}); err != nil {
			return nil, fmt.Errorf("Failed to insert event: %s", err)
		}
		loaded++
	}
	sklog.Infof("... loaded %d swarming tasks.", loaded)
	return revisitLater, nil
}

// decodeTasks decodes a slice of events.Event into a slice of Swarming task
// metadata.
func decodeTasks(ev []*events.Event) ([]*swarming_api.SwarmingRpcsTaskRequestMetadata, error) {
	rv := make([]*swarming_api.SwarmingRpcsTaskRequestMetadata, 0, len(ev))
	for _, e := range ev {
		var t swarming_api.SwarmingRpcsTaskRequestMetadata
		if err := gob.NewDecoder(bytes.NewBuffer(e.Data)).Decode(&t); err != nil {
			return nil, err
		}
		rv = append(rv, &t)
	}
	return rv, nil
}

// addMetric adds a dynamic metric to the given event stream with the given
// metric name. It aggregates the data points returned by the given helper
// function and computes the mean for each tag set. This simplifies the addition
// of metrics in StartSwarmingTaskMetrics.
func addMetric(s *events.EventStream, metric string, period time.Duration, fn func(*swarming_api.SwarmingRpcsTaskRequestMetadata) (int64, error)) error {
	tags := map[string]string{
		"metric": metric,
	}
	f := func(ev []*events.Event) ([]map[string]string, []float64, error) {
		if len(ev) == 0 {
			return []map[string]string{}, []float64{}, nil
		}
		tasks, err := decodeTasks(ev)
		if err != nil {
			return nil, nil, err
		}
		tagSets := map[string]map[string]string{}
		totals := map[string]int64{}
		counts := map[string]int{}
		for _, t := range tasks {
			val, err := fn(t)
			if err == errNoValue {
				continue
			}
			if err != nil {
				return nil, nil, err
			}
			tags := map[string]string{
				"task-name": t.TaskResult.Name,
			}
			for _, dim := range t.Request.Properties.Dimensions {
				tags[dim.Key] = dim.Value
			}
			key, err := util.MD5Params(tags)
			if err != nil {
				return nil, nil, err
			}
			tagSets[key] = tags
			totals[key] += val
			counts[key]++
		}
		tagSetsList := make([]map[string]string, 0, len(tagSets))
		vals := make([]float64, 0, len(tagSets))
		for key, tags := range tagSets {
			tagSetsList = append(tagSetsList, tags)
			vals = append(vals, float64(totals[key])/float64(counts[key]))
		}
		return tagSetsList, vals, nil
	}
	return s.DynamicMetric(tags, period, f)
}

// taskDuration returns the duration of the task in milliseconds.
func taskDuration(t *swarming_api.SwarmingRpcsTaskRequestMetadata) (int64, error) {
	completedTime, err := swarming.Completed(t)
	if err != nil {
		return 0.0, err
	}
	startTime, err := swarming.Started(t)
	if err != nil {
		return 0.0, err
	}
	return int64(completedTime.Sub(startTime).Seconds() * float64(1000.0)), nil
}

// taskPendingTime returns the pending time of the task in milliseconds.
func taskPendingTime(t *swarming_api.SwarmingRpcsTaskRequestMetadata) (int64, error) {
	createdTime, err := swarming.Created(t)
	if err != nil {
		return 0.0, err
	}
	startTime, err := swarming.Started(t)
	if err != nil {
		return 0.0, err
	}
	return int64(startTime.Sub(createdTime).Seconds() * float64(1000.0)), nil
}

// taskOverheadBot returns the bot overhead for the task in milliseconds.
func taskOverheadBot(t *swarming_api.SwarmingRpcsTaskRequestMetadata) (int64, error) {
	if t.TaskResult.PerformanceStats == nil {
		return 0, errNoValue
	} else {
		return int64(t.TaskResult.PerformanceStats.BotOverhead * float64(1000.0)), nil
	}
}

// taskOverheadUpload returns the upload overhead for the task in milliseconds.
func taskOverheadUpload(t *swarming_api.SwarmingRpcsTaskRequestMetadata) (int64, error) {
	if t.TaskResult.PerformanceStats == nil {
		return 0, errNoValue
	} else if t.TaskResult.PerformanceStats.IsolatedUpload == nil {
		return 0, errNoValue
	} else {
		return int64(t.TaskResult.PerformanceStats.IsolatedUpload.Duration * float64(1000.0)), nil
	}
}

// taskOverheadDownload returns the download overhead for the task in milliseconds.
func taskOverheadDownload(t *swarming_api.SwarmingRpcsTaskRequestMetadata) (int64, error) {
	if t.TaskResult.PerformanceStats == nil {
		return 0, errNoValue
	} else if t.TaskResult.PerformanceStats.IsolatedDownload == nil {
		return 0, errNoValue
	} else {
		return int64(t.TaskResult.PerformanceStats.IsolatedDownload.Duration * float64(1000.0)), nil
	}
}

// setupMetrics creates the event metrics for Swarming tasks.
func setupMetrics(workdir string) (events.EventDB, *events.EventMetrics, error) {
	edb, err := events.NewEventDB(path.Join(workdir, "swarming-tasks.bdb"))
	if err != nil {
		return nil, nil, err
	}
	em, err := events.NewEventMetrics(edb, MEASUREMENT_SWARMING_TASKS)
	if err != nil {
		return nil, nil, err
	}
	s := em.GetEventStream(STREAM_SWARMING_TASKS)

	// Add metrics.
	for _, period := range []time.Duration{24 * time.Hour, 7 * 24 * time.Hour} {
		// Duration.
		if err := addMetric(s, "duration", period, taskDuration); err != nil {
			return nil, nil, err
		}

		// Pending time.
		if err := addMetric(s, "pending-time", period, taskPendingTime); err != nil {
			return nil, nil, err
		}

		// Overhead (bot).
		if err := addMetric(s, "overhead-bot", period, taskOverheadBot); err != nil {
			return nil, nil, err
		}

		// Overhead (upload).
		if err := addMetric(s, "overhead-upload", period, taskOverheadUpload); err != nil {
			return nil, nil, err
		}

		// Overhead (download).
		if err := addMetric(s, "overhead-download", period, taskOverheadDownload); err != nil {
			return nil, nil, err
		}
	}
	return edb, em, nil
}

// startLoadingTasks initiates the goroutine which periodically loads Swarming
// tasks into the EventDB.
func startLoadingTasks(swarm swarming.ApiClient, ctx context.Context, edb events.EventDB) {
	// Start collecting the metrics.
	lv := metrics2.NewLiveness("last-successful-swarming-task-metrics")
	now := time.Now()
	lastLoad := now.Add(-2 * time.Minute)
	revisitTasks := []string{}
	go util.RepeatCtx(10*time.Minute, ctx, func() {
		revisit, err := loadSwarmingTasks(swarm, edb, lastLoad, now, revisitTasks)
		if err != nil {
			sklog.Errorf("Failed to load swarming tasks into metrics: %s", err)
		} else {
			lastLoad = now
			revisitTasks = revisit
			lv.Reset()
		}
	})
}

// StartSwarmingTaskMetrics initiates a goroutine which loads Swarming task
// results and computes metrics.
func StartSwarmingTaskMetrics(workdir string, swarm swarming.ApiClient, ctx context.Context) error {
	edb, _, err := setupMetrics(workdir)
	if err != nil {
		return err
	}
	startLoadingTasks(swarm, ctx, edb)
	return nil
}
