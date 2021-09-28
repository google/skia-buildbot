package swarming_metrics

import (
	"context"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
	bt_testutil "go.skia.org/infra/go/bt/testutil"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/metrics2/events"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/swarming/mocks"
	"go.skia.org/infra/go/taskname"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/ingest/format"
	"go.skia.org/infra/perf/go/perfclient"
)

func makeTask(id, name string, created, started, completed time.Time, dims map[string]string, extraTags map[string]string, botOverhead, downloadOverhead, uploadOverhead time.Duration) *swarming_api.SwarmingRpcsTaskRequestMetadata {
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
		duration = float64(completed.Sub(started) / time.Second)
	}
	return &swarming_api.SwarmingRpcsTaskRequestMetadata{
		Request: &swarming_api.SwarmingRpcsTaskRequest{
			CreatedTs: created.UTC().Format(swarming.TIMESTAMP_FORMAT),
			Tags:      tags,
			Name:      name,
			TaskSlices: []*swarming_api.SwarmingRpcsTaskSlice{
				{
					Properties: &swarming_api.SwarmingRpcsTaskProperties{
						Dimensions: dimensions,
					},
				},
			},
		},
		TaskId: id,
		TaskResult: &swarming_api.SwarmingRpcsTaskResult{
			CreatedTs:   created.UTC().Format(swarming.TIMESTAMP_FORMAT),
			CompletedTs: completed.UTC().Format(swarming.TIMESTAMP_FORMAT),
			DedupedFrom: "",
			Duration:    duration,
			Name:        name,
			PerformanceStats: &swarming_api.SwarmingRpcsPerformanceStats{
				BotOverhead: float64(botOverhead / time.Second),
				IsolatedDownload: &swarming_api.SwarmingRpcsOperationStats{
					Duration:            float64(downloadOverhead / time.Second),
					TotalBytesItemsCold: 50000000.0,
				},
				IsolatedUpload: &swarming_api.SwarmingRpcsOperationStats{
					Duration:            float64(uploadOverhead / time.Second),
					TotalBytesItemsCold: 70000000.0,
				},
			},
			StartedTs: started.UTC().Format(swarming.TIMESTAMP_FORMAT),
			State:     swarming.TASK_STATE_COMPLETED,
			TaskId:    id,
		},
	}
}

func TestLoadSwarmingTasks(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	wd, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, wd)

	// Fake some tasks in Swarming.
	swarm := &mocks.ApiClient{}
	defer swarm.AssertExpectations(t)
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

	t1 := makeTask("1", "my-task", cr, st, co, d, nil, 0.0, 0.0, 0.0)
	t2 := makeTask("2", "my-task", cr.Add(time.Second), st, time.Time{}, d, nil, 0.0, 0.0, 0.0)
	t2.TaskResult.State = swarming.TASK_STATE_RUNNING
	swarm.On("ListTasks", testutils.AnyContext, lastLoad, now, []string{"pool:Skia"}, "").Return([]*swarming_api.SwarmingRpcsTaskRequestMetadata{t1, t2}, nil)

	btProject, btInstance, cleanup := bt_testutil.SetupBigTable(t, events.BT_TABLE, events.BT_COLUMN_FAMILY)
	defer cleanup()
	edb, err := events.NewBTEventDB(context.Background(), btProject, btInstance, nil)
	require.NoError(t, err)

	// Load Swarming tasks.
	revisit := []string{}
	revisit, err = loadSwarmingTasks(ctx, swarm, "Skia", edb, pc, mp, lastLoad, now, revisit)
	require.NoError(t, err)

	// Ensure that we inserted the expected task and added the other to
	// the revisit list.
	require.Equal(t, 1, len(revisit))
	assertCount := func(from, to time.Time, expect int) {
		require.NoError(t, testutils.EventuallyConsistent(5*time.Second, func() error {
			ev, err := edb.Range(streamForPool("Skia"), from, to)
			require.NoError(t, err)
			if len(ev) != expect {
				return testutils.TryAgainErr
			}
			return nil
		}))
	}
	assertCount(lastLoad, now, 1)

	// datahopper will follow up on the revisit list (which is t2's id)
	swarm.On("GetTaskMetadata", testutils.AnyContext, "2").Return(t2, nil)

	// The second task is finished.
	t2.TaskResult.State = swarming.TASK_STATE_COMPLETED
	t2.TaskResult.CompletedTs = now.Add(5 * time.Minute).UTC().Format(swarming.TIMESTAMP_FORMAT)

	lastLoad = now
	now = now.Add(10 * time.Minute)

	// This is empty because datahopper will pull in the task data from revisit
	swarm.On("ListTasks", testutils.AnyContext, lastLoad, now, []string{"pool:Skia"}, "").Return([]*swarming_api.SwarmingRpcsTaskRequestMetadata{}, nil)

	// Load Swarming tasks again.
	revisit, err = loadSwarmingTasks(ctx, swarm, "Skia", edb, pc, mp, lastLoad, now, revisit)
	require.NoError(t, err)

	// Ensure that we loaded details for the unfinished task from the last
	// attempt.
	require.Equal(t, 0, len(revisit))
	assertCount(now.Add(-time.Hour), now, 2)
}

