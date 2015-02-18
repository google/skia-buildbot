// fix_archives_on_workers is an application that validates the webpage archives
// of the specified pageset type on all workers and deletes the archives which
// are found to deliver inconsistent results.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/skia-dev/glog"
	"skia.googlesource.com/buildbot.git/ct/go/util"
	"skia.googlesource.com/buildbot.git/go/common"
)

var (
	emails                        = flag.String("emails", "", "The comma separated email addresses to notify when the task is picked up and completes.")
	pagesetType                   = flag.String("pageset_type", "", "The type of pagesets to use. Eg: 10k, Mobile10k, All.")
	chromiumBuild                 = flag.String("chromium_build", "", "The chromium build to use for this capture_archives run.")
	repeatBenchmark               = flag.Int("repeat_benchmark", 1, "The number of times the benchmark should be repeated.")
	runID                         = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	benchmarkName                 = flag.String("benchmark_name", util.BENCHMARK_REPAINT, "The telemetry benchmark to run on all workers.")
	benchmarkHeaderToCheck        = flag.String("benchmark_header_to_check", "mean_frame_time (ms)", "The benchmark header this task will validate.")
	deletePageset                 = flag.Bool("delete_pageset", true, "If an archive is found to be inconsistent then delete it's corresponding pageset.")
	percentageChangeThreshold     = flag.Float64("perc_change_threshold", 5, "An archive is considered inconsistent if the percentage changes are beyond this threshold.")
	resourceMissingCountThreshold = flag.Int("res_missing_count_threshold", 50, "An archive is considered inconsistent if the number of missing resources is beyond this threshold.")

	taskCompletedSuccessfully = false
	outputRemoteLink          = util.MASTER_LOGSERVER_LINK
)

func sendEmail(recipients []string) {
	// Send completion email.
	emailSubject := fmt.Sprintf("Cluster telemetry fix archives task has completed (%s)", *runID)
	failureHtml := ""
	if !taskCompletedSuccessfully {
		emailSubject += " with failures"
		failureHtml = util.FailureEmailHtml
	}
	bodyTemplate := `
	The Cluster telemetry fix archives task on %s pageset has completed.<br/>
	%s
	The output of your script is available <a href='%s'>here</a>.<br/><br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, *pagesetType, failureHtml, outputRemoteLink)
	if err := util.SendEmail(recipients, emailSubject, emailBody); err != nil {
		glog.Errorf("Error while sending email: %s", err)
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
	util.SendTaskStartEmail(emailsArr, "Fix archives")
	// Ensure webapp is updated and completion email is sent even if task
	// fails.
	defer sendEmail(emailsArr)

	// Cleanup tmp files after the run.
	defer util.CleanTmpDir()
	// Finish with glog flush and how long the task took.
	defer util.TimeTrack(time.Now(), "Running fix archives task on workers")
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

	// Run the fix_archives script on all workers.
	fixArchivesCmdTemplate := "DISPLAY=:0 fix_archives --worker_num={{.WorkerNum}} --log_dir={{.LogDir}} " +
		"--pageset_type={{.PagesetType}} --chromium_build={{.ChromiumBuild}} --run_id={{.RunID}} " +
		"--repeat_benchmark={{.RepeatBenchmark}} --benchmark_name={{.BenchmarkName}} " +
		"--benchmark_header_to_check=\"{{.BenchmarkHeaderToCheck}}\" --delete_pageset={{.DeletePageset}} " +
		"--perc_change_threshold={{.PercentageChangeThreshold}} --res_missing_count_threshold={{.ResourceMissingCountThreshold}};"
	fixArchivesTemplateParsed := template.Must(template.New("fix_archives_cmd").Parse(fixArchivesCmdTemplate))
	fixArchivesCmdBytes := new(bytes.Buffer)
	fixArchivesTemplateParsed.Execute(fixArchivesCmdBytes, struct {
		WorkerNum                     string
		LogDir                        string
		PagesetType                   string
		ChromiumBuild                 string
		RunID                         string
		RepeatBenchmark               int
		BenchmarkName                 string
		BenchmarkHeaderToCheck        string
		DeletePageset                 bool
		PercentageChangeThreshold     float64
		ResourceMissingCountThreshold int
	}{
		WorkerNum:                     util.WORKER_NUM_KEYWORD,
		LogDir:                        util.GLogDir,
		PagesetType:                   *pagesetType,
		ChromiumBuild:                 *chromiumBuild,
		RunID:                         *runID,
		RepeatBenchmark:               *repeatBenchmark,
		BenchmarkName:                 *benchmarkName,
		BenchmarkHeaderToCheck:        *benchmarkHeaderToCheck,
		DeletePageset:                 *deletePageset,
		PercentageChangeThreshold:     *percentageChangeThreshold,
		ResourceMissingCountThreshold: *resourceMissingCountThreshold,
	})
	cmd := []string{
		fmt.Sprintf("cd %s;", util.CtTreeDir),
		"git pull;",
		"make all;",
		// The main command that runs fix_archives on all workers.
		fixArchivesCmdBytes.String(),
	}
	// Setting a 1 day timeout since it may take a while to validate archives.
	if _, err := util.SSH(strings.Join(cmd, " "), util.Slaves, 1*24*time.Hour); err != nil {
		glog.Errorf("Error while running cmd %s: %s", cmd, err)
		return
	}

	taskCompletedSuccessfully = true
}
