/*
	Funnels data from Google-internal sources into the buildbot and task scheduler databases.

  Android builds continuously in tradefed at the master branch with the latest roll of Skia. There
  is also another branch 'git_master-skia' which contains the HEAD of Skia instead of the last roll
  of Skia. Using the Android Build APIs, this application continuously ingests builds from
  git_master-skia and pushes them into the buildbot database so they appear on status.skia.org.
  Since the target names may contain sensitive information they are obfuscated when pushed to the
  buildbot database.

  Build info can also be POSTed to this application to directly ingest builds from Google3.

  This application also contains a redirector that will take links to the obfuscated target name and
  build and return a link to the internal page with the detailed build info.
*/
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"go.skia.org/infra/go/androidbuild"
	androidbuildinternal "go.skia.org/infra/go/androidbuildinternal/v2beta1"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/go/webhook"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/remote_db"
	storage "google.golang.org/api/storage/v1"
)

const (
	FAKE_MASTER     = "client.skia.fake_internal"
	FAKE_BUILDSLAVE = "fake_internal_buildslave"

	// SKIA_BRANCH is the name of the git branch we sync Skia to regularly.
	SKIA_BRANCH = "git_master-skia"

	// MASTER is the git master branch we may check builds against.
	MASTER = "git_master"

	GOOGLE3_AUTOROLLER_TARGET_NAME = "Google3-Autoroller"
)

// flags
var (
	buildbotDbHost     = flag.String("buildbot_db_host", "skia-datahopper2:8000", "Where the Skia buildbot database is hosted.")
	taskSchedulerUrl   = flag.String("task_scheduler_url", "https://skia-task-scheduler:8000/json/task", "URL for the task scheduler JSON API POST/PUT handlers.")
	taskSchedulerDbUrl = flag.String("task_db_url", "http://skia-task-scheduler:8008/db/", "Where the Skia task scheduler database is hosted.")
	port               = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	workdir            = flag.String("workdir", ".", "Working directory used by data processors.")
	local              = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	targetList         = flag.String("targets", "", "The targets to monitor, a space separated list.")
	codenameDbDir      = flag.String("codename_db_dir", "codenames", "The location of the leveldb database that holds the mappings between targets and their codenames.")
	period             = flag.Duration("period", 5*time.Minute, "The time between ingestion runs.")

	influxHost     = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxUser     = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	influxPassword = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxDatabase = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")
)

var (
	// terminal_build_status are the tradefed build status's that mean the build is done.
	terminal_build_status = []string{"complete", "error"}

	// codenameDB is a leveldb to store codenames and their deobfuscated counterparts.
	codenameDB *leveldb.DB

	// buildbotDB is a buildbot.DB instance used for ingesting build data.
	buildbotDB buildbot.DB

	// taskDB is a remote db.TaskReader used for looking up previously-added tasks by ID.
	taskDB db.TaskReader

	// repos provides information about the Git repositories in workdir.
	repos repograph.Map

	// tradefedLiveness is a metric for the time since last successful run through step().
	tradefedLiveness = metrics2.NewLiveness("android-internal-ingest", nil)

	// noCodenameTargets is a set of targets that do not need to be obfuscated.
	noCodenameTargets = map[string]bool{
		GOOGLE3_AUTOROLLER_TARGET_NAME: true,
	}

	// ingestBuildWebhookCodenames is the set of codenames we expect in ingestBuildHandler.
	ingestBuildWebhookCodenames = map[string]bool{
		GOOGLE3_AUTOROLLER_TARGET_NAME: true,
	}

	// ingestBuildWebhookLiveness maps a target codename to a metric for the time since last
	// successful build ingestion.
	ingestBuildWebhookLiveness = map[string]metrics2.Liveness{}

	httpClient = httputils.NewTimeoutClient()
)

// isFinished returns true if the Build has finished running.
func isFinished(b *androidbuildinternal.Build) bool {
	return util.In(b.BuildAttemptStatus, terminal_build_status)
}

