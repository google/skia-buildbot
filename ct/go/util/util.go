// Utility that contains methods for both CT master and worker scripts.
package util

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"

	"go.skia.org/infra/go/cas"
	"go.skia.org/infra/go/cas/rbe"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/specs"
)

const (
	MAX_SYNC_TRIES = 3

	TS_FORMAT = "20060102150405"

	MAX_SIMULTANEOUS_SWARMING_TASKS_PER_RUN = 10000

	PATCH_LIMIT = 1 << 26
)

var (
	CIPD_PATHS = []string{
		"cipd_bin_packages",
		"cipd_bin_packages/bin",
		"cipd_bin_packages/cpython",
		"cipd_bin_packages/cpython/bin",
		"cipd_bin_packages/cpython3",
		"cipd_bin_packages/cpython3/bin",
	}
)

// CasSpecs for master scripts.
func CasCreatePagesetsMaster() *CasSpec {
	return &CasSpec{
		Paths: []string{"bin/create_pagesets_on_workers"},
		IncludeCasSpecs: []*CasSpec{
			CasPython(),
			CasCreatePagesets(),
		},
	}
}
func CasCaptureArchivesMaster() *CasSpec {
	return &CasSpec{
		Paths: []string{"bin/capture_archives_on_workers"},
		IncludeCasSpecs: []*CasSpec{
			CasPython(),
			CasIsolateTelemetryLinux(),
			CasCaptureArchives(),
		},
	}
}
func CasChromiumAnalysisMaster() *CasSpec {
	return &CasSpec{
		Paths: []string{"bin/run_chromium_analysis_on_workers"},
		IncludeCasSpecs: []*CasSpec{
			CasPython(),
			CasBuildRepoLinux(),
			CasIsolateTelemetryLinux(),
			CasChromiumAnalysisLinux(),
		},
	}
}
func CasChromiumPerfMaster() *CasSpec {
	return &CasSpec{
		Paths: []string{"bin/run_chromium_perf_on_workers"},
		IncludeCasSpecs: []*CasSpec{
			CasPython(),
			CasBuildRepoLinux(),
			CasIsolateTelemetryLinux(),
			CasChromiumPerfLinux(),
		},
	}
}
func CasMetricsAnalysisMaster() *CasSpec {
	return &CasSpec{
		Paths: []string{"bin/metrics_analysis_on_workers"},
		IncludeCasSpecs: []*CasSpec{
			CasPython(),
			CasIsolateTelemetryLinux(),
			CasMetricsAnalysis(),
		},
	}
}

// CasSpecs for worker scripts.
func CasCreatePagesets() *CasSpec {
	return &CasSpec{
		Paths:           []string{"bin/create_pagesets"},
		IncludeCasSpecs: []*CasSpec{CasPython()},
	}
}
func CasCaptureArchives() *CasSpec {
	return &CasSpec{
		Paths: []string{"bin/capture_archives"},
	}
}
func CasChromiumAnalysisLinux() *CasSpec {
	return &CasSpec{
		Paths:           []string{"bin/run_chromium_analysis"},
		IncludeCasSpecs: []*CasSpec{CasPython()},
	}
}
func CasChromiumPerfLinux() *CasSpec {
	return &CasSpec{
		Paths:           []string{"bin/run_chromium_perf"},
		IncludeCasSpecs: []*CasSpec{CasPython()},
	}
}
func CasMetricsAnalysis() *CasSpec {
	return &CasSpec{
		Paths:           []string{"bin/metrics_analysis"},
		IncludeCasSpecs: []*CasSpec{CasPython()},
	}
}

// CasSpecs for build scripts.
func CasBuildRepoLinux() *CasSpec {
	return &CasSpec{
		Paths:           []string{"bin/build_repo"},
		IncludeCasSpecs: []*CasSpec{CasPython()},
	}
}
func CasIsolateTelemetryLinux() *CasSpec {
	return &CasSpec{
		Paths:           []string{"bin/isolate_telemetry"},
		IncludeCasSpecs: []*CasSpec{CasPython()},
	}
}
func CasPython() *CasSpec {
	return &CasSpec{
		Paths: []string{"py/"},
	}
}

// CasSpec describes a set of files to upload to CAS.
type CasSpec struct {
	Paths           []string
	IncludeCasSpecs []*CasSpec
	IncludeDigests  []string
}

func TimeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	sklog.Infof("===== %s took %s =====", name, elapsed)
}

// ExecuteCmd calls ExecuteCmdWithConfigurableLogging with logStdout and logStderr set to true.
func ExecuteCmd(ctx context.Context, binary string, args, env []string, timeout time.Duration, stdout, stderr io.Writer) error {
	return ExecuteCmdWithConfigurableLogging(ctx, binary, args, env, timeout, stdout, stderr, true, true)
}

// ExecuteCmdWithConfigurableLogging executes the specified binary with the specified args and env.
// Stdout and Stderr are written to stdout and stderr respectively if specified. If not specified
// then Stdout and Stderr will be outputted only to sklog.
func ExecuteCmdWithConfigurableLogging(ctx context.Context, binary string, args, env []string, timeout time.Duration, stdout, stderr io.Writer, logStdout, logStderr bool) error {
	return exec.Run(ctx, &exec.Command{
		Name:        binary,
		Args:        args,
		Env:         env,
		InheritPath: true,
		Timeout:     timeout,
		LogStdout:   logStdout,
		Stdout:      stdout,
		LogStderr:   logStderr,
		Stderr:      stderr,
	})
}

// SyncDir runs "git pull" and "gclient sync" on the specified directory.
// The revisions map enforces revision/hash for the solutions with the format
// branch@rev.
func SyncDir(ctx context.Context, dir string, revisions map[string]string, additionalArgs []string, gitExec string) error {
	err := os.Chdir(dir)
	if err != nil {
		return fmt.Errorf("Could not chdir to %s: %s", dir, err)
	}

	for i := 0; i < MAX_SYNC_TRIES; i++ {
		if i > 0 {
			sklog.Warningf("%d. retry for syncing %s", i, dir)
		}

		err = syncDirStep(ctx, revisions, additionalArgs, gitExec)
		if err == nil {
			break
		}
		sklog.Errorf("Error syncing %s: %s", dir, err)
	}

	if err != nil {
		sklog.Errorf("Failed to sync %s after %d attempts", dir, MAX_SYNC_TRIES)
	}
	return err
}

func syncDirStep(ctx context.Context, revisions map[string]string, additionalArgs []string, gitExec string) error {
	err := ExecuteCmd(ctx, gitExec, []string{"pull"}, []string{}, GIT_PULL_TIMEOUT, nil, nil)
	if err != nil {
		return fmt.Errorf("Error running git pull: %s", err)
	}
	syncCmd := []string{"sync", "--force"}
	syncCmd = append(syncCmd, additionalArgs...)
	for branch, rev := range revisions {
		syncCmd = append(syncCmd, "--revision")
		syncCmd = append(syncCmd, fmt.Sprintf("%s@%s", branch, rev))
	}
	err = ExecuteCmd(ctx, BINARY_GCLIENT, syncCmd, []string{}, GCLIENT_SYNC_TIMEOUT, nil, nil)
	if err != nil {
		return fmt.Errorf("Error running gclient sync: %s", err)
	}
	return nil
}

func runSkiaGnGen(ctx context.Context, clangLocation, gnExtraArgs string) error {
	// Run "bin/fetch-gn".
	util.LogErr(ExecuteCmd(ctx, "bin/fetch-gn", []string{}, []string{}, FETCH_GN_TIMEOUT, nil,
		nil))
	// gn gen out/Release '--args=cc="/home/chrome-bot/test/clang_linux/bin/clang" cxx="/home/chrome-bot/test/clang_linux/bin/clang++" extra_cflags=["-B/home/chrome-bot/test/clang_linux/bin"] extra_ldflags=["-B/home/chrome-bot/test/clang_linux/bin", "-fuse-ld=lld"] is_debug=false target_cpu="x86_64"'
	gnArgs := fmt.Sprintf("--args=cc=\"%s/bin/clang\" cxx=\"%s/bin/clang++\" extra_cflags=[\"-B%s/bin\"] extra_ldflags=[\"-B%s/bin\", \"-fuse-ld=lld\"] is_debug=false target_cpu=\"x86_64\"", clangLocation, clangLocation, clangLocation, clangLocation)
	if gnExtraArgs != "" {
		gnArgs += " " + gnExtraArgs
	}
	if err := ExecuteCmd(ctx, "buildtools/linux64/gn", []string{"gen", "out/Release", gnArgs}, os.Environ(), GN_GEN_TIMEOUT, nil, nil); err != nil {
		return fmt.Errorf("Error while running gn: %s", err)
	}
	return nil
}

