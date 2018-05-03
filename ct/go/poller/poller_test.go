package main

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"go.skia.org/infra/ct/go/ct_autoscaler"
	"go.skia.org/infra/ct/go/ctfe/admin_tasks"
	"go.skia.org/infra/ct/go/ctfe/capture_skps"
	"go.skia.org/infra/ct/go/ctfe/chromium_builds"
	"go.skia.org/infra/ct/go/ctfe/chromium_perf"
	"go.skia.org/infra/ct/go/ctfe/lua_scripts"
	"go.skia.org/infra/ct/go/ctfe/metrics_analysis"
	"go.skia.org/infra/ct/go/ctfe/pixel_diff"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	"go.skia.org/infra/ct/go/frontend"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/testutils"
	skutil "go.skia.org/infra/go/util"

	expect "github.com/stretchr/testify/assert"
	assert "github.com/stretchr/testify/require"
)

// CommonCols without TsStarted or TsCompleted set.
func pendingCommonCols() task_common.CommonCols {
	return task_common.CommonCols{
		Id:       42,
		TsAdded:  sql.NullInt64{Int64: 20080726180513, Valid: true},
		Username: "nobody@chromium.org",
	}
}

// Given a command generated by one of the Execute methods in main.go, extracts the value of the
// run_id command-line flag. If not found, signals a test failure.
func getRunId(t *testing.T, cmd *exec.Command) string {
	regexp := regexp.MustCompile("^--run_id=(.*)$")
	for _, arg := range cmd.Args {
		match := regexp.FindStringSubmatch(arg)
		if match != nil && len(match) >= 2 {
			return match[1]
		}
	}
	assert.Contains(t, strings.Join(cmd.Args, " "), "--run_id=")
	assert.FailNow(t, "getRunId is broken")
	return ""
}

// Checks that the contents of filepath are expected; otherwise signals a test failure.
func assertFileContents(t *testing.T, filepath, expected string) {
	actual, err := ioutil.ReadFile(filepath)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(actual))
}

func pendingChromiumPerfTask() ChromiumPerfTask {
	return ChromiumPerfTask{
		DBTask: chromium_perf.DBTask{
			CommonCols:           pendingCommonCols(),
			Benchmark:            "benchmark",
			Platform:             "Linux",
			PageSets:             "All",
			RepeatRuns:           1,
			BenchmarkArgs:        "benchmarkargs",
			BrowserArgsNoPatch:   "banp",
			BrowserArgsWithPatch: "bawp",
			Description:          "description",
			ChromiumPatch:        "chromiumpatch",
			SkiaPatch:            "skiapatch",
		},
	}
}

func TestChromiumPerfExecute(t *testing.T) {
	testutils.SmallTest(t)
	mockRun := exec.CommandCollector{}
	mockRun.SetDelegateRun(func(cmd *exec.Command) error {
		runId := getRunId(t, cmd)
		assertFileContents(t, filepath.Join(os.TempDir(), runId+".chromium.patch"),
			"chromiumpatch\n")
		assertFileContents(t, filepath.Join(os.TempDir(), runId+".skia.patch"),
			"skiapatch\n")
		return nil
	})
	ctx := exec.NewContext(context.Background(), mockRun.Run)
	task := pendingChromiumPerfTask()
	err := task.Execute(ctx)
	assert.NoError(t, err)
	assert.Len(t, mockRun.Commands(), 1)
	cmd := mockRun.Commands()[0]
	expect.Equal(t, "run_chromium_perf_on_workers", cmd.Name)
	expect.Contains(t, cmd.Args, "--gae_task_id=42")
	expect.Contains(t, cmd.Args, "--description=description")
	expect.Contains(t, cmd.Args, "--emails=nobody@chromium.org")
	expect.Contains(t, cmd.Args, "--benchmark_name=benchmark")
	expect.Contains(t, cmd.Args, "--target_platform=Linux")
	expect.Contains(t, cmd.Args, "--pageset_type=All")
	expect.Contains(t, cmd.Args, "--repeat_benchmark=1")
	expect.Contains(t, cmd.Args, "--benchmark_extra_args=benchmarkargs")
	expect.Contains(t, cmd.Args, "--browser_extra_args_nopatch=banp")
	expect.Contains(t, cmd.Args, "--browser_extra_args_withpatch=bawp")
	runId := getRunId(t, cmd)
	expect.Contains(t, cmd.Args, "--log_id="+runId)
	expect.NotNil(t, cmd.Timeout)
}

