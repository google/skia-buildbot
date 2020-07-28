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

	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/specs"
)

const (
	MAX_SYNC_TRIES = 3

	TS_FORMAT = "20060102150405"

	REMOVE_INVALID_SKPS_WORKER_POOL = 20

	MAX_SIMULTANEOUS_SWARMING_TASKS_PER_RUN = 10000

	PATCH_LIMIT = 1 << 26
)

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

// BuildSkiaSKPInfo builds "skpinfo" in the Skia trunk directory.
// Returns the directory that contains the binary.
func BuildSkiaSKPInfo(ctx context.Context, runningOnSwarming bool) (string, error) {
	if err := os.Chdir(SkiaTreeDir); err != nil {
		return "", fmt.Errorf("Could not chdir to %s: %s", SkiaTreeDir, err)
	}
	// Run gn gen
	clangDir := filepath.Join(filepath.Dir(filepath.Dir(os.Args[0])), "clang_linux")
	if err := runSkiaGnGen(ctx, clangDir, "" /* gnExtraArgs */); err != nil {
		return "", fmt.Errorf("Error while running gn: %s", err)
	}
	// Run "ninja -C out/Release -j100 skpinfo".
	// Use the full system env when building.
	outPath := filepath.Join(SkiaTreeDir, "out", "Release")
	args := []string{"-C", outPath, "-j100", BINARY_SKPINFO}
	return outPath, ExecuteCmd(ctx, filepath.Join(DepotToolsDir, "ninja"), args, os.Environ(), NINJA_TIMEOUT, nil, nil)
}