// buildFromCommit builds a buildbot.Build from the commit and the info
// returned from the Apiary API.  It also returns a key that uniqely identifies
// this build.
func buildFromCommit(build *androidbuildinternal.Build, commit *vcsinfo.ShortCommit) (string, *buildbot.Build) {
	codename := util.StringToCodeName(build.Target.Name)
	key := build.Branch + ":" + build.Target.Name + ":" + build.BuildId
	b := &buildbot.Build{
		Builder:     codename,
		Master:      FAKE_MASTER,
		Number:      0,
		BuildSlave:  FAKE_BUILDSLAVE,
		Branch:      "master",
		Commits:     nil,
		GotRevision: commit.Hash,
		Properties: [][]interface{}{
			[]interface{}{"androidinternal_buildid", build.BuildId, "tradefed"},
			[]interface{}{"buildbotURL", "https://internal.skia.org/", "tradefed"},
		},
		PropertiesStr: "",
		Results:       buildbot.BUILDBOT_FAILURE,
		Steps:         nil,
		Started:       util.UnixMillisToTime(build.CreationTimestamp),
		Comments:      nil,
		Repository:    common.REPO_SKIA,
	}
	// Fill in PropertiesStr based on Properties.
	props, err := json.Marshal(b.Properties)
	if err == nil {
		b.PropertiesStr = string(props)
	} else {
		sklog.Errorf("Failed to encode properties: %s", err)
	}
	if build.Successful {
		b.Results = buildbot.BUILDBOT_SUCCESS
	}
	// Only fill in Finished if the build has completed.
	if isFinished(build) {
		b.Finished = time.Now().UTC()
	}
	return key, b
}

// brokenOnMaster returns true if recent builds on master near the given buildID are unsuccessful.
func brokenOnMaster(buildService *androidbuildinternal.Service, target, buildID string) bool {
	r, err := buildService.Build.List().Branch(MASTER).BuildType("submitted").StartBuildId(buildID).Target(target).MaxResults(4).Do()
	if err != nil {
		return false
	}
	for _, b := range r.Builds {
		if isFinished(b) && !b.Successful {
			return true
		}
	}
	return false
}

// ingestBuild encapsulates many of the steps of ingesting a build:
//   - Record the mapping between the codename (build.Builder) and the internal target name.
//   - If no matching build exists, assign a new build number for this build and insert it.
//   - Otherwise, update the existing build to match the given build.
func ingestBuild(build *buildbot.Build, commitHash, target string) error {
	// Store build.Builder (the codename) with its pair build.Target.Name in a local leveldb to serve redirects.
	if err := codenameDB.Put([]byte(build.Builder), []byte(target), nil); err != nil {
		sklog.Errorf("Failed to write codename to data store: %s", err)
	}

	buildNumber, err := buildbotDB.GetBuildNumberForCommit(build.Master, build.Builder, commitHash)
	if err != nil {
		return fmt.Errorf("Failed to find the build in the database: %s", err)
	}
	sklog.Infof("GetBuildNumberForCommit at hash: %s returned %d", commitHash, buildNumber)
	var existingBuild *buildbot.Build
	if buildNumber != -1 {
		existingBuild, err = buildbotDB.GetBuildFromDB(build.Master, build.Builder, buildNumber)
		if err != nil {
			return fmt.Errorf("Failed to retrieve build from database: %s", err)
		}
	}
	taskId := ""
	if existingBuild == nil {
		// This is a new build we've never seen before, so add it to the buildbot database.

		// TODO(benjaminwagner): This logic won't work well for concurrent requests. Revisit
		// after borenet's "giant datahopper change."

		// First calculate a new unique build.Number.
		number, err := buildbotDB.GetMaxBuildNumber(build.Master, build.Builder)
		if err != nil {
			return fmt.Errorf("Failed to find next build number: %s", err)
		}
		build.Number = number + 1
		sklog.Infof("Writing new build to the database: %s %d", build.Builder, build.Number)
	} else {
		// If the state of the build has changed then write it to the buildbot database.
		build.Number = buildNumber
		// Retrieve Task ID from existingBuild.
		if id, err := existingBuild.GetStringProperty("taskId"); err == nil {
			taskId = id
		}
		sklog.Infof("Writing updated build to the database: %s %d", build.Builder, build.Number)
	}
	if taskId == "" {
		taskId, err = addTask(build)
		if err != nil {
			return err
		}
	} else {
		if err := updateTask(taskId, build); err != nil {
			return err
		}
	}
	// Save task ID in build.Properties.
	build.Properties = append(build.Properties, []interface{}{"taskId", taskId, "datahopper_internal"})
	props, err := json.Marshal(build.Properties)
	if err == nil {
		build.PropertiesStr = string(props)
	} else {
		sklog.Errorf("Failed to encode properties: %s", err)
	}
	if err := buildbot.IngestBuild(buildbotDB, build, repos); err != nil {
		return fmt.Errorf("Failed to ingest build: %s", err)
	}
	return nil
}

