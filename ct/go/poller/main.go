/*
	The Cluster Telemetry poller checks for new pending tasks by polling the Cluster Telemetry
	frontend. Pending tasks are picked up according to the order they were added to CTFE.
	When picked up, tasks are immediately executed. There could be multiple tasks running at the
	same time.
*/

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/ct/go/ctfe/admin_tasks"
	"go.skia.org/infra/ct/go/ctfe/capture_skps"
	"go.skia.org/infra/ct/go/ctfe/chromium_analysis"
	"go.skia.org/infra/ct/go/ctfe/chromium_builds"
	"go.skia.org/infra/ct/go/ctfe/chromium_perf"
	"go.skia.org/infra/ct/go/ctfe/lua_scripts"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	"go.skia.org/infra/ct/go/frontend"
	"go.skia.org/infra/ct/go/master_scripts/master_common"
	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/metrics2"
	skutil "go.skia.org/infra/go/util"
)

// flags
var (
	dryRun         = flag.Bool("dry_run", false, "If true, just log the commands that would be executed; don't actually execute the commands. Still polls CTFE for pending tasks, but does not post updates.")
	influxHost     = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxUser     = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	influxPassword = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxDatabase = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")
	pollInterval   = flag.Duration("poll_interval", 30*time.Second, "How often to poll CTFE for new pending tasks.")
	// Value of --log_dir flag to pass to subcommands. Will be set in main.
	logDir = "/b/storage/glog"
	// Mutex that controls updating and building of the local checkout.
	repoMtx = sync.Mutex{}
	// Map that holds all picked up tasks. Used to ensure same task is not picked up more than once.
	pickedUpTasks = map[string]string{}
	// Mutex that controls access to the above map.
	tasksMtx = sync.Mutex{}
)

// Enum of states that the poller could be in. Satisfies the fmt.Stringer interface.
type TaskType uint32

const (
	IDLE TaskType = iota
	UPDATE_AND_BUILD
	CHROMIUM_PERF
	CAPTURE_SKPS
	LUA_SCRIPT
	CHROMIUM_BUILD
	RECREATE_PAGE_SETS
	RECREATE_WEBPAGE_ARCHIVES
	CHROMIUM_ANALYSIS
	POLL
)

func (t TaskType) String() string {
	switch t {
	case IDLE:
		return "IDLE"
	case UPDATE_AND_BUILD:
		return "UPDATE_AND_BUILD"
	case CHROMIUM_PERF:
		return "CHROMIUM_PERF"
	case CAPTURE_SKPS:
		return "CAPTURE_SKPS"
	case LUA_SCRIPT:
		return "LUA_SCRIPT"
	case CHROMIUM_BUILD:
		return "CHROMIUM_BUILD"
	case RECREATE_PAGE_SETS:
		return "RECREATE_PAGE_SETS"
	case RECREATE_WEBPAGE_ARCHIVES:
		return "RECREATE_WEBPAGE_ARCHIVES"
	case CHROMIUM_ANALYSIS:
		return "CHROMIUM_ANALYSIS"
	case POLL:
		return "POLL"
	default:
		return "(unknown)"
	}
}

// Runs "git pull; make all".
func updateAndBuild() error {
	repoMtx.Lock()
	defer repoMtx.Unlock()
	makefilePath := ctutil.CtTreeDir

	// TODO(benjaminwagner): Should this also do 'go get -u ...' and/or 'gclient sync'?
	err := exec.Run(&exec.Command{
		Name:      "git",
		Args:      []string{"pull"},
		Dir:       makefilePath,
		Timeout:   ctutil.GIT_PULL_TIMEOUT,
		LogStdout: true,
		LogStderr: true,
	})
	if err != nil {
		return err
	}
	return exec.Run(&exec.Command{
		Name:      "make",
		Args:      []string{"all"},
		Dir:       makefilePath,
		Timeout:   ctutil.MAKE_ALL_TIMEOUT,
		LogStdout: true,
		LogStderr: true,
	})
}

// Specifies the methods that poll requires for each type of task.
type Task interface {
	GetTaskName() string
	GetCommonCols() *task_common.CommonCols
	// Writes any files required by the task and then uses exec.Run to run the task command.
	Execute() error
	// Returns the corresponding UpdateTaskVars instance of this Task. The
	// returned instance is not populated.
	GetUpdateTaskVars() task_common.UpdateTaskVars
}

