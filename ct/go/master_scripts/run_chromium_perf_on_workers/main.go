// run_chromium_perf_on_workers is an application that runs the specified telemetry
// benchmark on all CT workers and uploads the results to Google Storage. The
// requester is emailed when the task is done.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
)

var (
	emails                    = flag.String("emails", "", "The comma separated email addresses to notify when the task is picked up and completes.")
	gaeTaskID                 = flag.Int("gae_task_id", -1, "The key of the App Engine task. This task will be updated when the task is completed.")
	pagesetType               = flag.String("pageset_type", "", "The type of pagesets to use. Eg: 10k, Mobile10k, All.")
	benchmarkName             = flag.String("benchmark_name", "", "The telemetry benchmark to run on the workers.")
	benchmarkExtraArgs        = flag.String("benchmark_extra_args", "", "The extra arguments that are passed to the specified benchmark.")
	browserExtraArgsNoPatch   = flag.String("browser_extra_args_nopatch", "", "The extra arguments that are passed to the browser while running the benchmark for the nopatch case.")
	browserExtraArgsWithPatch = flag.String("browser_extra_args_withpatch", "", "The extra arguments that are passed to the browser while running the benchmark for the withpatch case.")
	repeatBenchmark           = flag.Int("repeat_benchmark", 1, "The number of times the benchmark should be repeated. For skpicture_printer benchmark this value is always 1.")
	targetPlatform            = flag.String("target_platform", util.PLATFORM_ANDROID, "The platform the benchmark will run on (Android / Linux).")
	runID                     = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	varianceThreshold         = flag.Float64("variance_threshold", 0.0, "The variance threshold to use when comparing the resultant CSV files.")
	discardOutliers           = flag.Float64("discard_outliers", 0.0, "The percentage of outliers to discard when comparing the result CSV files.")

	taskCompletedSuccessfully = false

	htmlOutputLink      = util.MASTER_LOGSERVER_LINK
	skiaPatchLink       = util.MASTER_LOGSERVER_LINK
	blinkPatchLink      = util.MASTER_LOGSERVER_LINK
	chromiumPatchLink   = util.MASTER_LOGSERVER_LINK
	noPatchOutputLink   = util.MASTER_LOGSERVER_LINK
	withPatchOutputLink = util.MASTER_LOGSERVER_LINK
)