// GetCipdPackageFromAsset returns a string of the format "path:package_name:version".
// It returns the latest version of the asset via gitiles.
func GetCipdPackageFromAsset(assetName string) (string, error) {
	// Find the latest version of the asset from gitiles.
	assetVersionFilePath := path.Join("infra", "bots", "assets", assetName, "VERSION")
	var buf bytes.Buffer
	if err := gitiles.NewRepo(common.REPO_SKIA, nil).ReadFile(context.Background(), assetVersionFilePath, &buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:skia/bots/%s:version:%s", assetName, assetName, strings.TrimSpace(buf.String())), nil
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

// ValidateSKPs moves all root_dir/index/dir_name/*.skp into the root_dir/index
// and validates them. SKPs that fail validation are logged and deleted.
func ValidateSKPs(ctx context.Context, pathToSkps, pathToPyFiles, pathToSkpinfo string) error {
	// This slice will be used to run remove_invalid_skp.py.
	skps := []string{}
	// List all directories in pathToSkps and copy out the skps.
	indexDirs, err := filepath.Glob(path.Join(pathToSkps, "*"))
	if err != nil {
		return fmt.Errorf("Unable to read %s: %s", pathToSkps, err)
	}
	for _, indexDir := range indexDirs {
		index := path.Base(indexDir)
		skpFileInfos, err := ioutil.ReadDir(indexDir)
		if err != nil {
			return fmt.Errorf("Unable to read %s: %s", indexDir, err)
		}
		for _, fileInfo := range skpFileInfos {
			if !fileInfo.IsDir() {
				// We are only interested in directories.
				continue
			}
			skpName := fileInfo.Name()
			// Find the largest layer in this directory.
			layerInfos, err := ioutil.ReadDir(filepath.Join(pathToSkps, index, skpName))
			if err != nil {
				sklog.Errorf("Unable to read %s: %s", filepath.Join(pathToSkps, index, skpName), err)
			}
			if len(layerInfos) > 0 {
				largestLayerInfo := layerInfos[0]
				for _, layerInfo := range layerInfos {
					if layerInfo.Size() > largestLayerInfo.Size() {
						largestLayerInfo = layerInfo
					}
				}
				// Only save SKPs greater than 6000 bytes. Less than that are probably
				// malformed.
				if largestLayerInfo.Size() > 6000 {
					layerPath := filepath.Join(pathToSkps, index, skpName, largestLayerInfo.Name())
					destSKP := filepath.Join(pathToSkps, index, skpName+".skp")
					Rename(layerPath, destSKP)
					skps = append(skps, destSKP)
				} else {
					sklog.Warningf("Skipping %s because size was less than 6000 bytes", skpName)
				}
			}
			// We extracted what we needed from the directory, now delete it.
			util.RemoveAll(filepath.Join(pathToSkps, index, skpName))
		}
	}

	// Create channel that contains all SKP file paths. This channel will
	// be consumed by the worker pool below to run remove_invalid_skp.py in
	// parallel.
	skpsChannel := make(chan string, len(skps))
	for _, skp := range skps {
		skpsChannel <- skp
	}
	close(skpsChannel)

	sklog.Info("Calling remove_invalid_skp.py")

	pathToRemoveSKPs := filepath.Join(pathToPyFiles, "remove_invalid_skp.py")

	var wg sync.WaitGroup

	// Loop through workers in the worker pool.
	for i := 0; i < REMOVE_INVALID_SKPS_WORKER_POOL; i++ {
		// Increment the WaitGroup counter.
		wg.Add(1)

		// Create and run a goroutine closure that captures SKPs.
		go func(i int) {
			// Decrement the WaitGroup counter when the goroutine completes.
			defer wg.Done()

			for skpPath := range skpsChannel {
				args := []string{
					pathToRemoveSKPs,
					"--path_to_skp=" + skpPath,
					"--path_to_skpinfo=" + pathToSkpinfo,
				}
				sklog.Infof("Executing remove_invalid_skp.py with goroutine#%d", i+1)
				// Execute the command with stdout not logged. It otherwise outputs
				// tons of log msgs.
				util.LogErr(exec.Run(ctx, &exec.Command{
					Name:        "python",
					Args:        args,
					Env:         []string{},
					InheritPath: true,
					Timeout:     REMOVE_INVALID_SKPS_TIMEOUT,
					LogStdout:   false,
					Stdout:      nil,
					LogStderr:   true,
					Stderr:      nil,
				}))
			}
		}(i)
	}

	// Wait for all spawned goroutines to complete.
	wg.Wait()

	return nil
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

// GetIsolateClient creates an isolate client using the specified service account.
func GetIsolateClient(serviceAccountJSON string) (*isolate.Client, error) {
	workDir, err := ioutil.TempDir(StorageDir, "swarming_work_")
	if err != nil {
		return nil, fmt.Errorf("Could not get temp dir: %s", err)
	}
	return isolate.NewClientWithServiceAccount(workDir, isolate.ISOLATE_SERVER_URL_PRIVATE, serviceAccountJSON)
}

// TriggerSwarmingTask returns the number of triggered tasks and an error (if any).
func TriggerSwarmingTask(ctx context.Context, pagesetType, taskPrefix, isolateName, runID, serviceAccountJSON, targetPlatform string, hardTimeout, ioTimeout time.Duration, priority, maxPagesPerBot, numPages int, runOnGCE, local bool, repeatValue int, baseCmd, isolateDeps []string, swarmingClient swarming.ApiClient) (int, error) {
	// Get path to isolate files.
	pathToIsolates, err := GetPathToIsolates(local, false)
	if err != nil {
		return 0, fmt.Errorf("Could not get path to isolates: %s", err)
	}
	// Find which os to use.
	osType := "linux"
	if targetPlatform == PLATFORM_WINDOWS {
		osType = "win"
	}

	// Isolate and use the isolate hash for all triggered tasks.
	isolateClient, err := GetIsolateClient(serviceAccountJSON)
	if err != nil {
		return 0, fmt.Errorf("Could not instantiate isolate client: %s", err)
	}
	isolateTask := &isolate.Task{
		BaseDir:     pathToIsolates,
		IsolateFile: path.Join(pathToIsolates, isolateName),
		Deps:        isolateDeps,
		OsType:      osType,
	}
	isolateHashes, _, err := isolateClient.IsolateTasks(ctx, []*isolate.Task{isolateTask})
	if err != nil {
		return 0, fmt.Errorf("Could not isolate the telemetry isolate task: %s", err)
	}
	if len(isolateHashes) != 1 {
		return 0, fmt.Errorf("Expected one hash when isolating telemetry isolate task. Not: %+v", isolateHashes)
	}
	isolateHash := isolateHashes[0]

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
				req := MakeSwarmingTaskRequest(ctx, taskName, isolateHash, cipdPkgs, cmd, []string{"name:" + taskName, "runid:" + runID}, dimensions, map[string]string{"PATH": "cipd_bin_packages"}, int64(priority), ioTimeout)
				resp, err := swarmingClient.TriggerTask(req)
				if err != nil {
					sklog.Errorf("Could not trigger swarming task %s: %s", taskName, err)
					return
				}

				_, state, err := pollSwarmingTaskToCompletion(ctx, resp.TaskId, swarmingClient, isolateClient)
				if err != nil {
					sklog.Errorf("task %s failed: %s", taskName, err)
					if state == swarming.TASK_STATE_KILLED {
						sklog.Infof("task %s was killed (either manually or via CT's delete button). Not going to retry it.", taskName)
						return
					}
					sklog.Infof("Retrying task %s with high priority %d", taskName, TASKS_PRIORITY_HIGH)
					req.Priority = TASKS_PRIORITY_HIGH
					retryResp, err := swarmingClient.TriggerTask(req)
					if err != nil {
						sklog.Errorf("Could not trigger swarming retry task %s: %s", taskName, err)
						return
					}
					if _, _, err := pollSwarmingTaskToCompletion(ctx, retryResp.TaskId, swarmingClient, isolateClient); err != nil {
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

// GetPathToIsolates returns the location of CT's isolates.
// local should be set to true if we need the location of isolates when debugging locally.
// runOnMaster should be set if we need the location of isolates on ctfe.
// If both are false then it is assumed that we are running on a swarming bot.
func GetPathToIsolates(local, runOnMaster bool) (string, error) {
	if local {
		_, currentFile, _, _ := runtime.Caller(0)
		return filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(currentFile))), "isolates"), nil
	} else if runOnMaster {
		return filepath.Join("/", "usr", "local", "share", "ctfe", "isolates"), nil
	} else {
		return filepath.Abs(filepath.Join(filepath.Dir(filepath.Dir(os.Args[0])), "share", "ctfe", "isolates"))
	}
}

// GetPathToPyFiles returns the location of CT's python scripts.
// local should be set to true if we need the location of py files when debugging locally.
func GetPathToPyFiles(local bool) (string, error) {
	if local {
		_, currentFile, _, _ := runtime.Caller(0)
		return filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(currentFile))), "py"), nil
	} else {
		return filepath.Abs(filepath.Join(filepath.Dir(filepath.Dir(os.Args[0])), "share", "ctfe", "py"))
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
	err := ExecuteCmd(ctx, "python", args, []string{}, CSV_MERGER_TIMEOUT, nil, nil)
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
		"--user-agent=" + decodedPageset.UserAgent,
		"--urls-list=" + decodedPageset.UrlsList,
		"--archive-data-file=" + decodedPageset.ArchiveDataFile,
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
	// Append the original environment as well.
	for _, e := range os.Environ() {
		env = append(env, e)
	}

	if targetPlatform == PLATFORM_ANDROID {
		// Reset android logcat prior to the run so that we can examine the logs later.
		util.LogErr(ExecuteCmd(ctx, BINARY_ADB, []string{"logcat", "-c"}, []string{}, ADB_ROOT_TIMEOUT, nil, nil))
	}

	// Create buffer for capturing the stdout and stderr of the benchmark run.
	var b bytes.Buffer
	if _, err := b.WriteString(fmt.Sprintf("========== Stdout and stderr for %s ==========\n", pagesetPath)); err != nil {
		return "", fmt.Errorf("Error writing to output buffer: %s", err)
	}
	if err := ExecuteCmdWithConfigurableLogging(ctx, "python", args, env, time.Duration(timeoutSecs)*time.Second, &b, &b, false, false); err != nil {
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
	err = ExecuteCmd(ctx, "python", args, []string{}, CSV_PIVOT_TABLE_MERGER_TIMEOUT, nil, nil)
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
// isolated output hash (if it exists) and the state of the swarming task if there is no error.
// TODO(rmistry): Use pubsub instead.
func pollSwarmingTaskToCompletion(ctx context.Context, taskId string, swarmingClient swarming.ApiClient, isolateClient *isolate.Client) (string, string, error) {
	for range time.Tick(2 * time.Minute) {
		swarmingTask, err := swarmingClient.GetTask(taskId, false)
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
			} else {
				sklog.Infof("The task %s successfully completed", taskId)
				if swarmingTask.OutputsRef == nil {
					return "", swarmingTask.State, nil
				} else {
					return swarmingTask.OutputsRef.Isolated, swarmingTask.State, nil
				}
			}
		default:
			sklog.Errorf("Unknown swarming state %v in %v", swarmingTask.State, swarmingTask)
		}
	}
	return "", "", nil
}

// TriggerIsolateTelemetrySwarmingTask creates a isolated.gen.json file using ISOLATE_TELEMETRY_ISOLATE,
// archives it, and triggers its swarming task. The swarming task will run the isolate_telemetry
// worker script which will return the isolate hash.
func TriggerIsolateTelemetrySwarmingTask(ctx context.Context, taskName, runID, chromiumHash, serviceAccountJSON, targetPlatform string, patches []string, hardTimeout, ioTimeout time.Duration, local bool, swarmingClient swarming.ApiClient) (string, error) {
	// Get path to isolate files.
	pathToIsolates, err := GetPathToIsolates(local, false)
	if err != nil {
		return "", fmt.Errorf("Could not get path to isolates: %s", err)
	}
	// Find which dimensions, os and CIPD pkgs to use.
	dimensions := GCE_LINUX_BUILDER_DIMENSIONS
	osType := "linux"
	cipdPkgs := []string{}
	cipdPkgs = append(cipdPkgs, cipd.GetStrCIPDPkgs(specs.CIPD_PKGS_ISOLATE)...)
	if targetPlatform == PLATFORM_WINDOWS {
		dimensions = GCE_WINDOWS_BUILDER_DIMENSIONS
		osType = "win"
		cipdPkgs = append(cipdPkgs, cipd.GetStrCIPDPkgs(cipd.PkgsGit[cipd.PlatformWindowsAmd64])...)
		cipdPkgs = append(cipdPkgs, LUCI_AUTH_CIPD_PACKAGE_WIN)
	} else {
		cipdPkgs = append(cipdPkgs, cipd.GetStrCIPDPkgs(cipd.PkgsGit[cipd.PlatformLinuxAmd64])...)
		cipdPkgs = append(cipdPkgs, LUCI_AUTH_CIPD_PACKAGE_LINUX)
	}

	// Isolate the task.
	isolateClient, err := GetIsolateClient(serviceAccountJSON)
	if err != nil {
		return "", fmt.Errorf("Could not instantiate isolate client: %s", err)
	}
	isolateTask := &isolate.Task{
		BaseDir:     pathToIsolates,
		IsolateFile: path.Join(pathToIsolates, ISOLATE_TELEMETRY_ISOLATE),
		Deps:        []string{},
		OsType:      osType,
	}
	isolateHashes, _, err := isolateClient.IsolateTasks(ctx, []*isolate.Task{isolateTask})
	if err != nil {
		return "", fmt.Errorf("Could not isolate the telemetry isolate task: %s", err)
	}
	if len(isolateHashes) != 1 {
		return "", fmt.Errorf("Expected one hash when isolating telemetry isolate task. Not: %+v", isolateHashes)
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
		"--out=${ISOLATED_OUTDIR}",
	}
	req := MakeSwarmingTaskRequest(ctx, taskName, isolateHashes[0], cipdPkgs, cmd, []string{"name:" + taskName, "runid:" + runID}, dimensions, map[string]string{"PATH": "cipd_bin_packages"}, swarming.RECOMMENDED_PRIORITY, ioTimeout)
	resp, err := swarmingClient.TriggerTask(req)
	if err != nil {
		return "", fmt.Errorf("Could not trigger swarming task %s: %s", taskName, err)
	}
	isolateHash, _, err := pollSwarmingTaskToCompletion(ctx, resp.TaskId, swarmingClient, isolateClient)
	if err != nil {
		return "", fmt.Errorf("Could not collect task ID %s: %s", resp.TaskId, err)
	}

	// Download isolate output of the task.
	outputDir, err := ioutil.TempDir("", fmt.Sprintf("download_%s", resp.TaskId))
	if err != nil {
		return "", fmt.Errorf("Failed to create temporary dir: %s", err)
	}
	defer util.RemoveAll(outputDir)
	if err := isolateClient.DownloadIsolateHash(ctx, isolateHash, outputDir, "files-list.txt"); err != nil {
		return "", fmt.Errorf("Could not download %s: %s", isolateHash, err)
	}
	outputFile := filepath.Join(outputDir, ISOLATE_TELEMETRY_FILENAME)
	contents, err := ioutil.ReadFile(outputFile)
	if err != nil {
		return "", fmt.Errorf("Could not read outputfile %s: %s", outputFile, err)
	}
	return strings.Trim(string(contents), "\n"), nil
}

func MakeSwarmingTaskRequest(ctx context.Context, taskName, isolatedHash string, cipdPkgs, cmd, tags []string, dims, envPrefixes map[string]string, priority int64, ioTimeoutSecs time.Duration) *swarming_api.SwarmingRpcsNewTaskRequest {
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
				Value: []string{v},
			})
		}
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
					CipdInput:            cipdInput,
					Command:              cmd,
					Dimensions:           swarmingDims,
					EnvPrefixes:          swarmingEnvPrefixes,
					ExecutionTimeoutSecs: int64(executionTimeoutSecs.Seconds()),
					InputsRef: &swarming_api.SwarmingRpcsFilesRef{
						Isolated:       isolatedHash,
						Isolatedserver: isolate.ISOLATE_SERVER_URL_PRIVATE,
						Namespace:      isolate.DEFAULT_NAMESPACE,
					},
					IoTimeoutSecs: int64(ioTimeoutSecs.Seconds()),
				},
				WaitForCapacity: false,
			},
		},
		User: CT_SERVICE_ACCOUNT,
	}
}

