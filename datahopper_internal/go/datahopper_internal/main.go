/*
	Pulls data from the Android Build APIs and funnels that into the buildbot database.

  Android builds continuously in tradefed at the master branch with the latest roll
  of Skia. There is also another branch 'git_master-skia' which contains the HEAD
  of Skia instead of the last roll of Skia. This application continuously ingests
  builds from git_master-skia and pushes them into the buildbot database so they
  appear on status.skia.org. Since the target names may contain sensitive informaation
  they are obfuscated when pushed to the buildbot database.

  This application also contains a redirector that will take links to the obfuscated
  target name and build and return a link to the internal tradefed page with the
  detailed build info.
*/
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"go.skia.org/infra/go/androidbuild"
	"go.skia.org/infra/go/androidbuildinternal/v2beta1"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metadata"
	skmetics "go.skia.org/infra/go/metrics"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/storage/v1"
)

const (
	FAKE_MASTER     = "client.skia.fake_internal"
	FAKE_BUILDSLAVE = "fake_internal_buildslave"

	// SKIA_BRANCH is the name of the git branch we sync Skia to regularly.
	SKIA_BRANCH = "git_master-skia"

	// MASTER is the git master branch we may check builds against.
	MASTER = "git_master"
)

// flags
var (
	port           = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	graphiteServer = flag.String("graphite_server", "skia-monitoring:2003", "Where is Graphite metrics ingestion server running.")
	workdir        = flag.String("workdir", ".", "Working directory used by data processors.")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	oauthCacheFile = flag.String("oauth_cache_file", "", "Path to the OAuth credential cache file.")
	targetList     = flag.String("targets", "", "The targets to monitor, a space separated list.")
	codenameDbDir  = flag.String("codename_db_dir", "codenames", "The location of the leveldb database that holds the mappings between targets and their codenames.")
)

var (
	// cache is an in-memory cache of buildbot.Build's we've seen.
	cache = map[string]*buildbot.Build{}

	// terminal_build_status are the tradefed build status's that mean the build is done.
	terminal_build_status = []string{"complete", "error"}

	// codenameDB is a leveldb to store codenames and their deobfuscated counterparts.
	codenameDB *leveldb.DB

	// liveness is a metric for the time since last successful run through step().
	liveness = skmetics.NewLiveness("android_internal_ingest")
)

// isFinished returns true if the Build has finished running.
func isFinished(b *androidbuildinternal.Build) bool {
	return util.In(b.BuildAttemptStatus, terminal_build_status)
}

// buildFromCommit builds a buildbot.Build from the commit and the info
// returned from the Apiary API.  It also returns a key that uniqely identifies
// this build.
func buildFromCommit(build *androidbuildinternal.Build, commit *gitinfo.ShortCommit) (string, *buildbot.Build) {
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
		Times:         nil,
		Started:       float64(build.CreationTimestamp) / 1000.0,
		Comments:      nil,
		Repository:    "https://skia.googlesource.com/skia.git",
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
		b.Finished = float64(time.Now().UTC().Unix())
	}
	return key, b
}

// buildsDiffer returns true if the given Build's have different finished or results status.
func buildsDiffer(a, b *buildbot.Build) bool {
	return a.IsFinished() != b.IsFinished() || a.Results != b.Results
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

// step does a single step in ingesting builds from tradefed and pushing the results into the buildbot database.
func step(targets []string, buildService *androidbuildinternal.Service, repos *gitinfo.RepoMap) {
	glog.Errorf("step: Begin")

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
				var cachedBuild *buildbot.Build
				// Only look at the first commit in the list. The commits always appear in reverse chronological order, so
				// the 0th entry is the most recent commit.
				c := commits[0]
				// Create a buildbot.Build from the build info.
				key, build := buildFromCommit(b, c)
				glog.Infof("Key: %s Hash: %s", key, c.Hash)
				cachedBuild = nil
				// Look for the build in the in-memory cache and fill in the cache if it's not there.
				var ok bool
				if cachedBuild, ok = cache[key]; !ok {
					// Store build.Builder (the codename) with its pair build.Target.Name in a local leveldb to serve redirects.
					if err := codenameDB.Put([]byte(build.Builder), []byte(b.Target.Name), nil); err != nil {
						glog.Errorf("Failed to write codename to data store: %s", err)
					}

					buildNumber, err := buildbot.GetBuildForCommit(build.Builder, FAKE_MASTER, c.Hash)
					if err != nil {
						glog.Errorf("Failed to find the build in the database: %s", err)
						continue
					}
					cachedBuild, err = buildbot.GetBuildFromDB(build.Builder, FAKE_MASTER, buildNumber)
					if err != nil {
						glog.Errorf("Failed to retrieve build from database: %s", err)
						continue
					}
					if cachedBuild == nil {
						// This is a new build we've never seen before, so add it to the buildbot database.

						// First calculate a new unique build.Number.

						number, err := buildbot.GetMaxBuildNumber(build.Builder)
						if err != nil {
							glog.Infof("Failed to find next build number: %s", err)
							continue
						}
						build.Number = number + 1
						if err := buildbot.IngestBuild(build, repos); err != nil {
							glog.Errorf("Failed to ingest build: %s", err)
							continue
						}
						cache[key] = build
						cachedBuild = build
					} else {
						glog.Infof("Repopulating the cache from db: %s %d", cachedBuild.Builder, cachedBuild.Number)
						cache[key] = cachedBuild
					}
				}
				// If the state of the build has changed then write it to the buildbot database.
				if buildsDiffer(build, cachedBuild) {
					// If this was a failure then we need to check that there is a mirror
					// failure on the main branch, at which point we will say that this
					// is a warning.
					if build.Results == buildbot.BUILDBOT_FAILURE && brokenOnMaster(buildService, target, b.BuildId) {
						build.Results = buildbot.BUILDBOT_WARNING
					}
					glog.Infof("Writing updated build to the database: %s %d", build.Builder, build.Number)
					if err := buildbot.IngestBuild(build, repos); err != nil {
						glog.Errorf("Failed to ingest build: %s", err)
					} else {
						cache[key] = build
					}
				}
			}
			liveness.Update()
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
	REDIRECT_TEMPLATE = `<!DOCTYPE html>
<html>
  <head><title>Redirect</title></head>
  <body>
  <p>You are being redirected to Launch Control. The un-obfuscated target name for <b>%s</b> is <b>%s</b>.</p>
  <p><a href='https://android-build-uber.corp.google.com/builds.html?branch=git_master-skia&lower_limit=%s&upper_limit=%s'>master-skia:%s</a></p>
  <p><a href='https://android-build-uber.corp.google.com/builds.html?branch=git_master&lower_limit=%s&upper_limit=%s'>master:%s</a></p>
  </body>
</html>
`
)

