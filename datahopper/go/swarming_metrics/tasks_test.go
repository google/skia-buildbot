package swarming_metrics

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	bt_testutil "go.skia.org/infra/go/bt/testutil"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/metrics2/events"
	"go.skia.org/infra/go/swarming/v2/mocks"
	"go.skia.org/infra/go/taskname"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/ingest/format"
	"go.skia.org/infra/perf/go/perfclient"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func makeTask(id, name string, created, started, completed time.Time, dims map[string]string, extraTags map[string]string, botOverhead, downloadOverhead, uploadOverhead time.Duration) *apipb.TaskRequestMetadataResponse {
	dimensions := make([]*apipb.StringPair, 0, len(dims))
	tags := make([]string, 0, len(dims))
	for k, v := range dims {
		dimensions = append(dimensions, &apipb.StringPair{
			Key:   k,
			Value: v,
		})
		tags = append(tags, fmt.Sprintf("%s:%s", k, v))
	}
	for k, v := range extraTags {
		tags = append(tags, fmt.Sprintf("%s:%s", k, v))
	}
	duration := float32(0.0)
	if !util.TimeIsZero(completed) {
		duration = float32(completed.Sub(started) / time.Second)
	}
	return &apipb.TaskRequestMetadataResponse{
		Request: &apipb.TaskRequestResponse{
			CreatedTs: timestamppb.New(created),
			Tags:      tags,
			Name:      name,
			TaskSlices: []*apipb.TaskSlice{
				{
					Properties: &apipb.TaskProperties{
						Dimensions: dimensions,
					},
				},
			},
		},
		TaskId: id,
		TaskResult: &apipb.TaskResultResponse{
			CreatedTs:   timestamppb.New(created),
			CompletedTs: timestamppb.New(completed),
			DedupedFrom: "",
			Duration:    duration,
			Name:        name,
			PerformanceStats: &apipb.PerformanceStats{
				BotOverhead: float32(botOverhead / time.Second),
				IsolatedDownload: &apipb.CASOperationStats{
					Duration:            float32(downloadOverhead / time.Second),
					TotalBytesItemsCold: 50000000.0,
				},
				IsolatedUpload: &apipb.CASOperationStats{
					Duration:            float32(uploadOverhead / time.Second),
					TotalBytesItemsCold: 70000000.0,
				},
			},
			StartedTs: timestamppb.New(started),
			State:     apipb.TaskState_COMPLETED,
			TaskId:    id,
		},
	}
}

func TestLoadSwarmingTasks(t *testing.T) {
	ctx := context.Background()
	wd, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, wd)

	// Fake some tasks in Swarming.
	swarm := &mocks.SwarmingV2Client{}
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
	t2.TaskResult.State = apipb.TaskState_RUNNING
	swarm.On("ListTasks", testutils.AnyContext, &apipb.TasksWithPerfRequest{
		Limit:                   1000,
		Start:                   timestamppb.New(lastLoad),
		Tags:                    []string{"pool:Skia"},
		IncludePerformanceStats: true,
		State:                   apipb.StateQuery_QUERY_ALL,
	}).Return(&apipb.TaskListResponse{
		Items: []*apipb.TaskResultResponse{t1.TaskResult, t2.TaskResult},
	}, nil)
	swarm.On("GetRequest", testutils.AnyContext, &apipb.TaskIdRequest{
		TaskId: t1.TaskId,
	}).Return(t1.Request, nil)

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
		require.Eventually(t, func() bool {
			ev, err := edb.Range(streamForPool("Skia"), from, to)
			require.NoError(t, err)
			return len(ev) == expect
		}, 5*time.Second, 100*time.Millisecond)
	}
	assertCount(lastLoad, now, 1)

	// datahopper will follow up on the revisit list (which is t2's id)
	swarm.On("GetResult", testutils.AnyContext, &apipb.TaskIdWithPerfRequest{
		TaskId:                  "2",
		IncludePerformanceStats: true,
	}).Return(t2.TaskResult, nil)

	// The second task is finished.
	t2.TaskResult.State = apipb.TaskState_COMPLETED
	t2.TaskResult.CompletedTs = timestamppb.New(now.Add(5 * time.Minute))

	lastLoad = now
	now = now.Add(10 * time.Minute)

	// This is empty because datahopper will pull in the task data from revisit
	swarm.On("ListTasks", testutils.AnyContext, &apipb.TasksWithPerfRequest{
		Limit:                   1000,
		Start:                   timestamppb.New(lastLoad),
		Tags:                    []string{"pool:Skia"},
		IncludePerformanceStats: true,
		State:                   apipb.StateQuery_QUERY_ALL,
	}).Return(&apipb.TaskListResponse{
		Items: []*apipb.TaskResultResponse{},
	}, nil)
	swarm.On("GetRequest", testutils.AnyContext, &apipb.TaskIdRequest{
		TaskId: t2.TaskId,
	}).Return(t2.Request, nil)

	// Load Swarming tasks again.
	revisit, err = loadSwarmingTasks(ctx, swarm, "Skia", edb, pc, mp, lastLoad, now, revisit)
	require.NoError(t, err)

	// Ensure that we loaded details for the unfinished task from the last
	// attempt.
	require.Equal(t, 0, len(revisit))
	assertCount(now.Add(-time.Hour), now, 2)
}