func pendingPixelDiffTask() PixelDiffTask {
	return PixelDiffTask{
		DBTask: pixel_diff.DBTask{
			CommonCols:           pendingCommonCols(),
			PageSets:             "All",
			BenchmarkArgs:        "benchmarkargs",
			BrowserArgsNoPatch:   "banp",
			BrowserArgsWithPatch: "bawp",
			Description:          "description",
			ChromiumPatch:        "chromiumpatch",
			SkiaPatch:            "skiapatch",
		},
	}
}

func TestPixelDiffExecute(t *testing.T) {
	testutils.SmallTest(t)
	mockRun := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mockRun.Run)
	task := pendingPixelDiffTask()
	err := task.Execute(ctx)
	assert.NoError(t, err)
	assert.Len(t, mockRun.Commands(), 1)
	cmd := mockRun.Commands()[0]
	expect.Equal(t, "pixel_diff_on_workers", cmd.Name)
	expect.Equal(t, len(cmd.Args), 12)
	expect.Contains(t, cmd.Args, "--gae_task_id=42")
	expect.Contains(t, cmd.Args, "--description=description")
	expect.Contains(t, cmd.Args, "--emails=nobody@chromium.org")
	expect.Contains(t, cmd.Args, "--pageset_type=All")
	expect.Contains(t, cmd.Args, "--benchmark_extra_args=benchmarkargs")
	expect.Contains(t, cmd.Args, "--browser_extra_args_nopatch=banp")
	expect.Contains(t, cmd.Args, "--browser_extra_args_withpatch=bawp")
	expect.Contains(t, cmd.Args, "--logtostderr")
	expect.Contains(t, cmd.Args, "--local=false")
	expect.Contains(t, cmd.Args, "--run_on_gce="+strconv.FormatBool(task.RunsOnGCEWorkers()))
	runId := getRunId(t, cmd)
	expect.Contains(t, cmd.Args, "--run_id="+runId)
	expect.Contains(t, cmd.Args, "--log_id="+runId)
	expect.NotNil(t, cmd.Timeout)
}

func pendingMetricsAnalysisTask() MetricsAnalysisTask {
	return MetricsAnalysisTask{
		DBTask: metrics_analysis.DBTask{
			CommonCols:         pendingCommonCols(),
			MetricName:         "loadingMetric",
			AnalysisOutputLink: "http://test/outputlink",
			BenchmarkArgs:      "benchmarkargs",
			Description:        "description",
			ChromiumPatch:      "chromiumpatch",
			CatapultPatch:      "catapultpatch",
		},
	}
}

func TestMetricsAnalysisExecute(t *testing.T) {
	testutils.SmallTest(t)
	mockRun := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mockRun.Run)
	task := pendingMetricsAnalysisTask()
	err := task.Execute(ctx)
	assert.NoError(t, err)
	assert.Len(t, mockRun.Commands(), 1)
	cmd := mockRun.Commands()[0]
	expect.Equal(t, "metrics_analysis_on_workers", cmd.Name)
	expect.Equal(t, len(cmd.Args), 10)
	expect.Contains(t, cmd.Args, "--gae_task_id=42")
	expect.Contains(t, cmd.Args, "--description=description")
	expect.Contains(t, cmd.Args, "--emails=nobody@chromium.org")
	expect.Contains(t, cmd.Args, "--metric_name=loadingMetric")
	expect.Contains(t, cmd.Args, "--analysis_output_link=http://test/outputlink")
	expect.Contains(t, cmd.Args, "--benchmark_extra_args=benchmarkargs")
	expect.Contains(t, cmd.Args, "--logtostderr")
	expect.Contains(t, cmd.Args, "--local=false")
	runId := getRunId(t, cmd)
	expect.Contains(t, cmd.Args, "--run_id="+runId)
	expect.Contains(t, cmd.Args, "--log_id="+runId)
	expect.NotNil(t, cmd.Timeout)
}