// GetCipdPackageFromAsset returns a string of the format "path:package_name:version".
// It returns the latest version of the asset via gitiles.
func GetCipdPackageFromAsset(assetName string) (string, error) {
	// Find the latest version of the asset from gitiles.
	assetVersionFilePath := path.Join("infra", "bots", "assets", assetName, "VERSION")
	contents, err := gitiles.NewRepo(common.REPO_SKIA, nil).ReadFile(context.Background(), assetVersionFilePath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:skia/bots/%s:version:%s", assetName, assetName, strings.TrimSpace(string(contents))), nil
}

// ResetCheckout resets the specified Git checkout.
func ResetCheckout(ctx context.Context, dir, resetTo, checkoutArg, gitExec string) error {
	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("Could not chdir to %s: %s", dir, err)
	}
	// Clear out remnants of incomplete rebases from .git/rebase-apply.
	rebaseArgs := []string{"rebase", "--abort"}
	util.LogErr(ExecuteCmd(ctx, gitExec, rebaseArgs, []string{}, GIT_REBASE_TIMEOUT, nil, nil))
	// Checkout the specified branch or argument (eg: --detach).
	checkoutArgs := []string{"checkout", checkoutArg}
	util.LogErr(ExecuteCmd(ctx, gitExec, checkoutArgs, []string{}, GIT_CHECKOUT_TIMEOUT, nil, nil))
	// Run "git reset --hard HEAD"
	resetArgs := []string{"reset", "--hard", resetTo}
	util.LogErr(ExecuteCmd(ctx, gitExec, resetArgs, []string{}, GIT_RESET_TIMEOUT, nil, nil))
	// Run "git clean -f"
	// Not doing "-d" here because it can delete directories like "/android_build_tools/aapt2/lib64/"
	// even if "/android_build_tools/aapt2/lib64/*.so" is in .gitignore.
	cleanArgs := []string{"clean", "-f"}
	util.LogErr(ExecuteCmd(ctx, gitExec, cleanArgs, []string{}, GIT_CLEAN_TIMEOUT, nil, nil))

	return nil
}

// ApplyPatch applies a patch to a Git checkout.
func ApplyPatch(ctx context.Context, patch, dir, gitExec string) error {
	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("Could not chdir to %s: %s", dir, err)
	}
	// Run "git apply --index -p1 --verbose --ignore-whitespace
	//      --ignore-space-change ${PATCH_FILE}"
	args := []string{"apply", "--index", "-p1", "--verbose", "--ignore-whitespace", "--ignore-space-change", patch}
	return ExecuteCmd(ctx, gitExec, args, []string{}, GIT_APPLY_TIMEOUT, nil, nil)
}

// CleanTmpDir deletes all tmp files from the caller because telemetry tends to
// generate a lot of temporary artifacts there and they take up root disk space.
func CleanTmpDir() {
	files, _ := ioutil.ReadDir(os.TempDir())
	for _, f := range files {
		util.RemoveAll(filepath.Join(os.TempDir(), f.Name()))
	}
}

func GetTimeFromTs(formattedTime string) time.Time {
	t, _ := time.Parse(TS_FORMAT, formattedTime)
	return t
}

func GetCurrentTs() string {
	return time.Now().UTC().Format(TS_FORMAT)
}

func GetCurrentTsInt64() int64 {
	ts, err := strconv.ParseInt(GetCurrentTs(), 10, 64)
	if err != nil {
		sklog.Fatalf("Could not parse timestamp: %s", err)
	}
	return ts
}

// Returns channel that contains all pageset file names without the timestamp
// file and pyc files.
func GetClosedChannelOfPagesets(fileInfos []os.FileInfo) chan string {
	pagesetsChannel := make(chan string, len(fileInfos))
	for _, fileInfo := range fileInfos {
		pagesetName := fileInfo.Name()
		pagesetBaseName := filepath.Base(pagesetName)
		if filepath.Ext(pagesetBaseName) == ".pyc" {
			// Ignore .pyc files.
			continue
		}
		pagesetsChannel <- pagesetName
	}
	close(pagesetsChannel)
	return pagesetsChannel
}

// Running benchmarks in parallel leads to multiple chrome instances coming up
// at the same time, when there are crashes chrome processes stick around which
// can severely impact the machine's performance. To stop this from
// happening chrome zombie processes are periodically killed.
func ChromeProcessesCleaner(ctx context.Context, locker sync.Locker, chromeCleanerTimer time.Duration) {
	for range time.Tick(chromeCleanerTimer) {
		sklog.Info("The chromeProcessesCleaner goroutine has started")
		sklog.Info("Waiting for all existing tasks to complete before killing zombie chrome processes")
		locker.Lock()
		util.LogErr(ExecuteCmd(ctx, "pkill", []string{"-9", "chrome"}, []string{}, PKILL_TIMEOUT, nil, nil))
		locker.Unlock()
	}
}

// Contains the data included in CT pagesets.
type PagesetVars struct {
	// A comma separated list of URLs.
	UrlsList string `json:"urls_list"`
	// Will be either "mobile" or "desktop".
	UserAgent string `json:"user_agent"`
	// The location of the web page's WPR data file.
	ArchiveDataFile string `json:"archive_data_file"`
}

func ReadPageset(pagesetPath string) (PagesetVars, error) {
	decodedPageset := PagesetVars{}
	pagesetContent, err := os.Open(pagesetPath)
	defer util.Close(pagesetContent)
	if err != nil {
		return decodedPageset, fmt.Errorf("Could not read %s: %s", pagesetPath, err)
	}
	if err := json.NewDecoder(pagesetContent).Decode(&decodedPageset); err != nil {
		return decodedPageset, fmt.Errorf("Could not JSON decode %s: %s", pagesetPath, err)
	}
	return decodedPageset, nil
}

// GetStartRange returns the range worker should start processing at based on its num and how many
// pages it is allowed to process.
func GetStartRange(workerNum, numPagesPerBot int) int {
	return ((workerNum - 1) * numPagesPerBot) + 1
}

// GetNumPagesPerBot returns the number of web pages each worker should process.
func GetNumPagesPerBot(repeatValue, maxPagesPerBot int) int {
	return int(math.Ceil(float64(maxPagesPerBot) / float64(repeatValue)))
}

// UploadToCAS uploads the given CasSpec and returns the resulting digest.
func UploadToCAS(ctx context.Context, casClient cas.CAS, casSpec *CasSpec, local, runOnMaster bool) (string, error) {
	casRoot, err := GetCASRoot(local, runOnMaster)
	if err != nil {
		return "", skerr.Wrapf(err, "failed to get CAS root")
	}

	// Gather the paths to upload.
	paths := util.NewStringSet()
	digests := util.NewStringSet()
	var gather func(*CasSpec)
	gather = func(casSpec *CasSpec) {
		paths.AddLists(casSpec.Paths)
		digests.AddLists(casSpec.IncludeDigests)
		for _, includeCasSpec := range casSpec.IncludeCasSpecs {
			gather(includeCasSpec)
		}
	}
	gather(casSpec)

	// Upload.
	rootDigest, err := casClient.Upload(ctx, casRoot, paths.Keys(), nil)
	if err != nil {
		return "", skerr.Wrapf(err, "failed to upload to CAS")
	}
	digests[rootDigest] = true
	mergedCASDigest, err := casClient.Merge(ctx, digests.Keys())
	if err != nil {
		return "", skerr.Wrapf(err, "failed to merge CAS digests")
	}
	return mergedCASDigest, nil
}