// taskStatus determines a db.TaskStatus equivalent to build's current status.
func taskStatus(build *buildbot.Build) db.TaskStatus {
	if build.IsFinished() {
		switch build.Results {
		case buildbot.BUILDBOT_SUCCESS, buildbot.BUILDBOT_WARNINGS:
			return db.TASK_STATUS_SUCCESS
		case buildbot.BUILDBOT_FAILURE:
			return db.TASK_STATUS_FAILURE
		case buildbot.BUILDBOT_EXCEPTION:
			return db.TASK_STATUS_MISHAP
		}
	} else if build.IsStarted() {
		return db.TASK_STATUS_RUNNING
	}
	return db.TASK_STATUS_PENDING
}

// doTaskRequest adds/updates task using a POST/PUT request to Task Scheduler.
func doTaskRequest(method string, task *db.Task) error {
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("Failed to marshal task %v: %s", task, err)
	}
	req, err := webhook.NewRequest(method, *taskSchedulerUrl, data)
	if err != nil {
		return fmt.Errorf("Could not create HTTP request: %s", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s request failed: %s", method, err)
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != 200 {
		response, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("Request Failed; response status code was %d: %s", resp.StatusCode, response)
	}
	var newTask *db.Task
	if err := json.NewDecoder(resp.Body).Decode(&newTask); err != nil {
		return fmt.Errorf("Unable to parse response from %s: %s", *taskSchedulerUrl, err)
	}
	*task = *newTask
	return nil
}

// setTaskFromBuild sets relevant fields of task based on build.
func setTaskFromBuild(task *db.Task, build *buildbot.Build) {
	task.Finished = build.Finished
	task.Name = build.Builder
	task.Properties = map[string]string{
		"buildbotBuilder": build.Builder,
		"buildbotMaster":  build.Master,
		"buildbotNumber":  strconv.Itoa(build.Number),
		"url":             fmt.Sprintf("https://internal.skia.org/builders/%s/builds/%d", build.Builder, build.Number),
	}
	task.Repo = build.Repository
	task.Revision = build.GotRevision
	task.Started = build.Started
	task.Status = taskStatus(build)
}

// addTask adds a task corresponding to build to the Task Scheduler DB, and returns the task ID.
func addTask(build *buildbot.Build) (string, error) {
	task := &db.Task{}
	setTaskFromBuild(task, build)
	sklog.Infof("Adding task corresponding to build %s %d: %v", build.Builder, build.Number, task)
	if err := doTaskRequest(http.MethodPost, task); err != nil {
		return "", err
	}
	return task.Id, nil
}

// updateTask modifies the task with the given id based on build.
func updateTask(id string, build *buildbot.Build) error {
	task, err := taskDB.GetTaskById(id)
	if err != nil {
		return err
	} else if task == nil {
		return fmt.Errorf("Can not find task %s for build %s %d!", id, build.Builder, build.Number)
	}
	orig := task.Copy()
	setTaskFromBuild(task, build)
	if reflect.DeepEqual(orig, task) {
		sklog.Infof("No changes for task %s corresponding to build %s %d", id, build.Builder, build.Number)
		return nil
	}

	sklog.Infof("Updating task %s corresponding to build %s %d", id, build.Builder, build.Number)
	return doTaskRequest(http.MethodPut, task)
}

// step does a single step in ingesting builds from tradefed and pushing the results into the buildbot database.
func step(targets []string, buildService *androidbuildinternal.Service) {
	sklog.Infof("step: Begin")

	if err := repos.Update(); err != nil {
		sklog.Errorf("Failed to update repos: %s", err)
		return
	}
	// Loop over every target and look for skia commits in the builds.
	for _, target := range targets {
		r, err := buildService.Build.List().Branch(SKIA_BRANCH).BuildType("submitted").Target(target).ExtraFields("changeInfo").MaxResults(40).Do()
		if err != nil {
			sklog.Errorf("Failed to load internal builds: %v", err)
			continue
		}
		// Iterate over the builds in reverse order so we ingest the earlier Git
		// hashes first and the more recent Git hashes later.
		for i := len(r.Builds) - 1; i >= 0; i-- {
			b := r.Builds[i]
			commits := androidbuild.CommitsFromChanges(b.Changes)
			sklog.Infof("Commits: %#v", commits)
			if len(commits) > 0 {
				// Only look at the first commit in the list. The commits always appear in reverse chronological order, so
				// the 0th entry is the most recent commit.
				c := commits[0]
				// Create a buildbot.Build from the build info.
				key, build := buildFromCommit(b, c)
				sklog.Infof("Key: %s Hash: %s", key, c.Hash)

				// If this was a failure then we need to check that there is a
				// mirror failure on the main branch, at which point we will say
				// that this is a warning (appears green on status).
				if build.Results == buildbot.BUILDBOT_FAILURE && brokenOnMaster(buildService, target, b.BuildId) {
					build.Results = buildbot.BUILDBOT_WARNINGS
				}
				if err := ingestBuild(build, c.Hash, target); err != nil {
					sklog.Error(err)
				}
			}
			tradefedLiveness.Reset()
		}
	}
}

// indexHandler handles the GET of the main page.
func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	if _, err := w.Write([]byte("Nothing to see here.")); err != nil {
		sklog.Errorf("Failed to write response: %s", err)
	}
}