func pendingCaptureSkpsTask() CaptureSkpsTask {
	return CaptureSkpsTask{
		DBTask: capture_skps.DBTask{
			CommonCols:  pendingCommonCols(),
			PageSets:    "All",
			ChromiumRev: "c14d891d44f0afff64e56ed7c9702df1d807b1ee",
			SkiaRev:     "586101c79b0490b50623e76c71a5fd67d8d92b08",
			Description: "description",
		},
	}
}

func TestCaptureSkpsExecute(t *testing.T) {
	testutils.SmallTest(t)
	mockRun := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mockRun.Run)
	task := pendingCaptureSkpsTask()
	err := task.Execute(ctx)
	assert.NoError(t, err)
	assert.Len(t, mockRun.Commands(), 1)
	cmd := mockRun.Commands()[0]
	expect.Equal(t, "capture_skps_on_workers", cmd.Name)
	expect.Contains(t, cmd.Args, "--gae_task_id=42")
	expect.Contains(t, cmd.Args, "--description=description")
	expect.Contains(t, cmd.Args, "--emails=nobody@chromium.org")
	expect.Contains(t, cmd.Args, "--pageset_type=All")
	expect.Contains(t, cmd.Args, "--chromium_build=c14d891d44f0af-586101c79b0490")
	runId := getRunId(t, cmd)
	expect.Contains(t, cmd.Args, "--log_id="+runId)
	expect.NotNil(t, cmd.Timeout)
}

func pendingLuaScriptTaskWithAggregator(ctx context.Context) LuaScriptTask {
	return LuaScriptTask{
		DBTask: lua_scripts.DBTask{
			CommonCols:          pendingCommonCols(),
			PageSets:            "All",
			ChromiumRev:         "c14d891d44f0afff64e56ed7c9702df1d807b1ee",
			SkiaRev:             "586101c79b0490b50623e76c71a5fd67d8d92b08",
			LuaScript:           `print("lualualua")`,
			LuaAggregatorScript: `print("aaallluuu")`,
			Description:         "description",
		},
	}
}

func TestLuaScriptExecuteWithAggregator(t *testing.T) {
	testutils.SmallTest(t)
	mockRun := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mockRun.Run)
	task := pendingLuaScriptTaskWithAggregator(ctx)
	mockRun.SetDelegateRun(func(cmd *exec.Command) error {
		runId := getRunId(t, cmd)
		assertFileContents(t, filepath.Join(os.TempDir(), runId+".lua"),
			`print("lualualua")`)
		assertFileContents(t, filepath.Join(os.TempDir(), runId+".aggregator"),
			`print("aaallluuu")`)
		return nil
	})
	err := task.Execute(ctx)
	assert.NoError(t, err)
	assert.Len(t, mockRun.Commands(), 1)
	cmd := mockRun.Commands()[0]
	expect.Equal(t, "run_lua_on_workers", cmd.Name)
	expect.Contains(t, cmd.Args, "--gae_task_id=42")
	expect.Contains(t, cmd.Args, "--description=description")
	expect.Contains(t, cmd.Args, "--emails=nobody@chromium.org")
	expect.Contains(t, cmd.Args, "--pageset_type=All")
	expect.Contains(t, cmd.Args, "--chromium_build=c14d891d44f0af-586101c79b0490")
	runId := getRunId(t, cmd)
	expect.Contains(t, cmd.Args, "--log_id="+runId)
	expect.NotNil(t, cmd.Timeout)
}

