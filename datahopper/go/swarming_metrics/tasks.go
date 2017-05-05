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
	STREAM_SWARMING_TASKS = "swarming-tasks"
)

var (
	errNoValue = fmt.Errorf("no value")
)

func loadSwarmingTasks(s swarming.ApiClient, edb events.EventDB, lastLoad, now time.Time, revisit []string) ([]string, error) {
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
	for _, t := range tasks {
		// Don't include de-duped tasks, as they'll skew the metrics down.
		if t.TaskResult.DedupedFrom != "" {
			continue
		}
		// Only include finished tasks.
		if t.TaskResult.State != "COMPLETED" {
			revisitLater = append(revisitLater, t.TaskId)
			continue
		}
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(t); err != nil {
			return nil, err
		}
		created, err := swarming.Created(t)
		if err != nil {
			return nil, err
		}
		if err := edb.Insert(&events.Event{
			Stream:    STREAM_SWARMING_TASKS,
			Timestamp: created,
			Data:      buf.Bytes(),
		}); err != nil {
			return nil, err
		}
	}
	return revisitLater, nil
}

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

func addMetric(s *events.EventStream, metric string, period time.Duration, fn func(*swarming_api.SwarmingRpcsTaskRequestMetadata) (int64, error)) error {
	tags := map[string]string{
		"metric": metric,
	}
	return s.DynamicMetric(tags, period, func(ev []*events.Event) ([]map[string]string, []float64, error) {
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
	})
}

func StartSwarmingTaskMetrics(workdir string, swarm swarming.ApiClient) error {
	edb, err := events.NewEventDB(path.Join(workdir, "swarming-tasks.bdb"))
	if err != nil {
		return err
	}
	em, err := events.NewEventMetrics(edb)
	if err != nil {
		return err
	}
	s := em.GetEventStream(STREAM_SWARMING_TASKS)

	// Add metrics.
	for _, period := range []time.Duration{24 * time.Hour, 7 * 24 * time.Hour} {
		// Duration.
		if err := addMetric(s, "duration", period, func(t *swarming_api.SwarmingRpcsTaskRequestMetadata) (int64, error) {
			return int64(t.TaskResult.Duration * float64(1000.0)), nil
		}); err != nil {
			return err
		}

		// Pending time.
		if err := addMetric(s, "pending-time", period, func(t *swarming_api.SwarmingRpcsTaskRequestMetadata) (int64, error) {
			createdTime, err := swarming.Created(t)
			if err != nil {
				return 0.0, err
			}
			startTime, err := swarming.Started(t)
			if err != nil {
				return 0.0, err
			}
			return int64(startTime.Sub(createdTime).Seconds() * float64(1000.0)), nil
		}); err != nil {
			return err
		}

		// Overhead (bot).
		if err := addMetric(s, "overhead-bot", period, func(t *swarming_api.SwarmingRpcsTaskRequestMetadata) (int64, error) {
			if t.TaskResult.PerformanceStats == nil {
				return 0, errNoValue
			} else {
				return int64(t.TaskResult.PerformanceStats.BotOverhead * float64(1000.0)), nil
			}
		}); err != nil {
			return err
		}

		// Overhead (upload).
		if err := addMetric(s, "overhead-upload", period, func(t *swarming_api.SwarmingRpcsTaskRequestMetadata) (int64, error) {
			if t.TaskResult.PerformanceStats == nil {
				return 0, errNoValue
			} else if t.TaskResult.PerformanceStats.IsolatedUpload == nil {
				return 0, errNoValue
			} else {
				return int64(t.TaskResult.PerformanceStats.IsolatedUpload.Duration * float64(1000.0)), nil
			}
		}); err != nil {
			return err
		}

		// Overhead (download).
		if err := addMetric(s, "overhead-download", period, func(t *swarming_api.SwarmingRpcsTaskRequestMetadata) (int64, error) {
			if t.TaskResult.PerformanceStats == nil {
				return 0, errNoValue
			} else if t.TaskResult.PerformanceStats.IsolatedDownload == nil {
				return 0, errNoValue
			} else {
				return int64(t.TaskResult.PerformanceStats.IsolatedDownload.Duration * float64(1000.0)), nil
			}
		}); err != nil {
			return err
		}
	}

	// Start collecting the metrics.
	lv := metrics2.NewLiveness("last-successful-swarming-task-metrics")
	now := time.Now()
	lastLoad := now.Add(-2 * time.Minute)
	revisitTasks := []string{}
	go util.RepeatCtx(10*time.Minute, context.Background(), func() {
		revisit, err := loadSwarmingTasks(swarm, edb, lastLoad, now, revisitTasks)
		if err != nil {
			sklog.Errorf("Failed to load swarming tasks into metrics: %s", err)
		} else {
			lastLoad = now
			revisitTasks = revisit
			lv.Reset()
		}
	})
	return nil
}
