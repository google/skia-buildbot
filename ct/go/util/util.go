// Utility that contains methods for both CT master and worker scripts.
package util

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"

	"github.com/skia-dev/glog"
)

const (
	MAX_SYNC_TRIES = 3

	TS_FORMAT = "20060102150405"

	REMOVE_INVALID_SKPS_WORKER_POOL = 20
)

// GetCTBareMetalWorkers returns an array of all CT bare-metal workers in the Cluster
// Telemetry Golo.
func GetCTBareMetalWorkers() []string {
	workers := make([]string, NUM_BARE_METAL_MACHINES)
	for i := 0; i < NUM_BARE_METAL_MACHINES; i++ {
		workers[i] = fmt.Sprintf(BARE_METAL_NAME_TEMPLATE, i+1)
	}
	return workers
}

func TimeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	glog.Infof("===== %s took %s =====", name, elapsed)
}

// ExecuteCmd executes the specified binary with the specified args and env. Stdout and Stderr are
// written to stdout and stderr respectively if specified. If not specified then Stdout and Stderr
// will be outputted only to glog.
func ExecuteCmd(binary string, args, env []string, timeout time.Duration, stdout, stderr io.Writer) error {
	return exec.Run(&exec.Command{
		Name:        binary,
		Args:        args,
		Env:         env,
		InheritPath: true,
		Timeout:     timeout,
		LogStdout:   true,
		Stdout:      stdout,
		LogStderr:   true,
		Stderr:      stderr,
	})
}

// SyncDir runs "git pull" and "gclient sync" on the specified directory.
func SyncDir(dir string) error {
	err := os.Chdir(dir)
	if err != nil {
		return fmt.Errorf("Could not chdir to %s: %s", dir, err)
	}

	for i := 0; i < MAX_SYNC_TRIES; i++ {
		if i > 0 {
			glog.Warningf("%d. retry for syncing %s", i, dir)
		}

		err = syncDirStep()
		if err == nil {
			break
		}
		glog.Errorf("Error syncing %s", dir)
	}

	if err != nil {
		glog.Errorf("Failed to sync %s after %d attempts", dir, MAX_SYNC_TRIES)
	}
	return err
}

func syncDirStep() error {
	err := ExecuteCmd(BINARY_GIT, []string{"pull"}, []string{}, GIT_PULL_TIMEOUT, nil, nil)
	if err != nil {
		return fmt.Errorf("Error running git pull: %s", err)
	}
	err = ExecuteCmd(BINARY_GCLIENT, []string{"sync"}, []string{}, GCLIENT_SYNC_TIMEOUT, nil,
		nil)
	if err != nil {
		return fmt.Errorf("Error running gclient sync: %s", err)
	}
	return nil
}

// BuildSkiaTools builds "tools" in the Skia trunk directory.
func BuildSkiaTools() error {
	if err := os.Chdir(SkiaTreeDir); err != nil {
		return fmt.Errorf("Could not chdir to %s: %s", SkiaTreeDir, err)
	}
	// Run "make clean".
	util.LogErr(ExecuteCmd(BINARY_MAKE, []string{"clean"}, []string{}, MAKE_CLEAN_TIMEOUT, nil,
		nil))
	// Build tools.
	return ExecuteCmd(BINARY_MAKE, []string{"tools", "BUILDTYPE=Release"},
		[]string{"GYP_DEFINES=\"skia_warnings_as_errors=0\""}, MAKE_TOOLS_TIMEOUT, nil, nil)
}

// BuildPDFium builds "pdfium_test" in the PDFium repo directory.
func BuildPDFium() error {
	if err := os.Chdir(PDFiumTreeDir); err != nil {
		return fmt.Errorf("Could not chdir to %s: %s", SkiaTreeDir, err)
	}

	// Run "build/gyp_pdfium"
	if err := ExecuteCmd(path.Join("build_gyp", "gyp_pdfium"), []string{},
		[]string{"GYP_DEFINES=\"pdf_use_skia=1\"", "CPPFLAGS=\"-Wno-error\""}, GYP_PDFIUM_TIMEOUT, nil, nil); err != nil {
		return err
	}

	// Build pdfium_test.
	return ExecuteCmd(BINARY_NINJA, []string{"-C", "out/Debug", BINARY_PDFIUM_TEST},
		[]string{}, NINJA_TIMEOUT, nil, nil)
}