// TriggerSwarmingTask returns the number of triggered tasks and an error (if any).
func TriggerSwarmingTask(ctx context.Context, pagesetType, taskPrefix, runID, targetPlatform string, casSpec *CasSpec, hardTimeout, ioTimeout time.Duration, priority, maxPagesPerBot, numPages int, runOnGCE, local bool, repeatValue int, baseCmd []string, swarmingClient swarming.ApiClient, casClient cas.CAS) (int, error) {
	// Upload the task inputs to CAS.
	casDigest, err := UploadToCAS(ctx, casClient, casSpec, local, false)
	if err != nil {
		return 0, skerr.Wrapf(err, "failed to upload CAS inputs for task")
	}

	// Create swarming commands for all tasks.
	numPagesPerBot := GetNumPagesPerBot(repeatValue, maxPagesPerBot)
	numTasks := int(math.Ceil(float64(numPages) / float64(numPagesPerBot)))
	tasksToCmds := map[string][]string{}
	for i := 1; i <= numTasks; i++ {
		taskCmd := append(
			baseCmd,
			"--start_range="+strconv.Itoa(GetStartRange(i, numPagesPerBot)),
			"--num="+strconv.Itoa(numPagesPerBot),
		)
		if pagesetType != "" {
			taskCmd = append(taskCmd, "--pageset_type="+pagesetType)
		}
		taskName := fmt.Sprintf("%s_%d", taskPrefix, i)
		tasksToCmds[taskName] = taskCmd
	}

	// Find swarming dimensions to use.
	var dimensions map[string]string
	if runOnGCE {
		if targetPlatform == PLATFORM_WINDOWS {
			dimensions = GCE_WINDOWS_WORKER_DIMENSIONS
		} else {
			dimensions = GCE_LINUX_WORKER_DIMENSIONS
		}
	} else {
		if targetPlatform == PLATFORM_ANDROID {
			dimensions = GOLO_ANDROID_WORKER_DIMENSIONS
		} else {
			dimensions = GOLO_LINUX_WORKER_DIMENSIONS
		}
	}

	// The channel where batches of tasks to be triggered and collected will be sent to.
	chTasks := make(chan map[string][]string)
	// Kick off one goroutine to populate the above channel.
	go func() {
		defer close(chTasks)
		tmpMap := map[string][]string{}
		for task, cmds := range tasksToCmds {
			if len(tmpMap) >= MAX_SIMULTANEOUS_SWARMING_TASKS_PER_RUN {
				// Add the map to the channel.
				chTasks <- tmpMap
				// Reinitialize the temporary map.
				tmpMap = map[string][]string{}
			}
			tmpMap[task] = cmds
		}
		chTasks <- tmpMap
	}()

	cipdPkgs := []string{}
	if targetPlatform == PLATFORM_WINDOWS {
		cipdPkgs = append(cipdPkgs, LUCI_AUTH_CIPD_PACKAGE_WIN)
	} else {
		cipdPkgs = append(cipdPkgs, LUCI_AUTH_CIPD_PACKAGE_LINUX)
		cipdPkgs = append(cipdPkgs, cipd.GetStrCIPDPkgs(cipd.PkgsPython[cipd.PlatformLinuxAmd64])...)
	}
	if targetPlatform == PLATFORM_ANDROID {
		// Add adb CIPD package for Android runs.
		cipdPkgs = append(cipdPkgs, ADB_CIPD_PACKAGE)
	}

	// Trigger and collect swarming tasks.
	for tasksMap := range chTasks {
		// Collect all tasks and retrigger the ones that fail. Do this in a goroutine for
		// each task so that it is done in parallel and retries are immediately triggered
		// instead of at the end (see skbug.com/8191).
		var wg sync.WaitGroup
		for taskName, cmd := range tasksMap {
			wg.Add(1)
			// https://golang.org/doc/faq#closures_and_goroutines
			taskName := taskName
			cmd := cmd
			go func() {
				defer wg.Done()
				req, err := MakeSwarmingTaskRequest(ctx, taskName, casDigest, cipdPkgs, cmd, []string{"name:" + taskName, "runid:" + runID}, dimensions, map[string][]string{"PATH": CIPD_PATHS}, int64(priority), ioTimeout, casClient)
				if err != nil {
					sklog.Errorf("Failed to create Swarming task request for task %q: %s", taskName, err)
				}
				resp, err := swarmingClient.TriggerTask(ctx, req)
				if err != nil {
					sklog.Errorf("Could not trigger swarming task %s: %s", taskName, err)
					return
				}

				_, state, err := pollSwarmingTaskToCompletion(ctx, resp.TaskId, swarmingClient)
				if err != nil {
					sklog.Errorf("task %s failed: %s", taskName, err)
					if state == swarming.TASK_STATE_KILLED {
						sklog.Infof("task %s was killed (either manually or via CT's delete button). Not going to retry it.", taskName)
						return
					}
					sklog.Infof("Retrying task %s with high priority %d", taskName, TASKS_PRIORITY_HIGH)
					req.Priority = TASKS_PRIORITY_HIGH
					retryResp, err := swarmingClient.TriggerTask(ctx, req)
					if err != nil {
						sklog.Errorf("Could not trigger swarming retry task %s: %s", taskName, err)
						return
					}
					if _, _, err := pollSwarmingTaskToCompletion(ctx, retryResp.TaskId, swarmingClient); err != nil {
						sklog.Errorf("task %s failed inspite of a retry: %s", taskName, err)
						return
					}
				}
			}()
		}
		wg.Wait()

	}

	return numTasks, nil
}

// GetCASRoot returns the location of CT's CAS inputs. local should be set to
// true when debugging locally. runOnMaster should be set on ctfe. If both are
// false then it is assumed that we are running on a swarming bot.
func GetCASRoot(local, runOnMaster bool) (string, error) {
	if local {
		_, currentFile, _, _ := runtime.Caller(0)
		return filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(currentFile)))), nil
	} else if runOnMaster {
		return filepath.Join("/", "usr", "local", "share", "ctfe"), nil
	} else {
		return filepath.Abs(filepath.Join(filepath.Dir(filepath.Dir(os.Args[0]))))
	}
}

// GetPathToPyFiles returns the location of CT's python scripts.
// local should be set to true if we need the location of py files when debugging locally.
func GetPathToPyFiles(local bool) (string, error) {
	if local {
		_, currentFile, _, _ := runtime.Caller(0)
		return filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(currentFile))), "py"), nil
	} else {
		return filepath.Abs(filepath.Join(filepath.Dir(filepath.Dir(os.Args[0])), "py"))
	}
}

// GetPathToTelemetryBinaries returns the location of Telemetry binaries.
func GetPathToTelemetryBinaries(local bool) string {
	if local {
		return TelemetryBinariesDir
	} else {
		return filepath.Join(filepath.Dir(filepath.Dir(os.Args[0])), "tools", "perf")
	}
}

// GetPathToTelemetryBinaries returns the location of CT binaries in Telemetry.
func GetPathToTelemetryCTBinaries(local bool) string {
	return filepath.Join(GetPathToTelemetryBinaries(local), "contrib", "cluster_telemetry")
}

func MergeUploadCSVFiles(ctx context.Context, runID, pathToPyFiles string, gs *GcsUtil, totalPages, maxPagesPerBot int, handleStrings bool, repeatValue int) (string, []string, error) {
	localOutputDir := filepath.Join(StorageDir, BenchmarkRunsDir, runID)
	MkdirAll(localOutputDir, 0700)
	noOutputWorkers := []string{}
	// Copy outputs from all workers locally.
	numPagesPerBot := GetNumPagesPerBot(repeatValue, maxPagesPerBot)
	numTasks := int(math.Ceil(float64(totalPages) / float64(numPagesPerBot)))
	for i := 1; i <= numTasks; i++ {
		startRange := GetStartRange(i, numPagesPerBot)
		workerLocalOutputPath := filepath.Join(localOutputDir, strconv.Itoa(startRange)+".csv")
		workerRemoteOutputPath := filepath.Join(BenchmarkRunsDir, runID, strconv.Itoa(startRange), "outputs", runID+".output")
		respBody, err := gs.GetRemoteFileContents(workerRemoteOutputPath)
		if err != nil {
			sklog.Errorf("Could not fetch %s: %s", workerRemoteOutputPath, err)
			noOutputWorkers = append(noOutputWorkers, strconv.Itoa(i))
			continue
		}
		defer util.Close(respBody)
		out, err := os.Create(workerLocalOutputPath)
		if err != nil {
			return "", noOutputWorkers, fmt.Errorf("Unable to create file %s: %s", workerLocalOutputPath, err)
		}
		defer util.Close(out)
		defer util.Remove(workerLocalOutputPath)
		if _, err = io.Copy(out, respBody); err != nil {
			return "", noOutputWorkers, fmt.Errorf("Unable to copy to file %s: %s", workerLocalOutputPath, err)
		}
		// If an output is less than 20 bytes that means something went wrong on the worker.
		outputInfo, err := out.Stat()
		if err != nil {
			return "", noOutputWorkers, fmt.Errorf("Unable to stat file %s: %s", workerLocalOutputPath, err)
		}
		if outputInfo.Size() <= 20 {
			sklog.Errorf("Output file was less than 20 bytes %s: %s", workerLocalOutputPath, err)
			noOutputWorkers = append(noOutputWorkers, strconv.Itoa(i))
			continue
		}
	}
	// Call csv_merger.py to merge all results into a single results CSV.
	pathToCsvMerger := filepath.Join(pathToPyFiles, "csv_merger.py")
	outputFileName := runID + ".output"
	outputFilePath := filepath.Join(localOutputDir, outputFileName)
	args := []string{
		pathToCsvMerger,
		"--csv_dir=" + localOutputDir,
		"--output_csv_name=" + outputFilePath,
	}
	if handleStrings {
		args = append(args, "--handle_strings")
	}
	err := ExecuteCmd(ctx, BINARY_PYTHON, args, []string{fmt.Sprintf("VPYTHON_VIRTUALENV_ROOT=%s", os.TempDir())}, CSV_MERGER_TIMEOUT, nil, nil)
	if err != nil {
		return outputFilePath, noOutputWorkers, fmt.Errorf("Error running csv_merger.py: %s", err)
	}
	// Copy the output file to Google Storage.
	remoteOutputDir := path.Join(BenchmarkRunsStorageDir, runID, "consolidated_outputs")
	if err := gs.UploadFile(outputFileName, localOutputDir, remoteOutputDir); err != nil {
		return outputFilePath, noOutputWorkers, fmt.Errorf("Unable to upload %s to %s: %s", outputFileName, remoteOutputDir, err)
	}
	return outputFilePath, noOutputWorkers, nil
}