func sendEmail(recipients []string) {
	// Send completion email.
	emailSubject := fmt.Sprintf("Cluster telemetry chromium perf task has completed (%s)", *runID)
	failureHtml := ""
	if !taskCompletedSuccessfully {
		emailSubject += " with failures"
		failureHtml = util.FailureEmailHtml
	}
	bodyTemplate := `
	The chromium perf %s benchmark task on %s pageset has completed.<br/>
	%s
	The HTML output with differences between the base run and the patch run is <a href='%s'>here</a>.<br/>
	The patch(es) you specified are here:
	<a href='%s'>chromium</a>/<a href='%s'>blink</a>/<a href='%s'>skia</a>
	<br/><br/>
	You can schedule more runs <a href='%s'>here</a>.
	<br/><br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, *benchmarkName, *pagesetType, failureHtml, htmlOutputLink, chromiumPatchLink, blinkPatchLink, skiaPatchLink, util.ChromiumPerfTasksWebapp)
	if err := util.SendEmail(recipients, emailSubject, emailBody); err != nil {
		glog.Errorf("Error while sending email: %s", err)
		return
	}
}

func updateWebappTask() {
	// TODO(rmistry): Add ability to update the webapp without providing links.
	extraData := map[string]string{
		"skia_patch_link":              skiaPatchLink,
		"blink_patch_link":             blinkPatchLink,
		"chromium_patch_link":          chromiumPatchLink,
		"telemetry_nopatch_log_link":   noPatchOutputLink,
		"telemetry_withpatch_log_link": withPatchOutputLink,
		"html_output_link":             htmlOutputLink,
		"build_log_link":               util.MASTER_LOGSERVER_LINK,
	}
	if err := util.UpdateWebappTask(*gaeTaskID, util.UpdateChromiumPerfTasksWebapp, extraData); err != nil {
		glog.Errorf("Error while updating webapp task: %s", err)
		return
	}
}

func main() {
	common.Init()

	// Send start email.
	emailsArr := util.ParseEmails(*emails)
	emailsArr = append(emailsArr, util.CtAdmins...)
	if len(emailsArr) == 0 {
		glog.Error("At least one email address must be specified")
		return
	}
	util.SendTaskStartEmail(emailsArr, "Chromium perf")
	// Ensure webapp is updated and email is sent even if task fails.
	defer updateWebappTask()
	defer sendEmail(emailsArr)
	// Cleanup dirs after run completes.
	defer os.RemoveAll(filepath.Join(util.StorageDir, util.ChromiumPerfRunsDir))
	defer os.RemoveAll(filepath.Join(util.StorageDir, util.BenchmarkRunsDir))
	// Cleanup tmp files after the run.
	defer util.CleanTmpDir()
	// Finish with glog flush and how long the task took.
	defer util.TimeTrack(time.Now(), "Running chromium perf task on workers")
	defer glog.Flush()

	if *pagesetType == "" {
		glog.Error("Must specify --pageset_type")
		return
	}
	if *benchmarkName == "" {
		glog.Error("Must specify --benchmark_name")
		return
	}
	if *runID == "" {
		glog.Error("Must specify --run_id")
		return
	}

	// Instantiate GsUtil object.
	gs, err := util.NewGsUtil(nil)
	if err != nil {
		glog.Errorf("Could not instantiate gsutil object: %s", err)
		return
	}
	remoteOutputDir := filepath.Join(util.ChromiumPerfRunsDir, *runID)

	// Copy the patches to Google Storage.
	skiaPatchName := *runID + ".skia.patch"
	blinkPatchName := *runID + ".blink.patch"
	chromiumPatchName := *runID + ".chromium.patch"
	for _, patchName := range []string{skiaPatchName, blinkPatchName, chromiumPatchName} {
		if err := gs.UploadFile(patchName, os.TempDir(), remoteOutputDir); err != nil {
			glog.Errorf("Could not upload %s to %s: %s", patchName, remoteOutputDir, err)
			return
		}
	}
	skiaPatchLink = util.GS_HTTP_LINK + filepath.Join(util.GS_BUCKET_NAME, remoteOutputDir, skiaPatchName)
	blinkPatchLink = util.GS_HTTP_LINK + filepath.Join(util.GS_BUCKET_NAME, remoteOutputDir, blinkPatchName)
	chromiumPatchLink = util.GS_HTTP_LINK + filepath.Join(util.GS_BUCKET_NAME, remoteOutputDir, chromiumPatchName)

	// Create the two required chromium builds (with patch and without the patch).
	chromiumHash, skiaHash, err := util.CreateChromiumBuild(*runID, *targetPlatform, "", "", true)
	if err != nil {
		glog.Errorf("Could not create chromium build: %s", err)
		return
	}

	// Reboot all workers to start from a clean slate.
	util.RebootWorkers()

	// Run the run_chromium_perf script on all workers.
	runIDNoPatch := *runID + "-nopatch"
	runIDWithPatch := *runID + "-withpatch"
	chromiumBuildNoPatch := fmt.Sprintf("try-%s-%s-%s", chromiumHash, skiaHash, runIDNoPatch)
	chromiumBuildWithPatch := fmt.Sprintf("try-%s-%s-%s", chromiumHash, skiaHash, runIDWithPatch)
	runChromiumPerfCmdTemplate := "DISPLAY=:0 run_chromium_perf " +
		"--worker_num={{.WorkerNum}} --log_dir={{.LogDir}} --pageset_type={{.PagesetType}} " +
		"--chromium_build_nopatch={{.ChromiumBuildNoPatch}} --chromium_build_withpatch={{.ChromiumBuildWithPatch}} " +
		"--run_id_nopatch={{.RunIDNoPatch}} --run_id_withpatch={{.RunIDWithPatch}} " +
		"--benchmark_name={{.BenchmarkName}} --benchmark_extra_args=\"{{.BenchmarkExtraArgs}}\" " +
		"--browser_extra_args_nopatch=\"{{.BrowserExtraArgsNoPatch}}\" --browser_extra_args_withpatch=\"{{.BrowserExtraArgsWithPatch}}\" " +
		"--repeat_benchmark={{.RepeatBenchmark}} --target_platform={{.TargetPlatform}};"
	runChromiumPerfTemplateParsed := template.Must(template.New("run_chromium_perf_cmd").Parse(runChromiumPerfCmdTemplate))
	runChromiumPerfCmdBytes := new(bytes.Buffer)
	runChromiumPerfTemplateParsed.Execute(runChromiumPerfCmdBytes, struct {
		WorkerNum                 string
		LogDir                    string
		PagesetType               string
		ChromiumBuildNoPatch      string
		ChromiumBuildWithPatch    string
		RunIDNoPatch              string
		RunIDWithPatch            string
		BenchmarkName             string
		BenchmarkExtraArgs        string
		BrowserExtraArgsNoPatch   string
		BrowserExtraArgsWithPatch string
		RepeatBenchmark           int
		TargetPlatform            string
	}{
		WorkerNum:                 util.WORKER_NUM_KEYWORD,
		LogDir:                    util.GLogDir,
		PagesetType:               *pagesetType,
		ChromiumBuildNoPatch:      chromiumBuildNoPatch,
		ChromiumBuildWithPatch:    chromiumBuildWithPatch,
		RunIDNoPatch:              runIDNoPatch,
		RunIDWithPatch:            runIDWithPatch,
		BenchmarkName:             *benchmarkName,
		BenchmarkExtraArgs:        *benchmarkExtraArgs,
		BrowserExtraArgsNoPatch:   *browserExtraArgsNoPatch,
		BrowserExtraArgsWithPatch: *browserExtraArgsWithPatch,
		RepeatBenchmark:           *repeatBenchmark,
		TargetPlatform:            *targetPlatform,
	})
	cmd := []string{
		fmt.Sprintf("cd %s;", util.CtTreeDir),
		"git pull;",
		"make all;",
		// The main command that runs run_chromium_perf on all workers.
		runChromiumPerfCmdBytes.String(),
	}
	// Setting a 1 day timeout since it may take a while run benchmarks with many
	// repeats.
	if _, err := util.SSH(strings.Join(cmd, " "), util.Slaves, 1*24*time.Hour); err != nil {
		glog.Errorf("Error while running cmd %s: %s", cmd, err)
		return
	}

	// If "--output-format=csv" was specified then merge all CSV files and upload.
	if strings.Contains(*benchmarkExtraArgs, "--output-format=csv") {
		for _, runID := range []string{runIDNoPatch, runIDWithPatch} {
			if err := mergeUploadCSVFiles(runID, gs); err != nil {
				glog.Errorf("Unable to merge and upload CSV files for %s: %s", runID, err)
			}
		}
	}

	// Compare the resultant CSV files using csv_comparer.py
	noPatchCSVPath := filepath.Join(util.StorageDir, util.BenchmarkRunsDir, runIDNoPatch, runIDNoPatch+".output")
	withPatchCSVPath := filepath.Join(util.StorageDir, util.BenchmarkRunsDir, runIDWithPatch, runIDWithPatch+".output")
	htmlOutputDir := filepath.Join(util.StorageDir, util.ChromiumPerfRunsDir, *runID, "html")
	os.MkdirAll(htmlOutputDir, 0700)
	htmlRemoteDir := filepath.Join(remoteOutputDir, "html")
	htmlOutputLinkBase := util.GS_HTTP_LINK + filepath.Join(util.GS_BUCKET_NAME, htmlRemoteDir) + "/"
	htmlOutputLink = htmlOutputLinkBase + "index.html"
	noPatchOutputLink = util.GS_HTTP_LINK + filepath.Join(util.GS_BUCKET_NAME, util.BenchmarkRunsDir, runIDNoPatch, "consolidated_outputs", runIDNoPatch+".output")
	withPatchOutputLink = util.GS_HTTP_LINK + filepath.Join(util.GS_BUCKET_NAME, util.BenchmarkRunsDir, runIDWithPatch, "consolidated_outputs", runIDWithPatch+".output")
	// Construct path to the csv_comparer python script.
	_, currentFile, _, _ := runtime.Caller(0)
	pathToPyFiles := filepath.Join(
		filepath.Dir((filepath.Dir(filepath.Dir(filepath.Dir(currentFile))))),
		"py")
	pathToCsvComparer := filepath.Join(pathToPyFiles, "csv_comparer.py")
	args := []string{
		pathToCsvComparer,
		"--csv_file1=" + noPatchCSVPath,
		"--csv_file2=" + withPatchCSVPath,
		"--output_html=" + htmlOutputDir,
		"--variance_threshold=" + strconv.FormatFloat(*varianceThreshold, 'f', 2, 64),
		"--discard_outliers=" + strconv.FormatFloat(*discardOutliers, 'f', 2, 64),
		"--absolute_url=" + htmlOutputLinkBase,
		"--requester_email=" + *emails,
		"--skia_patch_link=" + skiaPatchLink,
		"--blink_patch_link=" + blinkPatchLink,
		"--chromium_patch_link=" + chromiumPatchLink,
		"--raw_csv_nopatch=" + noPatchOutputLink,
		"--raw_csv_withpatch=" + withPatchOutputLink,
		"--num_repeated=" + strconv.Itoa(*repeatBenchmark),
		"--target_platform=" + *targetPlatform,
		"--browser_args_nopatch=" + *browserExtraArgsNoPatch,
		"--browser_args_withpatch=" + *browserExtraArgsWithPatch,
		"--pageset_type=" + *pagesetType,
	}
	if err := util.ExecuteCmd("python", args, []string{}, 2*time.Hour, nil, nil); err != nil {
		glog.Errorf("Error running csv_comparer.py: %s", err)
		return
	}

	// Copy the HTML files to Google Storage.
	if err := gs.UploadDir(htmlOutputDir, htmlRemoteDir); err != nil {
		glog.Errorf("Could not upload %s to %s: %s", htmlOutputDir, htmlRemoteDir, err)
		return
	}

	taskCompletedSuccessfully = true
}

func mergeUploadCSVFiles(runID string, gs *util.GsUtil) error {
	localOutputDir := filepath.Join(util.StorageDir, util.BenchmarkRunsDir, runID)
	os.MkdirAll(localOutputDir, 0700)
	// Copy outputs from all slaves locally.
	for i := 0; i < util.NUM_WORKERS; i++ {
		workerNum := i + 1
		workerLocalOutputPath := filepath.Join(localOutputDir, fmt.Sprintf("slave%d", workerNum)+".csv")
		workerRemoteOutputPath := filepath.Join(util.BenchmarkRunsDir, runID, fmt.Sprintf("slave%d", workerNum), "outputs", runID+".output")
		respBody, err := gs.GetRemoteFileContents(workerRemoteOutputPath)
		if err != nil {
			glog.Errorf("Could not fetch %s: %s", workerRemoteOutputPath, err)
			// TODO(rmistry): Should we instead return here? We can only return
			// here if all 100 slaves reliably run without any failures which they
			// really should.
			continue
		}
		defer respBody.Close()
		out, err := os.Create(workerLocalOutputPath)
		if err != nil {
			return fmt.Errorf("Unable to create file %s: %s", workerLocalOutputPath, err)
		}
		defer out.Close()
		defer os.Remove(workerLocalOutputPath)
		if _, err = io.Copy(out, respBody); err != nil {
			return fmt.Errorf("Unable to copy to file %s: %s", workerLocalOutputPath, err)
		}
	}
	// Call csv_merger.py to merge all results into a single results CSV.
	_, currentFile, _, _ := runtime.Caller(0)
	pathToPyFiles := filepath.Join(
		filepath.Dir((filepath.Dir(filepath.Dir(filepath.Dir(currentFile))))),
		"py")
	pathToCsvMerger := filepath.Join(pathToPyFiles, "csv_merger.py")
	outputFileName := runID + ".output"
	args := []string{
		pathToCsvMerger,
		"--csv_dir=" + localOutputDir,
		"--output_csv_name=" + filepath.Join(localOutputDir, outputFileName),
	}
	if err := util.ExecuteCmd("python", args, []string{}, 1*time.Hour, nil, nil); err != nil {
		return fmt.Errorf("Error running csv_merger.py: %s", err)
	}
	// Copy the output file to Google Storage.
	remoteOutputDir := filepath.Join(util.BenchmarkRunsDir, runID, "consolidated_outputs")
	if err := gs.UploadFile(outputFileName, localOutputDir, remoteOutputDir); err != nil {
		return fmt.Errorf("Unable to upload %s to %s: %s", outputFileName, remoteOutputDir, err)
	}
	return nil
}