func TestLuaScriptExecuteWithoutAggregator(t *testing.T) {
	testutils.SmallTest(t)
	mockRun := exec.CommandCollector{}
	mockRun.SetDelegateRun(func(cmd *exec.Command) error {
		runId := getRunId(t, cmd)
		assertFileContents(t, filepath.Join(os.TempDir(), runId+".lua"),
			`print("lualualua")`)
		_, err := os.Stat(filepath.Join(os.TempDir(), runId+".aggregator"))
		expect.True(t, os.IsNotExist(err))
		return nil
	})
	ctx := exec.NewContext(context.Background(), mockRun.Run)
	task := LuaScriptTask{
		DBTask: lua_scripts.DBTask{
			CommonCols:          pendingCommonCols(),
			PageSets:            "All",
			ChromiumRev:         "c14d891d44f0afff64e56ed7c9702df1d807b1ee",
			SkiaRev:             "586101c79b0490b50623e76c71a5fd67d8d92b08",
			LuaScript:           `print("lualualua")`,
			LuaAggregatorScript: "",
			Description:         "description",
		},
	}
	err := task.Execute(ctx)
	assert.NoError(t, err)
	assert.Len(t, mockRun.Commands(), 1)
	cmd := mockRun.Commands()[0]
	expect.Equal(t, "run_lua_on_workers", cmd.Name)
	expect.Contains(t, cmd.Args, "--gae_task_id=42")
	expect.Contains(t, cmd.Args, "--emails=nobody@chromium.org")
	expect.Contains(t, cmd.Args, "--pageset_type=All")
	expect.Contains(t, cmd.Args, "--chromium_build=c14d891d44f0af-586101c79b0490")
	runId := getRunId(t, cmd)
	expect.Contains(t, cmd.Args, "--log_id="+runId)
	expect.NotNil(t, cmd.Timeout)
}

func pendingChromiumBuildTask() ChromiumBuildTask {
	return ChromiumBuildTask{
		DBTask: chromium_builds.DBTask{
			CommonCols:    pendingCommonCols(),
			ChromiumRev:   "c14d891d44f0afff64e56ed7c9702df1d807b1ee",
			ChromiumRevTs: sql.NullInt64{Int64: 20080726180513, Valid: true},
			SkiaRev:       "586101c79b0490b50623e76c71a5fd67d8d92b08",
		},
	}
}

func TestChromiumBuildExecute(t *testing.T) {
	testutils.SmallTest(t)
	mockRun := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mockRun.Run)
	task := pendingChromiumBuildTask()
	err := task.Execute(ctx)
	assert.NoError(t, err)
	assert.Len(t, mockRun.Commands(), 1)
	cmd := mockRun.Commands()[0]
	expect.Equal(t, "build_chromium", cmd.Name)
	expect.Contains(t, cmd.Args, "--gae_task_id=42")
	expect.Contains(t, cmd.Args, "--emails=nobody@chromium.org")
	expect.Contains(t, cmd.Args,
		"--chromium_hash=c14d891d44f0afff64e56ed7c9702df1d807b1ee")
	expect.Contains(t, cmd.Args,
		"--skia_hash=586101c79b0490b50623e76c71a5fd67d8d92b08")
	runId := getRunId(t, cmd)
	expect.Contains(t, cmd.Args, "--log_id="+runId)
	expect.NotNil(t, cmd.Timeout)
}

func pendingRecreatePageSetsTask() RecreatePageSetsTask {
	return RecreatePageSetsTask{
		RecreatePageSetsDBTask: admin_tasks.RecreatePageSetsDBTask{
			CommonCols: pendingCommonCols(),
			PageSets:   "All",
		},
	}
}

func TestRecreatePageSetsExecute(t *testing.T) {
	testutils.SmallTest(t)
	mockRun := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mockRun.Run)
	task := pendingRecreatePageSetsTask()
	err := task.Execute(ctx)
	assert.NoError(t, err)
	assert.Len(t, mockRun.Commands(), 1)
	cmd := mockRun.Commands()[0]
	expect.Equal(t, "create_pagesets_on_workers", cmd.Name)
	expect.Contains(t, cmd.Args, "--gae_task_id=42")
	expect.Contains(t, cmd.Args, "--emails=nobody@chromium.org")
	expect.Contains(t, cmd.Args, "--pageset_type=All")
	runId := getRunId(t, cmd)
	expect.Contains(t, cmd.Args, "--log_id="+runId)
	expect.NotNil(t, cmd.Timeout)
}