// Generates a hopefully-unique ID for this execution of this task.
func runId(task Task) string {
	// TODO(benjaminwagner): May need a way to ensure that run IDs are unique. It is currently
	// possible, though very unlikely, for GetOldestPendingTaskV2 to take
	// ~(pollInterval - 1 second) and for the returned task to fail very quickly, in which case
	// the next task would could start within the same second as the first task.
	return strings.SplitN(task.GetCommonCols().Username, "@", 2)[0] + "-" + ctutil.GetCurrentTs()
}

// Define frontend.ChromiumAnalysisDBTask here so we can add methods.
type ChromiumAnalysisTask struct {
	chromium_analysis.DBTask
}

func (task *ChromiumAnalysisTask) Execute() error {
	runId := runId(task)
	for fileSuffix, patch := range map[string]string{
		".chromium.patch":      task.ChromiumPatch,
		".catapult.patch":      task.CatapultPatch,
		".benchmark.patch":     task.BenchmarkPatch,
		".custom_webpages.csv": task.CustomWebpages,
	} {
		// Add an extra newline at the end because git sometimes rejects patches due to
		// missing newlines.
		patch = patch + "\n"
		patchPath := filepath.Join(os.TempDir(), runId+fileSuffix)
		if err := ioutil.WriteFile(patchPath, []byte(patch), 0666); err != nil {
			return err
		}
		defer skutil.Remove(patchPath)
	}
	return exec.Run(&exec.Command{
		Name: "run_chromium_analysis_on_workers",
		Args: []string{
			"--emails=" + task.Username,
			"--description=" + task.Description,
			"--gae_task_id=" + strconv.FormatInt(task.Id, 10),
			"--pageset_type=" + task.PageSets,
			"--benchmark_name=" + task.Benchmark,
			"--benchmark_extra_args=" + task.BenchmarkArgs,
			"--browser_extra_args=" + task.BrowserArgs,
			"--run_in_parallel=" + strconv.FormatBool(task.RunInParallel),
			"--target_platform=" + task.Platform,
			"--run_on_gce=" + strconv.FormatBool(task.RunOnGCE),
			"--run_id=" + runId,
			"--log_dir=" + logDir,
			"--log_id=" + runId,
			fmt.Sprintf("--local=%t", *master_common.Local),
		},
	})
}

// Define frontend.ChromiumPerfDBTask here so we can add methods.
type ChromiumPerfTask struct {
	chromium_perf.DBTask
}

func (task *ChromiumPerfTask) Execute() error {
	runId := runId(task)
	// TODO(benjaminwagner): Since run_chromium_perf_on_workers only reads these in order to
	// upload to Google Storage, eventually we should move the upload step here to avoid writing
	// to disk.
	for fileSuffix, patch := range map[string]string{
		".chromium.patch":      task.ChromiumPatch,
		".skia.patch":          task.SkiaPatch,
		".catapult.patch":      task.CatapultPatch,
		".benchmark.patch":     task.BenchmarkPatch,
		".custom_webpages.csv": task.CustomWebpages,
	} {
		// Add an extra newline at the end because git sometimes rejects patches due to
		// missing newlines.
		patch = patch + "\n"
		patchPath := filepath.Join(os.TempDir(), runId+fileSuffix)
		if err := ioutil.WriteFile(patchPath, []byte(patch), 0666); err != nil {
			return err
		}
		defer skutil.Remove(patchPath)
	}
	return exec.Run(&exec.Command{
		Name: "run_chromium_perf_on_workers",
		Args: []string{
			"--emails=" + task.Username,
			"--description=" + task.Description,
			"--gae_task_id=" + strconv.FormatInt(task.Id, 10),
			"--pageset_type=" + task.PageSets,
			"--benchmark_name=" + task.Benchmark,
			"--benchmark_extra_args=" + task.BenchmarkArgs,
			"--browser_extra_args_nopatch=" + task.BrowserArgsNoPatch,
			"--browser_extra_args_withpatch=" + task.BrowserArgsWithPatch,
			"--repeat_benchmark=" + strconv.FormatInt(task.RepeatRuns, 10),
			"--run_in_parallel=" + strconv.FormatBool(task.RunInParallel),
			"--target_platform=" + task.Platform,
			"--run_id=" + runId,
			"--log_dir=" + logDir,
			"--log_id=" + runId,
			fmt.Sprintf("--local=%t", *master_common.Local),
		},
	})
}

// Define frontend.CaptureSkpsDBTask here so we can add methods.
type CaptureSkpsTask struct {
	capture_skps.DBTask
}