func TestMetrics(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	wd, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, wd)

	// Fake a task in Swarming.
	swarm := &mocks.ApiClient{}
	defer swarm.AssertExpectations(t)
	pc := perfclient.NewMockPerfClient()
	defer pc.AssertExpectations(t)
	mp := taskname.NewMockTaskNameParser()
	defer mp.AssertExpectations(t)
	// This needs to be now, otherwise the metrics won't be aggregated
	// due to the requirement to list the period (e.g. 24h)
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
	swarm.On("ListTasks", testutils.AnyContext, lastLoad, now, []string{"pool:Skia"}, "").Return([]*swarming_api.SwarmingRpcsTaskRequestMetadata{t1}, nil)

	// Setup the metrics.
	btProject, btInstance, cleanup := bt_testutil.SetupBigTable(t, events.BT_TABLE, events.BT_COLUMN_FAMILY)
	defer cleanup()
	edb, em, err := setupMetrics(context.Background(), btProject, btInstance, "Skia", nil)
	require.NoError(t, err)

	// Load the Swarming task, ensure that it got inserted.
	revisit := []string{}
	revisit, err = loadSwarmingTasks(ctx, swarm, "Skia", edb, pc, mp, lastLoad, now, revisit)
	require.NoError(t, err)
	require.Equal(t, 0, len(revisit))
	ev, err := edb.Range(streamForPool("Skia"), lastLoad, now)
	require.NoError(t, err)
	require.Equal(t, 1, len(ev))

	// Forcibly update metrics.
	require.NoError(t, em.UpdateMetrics())

	// Ensure that each of the aggregation functions gets us the correct
	// values.

	checkMetricVal := func(metric string, expect float64) {
		tags := map[string]string{
			"metric":    metric,
			"os":        "Ubuntu",
			"period":    "24h0m0s",
			"pool":      "Skia",
			"stream":    streamForPool("Skia"),
			"task_name": "my-task",
		}
		for k := range includeDimensions {
			if _, ok := tags[k]; !ok {
				tags[k] = ""
			}
		}
		mx := metrics2.GetFloat64Metric(MEASUREMENT_SWARMING_TASKS, tags)
		require.NotNil(t, mx)
		require.Equal(t, expect, mx.Get())
	}

	checkMetricVal("duration", float64(co.Sub(st)/1000000))
	checkMetricVal("pending-time", float64(st.Sub(cr)/1000000))
	checkMetricVal("overhead-bot", 21000.0)
	checkMetricVal("overhead-upload", 13000.0)
	checkMetricVal("overhead-download", 7000.0)
	checkMetricVal("cas-cache-miss-download", 50000000.0)
	checkMetricVal("cas-cache-miss-upload", 70000000.0)
}