func TestMetrics(t *testing.T) {
	ctx := context.Background()
	wd, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, wd)

	// Fake a task in Swarming.
	swarm := &mocks.SwarmingV2Client{}
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
	swarm.On("ListTasks", testutils.AnyContext, &apipb.TasksWithPerfRequest{
		Limit:                   1000,
		Start:                   timestamppb.New(lastLoad),
		Tags:                    []string{"pool:Skia"},
		IncludePerformanceStats: true,
		State:                   apipb.StateQuery_QUERY_ALL,
	}).Return(&apipb.TaskListResponse{
		Items: []*apipb.TaskResultResponse{t1.TaskResult},
	}, nil)
	swarm.On("GetRequest", testutils.AnyContext, &apipb.TaskIdRequest{
		TaskId: t1.TaskId,
	}).Return(t1.Request, nil)

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
			"metric":      metric,
			"os":          "Ubuntu",
			"period":      "24h0m0s",
			"pool":        "Skia",
			"stream":      streamForPool("Skia"),
			"task_name":   "my-task",
			"aggregation": "",
		}
		for k := range includeDimensions {
			if _, ok := tags[k]; !ok {
				tags[k] = ""
			}
		}
		mx := metrics2.GetFloat64Metric(fmt.Sprintf(measurementSwarmingTasksTmpl, "Skia"), tags)
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
	ctx := context.Background()
	wd, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, wd)

	// Fake some tasks in Swarming.
	swarm := &mocks.SwarmingV2Client{}
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
	t2.TaskResult.State = apipb.TaskState_RUNNING
	t3 := makeTask("3", "my-task", cr.Add(2*time.Second), st, now.Add(-time.Minute), d, nil, 47*time.Second, 3*time.Second, 34*time.Second)
	t3.TaskResult.State = apipb.TaskState_BOT_DIED
	t4 := makeTask("4", "Test-MyOS", cr, st, co, d, map[string]string{
		"sk_revision":     "firstRevision",
		"sk_name":         "Test-MyOS",
		"sk_repo":         common.REPO_SKIA,
		"sk_issue":        "12345",
		"sk_patchset":     "6",
		"sk_issue_server": "https://skia-review.googlesource.com",
	}, 31*time.Second, 7*time.Second, 3*time.Second)

	swarm.On("ListTasks", testutils.AnyContext, &apipb.TasksWithPerfRequest{
		Limit:                   1000,
		Start:                   timestamppb.New(lastLoad),
		Tags:                    []string{"pool:Skia"},
		IncludePerformanceStats: true,
		State:                   apipb.StateQuery_QUERY_ALL,
	}).Return(&apipb.TaskListResponse{
		Items: []*apipb.TaskResultResponse{t1.TaskResult, t2.TaskResult, t3.TaskResult, t4.TaskResult},
	}, nil)
	swarm.On("GetRequest", testutils.AnyContext, &apipb.TaskIdRequest{
		TaskId: t1.TaskId,
	}).Return(t1.Request, nil)
	swarm.On("GetRequest", testutils.AnyContext, &apipb.TaskIdRequest{
		TaskId: t2.TaskId,
	}).Return(t2.Request, nil)
	swarm.On("GetRequest", testutils.AnyContext, &apipb.TaskIdRequest{
		TaskId: t4.TaskId,
	}).Return(t4.Request, nil)

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
	t2.TaskResult.State = apipb.TaskState_COMPLETED
	t2.TaskResult.CompletedTs = timestamppb.New(now.Add(5 * time.Minute))
	t2.TaskResult.Duration = float32(33 * time.Minute / time.Second)
	t2.TaskResult.Failure = true

	lastLoad = now
	now = now.Add(10 * time.Minute)
	// This is empty because datahopper will pull in the task data from revisit
	swarm.On("ListTasks", testutils.AnyContext, &apipb.TasksWithPerfRequest{
		Limit:                   1000,
		Start:                   timestamppb.New(lastLoad),
		Tags:                    []string{"pool:Skia"},
		IncludePerformanceStats: true,
		State:                   apipb.StateQuery_QUERY_ALL,
	}).Return(&apipb.TaskListResponse{
		Items: []*apipb.TaskResultResponse{},
	}, nil)

	// datahopper will follow up on the revisit list (which is t2's id)
	swarm.On("GetResult", testutils.AnyContext, &apipb.TaskIdWithPerfRequest{
		TaskId:                  "2",
		IncludePerformanceStats: true,
	}).Return(t2.TaskResult, nil)

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
	_, err = loadSwarmingTasks(ctx, swarm, "Skia", edb, pc, mp, lastLoad, now, revisit)
	require.NoError(t, err)
	pc.AssertNumberOfCalls(t, "PushToPerf", 3)

}