// GetStrFlagValue returns the defaultValue if the specified flag name is not in benchmarkArgs.
func GetStrFlagValue(benchmarkArgs, flagName, defaultValue string) string {
	if strings.Contains(benchmarkArgs, flagName) {
		r := regexp.MustCompile(flagName + `[= ](\w+)`)
		m := r.FindStringSubmatch(benchmarkArgs)
		if len(m) >= 2 {
			return m[1]
		}
	}
	// If we reached here then return the default Value.
	return defaultValue
}

// GetUserAgentValue returns the defaultValue if "--user-agent" is not specified in benchmarkArgs.
func GetUserAgentValue(benchmarkArgs, defaultValue string) string {
	return GetStrFlagValue(benchmarkArgs, USER_AGENT_FLAG, defaultValue)
}

// GetRepeatValue returns the defaultValue if "--pageset-repeat" is not specified in benchmarkArgs.
func GetRepeatValue(benchmarkArgs string, defaultValue int) int {
	return GetIntFlagValue(benchmarkArgs, PAGESET_REPEAT_FLAG, defaultValue)
}

// GetRunBenchmarkTimeoutValue returns the defaultValue if "--run_benchmark_timeout" is not specified in benchmarkArgs.
func GetRunBenchmarkTimeoutValue(benchmarkArgs string, defaultValue int) int {
	return GetIntFlagValue(benchmarkArgs, RUN_BENCHMARK_TIMEOUT_FLAG, defaultValue)
}

// GetMaxPagesPerBotValue returns the defaultValue if "--max-pages-per-bot" is not specified in benchmarkArgs.
func GetMaxPagesPerBotValue(benchmarkArgs string, defaultValue int) int {
	return GetIntFlagValue(benchmarkArgs, MAX_PAGES_PER_BOT, defaultValue)
}

// GetNumAnalysisRetriesValue returns the defaultValue if "--num-analysis-retries" is not specified in benchmarkArgs.
func GetNumAnalysisRetriesValue(benchmarkArgs string, defaultValue int) int {
	return GetIntFlagValue(benchmarkArgs, NUM_ANALYSIS_RETRIES, defaultValue)
}

// GetIntFlagValue returns the defaultValue if the specified flag name is not in benchmarkArgs.
func GetIntFlagValue(benchmarkArgs, flagName string, defaultValue int) int {
	if strings.Contains(benchmarkArgs, flagName) {
		r := regexp.MustCompile(flagName + `[= ](\d+)`)
		m := r.FindStringSubmatch(benchmarkArgs)
		if len(m) != 0 {
			ret, err := strconv.Atoi(m[1])
			if err != nil {
				return defaultValue
			}
			return ret
		}
	}
	// If we reached here then return the default Value.
	return defaultValue
}

func RemoveFlagsFromArgs(benchmarkArgs string, flags ...string) string {
	for _, f := range flags {
		re, err := regexp.Compile(fmt.Sprintf(`\s*%s(=[[:alnum:]]*)?\s*`, f))
		if err != nil {
			sklog.Warningf("Could not compile flag regex with %s: %s", f, err)
			continue
		}
		benchmarkArgs = re.ReplaceAllString(benchmarkArgs, " ")
	}
	// Remove extra whitespace.
	return strings.Join(strings.Fields(benchmarkArgs), " ")
}

// RunBenchmark runs the specified benchmark with the specified arguments. It prints the output of
// the run_benchmark command and also returns the output in case the caller needs to do any
// post-processing on it. In case of any errors the output will be empty.
func RunBenchmark(ctx context.Context, fileInfoName, pathToPagesets, pathToPyFiles, localOutputDir, chromiumBinary, runID, browserExtraArgs, benchmarkName, targetPlatform, benchmarkExtraArgs, pagesetType string, defaultRepeatValue int, runOnSwarming bool) (string, error) {
	pagesetBaseName := filepath.Base(fileInfoName)
	if filepath.Ext(pagesetBaseName) == ".pyc" {
		// Ignore .pyc files.
		return "", nil
	}
	// Read the pageset.
	pagesetName := strings.TrimSuffix(pagesetBaseName, filepath.Ext(pagesetBaseName))
	pagesetPath := filepath.Join(pathToPagesets, fileInfoName)
	decodedPageset, err := ReadPageset(pagesetPath)
	if err != nil {
		return "", fmt.Errorf("Could not read %s: %s", pagesetPath, err)
	}
	sklog.Infof("===== Processing %s for %s =====", pagesetPath, runID)
	args := []string{
		filepath.Join(GetPathToTelemetryBinaries(!runOnSwarming), BINARY_RUN_BENCHMARK),
		benchmarkName,
		"--also-run-disabled-tests",
		"--urls-list=" + decodedPageset.UrlsList,
		"--archive-data-file=" + decodedPageset.ArchiveDataFile,
	}
	if GetUserAgentValue(benchmarkExtraArgs, "") == "" {
		// Add --user-agent only if the flag is not already specified. See skbug.com/11283 for context.
		args = append(args, "--user-agent="+decodedPageset.UserAgent)
	}

	// Need to capture output for all benchmarks.
	outputDirArgValue := filepath.Join(localOutputDir, pagesetName)
	args = append(args, "--output-dir="+outputDirArgValue)
	// Figure out which browser and device should be used.
	if targetPlatform == PLATFORM_ANDROID {
		if err := InstallChromeAPK(ctx, chromiumBinary); err != nil {
			return "", fmt.Errorf("Error while installing APK: %s", err)
		}
		if strings.Contains(chromiumBinary, CUSTOM_APK_DIR_NAME) {
			// TODO(rmistry): Not sure if the custom APK will always be called android-chrome. Might have to
			// make this configurable if it can vary or unzip and use sed to get the app name from the APK.
			args = append(args, "--browser=android-chrome")
		} else {
			args = append(args, "--browser=android-chromium")
		}
	} else {
		args = append(args, "--browser=exact", "--browser-executable="+chromiumBinary)
		args = append(args, "--device=desktop")
	}

	// Calculate the timeout.
	timeoutSecs := GetRunBenchmarkTimeoutValue(benchmarkExtraArgs, PagesetTypeToInfo[pagesetType].RunChromiumPerfTimeoutSecs)
	repeatBenchmark := GetRepeatValue(benchmarkExtraArgs, defaultRepeatValue)
	if repeatBenchmark > 0 {
		args = append(args, fmt.Sprintf("%s=%d", PAGESET_REPEAT_FLAG, repeatBenchmark))
		// Increase the timeoutSecs if repeats are used.
		timeoutSecs = timeoutSecs * repeatBenchmark
	}
	sklog.Infof("Using %d seconds for timeout", timeoutSecs)

	// Remove from benchmarkExtraArgs "special" flags that are recognized by CT but not
	// by the run_benchmark script.
	benchmarkExtraArgs = RemoveFlagsFromArgs(benchmarkExtraArgs, RUN_BENCHMARK_TIMEOUT_FLAG, MAX_PAGES_PER_BOT, NUM_ANALYSIS_RETRIES)
	// Split benchmark args if not empty and append to args.
	if benchmarkExtraArgs != "" {
		args = append(args, strings.Fields(benchmarkExtraArgs)...)
	}

	// Add browserArgs if not empty to args.
	if browserExtraArgs != "" {
		args = append(args, "--extra-browser-args="+browserExtraArgs)
	}
	env := []string{}
	if targetPlatform != PLATFORM_WINDOWS {
		// Set the DISPLAY.
		env = append(env, "DISPLAY=:0")
	}
	pythonExec := BINARY_VPYTHON3
	// Set VPYTHON_VIRTUALENV_ROOT for vpython
	env = append(env, fmt.Sprintf("VPYTHON_VIRTUALENV_ROOT=%s", os.TempDir()))
	// Append the original environment as well.
	for _, e := range os.Environ() {
		env = append(env, e)
	}
	if targetPlatform == PLATFORM_WINDOWS {
		// Could not figure out how to make vpython work on windows so use python instead.
		// The downside of this is that we might have to keep installing packages on win GCE
		// instances.
		pythonExec = BINARY_PYTHON
	} else if targetPlatform == PLATFORM_ANDROID {
		env = append(env, "BOTO_CONFIG=/home/chrome-bot/.boto.puppet-bak")
		// Reset android logcat prior to the run so that we can examine the logs later.
		util.LogErr(ExecuteCmd(ctx, BINARY_ADB, []string{"logcat", "-c"}, []string{}, ADB_ROOT_TIMEOUT, nil, nil))
	} else if targetPlatform == PLATFORM_LINUX {
		env = append(env, "BOTO_CONFIG=/home/chrome-bot/.boto.puppet-bak")
	}

	// Create buffer for capturing the stdout and stderr of the benchmark run.
	var b bytes.Buffer
	if _, err := b.WriteString(fmt.Sprintf("========== Stdout and stderr for %s ==========\n", pagesetPath)); err != nil {
		return "", fmt.Errorf("Error writing to output buffer: %s", err)
	}
	if err := ExecuteCmdWithConfigurableLogging(ctx, pythonExec, args, env, time.Duration(timeoutSecs)*time.Second, &b, &b, false, false); err != nil {
		if targetPlatform == PLATFORM_ANDROID {
			// Kill the port-forwarder to start from a clean slate.
			util.LogErr(ExecuteCmdWithConfigurableLogging(ctx, "pkill", []string{"-f", "forwarder_host"}, []string{}, PKILL_TIMEOUT, &b, &b, false, false))
		}
		output, getErr := GetRunBenchmarkOutput(b)
		util.LogErr(getErr)
		fmt.Println(output)
		return "", fmt.Errorf("Run benchmark command failed with: %s", err)
	}

	// Append logcat output if we ran on Android.
	if targetPlatform == PLATFORM_ANDROID {
		if err := ExecuteCmdWithConfigurableLogging(ctx, BINARY_ADB, []string{"logcat", "-d"}, env, ADB_ROOT_TIMEOUT, &b, &b, false, false); err != nil {
			return "", fmt.Errorf("Error running logcat -d: %s", err)
		}
	}

	output, err := GetRunBenchmarkOutput(b)
	if err != nil {
		return "", fmt.Errorf("Could not get run benchmark output: %s", err)
	}
	// Print the output and return.
	fmt.Println(output)
	return output, nil
}