const (
	LAUNCH_CONTROL_BUILD_REDIRECT_TEMPLATE = `<!DOCTYPE html>
<html>
  <head><title>Redirect</title></head>
  <body>
  <p>You are being redirected to Launch Control. The un-obfuscated target name for <b>%s</b> is <b>%s</b>.</p>
  <p><a href='https://android-build-uber.corp.google.com/builds.html?branch=git_master-skia&lower_limit=%s&upper_limit=%s'>master-skia:%s</a></p>
  <p><a href='https://android-build-uber.corp.google.com/builds.html?branch=git_master&lower_limit=%s&upper_limit=%s'>master:%s</a></p>
  </body>
</html>
`

	LAUNCH_CONTROL_BUILDER_REDIRECT_TEMPLATE = `<!DOCTYPE html>
<html>
  <head><title>Redirect</title></head>
  <body>
  <p>You are being redirected to Launch Control. The un-obfuscated target name for <b>%s</b> is <b>%s</b>.</p>
  <p><a href='https://android-build-uber.corp.google.com/builds.html?branch=git_master-skia'>master-skia:%s</a></p>
  <p><a href='https://android-build-uber.corp.google.com/builds.html?branch=git_master'>master:%s</a></p>
  </body>
</html>
`

	GOOGLE3_AUTOROLLER_BORGCRON_REDIRECT_TEMPLATE = `<!DOCTYPE html>
<html>
  <head><title>Redirect</title></head>
  <body>
  <p>You are being redirected to the docs for the Google3 Autoroller.</p>
  <p><a href='https://sites.google.com/a/google.com/skia-infrastructure/docs/google3-autoroller'>Google3 Autoroller</a></p>
  </body>
</html>
`

	TEST_RESULTS_REDIRECT_TEMPLATE = `<!DOCTYPE html>
<html>
  <head><title>Redirect</title></head>
  <body>
  <p>You are being redirected to the test results for <b>%s</b>.</p>
  <p><a href='%s'>%s</a></p>
  </body>
</html>
`
)