func pendingRecreateWebpageArchivesTask() RecreateWebpageArchivesTask {
	return RecreateWebpageArchivesTask{
		RecreateWebpageArchivesDBTask: admin_tasks.RecreateWebpageArchivesDBTask{
			CommonCols:  pendingCommonCols(),
			PageSets:    "All",
			ChromiumRev: "c14d891d44f0afff64e56ed7c9702df1d807b1ee",
			SkiaRev:     "586101c79b0490b50623e76c71a5fd67d8d92b08",
		},
	}
}

func TestRecreateWebpageArchivesExecute(t *testing.T) {
	testutils.SmallTest(t)
	mockRun := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mockRun.Run)
	task := pendingRecreateWebpageArchivesTask()
	err := task.Execute(ctx)
	assert.NoError(t, err)
	assert.Len(t, mockRun.Commands(), 1)
	cmd := mockRun.Commands()[0]
	expect.Equal(t, "capture_archives_on_workers", cmd.Name)
	expect.Contains(t, cmd.Args, "--gae_task_id=42")
	expect.Contains(t, cmd.Args, "--emails=nobody@chromium.org")
	expect.Contains(t, cmd.Args, "--pageset_type=All")
	runId := getRunId(t, cmd)
	expect.Contains(t, cmd.Args, "--log_id="+runId)
	expect.NotNil(t, cmd.Timeout)
}

func TestAsPollerTask(t *testing.T) {
	testutils.SmallTest(t)
	ctx := context.Background()
	expect.Nil(t, asPollerTask(ctx, nil))
	{
		taskStruct := pendingChromiumPerfTask()
		taskInterface := asPollerTask(ctx, &taskStruct.DBTask)
		expect.Equal(t, taskStruct, *taskInterface.(*ChromiumPerfTask))
	}
	{
		taskStruct := pendingCaptureSkpsTask()
		taskInterface := asPollerTask(ctx, &taskStruct.DBTask)
		expect.Equal(t, taskStruct, *taskInterface.(*CaptureSkpsTask))
	}
	{
		taskStruct := pendingLuaScriptTaskWithAggregator(ctx)
		taskInterface := asPollerTask(ctx, &taskStruct.DBTask)
		expect.Equal(t, taskStruct, *taskInterface.(*LuaScriptTask))
	}
	{
		taskStruct := pendingChromiumBuildTask()
		taskInterface := asPollerTask(ctx, &taskStruct.DBTask)
		expect.Equal(t, taskStruct, *taskInterface.(*ChromiumBuildTask))
	}
	{
		taskStruct := pendingRecreatePageSetsTask()
		taskInterface := asPollerTask(ctx, &taskStruct.RecreatePageSetsDBTask)
		expect.Equal(t, taskStruct, *taskInterface.(*RecreatePageSetsTask))
	}
	{
		taskStruct := pendingRecreateWebpageArchivesTask()
		taskInterface := asPollerTask(ctx, &taskStruct.RecreateWebpageArchivesDBTask)
		expect.Equal(t, taskStruct, *taskInterface.(*RecreateWebpageArchivesTask))
	}
}

// Test that updateWebappTaskSetFailed works.
func TestUpdateWebappTaskSetFailed(t *testing.T) {
	testutils.SmallTest(t)
	task := pendingRecreateWebpageArchivesTask()
	mockServer := frontend.MockServer{}
	defer frontend.CloseTestServer(frontend.InitTestServer(&mockServer))
	err := updateWebappTaskSetFailed(&task)
	assert.NoError(t, err)
	assert.Len(t, mockServer.UpdateTaskReqs(), 1)
	updateReq := mockServer.UpdateTaskReqs()[0]
	assert.Equal(t, "/"+ctfeutil.UPDATE_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI, updateReq.Url)
	assert.NoError(t, updateReq.Error)
	assert.False(t, updateReq.Vars.TsStarted.Valid)
	assert.True(t, updateReq.Vars.TsCompleted.Valid)
	assert.True(t, updateReq.Vars.Failure.Valid)
	assert.True(t, updateReq.Vars.Failure.Bool)
	assert.False(t, updateReq.Vars.RepeatAfterDays.Valid)
	assert.Equal(t, int64(42), updateReq.Vars.Id)
}