func GetRunBenchmarkOutput(b bytes.Buffer) (string, error) {
	if _, err := b.WriteString("===================="); err != nil {
		return "", fmt.Errorf("Error writing to output buffer: %s", err)
	}
	return b.String(), nil
}

func MergeUploadCSVFilesOnWorkers(ctx context.Context, localOutputDir, pathToPyFiles, runID, remoteDir, valueColumnName string, gs *GcsUtil, startRange int, handleStrings, addRanks bool, pageRankToAdditionalFields map[string]map[string]string) error {
	// Move all results into a single directory.
	fileInfos, err := ioutil.ReadDir(localOutputDir)
	if err != nil {
		return fmt.Errorf("Unable to read %s: %s", localOutputDir, err)
	}
	for _, fileInfo := range fileInfos {
		if !fileInfo.IsDir() {
			continue
		}
		outputFile := filepath.Join(localOutputDir, fileInfo.Name(), "results.csv")
		newFile := filepath.Join(localOutputDir, fmt.Sprintf("%s.csv", fileInfo.Name()))
		if err := os.Rename(outputFile, newFile); err != nil {
			sklog.Errorf("Could not rename %s to %s: %s", outputFile, newFile, err)
			continue
		}

		if addRanks || len(pageRankToAdditionalFields) != 0 {
			headers, values, err := GetRowsFromCSV(newFile)
			if err != nil {
				sklog.Errorf("Could not read %s: %s", newFile, err)
				continue
			}
			// Add the rank of the page to the CSV file.
			pageRank := fileInfo.Name()
			pageNameWithRank := ""
			for i := range headers {
				for j := range values {
					if headers[i] == "stories" && addRanks {
						pageNameWithRank = fmt.Sprintf("%s (#%s)", values[j][i], pageRank)
						values[j][i] = pageNameWithRank
					}
				}
			}
			// Add additionalFields (if any) to the output CSV.
			if additionalFields, ok := pageRankToAdditionalFields[fileInfo.Name()]; ok {
				for h, v := range additionalFields {
					valueLine := make([]string, len(headers))
					for i := range headers {
						if headers[i] == "name" {
							valueLine[i] = h
						} else if headers[i] == valueColumnName {
							valueLine[i] = v
						} else if headers[i] == "stories" && addRanks {
							valueLine[i] = pageNameWithRank
						} else {
							valueLine[i] = ""
						}
					}
					values = append(values, valueLine)
				}
			}
			if err := writeRowsToCSV(newFile, headers, values); err != nil {
				sklog.Errorf("Could not write to %s: %s", newFile, err)
				continue
			}
		}
	}
	// Call csv_pivot_table_merger.py to merge all results into a single results CSV.
	pathToCsvMerger := filepath.Join(pathToPyFiles, "csv_pivot_table_merger.py")
	outputFileName := runID + ".output"
	args := []string{
		pathToCsvMerger,
		"--csv_dir=" + localOutputDir,
		"--output_csv_name=" + filepath.Join(localOutputDir, outputFileName),
		"--value_column_name=" + valueColumnName,
	}
	if handleStrings {
		args = append(args, "--handle_strings")
	}
	err = ExecuteCmd(ctx, BINARY_PYTHON, args, []string{fmt.Sprintf("VPYTHON_VIRTUALENV_ROOT=%s", os.TempDir())}, CSV_PIVOT_TABLE_MERGER_TIMEOUT, nil, nil)
	if err != nil {
		return fmt.Errorf("Error running csv_pivot_table_merger.py: %s", err)
	}
	// Check to see if the output CSV has more than just the header line.
	// TODO(rmistry): Inefficient to count all the lines when we really only want to know if
	// it's > or <= 1 line.
	lines, err := fileutil.CountLines(filepath.Join(localOutputDir, outputFileName))
	if err != nil {
		return fmt.Errorf("Could not count lines from %s: %s", filepath.Join(localOutputDir, outputFileName), err)
	}
	if lines <= 1 {
		return fmt.Errorf("%s has %d lines. More than 1 line is expected.", filepath.Join(localOutputDir, outputFileName), lines)
	}

	// Copy the output file to Google Storage.
	remoteOutputDir := path.Join(remoteDir, strconv.Itoa(startRange), "outputs")
	if err := gs.UploadFile(outputFileName, localOutputDir, remoteOutputDir); err != nil {
		return fmt.Errorf("Unable to upload %s to %s: %s", outputFileName, remoteOutputDir, err)
	}
	return nil
}

// GetRowsFromCSV reads the provided CSV and returns it's headers (first row)
// and values (all other rows).
func GetRowsFromCSV(csvPath string) ([]string, [][]string, error) {
	csvFile, err := os.Open(csvPath)
	defer util.Close(csvFile)
	if err != nil {
		return nil, nil, fmt.Errorf("Could not open %s: %s", csvPath, err)
	}
	reader := csv.NewReader(csvFile)
	reader.FieldsPerRecord = -1
	rawCSVdata, err := reader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("Could not read %s: %s", csvPath, err)
	}
	if len(rawCSVdata) < 2 {
		return nil, nil, fmt.Errorf("No data in %s", csvPath)
	}
	return rawCSVdata[0], rawCSVdata[1:], nil
}