// ResetCheckout resets the specified Git checkout.
func ResetCheckout(dir string) error {
	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("Could not chdir to %s: %s", dir, err)
	}
	// Run "git reset --hard HEAD"
	resetArgs := []string{"reset", "--hard", "HEAD"}
	util.LogErr(ExecuteCmd(BINARY_GIT, resetArgs, []string{}, GIT_RESET_TIMEOUT, nil, nil))
	// Run "git clean -f -d"
	cleanArgs := []string{"clean", "-f", "-d"}
	util.LogErr(ExecuteCmd(BINARY_GIT, cleanArgs, []string{}, GIT_CLEAN_TIMEOUT, nil, nil))

	return nil
}

// ApplyPatch applies a patch to a Git checkout.
func ApplyPatch(patch, dir string) error {
	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("Could not chdir to %s: %s", dir, err)
	}
	// Run "git apply --index -p1 --verbose --ignore-whitespace
	//      --ignore-space-change ${PATCH_FILE}"
	args := []string{"apply", "--index", "-p1", "--verbose", "--ignore-whitespace", "--ignore-space-change", patch}
	return ExecuteCmd(BINARY_GIT, args, []string{}, GIT_APPLY_TIMEOUT, nil, nil)
}

// CleanTmpDir deletes all tmp files from the caller because telemetry tends to
// generate a lot of temporary artifacts there and they take up root disk space.
func CleanTmpDir() {
	files, _ := ioutil.ReadDir(os.TempDir())
	for _, f := range files {
		util.RemoveAll(filepath.Join(os.TempDir(), f.Name()))
	}
}

// Get a direct link to the log of this task on the master's logserver.
func GetMasterLogLink(runID string) string {
	programName := filepath.Base(os.Args[0])
	return fmt.Sprintf("%s/%s.%s.%s.log.INFO.%s", MASTER_LOGSERVER_LINK, programName, MASTER_NAME, CtUser, runID)
}

func GetTimeFromTs(formattedTime string) time.Time {
	t, _ := time.Parse(TS_FORMAT, formattedTime)
	return t
}