func (task *CaptureSkpsTask) Execute() error {
	runId := runId(task)
	chromiumBuildDir := ctutil.ChromiumBuildDir(task.ChromiumRev, task.SkiaRev, "")
	return exec.Run(&exec.Command{
		Name: "capture_skps_on_workers",
		Args: []string{
			"--emails=" + task.Username,
			"--description=" + task.Description,
			"--gae_task_id=" + strconv.FormatInt(task.Id, 10),
			"--pageset_type=" + task.PageSets,
			"--chromium_build=" + chromiumBuildDir,
			"--target_platform=Linux",
			"--run_id=" + runId,
			"--log_dir=" + logDir,
			"--log_id=" + runId,
			fmt.Sprintf("--local=%t", *master_common.Local),
		},
	})
}

// Define frontend.LuaScriptDBTask here so we can add methods.
type LuaScriptTask struct {
	lua_scripts.DBTask
}

func (task *LuaScriptTask) Execute() error {
	runId := runId(task)
	chromiumBuildDir := ctutil.ChromiumBuildDir(task.ChromiumRev, task.SkiaRev, "")
	// TODO(benjaminwagner): Since run_lua_on_workers only reads the lua script in order to
	// upload to Google Storage, eventually we should move the upload step here to avoid writing
	// to disk. Not sure if we can/should do the same for the aggregator script.
	luaScriptName := runId + ".lua"
	luaScriptPath := filepath.Join(os.TempDir(), luaScriptName)
	if err := ioutil.WriteFile(luaScriptPath, []byte(task.LuaScript), 0666); err != nil {
		return err
	}
	defer skutil.Remove(luaScriptPath)
	if task.LuaAggregatorScript != "" {
		luaAggregatorName := runId + ".aggregator"
		luaAggregatorPath := filepath.Join(os.TempDir(), luaAggregatorName)
		if err := ioutil.WriteFile(luaAggregatorPath, []byte(task.LuaAggregatorScript), 0666); err != nil {
			return err
		}
		defer skutil.Remove(luaAggregatorPath)
	}
	return exec.Run(&exec.Command{
		Name: "run_lua_on_workers",
		Args: []string{
			"--emails=" + task.Username,
			"--description=" + task.Description,
			"--gae_task_id=" + strconv.FormatInt(task.Id, 10),
			"--pageset_type=" + task.PageSets,
			"--chromium_build=" + chromiumBuildDir,
			"--run_id=" + runId,
			"--log_dir=" + logDir,
			"--log_id=" + runId,
			fmt.Sprintf("--local=%t", *master_common.Local),
		},
	})
}

// Define frontend.ChromiumBuildDBTask here so we can add methods.
type ChromiumBuildTask struct {
	chromium_builds.DBTask
}

func (task *ChromiumBuildTask) Execute() error {
	runId := runId(task)
	return exec.Run(&exec.Command{
		Name: "build_chromium",
		Args: []string{
			"--emails=" + task.Username,
			"--gae_task_id=" + strconv.FormatInt(task.Id, 10),
			"--run_id=" + runId,
			"--target_platform=Linux",
			"--chromium_hash=" + task.ChromiumRev,
			"--skia_hash=" + task.SkiaRev,
			"--log_dir=" + logDir,
			"--log_id=" + runId,
			fmt.Sprintf("--local=%t", *master_common.Local),
		},
	})
}

// Define frontend.RecreatePageSetsDBTask here so we can add methods.
type RecreatePageSetsTask struct {
	admin_tasks.RecreatePageSetsDBTask
}

func (task *RecreatePageSetsTask) Execute() error {
	runId := runId(task)
	return exec.Run(&exec.Command{
		Name: "create_pagesets_on_workers",
		Args: []string{
			"--emails=" + task.Username,
			"--gae_task_id=" + strconv.FormatInt(task.Id, 10),
			"--run_id=" + runId,
			"--pageset_type=" + task.PageSets,
			"--log_dir=" + logDir,
			"--log_id=" + runId,
			fmt.Sprintf("--local=%t", *master_common.Local),
		},
	})
}

// Define frontend.RecreateWebpageArchivesDBTask here so we can add methods.
type RecreateWebpageArchivesTask struct {
	admin_tasks.RecreateWebpageArchivesDBTask
}

func (task *RecreateWebpageArchivesTask) Execute() error {
	runId := runId(task)
	return exec.Run(&exec.Command{
		Name: "capture_archives_on_workers",
		Args: []string{
			"--emails=" + task.Username,
			"--gae_task_id=" + strconv.FormatInt(task.Id, 10),
			"--run_id=" + runId,
			"--pageset_type=" + task.PageSets,
			"--log_dir=" + logDir,
			"--log_id=" + runId,
			fmt.Sprintf("--local=%t", *master_common.Local),
		},
	})
}