// redirectHandler handles redirecting to the correct internal build page.
func redirectHandler(w http.ResponseWriter, r *http.Request) {
	if login.LoggedInAs(r) == "" {
		r.Header.Set("Referer", r.URL.String())
		http.Redirect(w, r, login.LoginURL(w, r), 302)
		return
	} else if !login.IsGoogler(r) {
		errStr := "Cannot view; user is not a logged-in Googler."
		httputils.ReportError(w, r, fmt.Errorf(errStr), errStr)
		return
	}
	vars := mux.Vars(r)
	codename := vars["codename"]
	buildNumberStr := vars["buildNumber"]
	target, err := codenameDB.Get([]byte(codename), nil)
	if err != nil {
		httputils.ReportError(w, r, err, "Not a valid target codename.")
		return
	}
	buildNumber, err := strconv.Atoi(buildNumberStr)
	if err != nil {
		httputils.ReportError(w, r, err, "Not a valid build number.")
		return
	}
	build, err := buildbotDB.GetBuildFromDB(FAKE_MASTER, string(codename), buildNumber)
	if err != nil {
		httputils.ReportError(w, r, err, "Could not find a matching build.")
	}
	result := ""
	if id, err := build.GetStringProperty("androidinternal_buildid"); err == nil {
		result = fmt.Sprintf(LAUNCH_CONTROL_BUILD_REDIRECT_TEMPLATE, codename, target, id, id, target, id, id, target)
	} else if link, err := build.GetStringProperty("testResultsLink"); err == nil {
		result = fmt.Sprintf(TEST_RESULTS_REDIRECT_TEMPLATE, target, link, link)
	} else if cl, err := build.GetStringProperty("changeListNumber"); err == nil {
		link = fmt.Sprintf("http://cl/%s", cl)
		result = fmt.Sprintf(TEST_RESULTS_REDIRECT_TEMPLATE, target, link, link)
	}
	if result == "" {
		sklog.Errorf("No redirect for %#v", build)
		httputils.ReportError(w, r, nil, "No redirect for this build.")
		return
	}
	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write([]byte(result)); err != nil {
		httputils.ReportError(w, r, err, "Failed to write response")
	}
}

type Mapping struct {
	Codename string
	Target   string
}