func GetCurrentTs() string {
	return time.Now().UTC().Format(TS_FORMAT)
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
func ChromeProcessesCleaner(locker sync.Locker, chromeCleanerTimer time.Duration) {
	for _ = range time.Tick(chromeCleanerTimer) {
		glog.Info("The chromeProcessesCleaner goroutine has started")
		glog.Info("Waiting for all existing tasks to complete before killing zombie chrome processes")
		locker.Lock()
		util.LogErr(ExecuteCmd("pkill", []string{"-9", "chrome"}, []string{}, PKILL_TIMEOUT, nil, nil))
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
func ValidateSKPs(pathToSkps, pathToPyFiles string) error {
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
				glog.Errorf("Unable to read %s: %s", filepath.Join(pathToSkps, index, skpName), err)
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
					util.Rename(layerPath, destSKP)
					skps = append(skps, destSKP)
				} else {
					glog.Warningf("Skipping %s because size was less than 6000 bytes", skpName)
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

	glog.Info("Calling remove_invalid_skp.py")
	// Sync Skia tree.
	util.LogErr(SyncDir(SkiaTreeDir))
	// Build tools.
	util.LogErr(BuildSkiaTools())
	// Run remove_invalid_skp.py in parallel goroutines.
	// Construct path to the python script.
	pathToRemoveSKPs := filepath.Join(pathToPyFiles, "remove_invalid_skp.py")
	pathToSKPInfo := filepath.Join(SkiaTreeDir, "out", "Release", "skpinfo")

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
					"--path_to_skpinfo=" + pathToSKPInfo,
				}
				glog.Infof("Executing remove_invalid_skp.py with goroutine#%d", i+1)
				// Execute the command with stdout not logged. It otherwise outputs
				// tons of log msgs.
				util.LogErr(exec.Run(&exec.Command{
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
// artifacts it is allowed to process.
func GetStartRange(workerNum, artifactsPerWorker int) int {
	return ((workerNum - 1) * artifactsPerWorker) + 1
}

func TriggerSwarmingTask(pagesetType, taskPrefix, isolateName, runID string, hardTimeout, ioTimeout time.Duration, maxPagesPerBot int, isolateExtraArgs, dimensions map[string]string) error {
	// Instantiate the swarming client.
	workDir, err := ioutil.TempDir("", "swarming_work_")
	if err != nil {
		return fmt.Errorf("Could not get temp dir: %s", err)
	}
	s, err := swarming.NewSwarmingClient(workDir)
	if err != nil {
		return fmt.Errorf("Could not instantiate swarming client: %s", err)
	}
	defer s.Cleanup()
	// Create isolated.gen.json files from tasks.
	genJSONs := []string{}
	// Get path to isolate files.
	_, currentFile, _, _ := runtime.Caller(0)
	pathToIsolates := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(currentFile))), "isolates")
	for i := 1; i <= PagesetTypeToInfo[pagesetType].NumPages/maxPagesPerBot; i++ {
		isolateArgs := map[string]string{
			"START_RANGE":  strconv.Itoa(GetStartRange(i, maxPagesPerBot)),
			"NUM":          strconv.Itoa(maxPagesPerBot),
			"PAGESET_TYPE": pagesetType,
		}
		// Add isolateExtraArgs (if specified) into the isolateArgs.
		for k, v := range isolateExtraArgs {
			isolateArgs[k] = v
		}
		taskName := fmt.Sprintf("%s_%d", taskPrefix, i)
		genJSON, err := s.CreateIsolatedGenJSON(path.Join(pathToIsolates, isolateName), s.WorkDir, "linux", taskName, isolateArgs, []string{})
		if err != nil {
			return fmt.Errorf("Could not create isolated.gen.json for task %s: %s", taskName, err)
		}
		genJSONs = append(genJSONs, genJSON)
	}

	// Batcharchive the tasks. Do not batcharchive more than 1000 at a time.
	tasksToHashes := map[string]string{}
	for i := 0; i < len(genJSONs); i += 1000 {
		startRange := i
		endRange := util.MinInt(len(genJSONs), i+1000)
		t, err := s.BatchArchiveTargets(genJSONs[startRange:endRange], BATCHARCHIVE_TIMEOUT)
		if err != nil {
			return fmt.Errorf("Could not batch archive targets: %s", err)
		}
		// Add the above map to tasksToHashes.
		for k, v := range t {
			tasksToHashes[k] = v
		}
		// Sleep for a sec to give the swarming server some time to recuperate.
		time.Sleep(time.Second)
	}

	if len(genJSONs) != len(tasksToHashes) {
		return fmt.Errorf("len(genJSONs) was %d and len(tasksToHashes) was %d", len(genJSONs), len(tasksToHashes))
	}
	// Trigger swarming using the isolate hashes.
	tasks, err := s.TriggerSwarmingTasks(tasksToHashes, dimensions, map[string]string{"runid": runID}, swarming.RECOMMENDED_PRIORITY, 2*24*time.Hour, hardTimeout, ioTimeout, false)
	if err != nil {
		return fmt.Errorf("Could not trigger swarming task: %s", err)
	}
	// Collect all tasks and collect the ones that fail.
	failedTasksToHashes := map[string]string{}
	for _, task := range tasks {
		if _, _, err := task.Collect(s); err != nil {
			glog.Errorf("task %s failed: %s", task.Title, err)
			failedTasksToHashes[task.Title] = tasksToHashes[task.Title]
			continue
		}
	}

	if len(failedTasksToHashes) > 0 {
		glog.Info("Retrying tasks that failed...")
		retryTasks, err := s.TriggerSwarmingTasks(failedTasksToHashes, dimensions, map[string]string{"runid": runID}, swarming.RECOMMENDED_PRIORITY, 2*24*time.Hour, hardTimeout, ioTimeout, false)
		if err != nil {
			return fmt.Errorf("Could not trigger swarming task: %s", err)
		}
		// Collect all tasks and log the ones that fail.
		for _, task := range retryTasks {
			if _, _, err := task.Collect(s); err != nil {
				glog.Errorf("task %s failed inspite of a retry: %s", task.Title, err)
				continue
			}
		}
	}
	return nil
}

// GetPathToPyFiles returns the location of CT's python scripts.
func GetPathToPyFiles(runOnSwarming bool) string {
	if runOnSwarming {
		return filepath.Join(filepath.Dir(filepath.Dir(os.Args[0])), "src", "go.skia.org", "infra", "ct", "py")
	} else {
		_, currentFile, _, _ := runtime.Caller(0)
		return filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(currentFile))), "py")
	}
}

