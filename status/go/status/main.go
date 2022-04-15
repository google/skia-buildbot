/*
	Provides roll-up statuses for Skia build/test/perf.
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"
	"unicode"

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/datastore"
	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.skia.org/infra/autoroll/go/status"
	autoroll_status "go.skia.org/infra/autoroll/go/status"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/status/go/capacity"
	"go.skia.org/infra/status/go/incremental"
	"go.skia.org/infra/status/go/lkgr"
	"go.skia.org/infra/status/go/rpc"
	task_driver_db "go.skia.org/infra/task_driver/go/db"
	bigtable_db "go.skia.org/infra/task_driver/go/db/bigtable"
	"go.skia.org/infra/task_driver/go/handlers"
	"go.skia.org/infra/task_driver/go/logs"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"go.skia.org/infra/task_scheduler/go/window"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

const (
	appName = "status"

	// The chrome infra auth group to use for restricting admin rights.
	adminAuthGroup = "google/skia-root@google.com"
	// The chrome infra auth group to use for restricting edit rights.
	editAuthGroup = "google/skia-staff@google.com"

	defaultCommitsToLoad = 35
	maxCommitsToLoad     = 100
)

var (
	autorollMtx         sync.RWMutex
	autorollStatusTwirp *rpc.GetAutorollerStatusesResponse = nil
	capacityClient      *capacity.CapacityClientImpl       = nil
	capacityTemplate    *template.Template                 = nil
	commitsTemplate     *template.Template                 = nil
	iCache              *incremental.IncrementalCacheImpl  = nil
	lkgrObj             *lkgr.LKGR                         = nil
	taskDb              db.RemoteDB                        = nil
	taskDriverDb        task_driver_db.DB                  = nil
	taskDriverLogs      *logs.LogsManager                  = nil
	tasksPerCommit      *tasksPerCommitCache               = nil
	tCache              cache.TaskCache                    = nil

	// autorollerIDsToNames maps autoroll frontend host to maps of roller IDs to
	// their human-friendly display names.
	autorollerIDsToNames = map[string]map[string]string{
		"autoroll.skia.org": {
			"skia-flutter-autoroll":     "Flutter",
			"skia-autoroll":             "Chrome",
			"angle-skia-autoroll":       "ANGLE",
			"dawn-skia-autoroll":        "Dawn",
			"skcms-skia-autoroll":       "skcms",
			"swiftshader-skia-autoroll": "SwiftSh",
			"vulkan-deps-skia-autoroll": "VkDeps",
		},
		"skia-autoroll.corp.goog": {
			"android-master-autoroll": "Android",
			"google3-autoroll":        "Google3",
		},
	}
)

// flags
var (
	chromeInfraAuthJWT = flag.String("chrome_infra_auth_jwt", "/var/secrets/skia-public-auth/key.json", "The JWT key for the service account that has access to chrome infra auth.")
	// TODO(borenet): Combine btInstance and firestoreInstance.
	btInstance                  = flag.String("bigtable_instance", "", "BigTable instance to use.")
	btProject                   = flag.String("bigtable_project", "", "GCE project to use for BigTable.")
	capacityRecalculateInterval = flag.Duration("capacity_recalculate_interval", 10*time.Minute, "How often to re-calculate capacity statistics.")
	firestoreInstance           = flag.String("firestore_instance", "", "Firestore instance to use, eg. \"production\"")
	gitstoreTable               = flag.String("gitstore_bt_table", "git-repos2", "BigTable table used for GitStore.")
	host                        = flag.String("host", "localhost", "HTTP service host")
	port                        = flag.String("port", ":8002", "HTTP service port (e.g., ':8002')")
	promPort                    = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	repoUrls                    = common.NewMultiStringFlag("repo", nil, "Repositories to query for status.")
	resourcesDir                = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	swarmingUrl                 = flag.String("swarming_url", "https://chromium-swarm.appspot.com", "URL of the Swarming server.")
	taskLogsUrlTemplate         = flag.String("task_logs_url_template", "https://ci.chromium.org/raw/build/logs.chromium.org/skia/{{TaskID}}/+/annotations", "Template URL for direct link to logs, with {{TaskID}} as placeholder.")
	taskSchedulerUrl            = flag.String("task_scheduler_url", "https://task-scheduler.skia.org", "URL of the Task Scheduler server.")
	testing                     = flag.Bool("testing", false, "Set to true for locally testing rules. No email will be sent.")
	treeStatusBaseUrl           = flag.String("tree_status_base_url", "https://tree-status.skia.org", "Repo specific tree status URLs will be created using this base url. Eg: https://tree-status.skia.org or https://skia-tree-status.corp.goog")

	podId string
	repos repograph.Map
	// Repos and associated templates for creating links to their commits.
	repoURLsByName map[string]string
)

// StringIsInteresting returns true iff the string contains non-whitespace characters.
func StringIsInteresting(s string) bool {
	for _, c := range s {
		if !unicode.IsSpace(c) {
			return true
		}
	}
	return false
}

func reloadTemplates() {
	// Change the current working directory to two directories up from this source file so that we
	// can read templates and serve static (res/) files.

	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}
	commitsTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "dist", "status.html"),
	))
	capacityTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "dist", "capacity.html"),
	))
}

func Init() {
	reloadTemplates()
}

// repoUrlToName returns a short repo nickname given a full repo URL.
func repoUrlToName(repoUrl string) string {
	// Special case: we like "infra" better than "buildbot".
	if repoUrl == common.REPO_SKIA_INFRA {
		return "infra"
	}
	return strings.TrimSuffix(path.Base(repoUrl), ".git")
}

// repoNameToUrl returns a full repo URL given a short nickname, or an error
// if no matching repo URL is found.
func repoNameToUrl(repoName string) (string, error) {
	// Special case: we like "infra" better than "buildbot".
	if repoName == "infra" {
		return common.REPO_SKIA_INFRA, nil
	}
	// Search the list of repos used by this server.
	for _, repoUrl := range *repoUrls {
		if repoUrlToName(repoUrl) == repoName {
			return repoUrl, nil
		}
	}
	return "", fmt.Errorf("No such repo.")
}

// Same as above, for new WIP Twirp server.
// TODO(westont): Refactor once Twirp server is in use.
func getRepoTwirp(repo string) (string, string, error) {
	repoURL, err := repoNameToUrl(repo)
	if err != nil {
		return "", "", err
	}
	return repoUrlToName(repoURL), repoURL, nil
}

func defaultHandler(w http.ResponseWriter, _ *http.Request) {
	defer metrics2.FuncTimer().Stop()
	w.Header().Set("Content-Type", "text/html")

	defaultRepo := repoUrlToName((*repoUrls)[0])

	// Don't use cached templates in testing mode.
	if *testing {
		reloadTemplates()
	}

	d := struct {
		Title             string
		SwarmingURL       string
		TreeStatusBaseURL string
		LogsURLTemplate   string
		TaskSchedulerURL  string
		DefaultRepo       string
		// Repo name to repo URL.
		Repos map[string]string
	}{
		Title:             fmt.Sprintf("Status: %s", defaultRepo),
		SwarmingURL:       *swarmingUrl,
		TreeStatusBaseURL: *treeStatusBaseUrl,
		LogsURLTemplate:   *taskLogsUrlTemplate,
		TaskSchedulerURL:  *taskSchedulerUrl,
		DefaultRepo:       defaultRepo,
		Repos:             repoURLsByName,
	}

	if err := commitsTemplate.Execute(w, d); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to expand template: %v", err), http.StatusInternalServerError)
	}
}

func capacityHandler(w http.ResponseWriter, _ *http.Request) {
	defer metrics2.FuncTimer().Stop()
	w.Header().Set("Content-Type", "text/html")

	defaultRepo := repoUrlToName((*repoUrls)[0])

	// Don't use cached templates in testing mode.
	if *testing {
		reloadTemplates()
	}

	d := struct {
		Title            string
		SwarmingURL      string
		LogsURLTemplate  string
		TaskSchedulerURL string
		DefaultRepo      string
		// Repo name to repo URL.
		Repos map[string]string
	}{
		Title:            "Capacity Statistics for Skia Bots",
		SwarmingURL:      *swarmingUrl,
		LogsURLTemplate:  *taskLogsUrlTemplate,
		TaskSchedulerURL: *taskSchedulerUrl,
		DefaultRepo:      defaultRepo,
		Repos:            repoURLsByName,
	}

	if err := capacityTemplate.Execute(w, d); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to expand template: %v", err), http.StatusInternalServerError)
	}
}

func lkgrHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write([]byte(lkgrObj.Get())); err != nil {
		httputils.ReportError(w, err, "Failed to write response.", http.StatusInternalServerError)
		return
	}
}

func getAutorollerStatusesTwirp() *rpc.GetAutorollerStatusesResponse {
	autorollMtx.RLock()
	defer autorollMtx.RUnlock()
	return autorollStatusTwirp
}

// Note: srv already has the twirp handlers on it when passed into this function.
func runServer(serverURL string, srv http.Handler) {
	topLevelRouter := mux.NewRouter()
	topLevelRouter.Use(login.RestrictViewer)
	topLevelRouter.Use(login.SessionMiddleware)
	// Our 'main' router doesn't include the Twirp server, since it would double gzip responses.
	topLevelRouter.PathPrefix(rpc.StatusServicePathPrefix).Handler(httputils.LoggingRequestResponse(srv))
	r := topLevelRouter.NewRoute().Subrouter()
	r.Use(httputils.LoggingGzipRequestResponse)
	r.HandleFunc("/", httputils.CorsHandler(defaultHandler))
	r.HandleFunc("/capacity", capacityHandler)
	r.HandleFunc("/lkgr", lkgrHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	r.HandleFunc(login.DEFAULT_OAUTH2_CALLBACK, login.OAuth2CallbackHandler)
	r.PathPrefix("/dist/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))
	handlers.AddTaskDriverHandlers(r, taskDriverDb, taskDriverLogs)
	var h http.Handler = topLevelRouter
	if !*testing {
		h = httputils.HealthzAndHTTPS(topLevelRouter)
	}
	h = httputils.XFrameOptionsDeny(h)
	http.Handle("/", h)
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

type autoRollStatus struct {
	autoroll_status.AutoRollMiniStatus
	Url string `json:"url"`
}

func main() {
	// Setup flags.
	common.InitWithMust(
		appName,
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)

	Init()
	serverURL := "https://" + *host
	if *testing {
		serverURL = "http://" + *host + *port
	}
	ctx := context.Background()

	podId = os.Getenv("POD_ID")
	if podId == "" {
		sklog.Error("POD_ID not defined; falling back to UUID.")
		podId = uuid.New().String()
	}

	repoURLsByName = make(map[string]string)
	for _, repoURL := range *repoUrls {
		repoURLsByName[repoUrlToName(repoURL)] = fmt.Sprintf(gitiles.CommitURL, repoURL, "")
	}

	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail, auth.ScopeGerrit, bigtable.Scope, pubsub.ScopePubSub, datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatal(err)
	}

	// Create LKGR object.
	lkgrObj, err = lkgr.New(ctx)
	if err != nil {
		sklog.Fatalf("Failed to create LKGR: %s", err)
	}
	lkgrObj.UpdateLoop(10*time.Minute, ctx)

	// Create remote Tasks DB.
	taskDb, err = firestore.NewDBWithParams(ctx, firestore.FIRESTORE_PROJECT, *firestoreInstance, ts)
	if err != nil {
		sklog.Fatalf("Failed to create Firestore DB client: %s", err)
	}

	criaTs, err := auth.NewJWTServiceAccountTokenSource("", *chromeInfraAuthJWT, auth.ScopeUserinfoEmail)
	if err != nil {
		sklog.Fatal(err)
	}
	criaClient := httputils.DefaultClientConfig().WithTokenSource(criaTs).With2xxOnly().Client()
	adminAllowed, err := allowed.NewAllowedFromChromeInfraAuth(criaClient, adminAuthGroup)
	if err != nil {
		sklog.Fatal(err)
	}
	editAllowed, err := allowed.NewAllowedFromChromeInfraAuth(criaClient, editAuthGroup)
	if err != nil {
		sklog.Fatal(err)
	}
	login.InitWithAllow(serverURL+login.DEFAULT_OAUTH2_CALLBACK, adminAllowed, editAllowed, nil)

	// Check out source code.
	if *repoUrls == nil {
		sklog.Fatal("At least one --repo is required.")
	}
	btConf := &bt_gitstore.BTConfig{
		ProjectID:  *btProject,
		InstanceID: *btInstance,
		TableID:    *gitstoreTable,
		AppProfile: appName,
	}
	repos, err = bt_gitstore.NewBTGitStoreMap(ctx, *repoUrls, btConf)
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Info("Checkout complete")

	// Cache for buildProgressHandler.
	tasksPerCommit, err = newTasksPerCommitCache(ctx, repos, 14*24*time.Hour, *btProject, *btInstance, ts)
	if err != nil {
		sklog.Fatalf("Failed to create tasksPerCommitCache: %s", err)
	}

	// Create the IncrementalCacheImpl.
	w, err := window.New(ctx, time.Minute, maxCommitsToLoad, repos)
	if err != nil {
		sklog.Fatalf("Failed to create time window: %s", err)
	}
	iCache, err = incremental.NewIncrementalCacheImpl(ctx, taskDb, w, repos, maxCommitsToLoad, *swarmingUrl, *taskSchedulerUrl)
	if err != nil {
		sklog.Fatalf("Failed to create IncrementalCacheImpl: %s", err)
	}
	iCache.UpdateLoop(ctx, 60*time.Second)

	// Create a regular task cache.
	tCache, err = cache.NewTaskCache(ctx, taskDb, w, nil)
	if err != nil {
		sklog.Fatalf("Failed to create TaskCache: %s", err)
	}
	lvTaskCache := metrics2.NewLiveness("status_task_cache")
	go util.RepeatCtx(ctx, 60*time.Second, func(ctx context.Context) {
		if err := tCache.Update(ctx); err != nil {
			sklog.Errorf("Failed to update TaskCache: %s", err)
		} else {
			lvTaskCache.Reset()
		}
	})

	// Capacity stats.
	capacityClient = capacity.New(tasksPerCommit.tcc, tCache, repos)
	capacityClient.StartLoading(ctx, *capacityRecalculateInterval)

	// Periodically obtain the autoroller statuses.
	if err := ds.InitWithOpt(common.PROJECT_ID, ds.AUTOROLL_NS, option.WithTokenSource(ts)); err != nil {
		sklog.Fatalf("Failed to initialize datastore: %s", err)
	}
	autorollStatusDB := status.NewDatastoreDB()
	updateAutorollStatus := func(ctx context.Context) error {
		statuses := map[string]autoRollStatus{}
		statusesTwirp := []*rpc.AutorollerStatus{}
		for host, subMap := range autorollerIDsToNames {
			for roller, friendlyName := range subMap {
				s, err := autorollStatusDB.Get(ctx, roller)
				if err != nil {
					return err
				}
				miniStatus := s.AutoRollMiniStatus
				url := fmt.Sprintf("https://%s/r/%s", host, roller)
				statuses[friendlyName] = autoRollStatus{
					AutoRollMiniStatus: miniStatus,
					Url:                url,
				}
				statusesTwirp = append(statusesTwirp,
					&rpc.AutorollerStatus{
						Name:           friendlyName,
						CurrentRollRev: miniStatus.CurrentRollRev,
						LastRollRev:    miniStatus.LastRollRev,
						Mode:           miniStatus.Mode,
						NumBehind:      int32(miniStatus.NumNotRolledCommits),
						NumFailed:      int32(miniStatus.NumFailedRolls),
						Url:            url})
			}
		}
		sort.Slice(statusesTwirp, func(i, j int) bool {
			return statusesTwirp[i].Name < statusesTwirp[j].Name
		})
		autorollMtx.Lock()
		defer autorollMtx.Unlock()
		autorollStatusTwirp = &rpc.GetAutorollerStatusesResponse{Rollers: statusesTwirp}
		return nil
	}
	if err := updateAutorollStatus(ctx); err != nil {
		sklog.Fatal(err)
	}
	go util.RepeatCtx(ctx, 60*time.Second, func(ctx context.Context) {
		if err := updateAutorollStatus(ctx); err != nil {
			sklog.Errorf("Failed to update autoroll status: %s", err)
		}
	})

	// Create the TaskDriver DB.
	taskDriverBtInstance := "staging" // Task Drivers aren't in prod yet.
	taskDriverDb, err = bigtable_db.NewBigTableDB(ctx, *btProject, taskDriverBtInstance, ts)
	if err != nil {
		sklog.Fatal(err)
	}
	taskDriverLogs, err = logs.NewLogsManager(ctx, *btProject, taskDriverBtInstance, ts)
	if err != nil {
		sklog.Fatal(err)
	}

	// Create Twirp Server.
	twirpServer := rpc.NewStatusServer(iCache, taskDb, capacityClient, getAutorollerStatusesTwirp, getRepoTwirp, maxCommitsToLoad, defaultCommitsToLoad, podId)

	// Run the server.
	runServer(serverURL, twirpServer)
}