// mappingHandler displays all codename to target mappings.
func mappingHandler(w http.ResponseWriter, r *http.Request) {
	if login.LoggedInAs(r) == "" {
		r.Header.Set("Referer", r.URL.String())
		http.Redirect(w, r, login.LoginURL(w, r), 302)
		return
	} else if !login.IsGoogler(r) {
		errStr := "Cannot view; user is not a logged-in Googler."
		httputils.ReportError(w, r, fmt.Errorf(errStr), errStr)
		return
	}

	iter := codenameDB.NewIterator(nil, nil)
	allMappings := []Mapping{}
	for iter.Next() {
		m := Mapping{Codename: string(iter.Key()), Target: string(iter.Value())}
		allMappings = append(allMappings, m)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(allMappings); err != nil {
		httputils.ReportError(w, r, err, "Failed to marshal mappings.")
		return
	}
}

// builderRedirectHandler handles redirecting to the correct internal builder page.
func builderRedirectHandler(w http.ResponseWriter, r *http.Request) {
	if login.LoggedInAs(r) == "" {
		r.Header.Set("Referer", r.URL.String())
		http.Redirect(w, r, login.LoginURL(w, r), 302)
		return
	} else if !login.IsGoogler(r) {
		errStr := "Cannot view; user is not a logged-in Googler."
		httputils.ReportError(w, r, fmt.Errorf(errStr), errStr)
		return
	}
	vars := mux.Vars(r)
	codename := vars["codename"]
	target, err := codenameDB.Get([]byte(codename), nil)
	if err != nil {
		httputils.ReportError(w, r, err, "Not a valid target codename.")
		return
	}
	w.Header().Set("Content-Type", "text/html")
	response := ""
	if string(target) == GOOGLE3_AUTOROLLER_TARGET_NAME {
		response = GOOGLE3_AUTOROLLER_BORGCRON_REDIRECT_TEMPLATE
	} else {
		response = fmt.Sprintf(LAUNCH_CONTROL_BUILDER_REDIRECT_TEMPLATE, codename, target, target, target)
	}
	if _, err := w.Write([]byte(response)); err != nil {
		sklog.Errorf("Failed to write response: %s", err)
	}
}

// ingestBuildHandler parses the JSON body as a build and ingests it. The request must be
// authenticated via the protocol implemented in the webhook package. The client should retry this
// request several times, because some errors may be temporary.
func ingestBuildHandler(w http.ResponseWriter, r *http.Request) {
	data, err := webhook.AuthenticateRequest(r)
	if err != nil {
		sklog.Errorf("Failed authentication in ingestBuildHandler: %s", err)
		httputils.ReportError(w, r, nil, "Failed authentication.")
		return
	}
	vars := map[string]string{}
	if err := json.Unmarshal(data, &vars); err != nil {
		sklog.Errorf("Failed to parse request: %s", err)
		httputils.ReportError(w, r, nil, "Failed to parse request.")
		return
	}
	target := vars["target"]
	commitHash := vars["commitHash"]
	status := vars["status"]
	if target == "" || commitHash == "" || status == "" {
		httputils.ReportError(w, r, nil, "Missing parameter.")
		return
	}
	cl := vars["changeListNumber"]
	link := vars["testResultsLink"]
	startTimeStr := vars["startTime"]
	finishTimeStr := vars["finishTime"]
	codename := ""
	if noCodenameTargets[target] {
		codename = target
	} else {
		codename = util.StringToCodeName(target)
	}
	if !ingestBuildWebhookCodenames[codename] {
		httputils.ReportError(w, r, nil, fmt.Sprintf("Unrecognized target (mapped to codename %s)", codename))
		return
	}
	buildbotResults, err := buildbot.ParseResultsString(status)
	if err != nil {
		sklog.Errorf("Invalid status parameter: %s", err)
		httputils.ReportError(w, r, nil, "Invalid status parameter.")
		return
	}
	startTime := time.Now().UTC()
	if startTimeStr != "" {
		if t, err := strconv.ParseInt(startTimeStr, 10, 64); err == nil {
			startTime = time.Unix(t, 0).UTC()
		} else {
			sklog.Errorf("Invalid startTime parameter: %s", err)
			httputils.ReportError(w, r, nil, "Invalid startTime parameter.")
			return
		}
	}
	finishTime := time.Now().UTC()
	if finishTimeStr != "" {
		if t, err := strconv.ParseInt(finishTimeStr, 10, 64); err == nil {
			finishTime = time.Unix(t, 0).UTC()
		} else {
			sklog.Errorf("Invalid finishTime parameter: %s", err)
			httputils.ReportError(w, r, nil, "Invalid finishTime parameter.")
			return
		}
	}
	b := &buildbot.Build{
		Builder:     codename,
		Master:      FAKE_MASTER,
		Number:      0,
		BuildSlave:  FAKE_BUILDSLAVE,
		Branch:      "master",
		Commits:     nil,
		GotRevision: commitHash,
		Properties: [][]interface{}{
			[]interface{}{"buildbotURL", "https://internal.skia.org/", "datahopper_internal"},
		},
		PropertiesStr: "",
		Results:       buildbotResults,
		Steps:         nil,
		Started:       startTime,
		Finished:      finishTime,
		Comments:      nil,
		Repository:    common.REPO_SKIA,
	}
	if cl != "" {
		if clNum, err := strconv.Atoi(cl); err == nil {
			b.Properties = append(b.Properties, []interface{}{"changeListNumber", strconv.Itoa(clNum), "datahopper_internal"})
		} else {
			sklog.Errorf("Invalid changeListNumber parameter: %s", err)
			httputils.ReportError(w, r, nil, "Invalid changeListNumber parameter.")
			return
		}
	}
	if link != "" {
		if url, err := url.Parse(link); err == nil {
			b.Properties = append(b.Properties, []interface{}{"testResultsLink", url.String(), "datahopper_internal"})
		} else {
			sklog.Errorf("Invalid testResultsLink parameter: %s", err)
			httputils.ReportError(w, r, nil, "Invalid testResultsLink parameter.")
			return
		}
	}
	// Fill in PropertiesStr based on Properties.
	props, err := json.Marshal(b.Properties)
	if err == nil {
		b.PropertiesStr = string(props)
	} else {
		sklog.Errorf("Failed to encode properties: %s", err)
	}
	if err := repos.Update(); err != nil {
		httputils.ReportError(w, r, err, "Failed to update repos.")
		return
	}
	if err := ingestBuild(b, commitHash, target); err != nil {
		sklog.Errorf("Failed to ingest build: %s", err)
		httputils.ReportError(w, r, nil, "Failed to ingest build.")
		return
	}
	if metric, present := ingestBuildWebhookLiveness[codename]; present {
		metric.Reset()
	}
}

// updateWebhookMetrics updates "oldest-untested-commit-age" metrics for all
// ingestBuildWebhookCodenames. This metric records the age of the oldest commit for which we have
// not ingested a build. Returns an error if any metric was not updated.
func updateWebhookMetrics() error {
	if err := repos.Update(); err != nil {
		return err
	}
	repo, ok := repos[common.REPO_SKIA]
	if !ok {
		return fmt.Errorf("Unknown repo: %s", common.REPO_SKIA)
	}
	for codename, _ := range ingestBuildWebhookCodenames {
		var untestedCommitInfo *repograph.Commit = nil
		if err := repo.Get("master").Recurse(func(c *repograph.Commit) (bool, error) {
			buildNumber, err := buildbotDB.GetBuildNumberForCommit(FAKE_MASTER, codename, c.Hash)
			if err != nil {
				return false, err
			}
			if buildNumber == -1 {
				untestedCommitInfo = c
				// No tests for this commit, check older.
				return true, nil
			}
			return false, nil
		}); err != nil {
			return err
		}

		metric := metrics2.GetInt64Metric("datahopper_internal.ingest-build-webhook.oldest-untested-commit-age", map[string]string{"codename": codename})
		if untestedCommitInfo == nil {
			// There are no untested commits.
			metric.Update(0)
		} else {
			metric.Update(int64(time.Since(untestedCommitInfo.Timestamp).Seconds()))
		}
		break
	}
	return nil
}

// startWebhookMetrics starts a goroutine to run updateWebhookMetrics.
func startWebhookMetrics() {
	// A metric to ensure the other metrics are being updated.
	metricLiveness := metrics2.NewLiveness("ingest-build-webhook-oldest-untested-commit-age-metric", nil)
	go func() {
		for _ = range time.Tick(common.SAMPLE_PERIOD) {
			if err := updateWebhookMetrics(); err != nil {
				sklog.Errorf("Failed to update metrics: %s", err)
				continue
			}
			metricLiveness.Reset()
		}
	}()
}

func main() {
	defer common.LogPanic()

	var err error

	// Global init to initialize glog and parse arguments.
	common.InitWithMetrics2("datahopper_internal", influxHost, influxUser, influxPassword, influxDatabase, local)

	if !*local {
		*targetList = metadata.Must(metadata.ProjectGet("datahopper_internal_targets"))
	}
	targets := strings.Split(*targetList, " ")
	sklog.Infof("Targets: %#v", targets)

	codenameDB, err = leveldb.OpenFile(*codenameDbDir, nil)
	if err != nil && errors.IsCorrupted(err) {
		codenameDB, err = leveldb.RecoverFile(*codenameDbDir, nil)
	}
	if err != nil {
		sklog.Fatalf("Failed to open codename leveldb at %s: %s", *codenameDbDir, err)
	}
	// Initialize the buildbot database.
	if *local {
		buildbotDB, err = buildbot.NewLocalDB(path.Join(*workdir, "buildbot.db"))
		if err != nil {
			sklog.Fatal(err)
		}
	} else {
		buildbotDB, err = buildbot.NewRemoteDB(*buildbotDbHost)
		if err != nil {
			sklog.Fatal(err)
		}
	}

	// Create remote Tasks DB.
	taskDB, err = remote_db.NewClient(*taskSchedulerDbUrl)
	if err != nil {
		sklog.Fatal(err)
	}

	redirectURL := fmt.Sprintf("http://localhost%s/oauth2callback/", *port)
	if !*local {
		redirectURL = "https://internal.skia.org/oauth2callback/"
	}
	if err := login.Init(redirectURL, login.DEFAULT_DOMAIN_WHITELIST); err != nil {
		sklog.Fatalf("Failed to initialize login system: %s", err)

	}

	if *local {
		webhook.InitRequestSaltForTesting()
	} else {
		webhook.MustInitRequestSaltFromMetadata()
	}

	// Ensure Skia repo is cloned and updated.
	repos, err = repograph.NewMap([]string{common.REPO_SKIA}, *workdir)
	if err != nil {
		sklog.Fatal(err)
	}

	// Initialize and start metrics.
	for codename, _ := range ingestBuildWebhookCodenames {
		ingestBuildWebhookLiveness[codename] = metrics2.NewLiveness("ingest-build-webhook.", map[string]string{"codename": codename})
	}
	startWebhookMetrics()

	// Ingest Android framework builds.
	go func() {
		sklog.Infof("Starting.")

		// In this case we don't want a backoff transport since the Apiary backend
		// seems to fail a lot, so we basically want to fall back to polling if a
		// call fails.
		client, err := auth.NewJWTServiceAccountClient("", "", &http.Transport{Dial: httputils.DialTimeout}, androidbuildinternal.AndroidbuildInternalScope, storage.CloudPlatformScope)
		if err != nil {
			sklog.Fatalf("Unable to create authenticated client: %s", err)
		}

		buildService, err := androidbuildinternal.New(client)
		if err != nil {
			sklog.Fatalf("Failed to obtain Android build service: %v", err)
		}
		sklog.Infof("Ready to start loop.")
		step(targets, buildService)
		for _ = range time.Tick(*period) {
			step(targets, buildService)
		}
	}()

	r := mux.NewRouter()
	r.HandleFunc("/", indexHandler)
	r.HandleFunc("/builders/{codename}/builds/{buildNumber}", redirectHandler)
	r.HandleFunc("/builders/{codename}", builderRedirectHandler)
	// Note: The unobfuscate-status extension relies on the below handler and its contents.
	r.HandleFunc("/mapping/", mappingHandler)
	r.HandleFunc("/ingestBuild", ingestBuildHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