func MergeUploadCSVFiles(runID, pathToPyFiles string, gs *GsUtil, totalPages, numPerWorker int) ([]string, error) {
	localOutputDir := filepath.Join(StorageDir, BenchmarkRunsDir, runID)
	util.MkdirAll(localOutputDir, 0700)
	noOutputSlaves := []string{}
	// Copy outputs from all slaves locally.
	for i := 0; i < totalPages/numPerWorker; i++ {
		startRange := (i * numPerWorker) + 1
		workerLocalOutputPath := filepath.Join(localOutputDir, strconv.Itoa(startRange)+".csv")
		workerRemoteOutputPath := filepath.Join(BenchmarkRunsDir, runID, strconv.Itoa(startRange), "outputs", runID+".output")
		respBody, err := gs.GetRemoteFileContents(workerRemoteOutputPath)
		if err != nil {
			glog.Errorf("Could not fetch %s: %s", workerRemoteOutputPath, err)
			noOutputSlaves = append(noOutputSlaves, strconv.Itoa(i+1))
			continue
		}
		defer util.Close(respBody)
		out, err := os.Create(workerLocalOutputPath)
		if err != nil {
			return noOutputSlaves, fmt.Errorf("Unable to create file %s: %s", workerLocalOutputPath, err)
		}
		defer util.Close(out)
		defer util.Remove(workerLocalOutputPath)
		if _, err = io.Copy(out, respBody); err != nil {
			return noOutputSlaves, fmt.Errorf("Unable to copy to file %s: %s", workerLocalOutputPath, err)
		}
		// If an output is less than 20 bytes that means something went wrong on the slave.
		outputInfo, err := out.Stat()
		if err != nil {
			return noOutputSlaves, fmt.Errorf("Unable to stat file %s: %s", workerLocalOutputPath, err)
		}
		if outputInfo.Size() <= 20 {
			glog.Errorf("Output file was less than 20 bytes %s: %s", workerLocalOutputPath, err)
			noOutputSlaves = append(noOutputSlaves, strconv.Itoa(i+1))
			continue
		}
	}
	// Call csv_merger.py to merge all results into a single results CSV.
	pathToCsvMerger := filepath.Join(pathToPyFiles, "csv_merger.py")
	outputFileName := runID + ".output"
	args := []string{
		pathToCsvMerger,
		"--csv_dir=" + localOutputDir,
		"--output_csv_name=" + filepath.Join(localOutputDir, outputFileName),
	}
	err := ExecuteCmd("python", args, []string{}, CSV_MERGER_TIMEOUT, nil, nil)
	if err != nil {
		return noOutputSlaves, fmt.Errorf("Error running csv_merger.py: %s", err)
	}
	// Copy the output file to Google Storage.
	remoteOutputDir := filepath.Join(BenchmarkRunsDir, runID, "consolidated_outputs")
	if err := gs.UploadFile(outputFileName, localOutputDir, remoteOutputDir); err != nil {
		return noOutputSlaves, fmt.Errorf("Unable to upload %s to %s: %s", outputFileName, remoteOutputDir, err)
	}
	return noOutputSlaves, nil
}

