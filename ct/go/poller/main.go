/*
	The Cluster Telemetry poller checks for new pending tasks by polling the Cluster Telemetry
	frontend. Tasks are executed serially.
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

	"github.com/skia-dev/glog"

	"go.skia.org/infra/ct/go/ctfe/admin_tasks"
	"go.skia.org/infra/ct/go/ctfe/capture_skps"
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
	_
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
	case POLL:
		return "POLL"
	default:
		return "(unknown)"
	}
}

// StatusTracker expects callers to pass this opaque token back to FinishTask.
type StatusTrackerToken interface{}

// Tracks what the poller is currently doing. Goroutine-safe.
type StatusTracker interface {
	// Indicates the current goroutine is entering the given state.
	StartTask(t TaskType) StatusTrackerToken
	// Exits the state entered with StartTask. err is nil if no error occurred.
	FinishTask(token StatusTrackerToken, err error)
}

// Implements StatusTracker and provides metrics for monitoring. Assumes a single goroutine
// executes all tasks.
type heartbeatStatusTracker struct {
	// Protects the remaining fields.
	mu sync.Mutex
	// Time of last call to StartTask or FinishTask.
	lastUpdateLiveness *metrics2.Liveness
	// State of the main goroutine.
	currentStatus TaskType
	// Updated with the value of currentStatus for monitoring.
	currentStatusGauge *metrics2.Int64Metric
	// Reports the duration of each type of task.
	taskDurations map[TaskType]*metrics2.Timer
	// Tracks the time of the last successful completion.
	lastSuccessLiveness map[TaskType]*metrics2.Liveness
	// Tracks the time of the last failure.
	lastFailureLiveness map[TaskType]*metrics2.Liveness
	// Stores any errors encountered in StartTask or FinishTask.
	errs []error
}

func NewHeartbeatStatusTracker() StatusTracker {
	h := &heartbeatStatusTracker{}
	h.currentStatusGauge = metrics2.GetInt64Metric("current-status")
	h.taskDurations = make(map[TaskType]*metrics2.Timer)
	for t := UPDATE_AND_BUILD; t <= POLL; t++ {
		h.taskDurations[t] = metrics2.NewTimer("duration", map[string]string{"task": t.String()})
	}
	h.lastUpdateLiveness = metrics2.NewLiveness("last-update")
	h.lastSuccessLiveness = make(map[TaskType]*metrics2.Liveness)
	h.lastFailureLiveness = make(map[TaskType]*metrics2.Liveness)
	for t := UPDATE_AND_BUILD; t <= POLL; t++ {
		h.lastSuccessLiveness[t] = metrics2.NewLiveness("last-success", map[string]string{"task": t.String()})
		h.lastFailureLiveness[t] = metrics2.NewLiveness("last-failure", map[string]string{"task": t.String()})
	}
	return h
}

func (h *heartbeatStatusTracker) StartTask(t TaskType) StatusTrackerToken {
	h.mu.Lock()
	defer h.mu.Unlock()
	if t == IDLE {
		h.errs = append(h.errs, fmt.Errorf("StartTask called with IDLE."))
		return nil
	}
	if h.currentStatus != IDLE {
		h.errs = append(h.errs, fmt.Errorf("StartTask called with %s when currentTask is %s.", t, h.currentStatus))
		return nil
	}
	h.currentStatus = t
	h.currentStatusGauge.Update(int64(h.currentStatus))
	h.lastUpdateLiveness.Reset()
	h.taskDurations[t].Start()
	return t
}

func (h *heartbeatStatusTracker) FinishTask(token StatusTrackerToken, err error) {
	t, ok := token.(TaskType)
	h.mu.Lock()
	defer h.mu.Unlock()
	if ok {
		if t == IDLE {
			h.errs = append(h.errs, fmt.Errorf("FinishTask got IDLE."))
		} else if t != h.currentStatus {
			h.errs = append(h.errs, fmt.Errorf("FinishTask called with %s but currentStatus is %s.", t, h.currentStatus))
		} else {
			h.taskDurations[t].Stop()
			if err == nil {
				h.lastSuccessLiveness[t].Reset()
			} else {
				h.lastFailureLiveness[t].Reset()
			}
		}
	} else if token != nil {
		h.errs = append(h.errs, fmt.Errorf("Expected argument to FinishTask to be TaskType, instead got %T: %#v", token, token))
	}
	h.currentStatus = IDLE
	h.currentStatusGauge.Update(int64(IDLE))
	h.lastUpdateLiveness.Reset()
}

// StartMetrics registers metrics which indicate the poller is running
// healthily and starts a goroutine to update them periodically.
func (h *heartbeatStatusTracker) StartMetrics() {
	healthyGauge := metrics2.GetInt64Metric("healthy")
	go func() {
		for _ = range time.Tick(common.SAMPLE_PERIOD) {
			h.mu.Lock()
			currentStatus := h.currentStatus
			errs := h.errs
			h.errs = nil
			h.mu.Unlock()
			expectPoll := false
			var expectedDuration time.Duration = 0
			switch currentStatus {
			case IDLE, POLL:
				expectPoll = true
				expectedDuration = *pollInterval
			case UPDATE_AND_BUILD:
				expectedDuration = ctutil.GIT_PULL_TIMEOUT + ctutil.MAKE_ALL_TIMEOUT
			case CHROMIUM_PERF:
				expectedDuration = ctutil.MASTER_SCRIPT_RUN_CHROMIUM_PERF_TIMEOUT
			case CAPTURE_SKPS:
				expectedDuration = ctutil.MASTER_SCRIPT_CAPTURE_SKPS_TIMEOUT
			case LUA_SCRIPT:
				expectedDuration = ctutil.MASTER_SCRIPT_RUN_LUA_TIMEOUT
			case CHROMIUM_BUILD:
				expectedDuration = ctutil.MASTER_SCRIPT_BUILD_CHROMIUM_TIMEOUT
			case RECREATE_PAGE_SETS:
				expectedDuration = ctutil.MASTER_SCRIPT_CREATE_PAGESETS_TIMEOUT
			case RECREATE_WEBPAGE_ARCHIVES:
				expectedDuration = ctutil.MASTER_SCRIPT_CAPTURE_ARCHIVES_TIMEOUT
			}
			// Provide a bit of head room.
			expectedDuration += 2 * time.Minute

			lastSuccessfulPoll := time.Duration(h.lastSuccessLiveness[POLL].Get()) * time.Second
			if expectPoll && lastSuccessfulPoll > 2*time.Minute {
				errs = append(errs, fmt.Errorf("Last successful poll was %s ago.", lastSuccessfulPoll))
			}
			timeSinceLastUpdate := time.Duration(h.lastUpdateLiveness.Get()) * time.Second
			if timeSinceLastUpdate > expectedDuration {
				errs = append(errs, fmt.Errorf("Task %s has not finished after %s.", currentStatus, timeSinceLastUpdate))
			}
			if len(errs) > 0 {
				for _, err := range errs {
					glog.Error(err)
				}
				healthyGauge.Update(0)
			} else {
				healthyGauge.Update(1)
			}
		}
	}()
}

var statusTracker StatusTracker = NewHeartbeatStatusTracker()

// Runs "git pull; make all".
func updateAndBuild() error {
	token := statusTracker.StartTask(UPDATE_AND_BUILD)
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
		statusTracker.FinishTask(token, err)
		return err
	}
	err = exec.Run(&exec.Command{
		Name:      "make",
		Args:      []string{"all"},
		Dir:       makefilePath,
		Timeout:   ctutil.MAKE_ALL_TIMEOUT,
		LogStdout: true,
		LogStderr: true,
	})
	statusTracker.FinishTask(token, err)
	return err
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

// Define frontend.ChromiumPerfDBTask here so we can add methods.
type ChromiumPerfTask struct {
	chromium_perf.DBTask
}

func (task *ChromiumPerfTask) Execute() error {
	token := statusTracker.StartTask(CHROMIUM_PERF)
	runId := runId(task)
	// TODO(benjaminwagner): Since run_chromium_perf_on_workers only reads these in order to
	// upload to Google Storage, eventually we should move the upload step here to avoid writing
	// to disk.
	for fileSuffix, patch := range map[string]string{
		".chromium.patch":  task.ChromiumPatch,
		".skia.patch":      task.SkiaPatch,
		".benchmark.patch": task.BenchmarkPatch,
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
	err := exec.Run(&exec.Command{
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
		Timeout: ctutil.MASTER_SCRIPT_RUN_CHROMIUM_PERF_TIMEOUT,
	})
	statusTracker.FinishTask(token, err)
	return err
}

// Define frontend.CaptureSkpsDBTask here so we can add methods.
type CaptureSkpsTask struct {
	capture_skps.DBTask
}

func (task *CaptureSkpsTask) Execute() error {
	token := statusTracker.StartTask(CAPTURE_SKPS)
	runId := runId(task)
	chromiumBuildDir := ctutil.ChromiumBuildDir(task.ChromiumRev, task.SkiaRev, "")
	err := exec.Run(&exec.Command{
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
		Timeout: ctutil.MASTER_SCRIPT_CAPTURE_SKPS_TIMEOUT,
	})
	statusTracker.FinishTask(token, err)
	return err
}

// Define frontend.LuaScriptDBTask here so we can add methods.
type LuaScriptTask struct {
	lua_scripts.DBTask
}

func (task *LuaScriptTask) Execute() error {
	token := statusTracker.StartTask(LUA_SCRIPT)
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
	err := exec.Run(&exec.Command{
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
		Timeout: ctutil.MASTER_SCRIPT_RUN_LUA_TIMEOUT,
	})
	statusTracker.FinishTask(token, err)
	return err
}

// Define frontend.ChromiumBuildDBTask here so we can add methods.
type ChromiumBuildTask struct {
	chromium_builds.DBTask
}

func (task *ChromiumBuildTask) Execute() error {
	token := statusTracker.StartTask(CHROMIUM_BUILD)
	runId := runId(task)
	err := exec.Run(&exec.Command{
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
		Timeout: ctutil.MASTER_SCRIPT_BUILD_CHROMIUM_TIMEOUT,
	})
	statusTracker.FinishTask(token, err)
	return err
}

// Define frontend.RecreatePageSetsDBTask here so we can add methods.
type RecreatePageSetsTask struct {
	admin_tasks.RecreatePageSetsDBTask
}

func (task *RecreatePageSetsTask) Execute() error {
	token := statusTracker.StartTask(RECREATE_PAGE_SETS)
	runId := runId(task)
	err := exec.Run(&exec.Command{
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
		Timeout: ctutil.MASTER_SCRIPT_CREATE_PAGESETS_TIMEOUT,
	})
	statusTracker.FinishTask(token, err)
	return err
}

// Define frontend.RecreateWebpageArchivesDBTask here so we can add methods.
type RecreateWebpageArchivesTask struct {
	admin_tasks.RecreateWebpageArchivesDBTask
}

func (task *RecreateWebpageArchivesTask) Execute() error {
	token := statusTracker.StartTask(RECREATE_WEBPAGE_ARCHIVES)
	runId := runId(task)
	err := exec.Run(&exec.Command{
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
		Timeout: ctutil.MASTER_SCRIPT_CAPTURE_ARCHIVES_TIMEOUT,
	})
	statusTracker.FinishTask(token, err)
	return err
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
	default:
		glog.Errorf("Missing case for %T in asPollerTask", otherTask)
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

func pollAndExecOnce() {
	token := statusTracker.StartTask(POLL)
	pending, err := frontend.GetOldestPendingTaskV2()
	statusTracker.FinishTask(token, err)
	if err != nil {
		glog.Error(err)
		return
	}
	task := asPollerTask(pending)
	if task == nil {
		return
	}
	taskName, id := task.GetTaskName(), task.GetCommonCols().Id
	glog.Infof("Preparing to execute task %s %d", taskName, id)
	if err = updateAndBuild(); err != nil {
		glog.Error(err)
		return
	}
	glog.Infof("Executing task %s %d", taskName, id)
	if err = task.Execute(); err == nil {
		glog.Infof("Completed task %s %d", taskName, id)
	} else {
		glog.Errorf("Task %s %d failed: %v", taskName, id, err)
		if !*dryRun {
			if err := updateWebappTaskSetFailed(task); err != nil {
				glog.Error(err)
			}
		}
	}
}

func main() {
	defer common.LogPanic()
	master_common.InitWithMetrics2("ct-poller", influxHost, influxUser, influxPassword, influxDatabase)

	if logDirFlag := flag.Lookup("log_dir"); logDirFlag != nil {
		logDir = logDirFlag.Value.String()
	}

	if *dryRun {
		exec.SetRunForTesting(func(command *exec.Command) error {
			glog.Infof("dry_run: %s", exec.DebugString(command))
			return nil
		})
	}

	statusTracker.(*heartbeatStatusTracker).StartMetrics()

	// Run immediately, since pollTick will not fire until after pollInterval.
	pollAndExecOnce()
	for _ = range time.Tick(*pollInterval) {
		pollAndExecOnce()
	}
}