func writeRowsToCSV(csvPath string, headers []string, values [][]string) error {
	csvFile, err := os.OpenFile(csvPath, os.O_WRONLY, 666)
	defer util.Close(csvFile)
	if err != nil {
		return fmt.Errorf("Could not open %s: %s", csvPath, err)
	}
	writer := csv.NewWriter(csvFile)
	defer writer.Flush()
	// Write the headers.
	if err := writer.Write(headers); err != nil {
		return fmt.Errorf("Could not write to %s: %s", csvPath, err)
	}
	// Write all values.
	for _, row := range values {
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("Could not write to %s: %s", csvPath, err)
		}
	}
	return nil
}

// pollSwarmingTaskToCompletion polls the specified swarming task till it completes. It returns the
// resulting CAS digest (if it exists) and the state of the swarming task if there is no error.
// TODO(rmistry): Use pubsub instead.
func pollSwarmingTaskToCompletion(ctx context.Context, taskId string, swarmingClient swarming.ApiClient) (string, string, error) {
	for range time.Tick(2 * time.Minute) {
		swarmingTask, err := swarmingClient.GetTask(ctx, taskId, false)
		if err != nil {
			return "", "", fmt.Errorf("Could not get task %s: %s", taskId, err)
		}
		switch swarmingTask.State {
		case swarming.TASK_STATE_BOT_DIED, swarming.TASK_STATE_CANCELED, swarming.TASK_STATE_EXPIRED, swarming.TASK_STATE_NO_RESOURCE, swarming.TASK_STATE_TIMED_OUT, swarming.TASK_STATE_KILLED:
			return "", swarmingTask.State, fmt.Errorf("The task %s exited early with state %v", taskId, swarmingTask.State)
		case swarming.TASK_STATE_PENDING:
			// The task is in pending state.
		case swarming.TASK_STATE_RUNNING:
			// The task is in running state.
		case swarming.TASK_STATE_COMPLETED:
			if swarmingTask.Failure {
				return "", swarmingTask.State, fmt.Errorf("The task %s failed", taskId)
			}
			sklog.Infof("The task %s successfully completed", taskId)
			if swarmingTask.CasOutputRoot == nil {
				return "", swarmingTask.State, nil
			}
			digest := rbe.DigestToString(swarmingTask.CasOutputRoot.Digest.Hash, swarmingTask.CasOutputRoot.Digest.SizeBytes)
			return digest, swarmingTask.State, nil
		default:
			sklog.Errorf("Unknown swarming state %v in %v", swarmingTask.State, swarmingTask)
		}
	}
	return "", "", nil
}

// TriggerIsolateTelemetrySwarmingTask triggers a swarming task which runs the
// isolate_telemetry worker script to upload telemetry to CAS and returns the
// resulting digest.
func TriggerIsolateTelemetrySwarmingTask(ctx context.Context, taskName, runID, chromiumHash, serviceAccountJSON, targetPlatform string, patches []string, hardTimeout, ioTimeout time.Duration, local bool, swarmingClient swarming.ApiClient, casClient cas.CAS) (string, error) {
	// Find which dimensions, os and CIPD pkgs to use.
	dimensions := GCE_LINUX_BUILDER_DIMENSIONS
	var casSpec *CasSpec
	cipdPkgs := []string{}
	cipdPkgs = append(cipdPkgs, cipd.GetStrCIPDPkgs(specs.CIPD_PKGS_ISOLATE)...)
	if targetPlatform == PLATFORM_WINDOWS {
		dimensions = GCE_WINDOWS_BUILDER_DIMENSIONS
		cipdPkgs = append(cipdPkgs, cipd.GetStrCIPDPkgs(cipd.PkgsGit[cipd.PlatformWindowsAmd64])...)
		cipdPkgs = append(cipdPkgs, LUCI_AUTH_CIPD_PACKAGE_WIN)
	} else {
		casSpec = CasIsolateTelemetryLinux()
		cipdPkgs = append(cipdPkgs, cipd.GetStrCIPDPkgs(cipd.PkgsGit[cipd.PlatformLinuxAmd64])...)
		cipdPkgs = append(cipdPkgs, LUCI_AUTH_CIPD_PACKAGE_LINUX)
	}

	// Upload task inputs to CAS.
	casDigest, err := UploadToCAS(ctx, casClient, casSpec, local, false)
	if err != nil {
		return "", skerr.Wrapf(err, "failed to upload task inputs to CAS")
	}

	// Trigger swarming task and wait for it to complete.
	cmd := []string{
		"luci-auth",
		"context",
		"--",
		"bin/isolate_telemetry",
		"-logtostderr",
		"--run_id=" + runID,
		"--chromium_hash=" + chromiumHash,
		"--patches=" + strings.Join(patches, ","),
		"--target_platform=" + targetPlatform,
		"--out=${ISOLATED_OUTDIR}",
	}
	req, err := MakeSwarmingTaskRequest(ctx, taskName, casDigest, cipdPkgs, cmd, []string{"name:" + taskName, "runid:" + runID}, dimensions, map[string][]string{"PATH": CIPD_PATHS}, swarming.RECOMMENDED_PRIORITY, ioTimeout, casClient)
	if err != nil {
		return "", skerr.Wrapf(err, "failed to create Swarming task request")
	}
	resp, err := swarmingClient.TriggerTask(ctx, req)
	if err != nil {
		return "", fmt.Errorf("Could not trigger swarming task %s: %s", taskName, err)
	}
	outputDigest, _, err := pollSwarmingTaskToCompletion(ctx, resp.TaskId, swarmingClient)
	if err != nil {
		return "", fmt.Errorf("Could not collect task ID %s: %s", resp.TaskId, err)
	}

	// Download CAS output of the task.
	outputDir, err := ioutil.TempDir("", fmt.Sprintf("download_%s", resp.TaskId))
	if err != nil {
		return "", fmt.Errorf("Failed to create temporary dir: %s", err)
	}
	defer util.RemoveAll(outputDir)
	if err := casClient.Download(ctx, outputDir, outputDigest); err != nil {
		return "", fmt.Errorf("Could not download %s: %s", outputDigest, err)
	}
	outputFile := filepath.Join(outputDir, ISOLATE_TELEMETRY_FILENAME)
	contents, err := ioutil.ReadFile(outputFile)
	if err != nil {
		return "", fmt.Errorf("Could not read outputfile %s: %s", outputFile, err)
	}
	return strings.Trim(string(contents), "\n"), nil
}

func MakeSwarmingTaskRequest(ctx context.Context, taskName, casDigest string, cipdPkgs, cmd, tags []string, dims map[string]string, envPrefixes map[string][]string, priority int64, ioTimeoutSecs time.Duration, casClient cas.CAS) (*swarming_api.SwarmingRpcsNewTaskRequest, error) {
	var cipdInput *swarming_api.SwarmingRpcsCipdInput
	if len(cipdPkgs) > 0 {
		cipdInput = &swarming_api.SwarmingRpcsCipdInput{
			Packages: make([]*swarming_api.SwarmingRpcsCipdPackage, 0, len(cipdPkgs)),
		}
		for _, p := range cipdPkgs {
			tokens := strings.SplitN(p, ":", 3)
			cipdInput.Packages = append(cipdInput.Packages, &swarming_api.SwarmingRpcsCipdPackage{
				Path:        tokens[0],
				PackageName: tokens[1],
				Version:     tokens[2],
			})
		}
	}

	swarmingDims := make([]*swarming_api.SwarmingRpcsStringPair, 0, len(dims))
	for k, v := range dims {
		swarmingDims = append(swarmingDims, &swarming_api.SwarmingRpcsStringPair{
			Key:   k,
			Value: v,
		})
	}

	var swarmingEnvPrefixes []*swarming_api.SwarmingRpcsStringListPair
	if len(envPrefixes) > 0 {
		swarmingEnvPrefixes = make([]*swarming_api.SwarmingRpcsStringListPair, 0, len(envPrefixes))
		for k, v := range envPrefixes {
			swarmingEnvPrefixes = append(swarmingEnvPrefixes, &swarming_api.SwarmingRpcsStringListPair{
				Key:   k,
				Value: v,
			})
		}
	}

	casInstance, err := rbe.GetCASInstance(casClient)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	casInput, err := swarming.MakeCASReference(casDigest, casInstance)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// CT runs use task authentication in swarming (see https://chrome-internal-review.googlesource.com/c/infradata/config/+/2878799/2#message-e3328dd455c1110cd2286a0c343b932594296ea3).
	// This does not allow more than 48hours validity duration (expiration time + hard timeout).
	executionTimeoutSecs := 36 * time.Hour
	expirationTimeoutSecs := 2*24*time.Hour - executionTimeoutSecs - time.Hour // Remove one hour to be safe.

	return &swarming_api.SwarmingRpcsNewTaskRequest{
		Name:           taskName,
		Priority:       priority,
		ServiceAccount: CT_SERVICE_ACCOUNT,
		Tags:           tags,
		TaskSlices: []*swarming_api.SwarmingRpcsTaskSlice{
			{
				ExpirationSecs: int64(expirationTimeoutSecs.Seconds()),
				Properties: &swarming_api.SwarmingRpcsTaskProperties{
					CasInputRoot:         casInput,
					CipdInput:            cipdInput,
					Command:              cmd,
					Dimensions:           swarmingDims,
					EnvPrefixes:          swarmingEnvPrefixes,
					ExecutionTimeoutSecs: int64(executionTimeoutSecs.Seconds()),
					IoTimeoutSecs:        int64(ioTimeoutSecs.Seconds()),
				},
				WaitForCapacity: false,
			},
		},
		User: CT_SERVICE_ACCOUNT,
	}, nil
}