func RunBenchmark(fileInfoName, pathToPagesets, pathToPyFiles, localOutputDir, chromiumBuildName, chromiumBinary, runID, browserExtraArgs, benchmarkName, targetPlatform, benchmarkExtraArgs, pagesetType string, repeatBenchmark int, retWebpageFailErr bool) error {
	pagesetBaseName := filepath.Base(fileInfoName)
	if filepath.Ext(pagesetBaseName) == ".pyc" {
		// Ignore .pyc files.
		return nil
	}
	// Read the pageset.
	pagesetName := strings.TrimSuffix(pagesetBaseName, filepath.Ext(pagesetBaseName))
	pagesetPath := filepath.Join(pathToPagesets, fileInfoName)
	decodedPageset, err := ReadPageset(pagesetPath)
	if err != nil {
		return fmt.Errorf("Could not read %s: %s", pagesetPath, err)
	}
	glog.Infof("===== Processing %s for %s =====", pagesetPath, runID)
	benchmark, present := BenchmarksToTelemetryName[benchmarkName]
	if !present {
		// If it is custom benchmark use the entered benchmark name.
		benchmark = benchmarkName
	}
	args := []string{
		filepath.Join(TelemetryBinariesDir, BINARY_RUN_BENCHMARK),
		benchmark,
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
		if err := InstallChromeAPK(chromiumBuildName); err != nil {
			return fmt.Errorf("Error while installing APK: %s", err)
		}
		args = append(args, "--browser=android-chromium")
	} else {
		args = append(args, "--browser=exact", "--browser-executable="+chromiumBinary)
		args = append(args, "--device=desktop")
	}
	// Split benchmark args if not empty and append to args.
	if benchmarkExtraArgs != "" {
		args = append(args, strings.Fields(benchmarkExtraArgs)...)
	}
	if repeatBenchmark > 0 {
		// Add the number of times to repeat.
		args = append(args, fmt.Sprintf("--page-repeat=%d", repeatBenchmark))
	}
	// Add browserArgs if not empty to args.
	if browserExtraArgs != "" {
		args = append(args, "--extra-browser-args="+browserExtraArgs)
	}
	// Set the PYTHONPATH to the pagesets and the telemetry dirs.
	env := []string{
		fmt.Sprintf("PYTHONPATH=%s:%s:%s:%s:$PYTHONPATH", pathToPagesets, TelemetryBinariesDir, TelemetrySrcDir, CatapultSrcDir),
		"DISPLAY=:0",
	}
	timeoutSecs := PagesetTypeToInfo[pagesetType].RunChromiumPerfTimeoutSecs
	if err := ExecuteCmd("python", args, env, time.Duration(timeoutSecs)*time.Second, nil, nil); err != nil {
		if retWebpageFailErr {
			return fmt.Errorf("Run benchmark command failed with: %s", err)
		} else {
			glog.Errorf("Run benchmark command failed with: %s", err)
		}
	}
	return nil
}

func MergeUploadCSVFilesOnWorkers(localOutputDir, pathToPyFiles, runID, remoteDir string, gs *GsUtil, startRange int, handleStrings bool) error {
	// Move all results into a single directory.
	fileInfos, err := ioutil.ReadDir(localOutputDir)
	if err != nil {
		return fmt.Errorf("Unable to read %s: %s", localOutputDir, err)
	}
	for _, fileInfo := range fileInfos {
		if !fileInfo.IsDir() {
			continue
		}
		outputFile := filepath.Join(localOutputDir, fileInfo.Name(), "results-pivot-table.csv")
		newFile := filepath.Join(localOutputDir, fmt.Sprintf("%s.csv", fileInfo.Name()))
		if err := os.Rename(outputFile, newFile); err != nil {
			glog.Errorf("Could not rename %s to %s: %s", outputFile, newFile, err)
			continue
		}
		// Add the rank of the page to the CSV file.
		headers, values, err := getRowsFromCSV(newFile)
		if err != nil {
			glog.Errorf("Could not read %s: %s", newFile, err)
			continue
		}
		pageRank := fileInfo.Name()
		for i := range headers {
			for j := range values {
				if headers[i] == "page" {
					values[j][i] = fmt.Sprintf("%s (#%s)", values[j][i], pageRank)
				}
			}
		}
		if err := writeRowsToCSV(newFile, headers, values); err != nil {
			glog.Errorf("Could not write to %s: %s", newFile, err)
			continue
		}
	}
	// Call csv_pivot_table_merger.py to merge all results into a single results CSV.
	pathToCsvMerger := filepath.Join(pathToPyFiles, "csv_pivot_table_merger.py")
	outputFileName := runID + ".output"
	args := []string{
		pathToCsvMerger,
		"--csv_dir=" + localOutputDir,
		"--output_csv_name=" + filepath.Join(localOutputDir, outputFileName),
	}
	if handleStrings {
		args = append(args, "--handle_strings")
	}
	err = ExecuteCmd("python", args, []string{}, CSV_PIVOT_TABLE_MERGER_TIMEOUT, nil,
		nil)
	if err != nil {
		return fmt.Errorf("Error running csv_pivot_table_merger.py: %s", err)
	}
	// Copy the output file to Google Storage.
	remoteOutputDir := filepath.Join(remoteDir, strconv.Itoa(startRange), "outputs")
	if err := gs.UploadFile(outputFileName, localOutputDir, remoteOutputDir); err != nil {
		return fmt.Errorf("Unable to upload %s to %s: %s", outputFileName, remoteOutputDir, err)
	}
	return nil
}

