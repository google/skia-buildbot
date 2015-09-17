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

	metrics "github.com/rcrowley/go-metrics"
	"go.skia.org/infra/ct/go/ctfe/admin_tasks"
	"go.skia.org/infra/ct/go/ctfe/capture_skps"
	"go.skia.org/infra/ct/go/ctfe/chromium_builds"
	"go.skia.org/infra/ct/go/ctfe/chromium_perf"
	"go.skia.org/infra/ct/go/ctfe/lua_scripts"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	"go.skia.org/infra/ct/go/frontend"
	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	skutil "go.skia.org/infra/go/util"
)

// flags
var (
	graphiteServer = flag.String("graphite_server", "localhost:2003", "Location of the Graphite metrics ingestion server.")
	// TODO(benjaminwagner): There are a lot of changes needed to make --local=true do something
	// useful:
	//  - ctutil.CtTreeDir must be set to the current working copy.
	//  - Each of these programs must add a --local flag that allows running locally:
	//    - run_chromium_perf_on_workers
	//    - capture_skps_on_workers
	//    - run_lua_on_workers
	//    - build_chromium
	//    - create_pagesets_on_workers
	//    - capture_archives_on_workers
	//    - check_workers_health
	//  - The Execute methods must add the --local flag and any other required flags.
	//  - May want to add a port option to allow running CTFE on a port other than 8000.
	local                     = flag.Bool("local", false, "Running locally if true. As opposed to in production. This option is not fully implemented.")
	dryRun                    = flag.Bool("dry_run", false, "If true, just log the commands that would be executed; don't actually execute the commands. Still polls CTFE for pending tasks, but does not post updates.")
	pollInterval              = flag.Duration("poll_interval", 30*time.Second, "How often to poll CTFE for new pending tasks.")
	workerHealthCheckInterval = flag.Duration("worker_health_check_interval", 30*time.Minute, "How often to check worker health.")
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
	CHECK_WORKER_HEALTH
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
	case CHECK_WORKER_HEALTH:
		return "CHECK_WORKER_HEALTH"
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
	lastUpdate time.Time
	// State of the main goroutine.
	currentStatus TaskType
	// Updated with the value of currentStatus for monitoring.
	currentStatusGauge metrics.Gauge
	// Reports the duration of each type of task.
	taskDurations map[TaskType]metrics.Histogram
	// Tracks the time of the last successful completion.
	lastSuccessTime map[TaskType]time.Time
	// Tracks the time of the last failure.
	lastFailureTime map[TaskType]time.Time
	// Stores any errors encountered in StartTask or FinishTask.
	errs []error
}

func NewHeartbeatStatusTracker() StatusTracker {
	h := &heartbeatStatusTracker{}
	h.currentStatusGauge = metrics.GetOrRegisterGauge("current-status", metrics.DefaultRegistry)
	h.taskDurations = make(map[TaskType]metrics.Histogram)
	for t := UPDATE_AND_BUILD; t <= POLL; t++ {
		// Using the values from metrics.NewTimer().
		s := metrics.NewExpDecaySample(1028, 0.015)
		h.taskDurations[t] = metrics.GetOrRegisterHistogram(fmt.Sprintf("duration-%s", t), metrics.DefaultRegistry, s)
	}
	h.lastSuccessTime = make(map[TaskType]time.Time)
	h.lastFailureTime = make(map[TaskType]time.Time)
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
	h.lastUpdate = time.Now()
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
			h.taskDurations[t].Update(int64(time.Since(h.lastUpdate).Seconds()))
			if err == nil {
				h.lastSuccessTime[t] = time.Now()
			} else {
				h.lastFailureTime[t] = time.Now()
			}
		}
	} else if token != nil {
		h.errs = append(h.errs, fmt.Errorf("Expected argument to FinishTask to be TaskType, instead got %T: %#v", token, token))
	}
	h.currentStatus = IDLE
	h.currentStatusGauge.Update(int64(IDLE))
	h.lastUpdate = time.Now()
}