func TriggerMasterScriptSwarmingTask(ctx context.Context, runID, taskName string, local bool, cmd []string, casSpec *CasSpec, swarmingClient swarming.ApiClient, casClient cas.CAS) (string, error) {
	// Master scripts only need linux versions of their cipd packages. But still need to specify
	// osType correctly so that exe binaries can be packaged for windows.
	cipdPkgs := []string{}
	cipdPkgs = append(cipdPkgs, cipd.GetStrCIPDPkgs(cipd.PkgsGit[cipd.PlatformLinuxAmd64])...)
	cipdPkgs = append(cipdPkgs, LUCI_AUTH_CIPD_PACKAGE_LINUX)

	// Upload the task inputs to CAS.
	casDigest, err := UploadToCAS(ctx, casClient, casSpec, local, true)
	if err != nil {
		return "", skerr.Wrapf(err, "failed to upload task inputs to CAS")
	}

	// Trigger swarming task.
	req, err := MakeSwarmingTaskRequest(ctx, taskName, casDigest, cipdPkgs, cmd, []string{"name:" + taskName, "runid:" + runID}, GCE_LINUX_MASTER_DIMENSIONS, map[string][]string{"PATH": CIPD_PATHS}, swarming.RECOMMENDED_PRIORITY, 3*24*time.Hour, casClient)
	if err != nil {
		return "", skerr.Wrapf(err, "failed to create Swarming task request")
	}
	resp, err := swarmingClient.TriggerTask(ctx, req)
	if err != nil {
		return "", fmt.Errorf("Could not trigger swarming task %s: %s", taskName, err)
	}
	return resp.TaskId, nil
}

// TriggerBuildRepoSwarmingTask triggers a swarming task which runs the
// build_repo worker script which will return a list of remote build
// directories.
func TriggerBuildRepoSwarmingTask(ctx context.Context, taskName, runID, repoAndTarget, targetPlatform, serviceAccountJSON, gnArgs string, hashes, patches, cipdPkgs []string, singleBuild, local bool, hardTimeout, ioTimeout time.Duration, swarmingClient swarming.ApiClient, casClient cas.CAS) ([]string, error) {
	// Find which os and CIPD pkgs to use.
	var casSpec *CasSpec
	if targetPlatform == PLATFORM_WINDOWS {
		cipdPkgs = append(cipdPkgs, cipd.GetStrCIPDPkgs(cipd.PkgsGit[cipd.PlatformWindowsAmd64])...)
		cipdPkgs = append(cipdPkgs, LUCI_AUTH_CIPD_PACKAGE_WIN)
	} else {
		casSpec = CasBuildRepoLinux()
		cipdPkgs = append(cipdPkgs, cipd.GetStrCIPDPkgs(cipd.PkgsGit[cipd.PlatformLinuxAmd64])...)
		cipdPkgs = append(cipdPkgs, LUCI_AUTH_CIPD_PACKAGE_LINUX)
	}

	// Upload task inputs to CAS.
	casDigest, err := UploadToCAS(ctx, casClient, casSpec, local, false)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to upload task inputs to CAS")
	}

	// Trigger swarming task and wait for it to complete.
	cmd := []string{
		"luci-auth",
		"context",
		"--",
		"bin/build_repo",
		"-logtostderr",
		"--run_id=" + runID,
		"--repo_and_target=" + repoAndTarget,
		"--gn_args=" + gnArgs,
		"--hashes=" + strings.Join(hashes, ","),
		"--patches=" + strings.Join(patches, ","),
		"--single_build=" + strconv.FormatBool(singleBuild),
		"--target_platform=" + targetPlatform,
		"--out=${ISOLATED_OUTDIR}",
	}
	var dimensions map[string]string
	if targetPlatform == PLATFORM_WINDOWS {
		dimensions = GCE_WINDOWS_BUILDER_DIMENSIONS
	} else if targetPlatform == PLATFORM_ANDROID {
		dimensions = GCE_ANDROID_BUILDER_DIMENSIONS
	} else {
		dimensions = GCE_LINUX_BUILDER_DIMENSIONS
	}

	req, err := MakeSwarmingTaskRequest(ctx, taskName, casDigest, cipdPkgs, cmd, []string{"name:" + taskName, "runid:" + runID}, dimensions, map[string][]string{"PATH": CIPD_PATHS}, swarming.RECOMMENDED_PRIORITY, ioTimeout, casClient)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create Swarming task request")
	}
	resp, err := swarmingClient.TriggerTask(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("Could not trigger swarming task %s: %s", taskName, err)
	}
	outputDigest, _, err := pollSwarmingTaskToCompletion(ctx, resp.TaskId, swarmingClient)
	if err != nil {
		return nil, fmt.Errorf("Could not collect task ID %s: %s", resp.TaskId, err)
	}

	// Download output of the task.
	outputDir, err := ioutil.TempDir("", fmt.Sprintf("download_%s", resp.TaskId))
	if err != nil {
		return nil, fmt.Errorf("Failed to create temporary dir: %s", err)
	}
	defer util.RemoveAll(outputDir)
	if err := casClient.Download(ctx, outputDir, outputDigest); err != nil {
		return nil, fmt.Errorf("Could not download %s: %s", outputDigest, err)
	}
	outputFile := filepath.Join(outputDir, BUILD_OUTPUT_FILENAME)
	contents, err := ioutil.ReadFile(outputFile)
	if err != nil {
		return nil, fmt.Errorf("Could not read outputfile %s: %s", outputFile, err)
	}
	return strings.Split(string(contents), ","), nil
}

func DownloadPatch(localPath, remotePath string, gs *GcsUtil) (int64, error) {
	respBody, err := gs.GetRemoteFileContents(remotePath)
	if err != nil {
		return -1, fmt.Errorf("Could not fetch %s: %s", remotePath, err)
	}
	defer util.Close(respBody)
	f, err := os.Create(localPath)
	if err != nil {
		return -1, fmt.Errorf("Could not create %s: %s", localPath, err)
	}
	defer util.Close(f)
	written, err := io.Copy(f, respBody)
	if err != nil {
		return -1, fmt.Errorf("Could not write to %s: %s", localPath, err)
	}
	return written, nil
}

func DownloadAndApplyPatch(ctx context.Context, patchName, localDir, remotePatchesDir, checkout, gitExec string, gs *GcsUtil) error {
	patchLocalPath := filepath.Join(localDir, patchName)
	patchRemotePath := filepath.Join(remotePatchesDir, patchName)
	written, err := DownloadPatch(patchLocalPath, patchRemotePath, gs)
	if err != nil {
		return fmt.Errorf("Could not download %s: %s", patchRemotePath, err)
	}
	// Apply patch to the local checkout.
	if written > 10 {
		if err := ApplyPatch(ctx, patchLocalPath, checkout, gitExec); err != nil {
			return fmt.Errorf("Could not apply patch in %s: %s", checkout, err)
		}
	}
	return nil
}

// GetArchivesNum returns the number of archives for the specified pagesetType.
// -1 is returned if USE_LIVE_SITES_FLAGS is specified or if there is an error.
func GetArchivesNum(gs *GcsUtil, benchmarkArgs, pagesetType string) (int, error) {
	if strings.Contains(benchmarkArgs, USE_LIVE_SITES_FLAGS) {
		return -1, nil
	}
	// Calculate the number of archives the workers worked with.
	archivesRemoteDir := filepath.Join(SWARMING_DIR_NAME, WEB_ARCHIVES_DIR_NAME, pagesetType)
	totalArchiveArtifacts, err := gs.GetRemoteDirCount(archivesRemoteDir)
	if err != nil {
		return -1, fmt.Errorf("Could not find archives in %s: %s", archivesRemoteDir, err)
	}
	// Each archive has a JSON file, a WPR file and a WPR.sha1 file.
	return totalArchiveArtifacts / 3, nil
}

