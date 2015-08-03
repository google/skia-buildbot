// run_skia_correctness_on_workers is an application that runs Skia correctness
// over the specified SKP repository on all CT workers and uploads the results to
// Google Storage. The requester is emailed when the task is done.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/ct/go/frontend"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
	skutil "go.skia.org/infra/go/util"
)

var (
	emails             = flag.String("emails", "", "The comma separated email addresses to notify when the task is picked up and completes.")
	gaeTaskID          = flag.Int64("gae_task_id", -1, "The key of the App Engine task. This task will be updated when the task is completed.")
	pagesetType        = flag.String("pageset_type", "", "The type of pagesets to use. Eg: 10k, Mobile10k, All.")
	chromiumBuild      = flag.String("chromium_build", "", "The chromium build to use for this capture_archives run.")
	renderPicturesArgs = flag.String("render_pictures_args", "", "The arguments to pass to render_pictures binary.")
	gpuNoPatchRun      = flag.Bool("gpu_nopatch_run", false, "Whether to build with GPU for the nopatch run.")
	gpuWithPatchRun    = flag.Bool("gpu_withpatch_run", false, "Whether to build with GPU for the withpatch run.")
	runID              = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")

	taskCompletedSuccessfully = false

	htmlOutputLink = util.MASTER_LOGSERVER_LINK
	skiaPatchLink  = util.MASTER_LOGSERVER_LINK
)

