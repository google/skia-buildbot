// run_benchmark_on_workers is an application that runs the specified telemetry
// benchmark on all CT workers and uploads the results to Google Storage. The
// requester is emailed when the task is done.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
)

var (
	emails             = flag.String("emails", "", "The comma separated email addresses to notify when the task is picked up and completes.")
	gaeTaskID          = flag.Int("gae_task_id", -1, "The key of the App Engine task. This task will be updated when the task is completed.")
	pagesetType        = flag.String("pageset_type", "", "The type of pagesets to use. Eg: 10k, Mobile10k, All.")
	chromiumBuild      = flag.String("chromium_build", "", "The chromium build to use for this capture_archives run.")
	benchmarkName      = flag.String("benchmark_name", "", "The telemetry benchmark to run on the workers.")
	benchmarkExtraArgs = flag.String("benchmark_extra_args", "", "The extra arguments that are passed to the specified benchmark.")
	browserExtraArgs   = flag.String("browser_extra_args", "", "The extra arguments that are passed to the browser while running the benchmark.")
	repeatBenchmark    = flag.Int("repeat_benchmark", 1, "The number of times the benchmark should be repeated. For skpicture_printer benchmark this value is always 1.")
	targetPlatform     = flag.String("target_platform", util.PLATFORM_ANDROID, "The platform the benchmark will run on (Android / Linux).")
	runID              = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	tryserverRun       = flag.Bool("tryserver_run", false, "Whether this script is run as part of the Chromium tryserver. If true then emails are not sent out, the webapp is not updated and the output directory is not deleted.")

	taskCompletedSuccessfully = false
	outputRemoteLink          = util.MASTER_LOGSERVER_LINK
)

