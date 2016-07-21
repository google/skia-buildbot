/*
	Funnels data from Google-internal sources into the buildbot database.

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
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"go.skia.org/infra/go/androidbuild"
	androidbuildinternal "go.skia.org/infra/go/androidbuildinternal/v2beta1"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/go/webhook"
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
	buildbotDbHost = flag.String("buildbot_db_host", "skia-datahopper2:8000", "Where the Skia buildbot database is hosted.")
	port           = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	workdir        = flag.String("workdir", ".", "Working directory used by data processors.")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	targetList     = flag.String("targets", "", "The targets to monitor, a space separated list.")
	codenameDbDir  = flag.String("codename_db_dir", "codenames", "The location of the leveldb database that holds the mappings between targets and their codenames.")
	period         = flag.Duration("period", 5*time.Minute, "The time between ingestion runs.")

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

	// db is a buildbot.DB instance used for ingesting build data.
	db buildbot.DB

	// repos provides information about the Git repositories in workdir.
	repos *gitinfo.RepoMap

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
	ingestBuildWebhookLiveness = map[string]*metrics2.Liveness{}
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
		glog.Errorf("Failed to encode properties: %s", err)
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
		glog.Errorf("Failed to write codename to data store: %s", err)
	}

	buildNumber, err := db.GetBuildNumberForCommit(build.Master, build.Builder, commitHash)
	if err != nil {
		return fmt.Errorf("Failed to find the build in the database: %s", err)
	}
	glog.Infof("GetBuildNumberForCommit at hash: %s returned %d", commitHash, buildNumber)
	var existingBuild *buildbot.Build
	if buildNumber != -1 {
		existingBuild, err = db.GetBuildFromDB(build.Master, build.Builder, buildNumber)
		if err != nil {
			return fmt.Errorf("Failed to retrieve build from database: %s", err)
		}
	}
	if existingBuild == nil {
		// This is a new build we've never seen before, so add it to the buildbot database.

		// TODO(benjaminwagner): This logic won't work well for concurrent requests. Revisit
		// after borenet's "giant datahopper change."

		// First calculate a new unique build.Number.
		number, err := db.GetMaxBuildNumber(build.Master, build.Builder)
		if err != nil {
			return fmt.Errorf("Failed to find next build number: %s", err)
		}
		build.Number = number + 1
		glog.Infof("Writing new build to the database: %s %d", build.Builder, build.Number)
	} else {
		// If the state of the build has changed then write it to the buildbot database.
		build.Number = buildNumber
		glog.Infof("Writing updated build to the database: %s %d", build.Builder, build.Number)
	}
	if err := buildbot.IngestBuild(db, build, repos); err != nil {
		return fmt.Errorf("Failed to ingest build: %s", err)
	}
	return nil
}

// step does a single step in ingesting builds from tradefed and pushing the results into the buildbot database.
func step(targets []string, buildService *androidbuildinternal.Service) {
	glog.Infof("step: Begin")

	if err := repos.Update(); err != nil {
		glog.Errorf("Failed to update repos: %s", err)
		return
	}
	// Loop over every target and look for skia commits in the builds.
	for _, target := range targets {
		r, err := buildService.Build.List().Branch(SKIA_BRANCH).BuildType("submitted").Target(target).ExtraFields("changeInfo").MaxResults(40).Do()
		if err != nil {
			glog.Errorf("Failed to load internal builds: %v", err)
			continue
		}
		// Iterate over the builds in reverse order so we ingest the earlier Git
		// hashes first and the more recent Git hashes later.
		for i := len(r.Builds) - 1; i >= 0; i-- {
			b := r.Builds[i]
			commits := androidbuild.CommitsFromChanges(b.Changes)
			glog.Infof("Commits: %#v", commits)
			if len(commits) > 0 {
				// Only look at the first commit in the list. The commits always appear in reverse chronological order, so
				// the 0th entry is the most recent commit.
				c := commits[0]
				// Create a buildbot.Build from the build info.
				key, build := buildFromCommit(b, c)
				glog.Infof("Key: %s Hash: %s", key, c.Hash)

				// If this was a failure then we need to check that there is a
				// mirror failure on the main branch, at which point we will say
				// that this is a warning (appears green on status).
				if build.Results == buildbot.BUILDBOT_FAILURE && brokenOnMaster(buildService, target, b.BuildId) {
					build.Results = buildbot.BUILDBOT_WARNINGS
				}
				if err := ingestBuild(build, c.Hash, target); err != nil {
					glog.Error(err)
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
		glog.Errorf("Failed to write response: %s", err)
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
	build, err := db.GetBuildFromDB(FAKE_MASTER, string(codename), buildNumber)
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
		glog.Errorf("No redirect for %#v", build)
		httputils.ReportError(w, r, nil, "No redirect for this build.")
		return
	}
	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write([]byte(result)); err != nil {
		httputils.ReportError(w, r, err, "Failed to write response")
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
		glog.Errorf("Failed to write response: %s", err)
	}
}

// ingestBuildHandler parses the JSON body as a build and ingests it. The request must be
// authenticated via the protocol implemented in the webhook package. The client should retry this
// request several times, because some errors may be temporary.
func ingestBuildHandler(w http.ResponseWriter, r *http.Request) {
	data, err := webhook.AuthenticateRequest(r)
	if err != nil {
		glog.Errorf("Failed authentication in ingestBuildHandler: %s", err)
		httputils.ReportError(w, r, nil, "Failed authentication.")
		return
	}
	vars := map[string]string{}
	if err := json.Unmarshal(data, &vars); err != nil {
		glog.Errorf("Failed to parse request: %s", err)
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
		glog.Errorf("Invalid status parameter: %s", err)
		httputils.ReportError(w, r, nil, "Invalid status parameter.")
		return
	}
	startTime := time.Now().UTC()
	if startTimeStr != "" {
		if t, err := strconv.ParseInt(startTimeStr, 10, 64); err == nil {
			startTime = time.Unix(t, 0).UTC()
		} else {
			glog.Errorf("Invalid startTime parameter: %s", err)
			httputils.ReportError(w, r, nil, "Invalid startTime parameter.")
			return
		}
	}
	finishTime := time.Now().UTC()
	if finishTimeStr != "" {
		if t, err := strconv.ParseInt(finishTimeStr, 10, 64); err == nil {
			finishTime = time.Unix(t, 0).UTC()
		} else {
			glog.Errorf("Invalid finishTime parameter: %s", err)
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
			glog.Errorf("Invalid changeListNumber parameter: %s", err)
			httputils.ReportError(w, r, nil, "Invalid changeListNumber parameter.")
			return
		}
	}
	if link != "" {
		if url, err := url.Parse(link); err == nil {
			b.Properties = append(b.Properties, []interface{}{"testResultsLink", url.String(), "datahopper_internal"})
		} else {
			glog.Errorf("Invalid testResultsLink parameter: %s", err)
			httputils.ReportError(w, r, nil, "Invalid testResultsLink parameter.")
			return
		}
	}
	// Fill in PropertiesStr based on Properties.
	props, err := json.Marshal(b.Properties)
	if err == nil {
		b.PropertiesStr = string(props)
	} else {
		glog.Errorf("Failed to encode properties: %s", err)
	}
	if err := repos.Update(); err != nil {
		glog.Errorf("Failed to update repos: %s", err)
		httputils.ReportError(w, r, nil, "Failed to update repos.")
		return
	}
	if err := ingestBuild(b, commitHash, target); err != nil {
		glog.Errorf("Failed to ingest build: %s", err)
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
	repo, err := repos.Repo(common.REPO_SKIA)
	if err != nil {
		return err
	}
	commitHashes := repo.LastN(100)
	N := len(commitHashes)
	for codename, _ := range ingestBuildWebhookCodenames {
		var untestedCommitInfo *vcsinfo.LongCommit = nil
		for i := 0; i < N; i++ {
			// commitHashes is ordered oldest to newest.
			commitHash := commitHashes[N-i-1]
			buildNumber, err := db.GetBuildNumberForCommit(FAKE_MASTER, codename, commitHash)
			if err != nil {
				return err
			}
			if buildNumber == -1 {
				commitInfo, err := repo.Details(commitHash, true)
				if err != nil {
					return err
				}
				if commitInfo.Branches["master"] {
					untestedCommitInfo = commitInfo
				}
				// No tests for this commit, check older.
				continue
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
				glog.Errorf("Failed to update metrics: %s", err)
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
	glog.Infof("Targets: %#v", targets)

	codenameDB, err = leveldb.OpenFile(*codenameDbDir, nil)
	if err != nil && errors.IsCorrupted(err) {
		codenameDB, err = leveldb.RecoverFile(*codenameDbDir, nil)
	}
	if err != nil {
		glog.Fatalf("Failed to open codename leveldb at %s: %s", *codenameDbDir, err)
	}
	// Initialize the buildbot database.
	if *local {
		db, err = buildbot.NewLocalDB(path.Join(*workdir, "buildbot.db"))
		if err != nil {
			glog.Fatal(err)
		}
	} else {
		db, err = buildbot.NewRemoteDB(*buildbotDbHost)
		if err != nil {
			glog.Fatal(err)
		}
	}

	var redirectURL = fmt.Sprintf("http://localhost%s/oauth2callback/", *port)
	if !*local {
		redirectURL = "https://internal.skia.org/oauth2callback/"
	}
	if err := login.InitFromMetadataOrJSON(redirectURL, login.DEFAULT_SCOPE, login.DEFAULT_DOMAIN_WHITELIST); err != nil {
		glog.Fatalf("Failed to initialize login system: %s", err)

	}

	if *local {
		webhook.InitRequestSaltForTesting()
	} else {
		webhook.MustInitRequestSaltFromMetadata()
	}

	repos = gitinfo.NewRepoMap(*workdir)
	// Ensure Skia repo is cloned and updated.
	if _, err := repos.Repo(common.REPO_SKIA); err != nil {
		glog.Fatalf("Unable to clone Skia repo at %s: %s", common.REPO_SKIA, err)
	}
	if err := repos.Update(); err != nil {
		glog.Fatalf("Failed to update repos: %s", err)
	}

	// Initialize and start metrics.
	for codename, _ := range ingestBuildWebhookCodenames {
		ingestBuildWebhookLiveness[codename] = metrics2.NewLiveness("ingest-build-webhook.", map[string]string{"codename": codename})
	}
	startWebhookMetrics()

	// Ingest Android framework builds.
	go func() {
		glog.Infof("Starting.")

		// In this case we don't want a backoff transport since the Apiary backend
		// seems to fail a lot, so we basically want to fall back to polling if a
		// call fails.
		client, err := auth.NewJWTServiceAccountClient("", "", &http.Transport{Dial: httputils.DialTimeout}, androidbuildinternal.AndroidbuildInternalScope, storage.CloudPlatformScope)
		if err != nil {
			glog.Fatalf("Unable to create authenticated client: %s", err)
		}

		buildService, err := androidbuildinternal.New(client)
		if err != nil {
			glog.Fatalf("Failed to obtain Android build service: %v", err)
		}
		glog.Infof("Ready to start loop.")
		step(targets, buildService)
		for _ = range time.Tick(*period) {
			step(targets, buildService)
		}
	}()

	r := mux.NewRouter()
	r.HandleFunc("/", indexHandler)
	r.HandleFunc("/builders/{codename}/builds/{buildNumber}", redirectHandler)
	r.HandleFunc("/builders/{codename}", builderRedirectHandler)
	r.HandleFunc("/ingestBuild", ingestBuildHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	glog.Fatal(http.ListenAndServe(*port, nil))
}