// Test that updateWebappTaskSetFailed returns an error when the server response indicates an error.
func TestUpdateWebappTaskSetFailedFailure(t *testing.T) {
	testutils.SmallTest(t)
	errstr := "You must be at least this tall to ride this ride."
	task := pendingRecreateWebpageArchivesTask()
	reqCount := 0
	mockServer := func(w http.ResponseWriter, r *http.Request) {
		reqCount++
		assert.Equal(t, "/"+ctfeutil.UPDATE_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI,
			r.URL.Path)
		defer skutil.Close(r.Body)
		httputils.ReportError(w, r, fmt.Errorf(errstr), errstr)
	}
	defer frontend.CloseTestServer(frontend.InitTestServer(http.HandlerFunc(mockServer)))
	err := updateWebappTaskSetFailed(&task)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errstr)
	assert.Equal(t, 1, reqCount)
}

func TestPollAndExecOnce(t *testing.T) {
	testutils.SmallTest(t)
	mockExec := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mockExec.Run)
	task := pendingRecreateWebpageArchivesTask()
	mockCTAutoscaler := &ct_autoscaler.MockCTAutoscaler{}
	mockServer := frontend.MockServer{}
	mockServer.SetCurrentTask(&task.RecreateWebpageArchivesDBTask)
	defer frontend.CloseTestServer(frontend.InitTestServer(&mockServer))
	wg := pollAndExecOnce(ctx, mockCTAutoscaler)
	wg.Wait()
	// Expect only one poll.
	expect.Equal(t, 1, mockServer.OldestPendingTaskReqCount())
	expect.Equal(t, 1, mockCTAutoscaler.RegisterGCETaskTimesCalled)
	expect.Equal(t, 1, mockCTAutoscaler.UnregisterGCETaskTimesCalled)
	// Expect three commands: git pull; make all; capture_archives_on_workers ...
	commands := mockExec.Commands()
	assert.Len(t, commands, 3)
	expect.Equal(t, "git pull", exec.DebugString(commands[0]))
	expect.Equal(t, "make all", exec.DebugString(commands[1]))
	expect.Equal(t, "capture_archives_on_workers", commands[2].Name)
	// No updates expected. (capture_archives_on_workers would send updates if it were
	// executed.)
	expect.Empty(t, mockServer.UpdateTaskReqs())
}

func TestPollAndExecOnceMultipleTasks(t *testing.T) {
	testutils.SmallTest(t)
	mockExec := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mockExec.Run)
	task1 := pendingRecreateWebpageArchivesTask()
	mockCTAutoscaler := &ct_autoscaler.MockCTAutoscaler{}
	mockServer := frontend.MockServer{}
	mockServer.SetCurrentTask(&task1.RecreateWebpageArchivesDBTask)
	defer frontend.CloseTestServer(frontend.InitTestServer(&mockServer))
	// Poll frontend and execute the first task.
	wg1 := pollAndExecOnce(ctx, mockCTAutoscaler)
	wg1.Wait() // Wait for task to return to make asserting commands deterministic.
	// Update current task.
	task2 := pendingChromiumPerfTask()
	mockServer.SetCurrentTask(&task2.DBTask)
	// Poll frontend and execute the second task.
	wg2 := pollAndExecOnce(ctx, mockCTAutoscaler)
	wg2.Wait() // Wait for task to return to make asserting commands deterministic.

	// Expect two pending task requests.
	expect.Equal(t, 2, mockServer.OldestPendingTaskReqCount())
	expect.Equal(t, 1, mockCTAutoscaler.RegisterGCETaskTimesCalled)
	expect.Equal(t, 1, mockCTAutoscaler.UnregisterGCETaskTimesCalled)
	// Expect six commands: git pull; make all; capture_archives_on_workers ...; git pull;
	// make all; run_chromium_perf_on_workers ...
	commands := mockExec.Commands()
	assert.Len(t, commands, 6)
	expect.Equal(t, "git pull", exec.DebugString(commands[0]))
	expect.Equal(t, "make all", exec.DebugString(commands[1]))
	expect.Equal(t, "capture_archives_on_workers", commands[2].Name)
	expect.Equal(t, "git pull", exec.DebugString(commands[3]))
	expect.Equal(t, "make all", exec.DebugString(commands[4]))
	expect.Equal(t, "run_chromium_perf_on_workers", commands[5].Name)
	// No updates expected when commands succeed.
	expect.Empty(t, mockServer.UpdateTaskReqs())
}