func TriggerMasterScriptSwarmingTask(ctx context.Context, runID, taskName, isolateFileName, serviceAccountJSON, targetPlatform string, local bool, cmd []string, swarmingClient swarming.ApiClient) (string, error) {
	// Master scripts only need linux versions of their cipd packages. But still need to specify
	// osType correctly so that exe binaries can be packaged for windows.
	osType := "linux"
	cipdPkgs := []string{}
	cipdPkgs = append(cipdPkgs, cipd.GetStrCIPDPkgs(cipd.PkgsGit[cipd.PlatformLinuxAmd64])...)
	cipdPkgs = append(cipdPkgs, LUCI_AUTH_CIPD_PACKAGE_LINUX)
	cipdPkgs = append(cipdPkgs, cipd.GetStrCIPDPkgs(specs.CIPD_PKGS_ISOLATE)...)
	if targetPlatform == PLATFORM_WINDOWS {
		osType = "win"
	}

	// Get path to isolate files.
	pathToIsolates, err := GetPathToIsolates(local, true)
	if err != nil {
		return "", fmt.Errorf("Could not get path to isolates: %s", err)
	}

	// Isolate the task.
	isolateClient, err := GetIsolateClient(serviceAccountJSON)
	if err != nil {
		return "", fmt.Errorf("Could not instantiate isolate client: %s", err)
	}
	isolateTask := &isolate.Task{
		BaseDir:     pathToIsolates,
		IsolateFile: path.Join(pathToIsolates, isolateFileName),
		Deps:        []string{},
		OsType:      osType,
	}
	isolateHashes, _, err := isolateClient.IsolateTasks(ctx, []*isolate.Task{isolateTask})
	if err != nil {
		return "", fmt.Errorf("Could not isolate the master script task: %s", err)
	}
	if len(isolateHashes) != 1 {
		return "", fmt.Errorf("Expected one hash when isolating the master script task. Not: %+v", isolateHashes)
	}

	// Trigger swarming task.
	req := MakeSwarmingTaskRequest(ctx, taskName, isolateHashes[0], cipdPkgs, cmd, []string{"name:" + taskName, "runid:" + runID}, GCE_LINUX_MASTER_DIMENSIONS, map[string]string{"PATH": "cipd_bin_packages"}, swarming.RECOMMENDED_PRIORITY, 3*24*time.Hour)
	resp, err := swarmingClient.TriggerTask(req)
	if err != nil {
		return "", fmt.Errorf("Could not trigger swarming task %s: %s", taskName, err)
	}
	return resp.TaskId, nil
}

