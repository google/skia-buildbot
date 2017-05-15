package swarming_metrics

import (
	"fmt"
	"io/ioutil"
	"path"
	"testing"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/metrics2/events"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"

	swarming_api "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	assert "github.com/stretchr/testify/require"
)

func makeTask(id, name string, created, started, completed time.Time, dims map[string]string, botOverhead, downloadOverhead, uploadOverhead float64) *swarming_api.SwarmingRpcsTaskRequestMetadata {
	dimensions := make([]*swarming_api.SwarmingRpcsStringPair, 0, len(dims))
	tags := make([]string, 0, len(dims))
	for k, v := range dims {
		dimensions = append(dimensions, &swarming_api.SwarmingRpcsStringPair{
			Key:   k,
			Value: v,
		})
		tags = append(tags, fmt.Sprintf("%s:%s", k, v))
	}
	duration := 0.0
	if !util.TimeIsZero(completed) {
		duration = float64(completed.Sub(started))
	}
	return &swarming_api.SwarmingRpcsTaskRequestMetadata{
		Request: &swarming_api.SwarmingRpcsTaskRequest{
			CreatedTs: created.UTC().Format(swarming.TIMESTAMP_FORMAT),
			Properties: &swarming_api.SwarmingRpcsTaskProperties{
				Dimensions: dimensions,
			},
			Tags: tags,
		},
		TaskId: id,
		TaskResult: &swarming_api.SwarmingRpcsTaskResult{
			CreatedTs:   created.UTC().Format(swarming.TIMESTAMP_FORMAT),
			CompletedTs: completed.UTC().Format(swarming.TIMESTAMP_FORMAT),
			DedupedFrom: "",
			Duration:    duration,
			Name:        name,
			PerformanceStats: &swarming_api.SwarmingRpcsPerformanceStats{
				BotOverhead: botOverhead,
				IsolatedDownload: &swarming_api.SwarmingRpcsOperationStats{
					Duration: downloadOverhead,
				},
				IsolatedUpload: &swarming_api.SwarmingRpcsOperationStats{
					Duration: uploadOverhead,
				},
			},
			StartedTs: started.UTC().Format(swarming.TIMESTAMP_FORMAT),
			State:     swarming.TASK_STATE_COMPLETED,
			TaskId:    id,
		},
	}
}

func TestLoadSwarmingTasks(t *testing.T) {
	testutils.MediumTest(t)

	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, wd)

	// Fake some tasks in Swarming.
	swarm := swarming.NewTestClient()
	now := time.Now()
	lastLoad := now.Add(-time.Hour)

	cr := now.Add(-30 * time.Minute)
	st := now.Add(-29 * time.Minute)
	co := now.Add(-15 * time.Minute)

	d := map[string]string{
		"os":   "Ubuntu",
		"pool": "Skia",
	}

	t1 := makeTask("1", "my-task", cr, st, co, d, 0.0, 0.0, 0.0)
	t2 := makeTask("2", "my-task", cr.Add(time.Second), st, util.TimeZero, d, 0.0, 0.0, 0.0)
	t2.TaskResult.State = swarming.TASK_STATE_RUNNING
	swarm.MockTasks([]*swarming_api.SwarmingRpcsTaskRequestMetadata{t1, t2})

	edb, err := events.NewEventDB(path.Join(wd, "events.db"))
	assert.NoError(t, err)

	// Load Swarming tasks.
	revisit := []string{}
	revisit, err = loadSwarmingTasks(swarm, edb, lastLoad, now, revisit)
	assert.NoError(t, err)

	// Ensure that we inserted the expected task and added the other to
	// the revisit list.
	assert.Equal(t, 1, len(revisit))
	ev, err := edb.Range(STREAM_SWARMING_TASKS, lastLoad, now)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ev))

	// The second task is finished.
	t2.TaskResult.State = swarming.TASK_STATE_COMPLETED
	t2.TaskResult.CompletedTs = now.Add(5 * time.Minute).UTC().Format(swarming.TIMESTAMP_FORMAT)
	swarm.MockTasks([]*swarming_api.SwarmingRpcsTaskRequestMetadata{t2})

	// Load Swarming tasks again.
	lastLoad = now
	now = now.Add(10 * time.Minute)
	revisit, err = loadSwarmingTasks(swarm, edb, lastLoad, now, revisit)
	assert.NoError(t, err)

	// Ensure that we loaded details for the unfinished task from the last
	// attempt.
	assert.Equal(t, 0, len(revisit))
	ev, err = edb.Range(STREAM_SWARMING_TASKS, now.Add(-time.Hour), now)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(ev))
}

func TestMetrics(t *testing.T) {
	testutils.MediumTest(t)

	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, wd)

	// Fake a task in Swarming.
	swarm := swarming.NewTestClient()
	now := time.Now()
	lastLoad := now.Add(-time.Hour)

	cr := now.Add(-30 * time.Minute)
	st := now.Add(-29 * time.Minute)
	co := now.Add(-15 * time.Minute)

	d := map[string]string{
		"os":   "Ubuntu",
		"pool": "Skia",
	}

	t1 := makeTask("1", "my-task", cr, st, co, d, 0.0, 0.0, 0.0)
	t1.TaskResult.PerformanceStats.BotOverhead = 21
	t1.TaskResult.PerformanceStats.IsolatedUpload.Duration = 13
	t1.TaskResult.PerformanceStats.IsolatedDownload.Duration = 7
	swarm.MockTasks([]*swarming_api.SwarmingRpcsTaskRequestMetadata{t1})

	// Setup the metrics.
	edb, em, err := setupMetrics(wd)
	assert.NoError(t, err)

	// Load the Swarming task, ensure that it got inserted.
	revisit := []string{}
	revisit, err = loadSwarmingTasks(swarm, edb, lastLoad, now, revisit)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(revisit))
	ev, err := edb.Range(STREAM_SWARMING_TASKS, lastLoad, now)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(ev))

	// Forcibly update metrics.
	assert.NoError(t, em.UpdateMetrics())

	// Ensure that each of the aggregation functions gets us the correct
	// values.

	checkMetricVal := func(metric string, expect float64) {
		tags := map[string]string{
			"metric":    metric,
			"os":        "Ubuntu",
			"period":    "24h0m0s",
			"stream":    STREAM_SWARMING_TASKS,
			"task-name": "my-task",
		}
		for k, _ := range DIMENSION_WHITELIST {
			if _, ok := tags[k]; !ok {
				tags[k] = ""
			}
		}
		mx := metrics2.GetFloat64Metric(MEASUREMENT_SWARMING_TASKS, tags)
		assert.NotNil(t, mx)
		assert.Equal(t, expect, mx.Get())
	}

	checkMetricVal("duration", float64(co.Sub(st)/1000000))
	checkMetricVal("pending-time", float64(st.Sub(cr)/1000000))
	checkMetricVal("overhead-bot", 21000.0)
	checkMetricVal("overhead-upload", 13000.0)
	checkMetricVal("overhead-download", 7000.0)
}