func TestPollAndExecOnceError(t *testing.T) {
	testutils.SmallTest(t)
	mockRun := exec.MockRun{}
	commandCollector := exec.CommandCollector{}
	commandCollector.SetDelegateRun(mockRun.Run)
	ctx := exec.NewContext(context.Background(), commandCollector.Run)
	task := pendingRecreateWebpageArchivesTask()
	mockCTAutoscaler := &ct_autoscaler.MockCTAutoscaler{}
	mockServer := frontend.MockServer{}
	mockServer.SetCurrentTask(&task.RecreateWebpageArchivesDBTask)
	defer frontend.CloseTestServer(frontend.InitTestServer(&mockServer))
	mockRun.AddRule("capture_archives_on_workers", fmt.Errorf("workers too lazy"))
	wg := pollAndExecOnce(ctx, mockCTAutoscaler)
	wg.Wait()
	// Expect only one poll.
	expect.Equal(t, 1, mockServer.OldestPendingTaskReqCount())
	expect.Equal(t, 1, mockCTAutoscaler.RegisterGCETaskTimesCalled)
	expect.Equal(t, 1, mockCTAutoscaler.UnregisterGCETaskTimesCalled)
	// Expect three commands: git pull; make all; capture_archives_on_workers ...
	commands := commandCollector.Commands()
	assert.Len(t, commands, 3)
	expect.Equal(t, "git pull", exec.DebugString(commands[0]))
	expect.Equal(t, "make all", exec.DebugString(commands[1]))
	expect.Equal(t, "capture_archives_on_workers", commands[2].Name)
	// Expect an update marking task failed when command fails to execute.
	assert.Len(t, mockServer.UpdateTaskReqs(), 1)
	updateReq := mockServer.UpdateTaskReqs()[0]
	assert.Equal(t, "/"+ctfeutil.UPDATE_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI, updateReq.Url)
	assert.NoError(t, updateReq.Error)
	assert.False(t, updateReq.Vars.TsStarted.Valid)
	assert.True(t, updateReq.Vars.TsCompleted.Valid)
	assert.True(t, updateReq.Vars.Failure.Valid)
	assert.True(t, updateReq.Vars.Failure.Bool)
	assert.False(t, updateReq.Vars.RepeatAfterDays.Valid)
	assert.Equal(t, int64(42), updateReq.Vars.Id)
}

func TestPollAndExecOnceNoTasks(t *testing.T) {
	testutils.SmallTest(t)
	mockCTAutoscaler := &ct_autoscaler.MockCTAutoscaler{}
	mockServer := frontend.MockServer{}
	mockServer.SetCurrentTask(nil)
	defer frontend.CloseTestServer(frontend.InitTestServer(&mockServer))
	mockExec := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mockExec.Run)
	// Poll frontend, no tasks.
	wg1 := pollAndExecOnce(ctx, mockCTAutoscaler)
	wg2 := pollAndExecOnce(ctx, mockCTAutoscaler)
	wg3 := pollAndExecOnce(ctx, mockCTAutoscaler)
	// Expect three polls.
	wg1.Wait()
	wg2.Wait()
	wg3.Wait()
	expect.Equal(t, 3, mockServer.OldestPendingTaskReqCount())
	expect.Equal(t, 0, mockCTAutoscaler.RegisterGCETaskTimesCalled)
	expect.Equal(t, 0, mockCTAutoscaler.UnregisterGCETaskTimesCalled)
	// Expect no commands.
	expect.Empty(t, mockExec.Commands())
	// No updates expected.
	expect.Empty(t, mockServer.UpdateTaskReqs())
}