func sendEmail(recipients []string) {
	// Send completion email.
	emailSubject := fmt.Sprintf("Cluster telemetry benchmark task has completed (%s)", *runID)
	failureHtml := ""
	if !taskCompletedSuccessfully {
		emailSubject += " with failures"
		failureHtml = util.FailureEmailHtml
	}
	bodyTemplate := `
	The Cluster telemetry %s benchmark task on %s pageset has completed.<br/>
	%s
	The output of your script is available <a href='%s'>here</a>.<br/><br/>
	You can schedule more runs <a href="%s">here</a>.<br/><br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, *benchmarkName, *pagesetType, failureHtml, outputRemoteLink, util.BenchmarkTasksWebapp)
	if err := util.SendEmail(recipients, emailSubject, emailBody); err != nil {
		glog.Errorf("Error while sending email: %s", err)
		return
	}
}

func updateWebappTask() {
	extraData := map[string]string{
		"output_link": outputRemoteLink,
	}
	if err := util.UpdateWebappTask(*gaeTaskID, util.UpdateBenchmarkTasksWebapp, extraData); err != nil {
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
	if !*tryserverRun {
		util.SendTaskStartEmail(emailsArr, "Run benchmark")
		// Ensure webapp is updated and completion email is sent even if task
		// fails.
		defer updateWebappTask()
		defer sendEmail(emailsArr)
	}
	// Cleanup tmp files after the run.
	defer util.CleanTmpDir()
	// Finish with glog flush and how long the task took.
	defer util.TimeTrack(time.Now(), "Running benchmark task on workers")
	defer glog.Flush()

	if *pagesetType == "" {
		glog.Error("Must specify --pageset_type")
		return
	}
	if *chromiumBuild == "" {
		glog.Error("Must specify --chromium_build")
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
		glog.Error(err)
		return
	}

	// Run the run_benchmark script on all workers.
	runBenchmarkCmdTemplate := "DISPLAY=:0 run_benchmark --worker_num={{.WorkerNum}} --log_dir={{.LogDir}} " +
		"--pageset_type={{.PagesetType}} --chromium_build={{.ChromiumBuild}} --run_id={{.RunID}} " +
		"--benchmark_name={{.BenchmarkName}} --benchmark_extra_args=\"{{.BenchmarkExtraArgs}}\" " +
		"--browser_extra_args=\"{{.BrowserExtraArgs}}\" --repeat_benchmark={{.RepeatBenchmark}} --target_platform={{.TargetPlatform}};"
	runBenchmarkTemplateParsed := template.Must(template.New("run_benchmark_cmd").Parse(runBenchmarkCmdTemplate))
	benchmarkCmdBytes := new(bytes.Buffer)
	runBenchmarkTemplateParsed.Execute(benchmarkCmdBytes, struct {
		WorkerNum          string
		LogDir             string
		PagesetType        string
		ChromiumBuild      string
		RunID              string
		BenchmarkName      string
		BenchmarkExtraArgs string
		BrowserExtraArgs   string
		RepeatBenchmark    int
		TargetPlatform     string
	}{
		WorkerNum:          util.WORKER_NUM_KEYWORD,
		LogDir:             util.GLogDir,
		PagesetType:        *pagesetType,
		ChromiumBuild:      *chromiumBuild,
		RunID:              *runID,
		BenchmarkName:      *benchmarkName,
		BenchmarkExtraArgs: *benchmarkExtraArgs,
		BrowserExtraArgs:   *browserExtraArgs,
		RepeatBenchmark:    *repeatBenchmark,
		TargetPlatform:     *targetPlatform,
	})
	cmd := []string{
		fmt.Sprintf("cd %s;", util.CtTreeDir),
		"git pull;",
		"make all;",
		// The main command that runs run_benchmark on all workers.
		benchmarkCmdBytes.String(),
	}
	// Setting a 2 day timeout since it may take a while to capture 1M SKPs.
	if _, err := util.SSH(strings.Join(cmd, " "), util.Slaves, 2*24*time.Hour); err != nil {
		glog.Errorf("Error while running cmd %s: %s", cmd, err)
		return
	}

	// If "--output-format=csv" was specified then merge all CSV files and upload.
	if strings.Contains(*benchmarkExtraArgs, "--output-format=csv") {
		localOutputDir := filepath.Join(util.StorageDir, util.BenchmarkRunsDir, *runID)
		os.MkdirAll(localOutputDir, 0700)
		if !*tryserverRun {
			defer os.RemoveAll(localOutputDir)
		}
		// Copy outputs from all slaves locally.
		for i := 0; i < util.NUM_WORKERS; i++ {
			workerNum := i + 1
			workerLocalOutputPath := filepath.Join(localOutputDir, fmt.Sprintf("slave%d", workerNum)+".csv")
			workerRemoteOutputPath := filepath.Join(util.BenchmarkRunsDir, *runID, fmt.Sprintf("slave%d", workerNum), "outputs", *runID+".output")
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
				glog.Errorf("Unable to create file %s: %s", workerLocalOutputPath, err)
				return
			}
			defer out.Close()
			defer os.Remove(workerLocalOutputPath)
			if _, err = io.Copy(out, respBody); err != nil {
				glog.Errorf("Unable to copy to file %s: %s", workerLocalOutputPath, err)
				return
			}
		}
		// Call csv_merger.py to merge all results into a single results CSV.
		_, currentFile, _, _ := runtime.Caller(0)
		pathToPyFiles := filepath.Join(
			filepath.Dir((filepath.Dir(filepath.Dir(filepath.Dir(currentFile))))),
			"py")
		pathToCsvMerger := filepath.Join(pathToPyFiles, "csv_merger.py")
		outputFileName := *runID + ".output"
		args := []string{
			pathToCsvMerger,
			"--csv_dir=" + localOutputDir,
			"--output_csv_name=" + filepath.Join(localOutputDir, outputFileName),
		}
		if err := util.ExecuteCmd("python", args, []string{}, 1*time.Hour, nil, nil); err != nil {
			glog.Errorf("Error running csv_merger.py: %s", err)
			return
		}
		// Copy the output file to Google Storage.
		remoteOutputDir := filepath.Join(util.BenchmarkRunsDir, *runID, "consolidated_outputs")
		outputRemoteLink = util.GS_HTTP_LINK + filepath.Join(util.GS_BUCKET_NAME, remoteOutputDir, outputFileName)
		if err := gs.UploadFile(outputFileName, localOutputDir, remoteOutputDir); err != nil {
			glog.Errorf("Unable to upload %s to %s: %s", outputFileName, remoteOutputDir, err)
			return
		}
	}

	taskCompletedSuccessfully = true
}