// Returns a poller Task containing the given task_common.Task, or nil if otherTask is nil.
func asPollerTask(otherTask task_common.Task) Task {
	if otherTask == nil {
		return nil
	}
	switch t := otherTask.(type) {
	case *chromium_perf.DBTask:
		return &ChromiumPerfTask{DBTask: *t}
	case *capture_skps.DBTask:
		return &CaptureSkpsTask{DBTask: *t}
	case *lua_scripts.DBTask:
		return &LuaScriptTask{DBTask: *t}
	case *chromium_builds.DBTask:
		return &ChromiumBuildTask{DBTask: *t}
	case *admin_tasks.RecreatePageSetsDBTask:
		return &RecreatePageSetsTask{RecreatePageSetsDBTask: *t}
	case *admin_tasks.RecreateWebpageArchivesDBTask:
		return &RecreateWebpageArchivesTask{RecreateWebpageArchivesDBTask: *t}
	case *chromium_analysis.DBTask:
		return &ChromiumAnalysisTask{DBTask: *t}
	default:
		sklog.Errorf("Missing case for %T in asPollerTask", otherTask)
		return nil
	}
}

// Notifies the frontend that task failed.
func updateWebappTaskSetFailed(task Task) error {
	updateVars := task.GetUpdateTaskVars()
	updateVars.GetUpdateTaskCommonVars().Id = task.GetCommonCols().Id
	updateVars.GetUpdateTaskCommonVars().SetCompleted(false)
	return frontend.UpdateWebappTaskV2(updateVars)
}

// pollAndExecOnce looks for the oldest pending task in CTFE. If one is found, then
// the local checkout is synced and built, and the picked up task is started in a
// go routine. The function returns without waiting for the task to finish and the
// WaitGroup of the goroutine is returned to the caller. The caller can then call
// wg.Wait() if they would like to wait for the task to finish.
func pollAndExecOnce() *sync.WaitGroup {
	pending, err := frontend.GetOldestPendingTaskV2()
	var wg sync.WaitGroup
	if err != nil {
		sklog.Error(err)
		return &wg
	}
	task := asPollerTask(pending)
	if task == nil {
		return &wg
	}

	taskName, id := task.GetTaskName(), task.GetCommonCols().Id
	tasksMtx.Lock()
	_, exists := pickedUpTasks[fmt.Sprintf("%s.%d", taskName, id)]
	tasksMtx.Unlock()
	if exists {
		return &wg
	}
	tasksMtx.Lock()
	pickedUpTasks[fmt.Sprintf("%s.%d", taskName, id)] = "1"
	tasksMtx.Unlock()

	sklog.Infof("Preparing to execute task %s %d", taskName, id)
	if err = updateAndBuild(); err != nil {
		sklog.Error(err)
		return &wg
	}
	sklog.Infof("Executing task %s %d", taskName, id)
	// Increment the WaitGroup counter.
	wg.Add(1)
	go func() {
		// Decrement the counter when the goroutine completes.
		defer wg.Done()
		if err = task.Execute(); err == nil {
			sklog.Infof("Completed task %s %d", taskName, id)
		} else {
			sklog.Errorf("Task %s %d failed: %v", taskName, id, err)
			if !*dryRun {
				if err := updateWebappTaskSetFailed(task); err != nil {
					sklog.Error(err)
				}
			}
		}
		tasksMtx.Lock()
		delete(pickedUpTasks, fmt.Sprintf("%s.%d", taskName, id))
		tasksMtx.Unlock()
	}()
	// Return the WaitGroup to allow some callers to call wg.Wait()
	return &wg
}

func main() {
	defer common.LogPanic()
	master_common.InitWithMetrics2("ct-poller", influxHost, influxUser, influxPassword, influxDatabase)

	if logDirFlag := flag.Lookup("log_dir"); logDirFlag != nil {
		logDir = logDirFlag.Value.String()
	}

	if *dryRun {
		exec.SetRunForTesting(func(command *exec.Command) error {
			sklog.Infof("dry_run: %s", exec.DebugString(command))
			return nil
		})
	}

	healthyGauge := metrics2.GetInt64Metric("healthy")

	// Run immediately, since pollTick will not fire until after pollInterval.
	pollAndExecOnce()
	for _ = range time.Tick(*pollInterval) {
		healthyGauge.Update(1)
		pollAndExecOnce()

	}
}