func TestPerfUpload(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	wd, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, wd)

	// Fake some tasks in Swarming.
	swarm := &mocks.ApiClient{}
	defer swarm.AssertExpectations(t)
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
	}, 17*time.Second, 5*time.Second, 4*time.Second)
	t2 := makeTask("2", "Perf-MyOS", cr.Add(time.Minute), st, time.Time{}, d, map[string]string{
		"sk_revision": "secondRevision",
		"sk_name":     "Perf-MyOS",
		"sk_repo":     common.REPO_SKIA,
	}, 37*time.Second, 23*time.Second, 4*time.Second)
	t2.TaskResult.State = swarming.TASK_STATE_RUNNING
	t3 := makeTask("3", "my-task", cr.Add(2*time.Second), st, now.Add(-time.Minute), d, nil, 47*time.Second, 3*time.Second, 34*time.Second)
	t3.TaskResult.State = swarming.TASK_STATE_BOT_DIED
	t4 := makeTask("4", "Test-MyOS", cr, st, co, d, map[string]string{
		"sk_revision":     "firstRevision",
		"sk_name":         "Test-MyOS",
		"sk_repo":         common.REPO_SKIA,
		"sk_issue":        "12345",
		"sk_patchset":     "6",
		"sk_issue_server": "https://skia-review.googlesource.com",
	}, 31*time.Second, 7*time.Second, 3*time.Second)

	swarm.On("ListTasks", testutils.AnyContext, lastLoad, now, []string{"pool:Skia"}, "").Return([]*swarming_api.SwarmingRpcsTaskRequestMetadata{t1, t2, t3, t4}, nil)

	btProject, btInstance, cleanup := bt_testutil.SetupBigTable(t, events.BT_TABLE, events.BT_COLUMN_FAMILY)
	defer cleanup()
	edb, err := events.NewBTEventDB(context.Background(), btProject, btInstance, nil)
	require.NoError(t, err)

	mp.On("ParseTaskName", "Test-MyOS").Return(map[string]string{
		"os":   "MyOS",
		"role": "Test",
	}, nil)

	pc.On("PushToPerf", now, "Test-MyOS", "task_duration", format.BenchData{
		Hash: "firstRevision",
		Key: map[string]string{
			"os":      "MyOS",
			"role":    "Test",
			"failure": "false",
		},
		Results: map[string]format.BenchResults{
			"Test-MyOS": {
				"task_duration": {
					"total_s":        float64((14*time.Minute + 17*time.Second) / time.Second),
					"task_step_s":    float64(14 * time.Minute / time.Second),
					"cas_overhead_s": 9.0,
					"all_overhead_s": 17.0,
				},
			},
		},
		Source: "datahopper",
	}).Return(nil)
	pc.On("PushToPerf", now, "Test-MyOS", "task_duration", format.BenchData{
		Hash:     "firstRevision",
		Issue:    "12345",
		PatchSet: "6",
		Key: map[string]string{
			"os":      "MyOS",
			"role":    "Test",
			"failure": "false",
		},
		Results: map[string]format.BenchResults{
			"Test-MyOS": {
				"task_duration": {
					"total_s":        float64((14*time.Minute + 31*time.Second) / time.Second),
					"task_step_s":    float64(14 * time.Minute / time.Second),
					"cas_overhead_s": 10.0,
					"all_overhead_s": 31.0,
				},
			},
		},
		Source:       "datahopper",
		PatchStorage: "gerrit",
	}).Return(nil)

	// Load Swarming tasks.
	revisit := []string{}
	revisit, err = loadSwarmingTasks(ctx, swarm, "Skia", edb, pc, mp, lastLoad, now, revisit)
	require.NoError(t, err)

	pc.AssertNumberOfCalls(t, "PushToPerf", 2)

	// The second task is finished.
	t2.TaskResult.State = swarming.TASK_STATE_COMPLETED
	t2.TaskResult.CompletedTs = now.Add(5 * time.Minute).UTC().Format(swarming.TIMESTAMP_FORMAT)
	t2.TaskResult.Duration = float64(33 * time.Minute / time.Second)
	t2.TaskResult.Failure = true

	lastLoad = now
	now = now.Add(10 * time.Minute)
	// This is empty because datahopper will pull in the task data from revisit
	swarm.On("ListTasks", testutils.AnyContext, lastLoad, now, []string{"pool:Skia"}, "").Return([]*swarming_api.SwarmingRpcsTaskRequestMetadata{}, nil)

	// datahopper will follow up on the revisit list (which is t2's id)
	swarm.On("GetTaskMetadata", testutils.AnyContext, "2").Return(t2, nil)

	mp.On("ParseTaskName", "Perf-MyOS").Return(map[string]string{
		"os":   "MyOS",
		"role": "Perf",
	}, nil)

	pc.On("PushToPerf", now, "Perf-MyOS", "task_duration", format.BenchData{
		Hash: "secondRevision",
		Key: map[string]string{
			"os":      "MyOS",
			"role":    "Perf",
			"failure": "true",
		},
		Results: map[string]format.BenchResults{
			"Perf-MyOS": {
				"task_duration": {
					"total_s":        float64((33*time.Minute + 37*time.Second) / time.Second),
					"task_step_s":    float64(33 * time.Minute / time.Second),
					"cas_overhead_s": 27.0,
					"all_overhead_s": 37.0,
				},
			},
		},
		Source: "datahopper",
	}).Return(nil)

	// Load Swarming tasks again.

	revisit, err = loadSwarmingTasks(ctx, swarm, "Skia", edb, pc, mp, lastLoad, now, revisit)
	require.NoError(t, err)
	pc.AssertNumberOfCalls(t, "PushToPerf", 3)

}