func getRowsFromCSV(csvPath string) ([]string, [][]string, error) {
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

// TriggerBuildRepoSwarmingTask creates a isolated.gen.json file using BUILD_REPO_ISOLATE,
// archives it, and triggers it's swarming task. The swarming task will run the build_repo
// worker script which will return a list of remote build directories.
func TriggerBuildRepoSwarmingTask(taskName, runID, repo, targetPlatform string, hashes, patches []string, singleBuild bool, hardTimeout, ioTimeout time.Duration) ([]string, error) {
	// Instantiate the swarming client.
	workDir, err := ioutil.TempDir("", "swarming_work_")
	if err != nil {
		return nil, fmt.Errorf("Could not get temp dir: %s", err)
	}
	s, err := swarming.NewSwarmingClient(workDir)
	if err != nil {
		return nil, fmt.Errorf("Could not instantiate swarming client: %s", err)
	}
	defer s.Cleanup()
	// Create isolated.gen.json.
	// Get path to isolate files.
	_, currentFile, _, _ := runtime.Caller(0)
	pathToIsolates := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(currentFile))), "isolates")
	isolateArgs := map[string]string{
		"RUN_ID":          runID,
		"REPO":            repo,
		"HASHES":          strings.Join(hashes, ","),
		"PATCHES":         strings.Join(patches, ","),
		"SINGLE_BUILD":    strconv.FormatBool(singleBuild),
		"TARGET_PLATFORM": targetPlatform,
	}
	genJSON, err := s.CreateIsolatedGenJSON(path.Join(pathToIsolates, BUILD_REPO_ISOLATE), s.WorkDir, "linux", taskName, isolateArgs, []string{})
	if err != nil {
		return nil, fmt.Errorf("Could not create isolated.gen.json for task %s: %s", taskName, err)
	}
	// Batcharchive the task.
	tasksToHashes, err := s.BatchArchiveTargets([]string{genJSON}, BATCHARCHIVE_TIMEOUT)
	if err != nil {
		return nil, fmt.Errorf("Could not batch archive target: %s", err)
	}
	// Trigger swarming using the isolate hash.
	var dimensions map[string]string
	if targetPlatform == "Android" {
		dimensions = GCE_ANDROID_BUILDER_DIMENSIONS
	} else {
		dimensions = GCE_LINUX_BUILDER_DIMENSIONS
	}
	tasks, err := s.TriggerSwarmingTasks(tasksToHashes, dimensions, map[string]string{"runid": runID}, swarming.RECOMMENDED_PRIORITY, 2*24*time.Hour, hardTimeout, ioTimeout, false)
	if err != nil {
		return nil, fmt.Errorf("Could not trigger swarming task: %s", err)
	}
	if len(tasks) != 1 {
		return nil, fmt.Errorf("Expected a single task instead got: %v", tasks)
	}
	// Collect all tasks and log the ones that fail.
	task := tasks[0]
	_, outputDir, err := task.Collect(s)
	if err != nil {
		return nil, fmt.Errorf("task %s failed: %s", task.Title, err)
	}
	outputFile := filepath.Join(outputDir, BUILD_OUTPUT_FILENAME)
	contents, err := ioutil.ReadFile(outputFile)
	if err != nil {
		return nil, fmt.Errorf("Could not read outputfile %s: %s", outputFile, err)
	}
	return strings.Split(string(contents), ","), nil
}

func DownloadPatch(localPath, remotePath string, gs *GsUtil) (int64, error) {
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

func DownloadAndApplyPatch(patchName, localDir, remotePatchesDir, checkout string, gs *GsUtil) error {
	patchLocalPath := filepath.Join(localDir, patchName)
	patchRemotePath := filepath.Join(remotePatchesDir, patchName)
	written, err := DownloadPatch(patchLocalPath, patchRemotePath, gs)
	if err != nil {
		return fmt.Errorf("Could not download %s: %s", patchRemotePath, err)
	}
	// Apply patch to the local checkout.
	if written > 10 {
		if err := ApplyPatch(patchLocalPath, checkout); err != nil {
			return fmt.Errorf("Could not apply patch in %s: %s", checkout, err)
		}
	}
	return nil
}