// redirectHandler handles redirecting to the correct tradefed page.
func redirectHandler(w http.ResponseWriter, r *http.Request) {
	if login.LoggedInAs(r) == "" {
		r.Header.Set("Referer", r.URL.String())
		http.Redirect(w, r, login.LoginURL(w, r), 302)
		return
	}
	vars := mux.Vars(r)
	codename := vars["codename"]
	buildNumberStr := vars["buildNumber"]
	target, err := codenameDB.Get([]byte(codename), nil)
	if err != nil {
		util.ReportError(w, r, err, "Not a valid target codename.")
		return
	}
	buildNumber, err := strconv.Atoi(buildNumberStr)
	if err != nil {
		util.ReportError(w, r, err, "Not a valid build number.")
		return
	}
	build, err := buildbot.GetBuildFromDB(string(codename), FAKE_MASTER, buildNumber)
	if err != nil {
		util.ReportError(w, r, err, "Could not find a matching build.")
	}
	id, ok := build.GetProperty("androidinternal_buildid").([]interface{})[1].(string)
	if !ok {
		util.ReportError(w, r, fmt.Errorf("Got %#v", id), "Could not find a matching build id.")
		return
	}
	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write([]byte(fmt.Sprintf(REDIRECT_TEMPLATE, codename, target, id, id, target, id, id, target))); err != nil {
		glog.Errorf("Failed to write response: %s", err)
	}
}

func main() {
	var err error
	// Setup flags.
	dbConf := buildbot.DBConfigFromFlags() // Global init to initialize glog and parse arguments.

	// Global init to initialize glog and parse arguments.
	common.InitWithMetrics("internal", graphiteServer)

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
	if !*local {
		if err := dbConf.GetPasswordFromMetadata(); err != nil {
			glog.Fatal(err)
		}
	}
	if err := dbConf.InitDB(); err != nil {
		glog.Fatal(err)
	}

	var cookieSalt = "notverysecret"
	var clientID = "31977622648-1873k0c1e5edaka4adpv1ppvhr5id3qm.apps.googleusercontent.com"
	var clientSecret = "cw0IosPu4yjaG2KWmppj2guj"
	var redirectURL = fmt.Sprintf("http://localhost%s/oauth2callback/", *port)
	if !*local {
		cookieSalt = metadata.Must(metadata.ProjectGet(metadata.COOKIESALT))
		clientID = metadata.Must(metadata.ProjectGet(metadata.CLIENT_ID))
		clientSecret = metadata.Must(metadata.ProjectGet(metadata.CLIENT_SECRET))
		redirectURL = "https://internal.skia.org/oauth2callback/"
	}
	login.Init(clientID, clientSecret, redirectURL, cookieSalt, login.DEFAULT_SCOPE, login.DEFAULT_DOMAIN_WHITELIST, *local)

	// Ingest Android framework builds.
	go func() {
		glog.Infof("Starting.")
		repos := gitinfo.NewRepoMap(*workdir)

		// Set up the oauth client.
		var client *http.Client
		var err error

		// In this case we don't want a backoff transport since the Apiary backend
		// seems to fail a lot, so we basically want to fail fast and try again on
		// the next poll.
		transport := &http.Transport{
			Dial: util.DialTimeout,
		}

		if *local {
			// Use a local client secret file to load data.
			client, err = auth.InstalledAppClient(*oauthCacheFile, "client_secret.json",
				transport,
				androidbuildinternal.AndroidbuildInternalScope,
				storage.CloudPlatformScope)
			if err != nil {
				glog.Fatalf("Unable to create installed app oauth client:%s", err)
			}
		} else {
			// Use compute engine service account.
			client = auth.GCEServiceAccountClient(transport)
		}

		buildService, err := androidbuildinternal.New(client)
		if err != nil {
			glog.Fatalf("Failed to obtain Android build service: %v", err)
		}
		glog.Infof("Ready to start loop.")
		step(targets, buildService, repos)
		for _ = range time.Tick(common.SAMPLE_PERIOD) {
			step(targets, buildService, repos)
		}
	}()

	r := mux.NewRouter()
	r.HandleFunc("/", indexHandler)
	r.HandleFunc("/{codename}/builds/{buildNumber}/", redirectHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	http.Handle("/", util.LoggingGzipRequestResponse(r))
	glog.Fatal(http.ListenAndServe(*port, nil))
}