// TriggerBuildRepoSwarmingTask creates a isolated.gen.json file using BUILD_REPO_ISOLATE,
// archives it, and triggers it's swarming task. The swarming task will run the build_repo
// worker script which will return a list of remote build directories.
func TriggerBuildRepoSwarmingTask(ctx context.Context, taskName, runID, repoAndTarget, targetPlatform, serviceAccountJSON string, hashes, patches, cipdPkgs []string, singleBuild, local bool, hardTimeout, ioTimeout time.Duration, swarmingClient swarming.ApiClient) ([]string, error) {
	// Get path to isolate files.
	pathToIsolates, err := GetPathToIsolates(local, false)
	if err != nil {
		return nil, fmt.Errorf("Could not get path to isolates: %s", err)
	}

	// Find which os and CIPD pkgs to use.
	osType := "linux"
	if targetPlatform == PLATFORM_WINDOWS {
		osType = "win"
		cipdPkgs = append(cipdPkgs, cipd.GetStrCIPDPkgs(cipd.PkgsGit[cipd.PlatformWindowsAmd64])...)
		cipdPkgs = append(cipdPkgs, LUCI_AUTH_CIPD_PACKAGE_WIN)
	} else {
		cipdPkgs = append(cipdPkgs, cipd.GetStrCIPDPkgs(cipd.PkgsGit[cipd.PlatformLinuxAmd64])...)
		cipdPkgs = append(cipdPkgs, LUCI_AUTH_CIPD_PACKAGE_LINUX)
	}

	// Isolate the task.
	isolateClient, err := GetIsolateClient(serviceAccountJSON)
	if err != nil {
		return nil, fmt.Errorf("Could not instantiate isolate client: %s", err)
	}
	isolateTask := &isolate.Task{
		BaseDir:     pathToIsolates,
		IsolateFile: path.Join(pathToIsolates, BUILD_REPO_ISOLATE),
		Deps:        []string{},
		OsType:      osType,
	}
	isolateHashes, _, err := isolateClient.IsolateTasks(ctx, []*isolate.Task{isolateTask})
	if err != nil {
		return nil, fmt.Errorf("Could not isolate the telemetry isolate task: %s", err)
	}
	if len(isolateHashes) != 1 {
		return nil, fmt.Errorf("Expected one hash when isolating telemetry isolate task. Not: %+v", isolateHashes)
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
	req := MakeSwarmingTaskRequest(ctx, taskName, isolateHashes[0], cipdPkgs, cmd, []string{"name:" + taskName, "runid:" + runID}, dimensions, map[string]string{"PATH": "cipd_bin_packages"}, swarming.RECOMMENDED_PRIORITY, ioTimeout)
	resp, err := swarmingClient.TriggerTask(req)
	if err != nil {
		return nil, fmt.Errorf("Could not trigger swarming task %s: %s", taskName, err)
	}
	isolateHash, _, err := pollSwarmingTaskToCompletion(ctx, resp.TaskId, swarmingClient, isolateClient)
	if err != nil {
		return nil, fmt.Errorf("Could not collect task ID %s: %s", resp.TaskId, err)
	}

	// Download isolate output of the task.
	outputDir, err := ioutil.TempDir("", fmt.Sprintf("download_%s", resp.TaskId))
	if err != nil {
		return nil, fmt.Errorf("Failed to create temporary dir: %s", err)
	}
	defer util.RemoveAll(outputDir)
	if err := isolateClient.DownloadIsolateHash(ctx, isolateHash, outputDir, "files-list.txt"); err != nil {
		return nil, fmt.Errorf("Could not download %s: %s", isolateHash, err)
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

func CreateCustomPagesets(webpages []string, pagesetsDir, targetPlatform string) error {
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
		pagesetPath := filepath.Join(pagesetsDir, fmt.Sprintf("%d.py", i+1))
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