// GetHashesFromBuild returns the Chromium and Skia hashes from a CT build string.
// Example build string: try-27af50f-d5dcd58-rmistry-20151026102511-nopatch.
func GetHashesFromBuild(chromiumBuild string) (string, string) {
	tokens := strings.Split(chromiumBuild, "-")
	return tokens[1], tokens[2]
}

// GetNumPages returns the number of specified custom webpages. If Custom
// webpages are not specified then the number of pages associated with the
// pageset type is returned.
func GetNumPages(pagesetType, customWebPagesFilePath string) (int, error) {
	customPages, err := GetCustomPages(customWebPagesFilePath)
	if err != nil {
		return PagesetTypeToInfo[pagesetType].NumPages, err
	}
	if len(customPages) == 0 {
		return PagesetTypeToInfo[pagesetType].NumPages, nil
	}
	return len(customPages), nil
}

// GetCustomPages returns the specified custom webpages. If Custom
// webpages are not specified then it returns an empty slice.
func GetCustomPages(customWebPagesFilePath string) ([]string, error) {
	csvFile, err := os.Open(customWebPagesFilePath)
	if err != nil {
		return nil, err
	}
	defer util.Close(csvFile)
	reader := csv.NewReader(csvFile)
	customPages := []string{}
	for {
		records, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, record := range records {
			if strings.TrimSpace(record) == "" {
				continue
			}
			customPages = append(customPages, record)
		}
	}
	return customPages, nil
}

func GetCustomPagesWithinRange(startRange, num int, customWebpages []string) []string {
	startIndex := startRange - 1
	endIndex := util.MinInt(startIndex+num, len(customWebpages))
	return customWebpages[startIndex:endIndex]
}

func CreateCustomPagesets(webpages []string, pagesetsDir, targetPlatform string, startRange int) error {
	// Empty the local dir.
	util.RemoveAll(pagesetsDir)
	// Create the local dir.
	MkdirAll(pagesetsDir, 0700)
	// Figure out which user agent to use.
	var userAgent string
	if targetPlatform == PLATFORM_ANDROID {
		userAgent = "mobile"
	} else {
		userAgent = "desktop"
	}
	for i, w := range webpages {
		pagesetPath := filepath.Join(pagesetsDir, fmt.Sprintf("%d.py", i+startRange))
		if err := WritePageset(pagesetPath, userAgent, DEFAULT_CUSTOM_PAGE_ARCHIVEPATH, w); err != nil {
			return err
		}
	}
	return nil
}

func GetAnalysisOutputLink(runID string) string {
	return GCS_HTTP_LINK + path.Join(GCSBucketName, BenchmarkRunsDir, runID, "consolidated_outputs", runID+".output")
}

func GetPerfRemoteDir(runID string) string {
	return path.Join(ChromiumPerfRunsStorageDir, runID)
}

func GetPerfRemoteHTMLDir(runID string) string {
	return path.Join(GetPerfRemoteDir(runID), "html")
}

func GetPerfOutputLinkBase(runID string) string {
	return GCS_HTTP_LINK + path.Join(GCSBucketName, GetPerfRemoteHTMLDir(runID)) + "/"
}

func GetPerfOutputLink(runID string) string {
	return GetPerfOutputLinkBase(runID) + "index.html"
}

func GetPerfNoPatchOutputLink(runID string) string {
	runIDNoPatch := fmt.Sprintf("%s-nopatch", runID)
	return GCS_HTTP_LINK + path.Join(GCSBucketName, BenchmarkRunsDir, runIDNoPatch, "consolidated_outputs", runIDNoPatch+".output")
}

func GetPerfWithPatchOutputLink(runID string) string {
	runIDWithPatch := fmt.Sprintf("%s-withpatch", runID)
	return GCS_HTTP_LINK + path.Join(GCSBucketName, BenchmarkRunsDir, runIDWithPatch, "consolidated_outputs", runIDWithPatch+".output")
}

func GetMetricsAnalysisOutputLink(runID string) string {
	return GCS_HTTP_LINK + path.Join(GCSBucketName, BenchmarkRunsDir, runID, "consolidated_outputs", runID+".output")
}

func SavePatchToStorage(patch string) (string, error) {

	if len(patch) > PATCH_LIMIT {
		return "", fmt.Errorf("Patch is too long with %d bytes; limit %d bytes", len(patch), PATCH_LIMIT)
	}

	// If sha1 below ever changes, then isEmptyPatch in ctfe.js will also need to
	// be updated.
	patchHash := sha1.Sum([]byte(patch))
	patchHashHex := hex.EncodeToString(patchHash[:])

	gs, err := NewGcsUtil(nil)
	if err != nil {
		return "", err
	}
	gsDir := "patches"
	patchFileName := fmt.Sprintf("%s.patch", patchHashHex)
	gsPath := path.Join(gsDir, patchFileName)

	res, err := gs.service.Objects.Get(GCSBucketName, gsPath).Do()
	if err != nil {
		sklog.Infof("This is expected for patches we have not seen before:\nCould not retrieve object metadata for %s: %s", gsPath, err)
	}
	if res == nil || res.Size != uint64(len(patch)) {
		// Patch does not exist in Google Storage yet so upload it.
		patchPath := filepath.Join(os.TempDir(), patchFileName)
		if err := ioutil.WriteFile(patchPath, []byte(patch), 0666); err != nil {
			return "", err
		}
		defer util.Remove(patchPath)
		if err := gs.UploadFile(patchFileName, os.TempDir(), gsDir); err != nil {
			return "", err
		}
	}

	return gsPath, nil
}

func GetPatchFromStorage(patchId string) (string, error) {
	gs, err := NewGcsUtil(nil)
	respBody, err := gs.GetRemoteFileContents(patchId)
	if err != nil {
		return "", fmt.Errorf("Could not fetch %s: %s", patchId, err)
	}
	defer util.Close(respBody)
	patch, err := ioutil.ReadAll(respBody)
	if err != nil {
		return "", fmt.Errorf("Could not read from %s: %s", patchId, err)
	}
	return string(patch), nil
}

func GetRankFromPageset(pagesetFileName string) (int, error) {
	// All CT pagesets are of the form [rank].py so just stripping out the
	// extension should give us the rank of the pageset.
	var extension = filepath.Ext(pagesetFileName)
	rank := pagesetFileName[0 : len(pagesetFileName)-len(extension)]
	return strconv.Atoi(rank)
}

type Pageset struct {
	UserAgent       string `json:"user_agent"`
	ArchiveDataFile string `json:"archive_data_file"`
	UrlsList        string `json:"urls_list"`
}

func WritePageset(filePath, userAgent, archiveFilePath, url string) error {
	pageSet := Pageset{
		UserAgent:       userAgent,
		ArchiveDataFile: archiveFilePath,
		UrlsList:        url,
	}
	b, err := json.Marshal(pageSet)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(filePath, b, 0644); err != nil {
		return err
	}
	return nil
}

type TimeoutTracker struct {
	timeoutCounter      int
	timeoutCounterMutex sync.Mutex
}

func (t *TimeoutTracker) Increment() {
	t.timeoutCounterMutex.Lock()
	defer t.timeoutCounterMutex.Unlock()
	t.timeoutCounter++
}

func (t *TimeoutTracker) Reset() {
	t.timeoutCounterMutex.Lock()
	defer t.timeoutCounterMutex.Unlock()
	t.timeoutCounter = 0
}

func (t *TimeoutTracker) Read() int {
	t.timeoutCounterMutex.Lock()
	defer t.timeoutCounterMutex.Unlock()
	return t.timeoutCounter
}

// MkdirAll creates the specified path and logs an error if one is returned.
func MkdirAll(name string, perm os.FileMode) {
	if err := os.MkdirAll(name, perm); err != nil {
		sklog.ErrorfWithDepth(1, "Failed to MkdirAll(%s, %v): %v", name, perm, err)
	}
}

// Rename renames the specified file and logs an error if one is returned.
func Rename(oldpath, newpath string) {
	if err := os.Rename(oldpath, newpath); err != nil {
		sklog.ErrorfWithDepth(1, "Failed to Rename(%s, %s): %v", oldpath, newpath, err)
	}
}
