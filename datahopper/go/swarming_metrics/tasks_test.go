package swarming_metrics

import (
	"fmt"
	"io/ioutil"
	"path"
	"testing"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/metrics2/events"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/taskname"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/ingestcommon"
	"go.skia.org/infra/perf/go/perfclient"

	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
)

func makeTask(id, name string, created, started, completed time.Time, dims map[string]string, extraTags map[string]string, botOverhead, downloadOverhead, uploadOverhead float64) *swarming_api.SwarmingRpcsTaskRequestMetadata {
	dimensions := make([]*swarming_api.SwarmingRpcsStringPair, 0, len(dims))
	tags := make([]string, 0, len(dims))
	for k, v := range dims {
		dimensions = append(dimensions, &swarming_api.SwarmingRpcsStringPair{
			Key:   k,
			Value: v,
		})
		tags = append(tags, fmt.Sprintf("%s:%s", k, v))
	}
	for k, v := range extraTags {
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
			Name: name,
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
	pc := perfclient.NewMockPerfClient()
	mp := taskname.NewMockTaskNameParser()
	now := time.Now()
	lastLoad := now.Add(-time.Hour)

	cr := now.Add(-30 * time.Minute)
	st := now.Add(-29 * time.Minute)
	co := now.Add(-15 * time.Minute)

	d := map[string]string{
		"os":   "Ubuntu",
		"pool": "Skia",
	}

	t1 := makeTask("1", "my-task", cr, st, co, d, nil, 0.0, 0.0, 0.0)
	t2 := makeTask("2", "my-task", cr.Add(time.Second), st, util.TimeZero, d, nil, 0.0, 0.0, 0.0)
	t2.TaskResult.State = swarming.TASK_STATE_RUNNING
	swarm.MockTasks([]*swarming_api.SwarmingRpcsTaskRequestMetadata{t1, t2})

	edb, err := events.NewEventDB(path.Join(wd, "events.db"))
	assert.NoError(t, err)

	pc.On("PushToPerf", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mp.On("ParseTaskName", mock.AnythingOfType("string")).Return(map[string]string{}, nil)

	// Load Swarming tasks.
	revisit := []string{}
	revisit, err = loadSwarmingTasks(swarm, edb, pc, mp, lastLoad, now, revisit)
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
	revisit, err = loadSwarmingTasks(swarm, edb, pc, mp, lastLoad, now, revisit)
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
	pc := perfclient.NewMockPerfClient()
	mp := taskname.NewMockTaskNameParser()
	now := time.Now()
	lastLoad := now.Add(-time.Hour)

	cr := now.Add(-30 * time.Minute)
	st := now.Add(-29 * time.Minute)
	co := now.Add(-15 * time.Minute)

	d := map[string]string{
		"os":   "Ubuntu",
		"pool": "Skia",
	}

	t1 := makeTask("1", "my-task", cr, st, co, d, nil, 0.0, 0.0, 0.0)
	t1.TaskResult.PerformanceStats.BotOverhead = 21
	t1.TaskResult.PerformanceStats.IsolatedUpload.Duration = 13
	t1.TaskResult.PerformanceStats.IsolatedDownload.Duration = 7
	swarm.MockTasks([]*swarming_api.SwarmingRpcsTaskRequestMetadata{t1})

	// Setup the metrics.
	edb, em, err := setupMetrics(wd)
	assert.NoError(t, err)

	pc.On("PushToPerf", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mp.On("ParseTaskName", mock.AnythingOfType("string")).Return(map[string]string{}, nil)

	// Load the Swarming task, ensure that it got inserted.
	revisit := []string{}
	revisit, err = loadSwarmingTasks(swarm, edb, pc, mp, lastLoad, now, revisit)
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
		for k := range DIMENSION_WHITELIST {
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

func TestPerfUpload(t *testing.T) {
	testutils.MediumTest(t)

	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, wd)

	// Fake some tasks in Swarming.
	swarm := swarming.NewTestClient()
	pc := perfclient.NewMockPerfClient()
	defer pc.AssertExpectations(t)
	mp := taskname.NewMockTaskNameParser()
	defer mp.AssertExpectations(t)
	now := time.Now()
	lastLoad := now.Add(-time.Hour)

	cr := now.Add(-30 * time.Minute)
	st := now.Add(-29 * time.Minute)
	co := now.Add(-15 * time.Minute)

	d := map[string]string{
		"os":   "Ubuntu",
		"pool": "Skia",
	}

	t1 := makeTask("1", "Test-MyOS (retry)", cr, st, co, d, map[string]string{
		"sk_revision": "firstRevision",
		"sk_name":     "Test-MyOS",
		"sk_repo":     common.REPO_SKIA,
	}, 0.0, 0.0, 0.0)
	t2 := makeTask("2", "Perf-MyOS", cr.Add(time.Minute), st, util.TimeZero, d, map[string]string{
		"sk_revision": "secondRevision",
		"sk_name":     "Perf-MyOS",
		"sk_repo":     common.REPO_SKIA,
	}, 0.0, 0.0, 0.0)
	t2.TaskResult.State = swarming.TASK_STATE_RUNNING
	t3 := makeTask("3", "my-task", cr.Add(2*time.Second), st, now.Add(-time.Minute), d, nil, 0.0, 0.0, 0.0)
	t3.TaskResult.State = swarming.TASK_STATE_BOT_DIED

	swarm.MockTasks([]*swarming_api.SwarmingRpcsTaskRequestMetadata{t1, t2, t3})

	edb, err := events.NewEventDB(path.Join(wd, "events.db"))
	assert.NoError(t, err)

	mp.On("ParseTaskName", "Test-MyOS").Return(map[string]string{
		"os":   "MyOS",
		"role": "Test",
	}, nil)

	pc.On("PushToPerf", now, "Test-MyOS", "task_duration", ingestcommon.BenchData{
		Hash: "firstRevision",
		Key: map[string]string{
			"os":      "MyOS",
			"role":    "Test",
			"failure": "false",
		},
		Results: map[string]ingestcommon.BenchResults{
			"Test-MyOS": {
				"task_duration": {
					"task_ms": float64(14 * time.Minute),
				},
			},
		},
	}).Return(nil)

	// Load Swarming tasks.
	revisit := []string{}
	revisit, err = loadSwarmingTasks(swarm, edb, pc, mp, lastLoad, now, revisit)
	assert.NoError(t, err)

	pc.AssertNumberOfCalls(t, "PushToPerf", 1)

	// The second task is finished.
	t2.TaskResult.State = swarming.TASK_STATE_COMPLETED
	t2.TaskResult.CompletedTs = now.Add(5 * time.Minute).UTC().Format(swarming.TIMESTAMP_FORMAT)
	t2.TaskResult.Duration = float64(33 * time.Minute)
	t2.TaskResult.Failure = true
	swarm.MockTasks([]*swarming_api.SwarmingRpcsTaskRequestMetadata{t2})

	lastLoad = now
	now = now.Add(10 * time.Minute)

	mp.On("ParseTaskName", "Perf-MyOS").Return(map[string]string{
		"os":   "MyOS",
		"role": "Perf",
	}, nil)

	pc.On("PushToPerf", now, "Perf-MyOS", "task_duration", ingestcommon.BenchData{
		Hash: "secondRevision",
		Key: map[string]string{
			"os":      "MyOS",
			"role":    "Perf",
			"failure": "true",
		},
		Results: map[string]ingestcommon.BenchResults{
			"Perf-MyOS": {
				"task_duration": {
					"task_ms": float64(33 * time.Minute),
				},
			},
		},
	}).Return(nil)

	// Load Swarming tasks again.

	revisit, err = loadSwarmingTasks(swarm, edb, pc, mp, lastLoad, now, revisit)
	assert.NoError(t, err)
	pc.AssertNumberOfCalls(t, "PushToPerf", 2)

}