// StartMetrics registers gauges with the graphite server that indicate the poller is running
// healthily and starts a goroutine to update them periodically.
func (h *heartbeatStatusTracker) StartMetrics() {
	timeSinceLastUpdateGauge := metrics.GetOrRegisterGaugeFloat64("time-since-last-update", metrics.DefaultRegistry)
	healthyGauge := metrics.GetOrRegisterGauge("healthy", metrics.DefaultRegistry)
	timeSinceLastSuccess := make(map[TaskType]metrics.GaugeFloat64)
	timeSinceLastFailure := make(map[TaskType]metrics.GaugeFloat64)
	for t := UPDATE_AND_BUILD; t <= POLL; t++ {
		timeSinceLastSuccess[t] = metrics.GetOrRegisterGaugeFloat64(fmt.Sprintf("time-since-last-success-%s", t), metrics.DefaultRegistry)
		timeSinceLastFailure[t] = metrics.GetOrRegisterGaugeFloat64(fmt.Sprintf("time-since-last-failure-%s", t), metrics.DefaultRegistry)
	}
	go func() {
		for _ = range time.Tick(common.SAMPLE_PERIOD) {
			h.mu.Lock()
			timeSinceLastUpdate := time.Since(h.lastUpdate)
			currentStatus := h.currentStatus
			for t := UPDATE_AND_BUILD; t <= POLL; t++ {
				if v, ok := h.lastSuccessTime[t]; ok {
					timeSinceLastSuccess[t].Update(time.Since(v).Seconds())
				}
				if v, ok := h.lastFailureTime[t]; ok {
					timeSinceLastFailure[t].Update(time.Since(v).Seconds())
				}
			}
			lastSuccessfulPoll := h.lastSuccessTime[POLL]
			errs := h.errs
			h.errs = nil
			h.mu.Unlock()
			timeSinceLastUpdateGauge.Update(timeSinceLastUpdate.Seconds())
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
			case CHECK_WORKER_HEALTH:
				expectedDuration = ctutil.CHECK_WORKERS_HEALTH_TIMEOUT
			}
			// Provide a bit of head room.
			expectedDuration += 2 * time.Minute

			if expectPoll && time.Since(lastSuccessfulPoll) > 2*time.Minute {
				errs = append(errs, fmt.Errorf("Last successful poll was at %s.", lastSuccessfulPoll))
			}
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
		".chromium.patch": task.ChromiumPatch,
		".blink.patch":    task.BlinkPatch,
		".skia.patch":     task.SkiaPatch,
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
			"--target_platform=" + task.Platform,
			"--run_id=" + runId,
			"--log_dir=" + logDir,
			"--log_id=" + runId,
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
			"--apply_patches=false",
			"--chromium_hash=" + task.ChromiumRev,
			"--skia_hash=" + task.SkiaRev,
			"--log_dir=" + logDir,
			"--log_id=" + runId,
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
	chromiumBuildDir := ctutil.ChromiumBuildDir(task.ChromiumRev, task.SkiaRev, "")
	err := exec.Run(&exec.Command{
		Name: "capture_archives_on_workers",
		Args: []string{
			"--emails=" + task.Username,
			"--gae_task_id=" + strconv.FormatInt(task.Id, 10),
			"--run_id=" + runId,
			"--pageset_type=" + task.PageSets,
			"--chromium_build=" + chromiumBuildDir,
			"--log_dir=" + logDir,
			"--log_id=" + runId,
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

func checkWorkerHealth() error {
	token := statusTracker.StartTask(CHECK_WORKER_HEALTH)
	err := exec.Run(&exec.Command{
		Name:    "check_workers_health",
		Args:    []string{"--log_dir=" + logDir},
		Timeout: ctutil.CHECK_WORKERS_HEALTH_TIMEOUT,
	})
	statusTracker.FinishTask(token, err)
	return err
}

func doWorkerHealthCheck() {
	if err := updateAndBuild(); err != nil {
		glog.Error(err)
		return
	}
	if err := checkWorkerHealth(); err != nil {
		glog.Error(err)
		return
	}
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
	common.InitWithMetrics("ct-poller", graphiteServer)

	if logDirFlag := flag.Lookup("log_dir"); logDirFlag != nil {
		logDir = logDirFlag.Value.String()
	}

	if *dryRun {
		exec.SetRunForTesting(func(command *exec.Command) error {
			glog.Infof("dry_run: %s", exec.DebugString(command))
			return nil
		})
	}
	if *local {
		frontend.InitForTesting("http://localhost:8000/")
	} else {
		frontend.MustInit()
	}

	statusTracker.(*heartbeatStatusTracker).StartMetrics()

	workerHealthTick := time.Tick(*workerHealthCheckInterval)
	pollTick := time.Tick(*pollInterval)
	// Run immediately, since pollTick will not fire until after pollInterval.
	pollAndExecOnce()
	for {
		select {
		case <-workerHealthTick:
			doWorkerHealthCheck()
		case <-pollTick:
			pollAndExecOnce()
		}
	}
}