func sendEmail(recipients []string) {
	// Send completion email.
	emailSubject := fmt.Sprintf("Cluster telemetry Skia correctness task has completed (%s)", *runID)
	failureHtml := ""
	if !taskCompletedSuccessfully {
		emailSubject += " with failures"
		failureHtml = util.FailureEmailHtml
	}
	bodyTemplate := `
	The skia correctness task on %s pageset has completed.<br/>
	%s
	The HTML output with differences between the base run and the patch run is <a href='%s'>here</a>.<br/>
	The patch you specified is <a href='%s'>here</a>.<br/><br/>
	You can schedule more runs <a href='%s'>here</a>.
	<br/><br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, *pagesetType, failureHtml, htmlOutputLink, skiaPatchLink, frontend.SkiaCorrectnessTasksWebapp)
	if err := util.SendEmail(recipients, emailSubject, emailBody); err != nil {
		glog.Errorf("Error while sending email: %s", err)
		return
	}
}

func updateWebappTask() {
	extraData := map[string]string{
		"patch_link":         skiaPatchLink,
		"slave1_output_link": util.WORKERS_LOGSERVER_LINK,
		"html_output_link":   htmlOutputLink,
	}
	if err := frontend.UpdateWebappTask(*gaeTaskID, frontend.UpdateSkiaCorrectnessTasksWebapp, extraData); err != nil {
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
	skutil.LogErr(util.SendTaskStartEmail(emailsArr, "Skia correctness"))
	// Ensure webapp is updated and email is sent even if task fails.
	defer updateWebappTask()
	defer sendEmail(emailsArr)
	// Cleanup tmp files after the run.
	defer util.CleanTmpDir()
	// Finish with glog flush and how long the task took.
	defer util.TimeTrack(time.Now(), "Running skia correctness task on workers")
	defer glog.Flush()

	if *pagesetType == "" {
		glog.Error("Must specify --pageset_type")
		return
	}
	if *chromiumBuild == "" {
		glog.Error("Must specify --chromium_build")
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
	remoteOutputDir := filepath.Join(util.SkiaCorrectnessRunsDir, *runID)

	// Copy the patch to Google Storage.
	patchName := *runID + ".patch"
	patchRemoteDir := filepath.Join(remoteOutputDir, "patches")
	if err := gs.UploadFile(patchName, os.TempDir(), patchRemoteDir); err != nil {
		glog.Errorf("Could not upload %s to %s: %s", patchName, patchRemoteDir, err)
		return
	}
	skiaPatchLink = util.GS_HTTP_LINK + filepath.Join(util.GS_BUCKET_NAME, patchRemoteDir, patchName)

	// Run the run_skia_correctness script on all workers.
	runSkiaCorrCmdTemplate := "DISPLAY=:0 run_skia_correctness --worker_num={{.WorkerNum}} --log_dir={{.LogDir}} " +
		"--pageset_type={{.PagesetType}} --chromium_build={{.ChromiumBuild}} --run_id={{.RunID}} " +
		"--render_pictures_args=\"{{.RenderPicturesArgs}}\" --gpu_nopatch_run={{.GpuNoPatchRun}} " +
		"--gpu_withpatch_run={{.GpuWithPatchRun}};"
	runSkiaCorrTemplateParsed := template.Must(template.New("run_skia_correctness_cmd").Parse(runSkiaCorrCmdTemplate))
	runSkiaCorrCmdBytes := new(bytes.Buffer)
	if err := runSkiaCorrTemplateParsed.Execute(runSkiaCorrCmdBytes, struct {
		WorkerNum          string
		LogDir             string
		PagesetType        string
		ChromiumBuild      string
		RunID              string
		RenderPicturesArgs string
		GpuNoPatchRun      string
		GpuWithPatchRun    string
	}{
		WorkerNum:          util.WORKER_NUM_KEYWORD,
		LogDir:             util.GLogDir,
		PagesetType:        *pagesetType,
		ChromiumBuild:      *chromiumBuild,
		RunID:              *runID,
		RenderPicturesArgs: *renderPicturesArgs,
		GpuNoPatchRun:      strconv.FormatBool(*gpuNoPatchRun),
		GpuWithPatchRun:    strconv.FormatBool(*gpuWithPatchRun),
	}); err != nil {
		glog.Errorf("Failed to execute template: %s", err)
		return
	}
	cmd := []string{
		fmt.Sprintf("cd %s;", util.CtTreeDir),
		"git pull;",
		"make all;",
		// The main command that runs run_skia_correctness on all workers.
		runSkiaCorrCmdBytes.String(),
	}
	if _, err := util.SSH(strings.Join(cmd, " "), util.Slaves, 4*time.Hour); err != nil {
		glog.Errorf("Error while running cmd %s: %s", cmd, err)
		return
	}

	localOutputDir := filepath.Join(util.StorageDir, util.SkiaCorrectnessRunsDir, *runID)
	localSummariesDir := filepath.Join(localOutputDir, "summaries")
	skutil.MkdirAll(localSummariesDir, 0700)
	defer skutil.RemoveAll(filepath.Join(util.StorageDir, util.SkiaCorrectnessRunsDir))
	// Copy outputs from all slaves locally.
	for i := 0; i < util.NUM_WORKERS; i++ {
		workerNum := i + 1
		workerLocalOutputPath := filepath.Join(localSummariesDir, fmt.Sprintf("slave%d", workerNum)+".json")
		workerRemoteOutputPath := filepath.Join(remoteOutputDir, fmt.Sprintf("slave%d", workerNum), fmt.Sprintf("slave%d", workerNum)+".json")
		respBody, err := gs.GetRemoteFileContents(workerRemoteOutputPath)
		if err != nil {
			glog.Errorf("Could not fetch %s: %s", workerRemoteOutputPath, err)
			// TODO(rmistry): Should we instead return here? We can only return
			// here if all 100 slaves reliably run without any failures which they
			// really should.
			continue
		}
		defer skutil.Close(respBody)
		out, err := os.Create(workerLocalOutputPath)
		if err != nil {
			glog.Errorf("Unable to create file %s: %s", workerLocalOutputPath, err)
			return
		}
		defer skutil.Close(out)
		defer skutil.Remove(workerLocalOutputPath)
		if _, err = io.Copy(out, respBody); err != nil {
			glog.Errorf("Unable to copy to file %s: %s", workerLocalOutputPath, err)
			return
		}
	}

	// Call json_summary_combiner.py to merge all results into a single results CSV.
	_, currentFile, _, _ := runtime.Caller(0)
	pathToPyFiles := filepath.Join(
		filepath.Dir((filepath.Dir(filepath.Dir(filepath.Dir(currentFile))))),
		"py")
	pathToJsonCombiner := filepath.Join(pathToPyFiles, "json_summary_combiner.py")
	localHtmlDir := filepath.Join(localOutputDir, "html")
	remoteHtmlDir := filepath.Join(remoteOutputDir, "html")
	baseHtmlLink := util.GS_HTTP_LINK + filepath.Join(util.GS_BUCKET_NAME, remoteHtmlDir) + "/"
	htmlOutputLink = baseHtmlLink + "index.html"
	skutil.MkdirAll(localHtmlDir, 0700)
	args := []string{
		pathToJsonCombiner,
		"--json_summaries_dir=" + localSummariesDir,
		"--output_html_dir=" + localHtmlDir,
		"--absolute_url=" + baseHtmlLink,
		"--render_pictures_args=" + *renderPicturesArgs,
		"--nopatch_gpu=" + strconv.FormatBool(*gpuNoPatchRun),
		"--withpatch_gpu=" + strconv.FormatBool(*gpuWithPatchRun),
	}
	if err := util.ExecuteCmd("python", args, []string{}, 1*time.Hour, nil, nil); err != nil {
		glog.Errorf("Error running json_summary_combiner.py: %s", err)
		return
	}

	// Copy the HTML files to Google Storage.
	if err := gs.UploadDir(localHtmlDir, remoteHtmlDir, true); err != nil {
		glog.Errorf("Could not upload %s to %s: %s", localHtmlDir, remoteHtmlDir, err)
		return
	}

	taskCompletedSuccessfully = true
}
