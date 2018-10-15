package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"math/rand"
	"net/http"
	netpprof "net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gevent"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/issues"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/db"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore"
	"go.skia.org/infra/golden/go/digeststore"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/search"
	"go.skia.org/infra/golden/go/status"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/tryjobs"
	"go.skia.org/infra/golden/go/tryjobstore"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/web"
	"google.golang.org/api/option"
	gstorage "google.golang.org/api/storage/v1"
	"google.golang.org/grpc"
)

const (
	// IMAGE_URL_PREFIX is path prefix used for all images (digests and diffs)
	IMAGE_URL_PREFIX = "/img/"

	// OAUTH2_CALLBACK_PATH is callback endpoint used for the Oauth2 flow
	OAUTH2_CALLBACK_PATH = "/oauth2callback/"

	// WHITELIST_ALL can be provided as the value for the whitelist file to whitelist all configurations
	WHITELIST_ALL = "all"
)

// Command line flags.
var (
	appTitle            = flag.String("app_title", "Skia Gold", "Title of the deployed up on the front end.")
	authoritative       = flag.Bool("authoritative", false, "Indicates that this instance should write changes that could be triggered on multiple instances running in parallel.")
	authWhiteList       = flag.String("auth_whitelist", login.DEFAULT_DOMAIN_WHITELIST, "White space separated list of domains and email addresses that are allowed to login.")
	cacheSize           = flag.Int("cache_size", 1, "Approximate cachesize used to cache images and diff metrics in GiB. This is just a way to limit caching. 0 means no caching at all. Use default for testing.")
	clientSecretFile    = flag.String("client_secret", "", "Client secret file for OAuth2 authentication.")
	cpuProfile          = flag.Duration("cpu_profile", 0, "Duration for which to profile the CPU usage. After this duration the program writes the CPU profile and exits.")
	defaultCorpus       = flag.String("default_corpus", "gm", "The corpus identifier shown by default on the frontend.")
	defaultMatchFields  = flag.String("match_fields", "name", "A comma separated list of fields that need to match when finding closest images.")
	diffServerGRPCAddr  = flag.String("diff_server_grpc", "", "The grpc port of the diff server. 'diff_server_http also needs to be set.")
	diffServerImageAddr = flag.String("diff_server_http", "", "The images serving address of the diff server. 'diff_server_grpc has to be set as well.")
	dsNamespace         = flag.String("ds_namespace", "", "Cloud datastore namespace to be used by this instance.")
	eventTopic          = flag.String("event_topic", "", "The pubsub topic to use for distributed events.")
	forceLogin          = flag.Bool("force_login", true, "Force the user to be authenticated for all requests.")
	gsBucketNames       = flag.String("gs_buckets", "skia-infra-gm,chromium-skia-gm", "Comma-separated list of google storage bucket that hold uploaded images.")
	hashesGSPath        = flag.String("hashes_gs_path", "", "GS path, where the known hashes file should be stored. If empty no file will be written. Format: <bucket>/<path>.")
	baselineGSPath      = flag.String("baseline_gs_path", "", "GS path, where the baseline file should be stored. If empty no file will be written. Format: <bucket>/<path>.")
	imageDir            = flag.String("image_dir", "/tmp/imagedir", "What directory to store test and diff images in.")
	indexInterval       = flag.Duration("idx_interval", 5*time.Minute, "Interval at which the indexer calculates the search index.")
	internalPort        = flag.String("internal_port", "", "HTTP service address for internal clients, e.g. probers. No authentication on this port.")
	local               = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	memProfile          = flag.Duration("memprofile", 0, "Duration for which to profile memory. After this duration the program writes the memory profile and exits.")
	nCommits            = flag.Int("n_commits", 50, "Number of recent commits to include in the analysis.")
	noCloudLog          = flag.Bool("no_cloud_log", false, "Disables cloud logging. Primarily for running locally.")
	port                = flag.String("port", ":9000", "HTTP service address (e.g., ':9000')")
	projectID           = flag.String("project_id", common.PROJECT_ID, "GCP project ID.")
	promPort            = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	pubWhiteList        = flag.String("public_whitelist", "", fmt.Sprintf("File name of a JSON5 file that contains a query with the traces to white list. If set to '%s' everything is included. This is required if force_login is false.", WHITELIST_ALL))
	redirectURL         = flag.String("redirect_url", "https://gold.skia.org/oauth2callback/", "OAuth2 redirect url. Only used when local=false.")
	resourcesDir        = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the directory relative to the source code files will be used.")
	gerritURL           = flag.String("gerrit_url", gerrit.GERRIT_SKIA_URL, "URL of the Gerrit instance where we retrieve CL metadata.")
	siteURL             = flag.String("site_url", "https://gold.skia.org", "URL where this app is hosted.")
	storageDir          = flag.String("storage_dir", "/tmp/gold-storage", "Directory to store reproducible application data.")
	gitRepoDir          = flag.String("git_repo_dir", "../../../skia", "Directory location for the Skia repo.")
	gitRepoURL          = flag.String("git_repo_url", "https://skia.googlesource.com/skia", "The URL to pass to git clone for the source repository.")
	serviceAccountFile  = flag.String("service_account_file", "", "Credentials file for service account.")
	showBotProgress     = flag.Bool("show_bot_progress", true, "Query status.skia.org for the progress of bot results.")
	traceservice        = flag.String("trace_service", "localhost:10000", "The address of the traceservice endpoint.")
)

func main() {
	var err error

	// Needed to use TimeSortableKey(...) which relies on an RNG. See docs there.
	rand.Seed(time.Now().UnixNano())

	mainTimer := timer.New("main init")

	// Setup DB flags. But don't specify a default host or default database
	// to avoid accidental writes.
	dbConf := database.ConfigFromFlags("", db.PROD_DB_PORT, database.USER_RW, "", db.MigrationSteps())

	// Parse the options. So we can configure logging.
	flag.Parse()

	// Set up the logging options.
	logOpts := []common.Opt{
		common.PrometheusOpt(promPort),
	}

	// Should we disable cloud logging.
	if !*noCloudLog {
		logOpts = append(logOpts, common.CloudLoggingOpt())
	}
	_, appName := filepath.Split(os.Args[0])
	common.InitWithMust(appName, logOpts...)

	ctx := context.Background()

	skiaversion.MustLogVersion()

	// Enable the memory profiler if memProfile was set.
	// TODO(stephana): This should be moved to a HTTP endpoint that
	// only responds to internal IP addresses/ports.
	if *memProfile > 0 {
		time.AfterFunc(*memProfile, func() {
			sklog.Infof("Writing Memory Profile")
			f, err := ioutil.TempFile("./", "memory-profile")
			if err != nil {
				sklog.Fatalf("Unable to create memory profile file: %s", err)
			}
			if err := pprof.WriteHeapProfile(f); err != nil {
				sklog.Fatalf("Unable to write memory profile file: %v", err)
			}
			util.Close(f)
			sklog.Infof("Memory profile written to %s", f.Name())

			os.Exit(0)
		})
	}

	if *cpuProfile > 0 {
		sklog.Infof("Writing CPU Profile")
		f, err := ioutil.TempFile("./", "cpu-profile")
		if err != nil {
			sklog.Fatalf("Unable to create cpu profile file: %s", err)
		}

		if err := pprof.StartCPUProfile(f); err != nil {
			sklog.Fatalf("Unable to write cpu profile file: %v", err)
		}
		time.AfterFunc(*cpuProfile, func() {
			pprof.StopCPUProfile()
			util.Close(f)
			sklog.Infof("CPU profile written to %s", f.Name())
			os.Exit(0)
		})
	}

	// Set the resource directory if it's empty. Useful for running locally.
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
		*resourcesDir += "/frontend"
	}

	// Set up login
	useRedirectURL := *redirectURL
	if *local {
		useRedirectURL = fmt.Sprintf("http://localhost%s/oauth2callback/", *port)
	}
	authWhiteList := metadata.GetWithDefault(metadata.AUTH_WHITE_LIST, login.DEFAULT_DOMAIN_WHITELIST)
	if err := login.Init(useRedirectURL, authWhiteList, *clientSecretFile); err != nil {
		sklog.Fatalf("Failed to initialize the login system: %s", err)
	}

	// Get the token source for the service account with access to GCS, the Monorail issue tracker,
	// cloud pubsub, and datastore.
	tokenSource, err := auth.NewJWTServiceAccountTokenSource("", *serviceAccountFile, gstorage.CloudPlatformScope, "https://www.googleapis.com/auth/userinfo.email")
	if err != nil {
		sklog.Fatalf("Failed to authenticate service account: %s", err)
	}
	// TODO(dogben): Ok to add request/dial timeouts?
	client := httputils.DefaultClientConfig().WithTokenSource(tokenSource).WithoutRetries().Client()

	// serviceName uniquely identifies this host and app and is used as ID for other services.
	nodeName, err := gevent.GetNodeName(appName, *local)
	if err != nil {
		sklog.Fatalf("Error getting unique service name: %s", err)
	}

	// If the addresses for a remote DiffStore were given, then set it up
	// otherwise create an embedded DiffStore instance.
	var diffStore diff.DiffStore = nil
	if (*diffServerGRPCAddr != "") || (*diffServerImageAddr != "") {
		// Create the client connection and connect to the server.
		conn, err := grpc.Dial(*diffServerGRPCAddr,
			grpc.WithInsecure(),
			grpc.WithDefaultCallOptions(
				grpc.MaxCallSendMsgSize(diffstore.MAX_MESSAGE_SIZE),
				grpc.MaxCallRecvMsgSize(diffstore.MAX_MESSAGE_SIZE)))
		if err != nil {
			sklog.Fatalf("Unable to connect to grpc service: %s", err)
		}

		codec := diffstore.MetricMapCodec{}
		diffStore, err = diffstore.NewNetDiffStore(conn, *diffServerImageAddr, codec)
		if err != nil {
			sklog.Fatalf("Unable to initialize NetDiffStore: %s", err)
		}
		sklog.Infof("DiffStore: NetDiffStore initiated.")
	} else {
		mapper := diffstore.NewGoldDiffStoreMapper(&diff.DiffMetrics{})
		diffStore, err = diffstore.NewMemDiffStore(client, *imageDir, strings.Split(*gsBucketNames, ","), diffstore.DEFAULT_GCS_IMG_DIR_NAME, *cacheSize, mapper)
		if err != nil {
			sklog.Fatalf("Allocating local DiffStore failed: %s", err)
		}
		sklog.Infof("DiffStore: MemDiffStore initiated.")
	}
	digestStore, err := digeststore.New(*storageDir)
	if err != nil {
		sklog.Fatal(err)
	}

	git, err := gitinfo.CloneOrUpdate(ctx, *gitRepoURL, *gitRepoDir, false)
	if err != nil {
		sklog.Fatal(err)
	}

	// Set up the event bus which can either be in-process or distributed
	// depending whether an PubSub topic was defined.
	var evt eventbus.EventBus = nil
	if *eventTopic != "" {
		evt, err = gevent.New(*projectID, *eventTopic, nodeName, option.WithTokenSource(tokenSource))
		if err != nil {
			sklog.Fatalf("Unable to create global event client. Got error: %s", err)
		}
		sklog.Infof("Global eventbus for topic '%s' and subscriber '%s' created.", *eventTopic, nodeName)
	} else {
		evt = eventbus.New()
	}

	// If this is an authoritative instance we need an authenticated Gerrit client
	// because it needs to write.
	gitcookiesPath := ""
	if *authoritative {
		// Set up an authenticated Gerrit client.
		gitcookiesPath = gerrit.DefaultGitCookiesPath()
		if !*local {
			if gitcookiesPath, err = gerrit.GitCookieAuthDaemonPath(); err != nil {
				sklog.Fatalf("Error retrieving git_cookie_authdaemon path: %s", err)
			}
		}
	}
	gerritAPI, err := gerrit.NewGerrit(*gerritURL, gitcookiesPath, nil)
	if err != nil {
		sklog.Fatalf("Failed to create Gerrit client: %s", err)
	}

	// Connect to traceDB and create the builders.
	db, err := tracedb.NewTraceServiceDBFromAddress(*traceservice, types.GoldenTraceBuilder)
	if err != nil {
		sklog.Fatalf("Failed to connect to tracedb: %s", err)
	}

	masterTileBuilder, err := tracedb.NewMasterTileBuilder(ctx, db, git, *nCommits, evt, filepath.Join(*storageDir, "cached-last-tile"))
	if err != nil {
		sklog.Fatalf("Failed to build trace/db.DB: %s", err)
	}

	gsClientOpt := &storage.GSClientOptions{
		HashesGSPath:   *hashesGSPath,
		BaselineGSPath: *baselineGSPath,
	}

	gsClient, err := storage.NewGStorageClient(client, gsClientOpt)
	if err != nil {
		sklog.Fatalf("Unable to create GStorageClient: %s", err)
	}

	if err := ds.InitWithOpt(*projectID, *dsNamespace, option.WithTokenSource(tokenSource)); err != nil {
		sklog.Fatalf("Unable to configure cloud datastore: %s", err)
	}

	// Set up the cloud expectations store, since at least the issue portion
	// will be used even if we use MySQL.
	expStore, issueExpStoreFactory, err := expstorage.NewCloudExpectationsStore(ds.DS, evt)
	if err != nil {
		sklog.Fatalf("Unable to configure cloud expectations store: %s", err)
	}

	// Check if we should set up a MySQL backend for some of the stores.
	useMySQL := (dbConf.Host != "") && (dbConf.Name != "")
	var vdb *database.VersionedDB

	// Set up MySQL if requested.
	if useMySQL {
		if !*local {
			if err := dbConf.GetPasswordFromMetadata(); err != nil {
				sklog.Fatal(err)
			}
		}
		vdb, err = dbConf.NewVersionedDB()
		if err != nil {
			sklog.Fatal(err)
		}

		if !vdb.IsLatestVersion() {
			sklog.Fatal("Wrong DB version. Please updated to latest version.")
		}

		// This uses MySQL to manage expecations for the master branch.
		expStore = expstorage.NewSQLExpectationStore(vdb)
	}

	tryjobStore, err := tryjobstore.NewCloudTryjobStore(ds.DS, issueExpStoreFactory, evt)
	if err != nil {
		sklog.Fatalf("Unable to instantiate tryjob store: %s", err)
	}

	// Extract the site URL
	storages := &storage.Storage{
		DiffStore:            diffStore,
		ExpectationsStore:    expstorage.NewCachingExpectationStore(expStore, evt),
		IssueExpStoreFactory: issueExpStoreFactory,
		MasterTileBuilder:    masterTileBuilder,
		DigestStore:          digestStore,
		NCommits:             *nCommits,
		EventBus:             evt,
		TryjobStore:          tryjobStore,
		TryjobMonitor:        tryjobs.NewTryjobMonitor(tryjobStore, gerritAPI, *siteURL, evt, *authoritative),
		GerritAPI:            gerritAPI,
		GStorageClient:       gsClient,
		Git:                  git,
	}

	// Load the whitelist if there is one and disable querying for issues.
	if *pubWhiteList != "" && *pubWhiteList != WHITELIST_ALL {
		if err := storages.LoadWhiteList(*pubWhiteList); err != nil {
			sklog.Fatalf("Empty or invalid white list file. A non-empty white list must be provided if force_login=false.")
		}
	}

	// Check if this is public instance. If so make sure there is a white list.
	if !*forceLogin && (*pubWhiteList == "") {
		sklog.Fatalf("Empty whitelist file. A non-empty white list must be provided if force_login=false.")
	}

	// openSite indicates whether this can expose all end-points. The user still has to be authenticated.
	openSite := (*pubWhiteList == WHITELIST_ALL) || *forceLogin

	// TODO(stephana): Remove this workaround to avoid circular dependencies once the 'storage' module is cleaned up.

	// If MySQL is configured we use it to store the ignore rules.
	if useMySQL {
		storages.IgnoreStore = ignore.NewSQLIgnoreStore(vdb, storages.ExpectationsStore, storages.GetTileStreamNow(time.Minute))
	} else if storages.IgnoreStore, err = ignore.NewCloudIgnoreStore(ds.DS, storages.ExpectationsStore, storages.GetTileStreamNow(time.Minute)); err != nil {
		sklog.Fatalf("Unable to create ignorestore: %s", err)
	}

	if err := ignore.Init(storages.IgnoreStore); err != nil {
		sklog.Fatalf("Failed to start monitoring for expired ignore rules: %s", err)
	}

	// Rebuild the index every two minutes.
	ixr, err := indexer.New(storages, *indexInterval)
	if err != nil {
		sklog.Fatalf("Failed to create indexer: %s", err)
	}

	searchAPI, err := search.NewSearchAPI(storages, ixr)
	if err != nil {
		sklog.Fatalf("Failed to create instance of search API: %s", err)
	}

	issueTracker := issues.NewMonorailIssueTracker(client)

	statusWatcher, err := status.New(storages)
	if err != nil {
		sklog.Fatalf("Failed to initialize status watcher: %s", err)
	}
	mainTimer.Stop()

	handlers := web.WebHandlers{
		Storages:      storages,
		StatusWatcher: statusWatcher,
		Indexer:       ixr,
		IssueTracker:  issueTracker,
		SearchAPI:     searchAPI,
	}

	router := mux.NewRouter()

	// Set up the resource to serve the image files.
	imgHandler, err := diffStore.ImageHandler(IMAGE_URL_PREFIX)
	if err != nil {
		sklog.Fatalf("Unable to get image handler: %s", err)
	}
	router.PathPrefix(IMAGE_URL_PREFIX).Handler(imgHandler)

	// New Polymer based UI endpoints.
	router.PathPrefix("/res/").HandlerFunc(web.MakeResourceHandler(*resourcesDir))
	router.HandleFunc(OAUTH2_CALLBACK_PATH, login.OAuth2CallbackHandler)

	// /_/hashes is used by the bots to find hashes it does not need to upload.
	router.HandleFunc("/_/hashes", handlers.TextAllHashesHandler).Methods("GET")
	router.HandleFunc("/json/version", skiaversion.JsonHandler)
	router.HandleFunc("/loginstatus/", login.StatusHandler)
	router.HandleFunc("/logout/", login.LogoutHandler)

	// json handlers only used by the new UI.
	router.HandleFunc("/json/byblame", handlers.JsonByBlameHandler).Methods("GET")
	router.HandleFunc("/json/list", handlers.JsonListTestsHandler).Methods("GET")
	router.HandleFunc("/json/paramset", handlers.JsonParamsHandler).Methods("GET")
	router.HandleFunc("/json/commits", handlers.JsonCommitsHandler).Methods("GET")
	router.HandleFunc("/json/diff", handlers.JsonDiffHandler).Methods("GET")
	router.HandleFunc("/json/details", handlers.JsonDetailsHandler).Methods("GET")
	router.HandleFunc("/json/triage", handlers.JsonTriageHandler).Methods("POST")
	router.HandleFunc("/json/clusterdiff", handlers.JsonClusterDiffHandler).Methods("GET")
	router.HandleFunc("/json/cmp", handlers.JsonCompareTestHandler).Methods("POST")
	router.HandleFunc("/json/triagelog", handlers.JsonTriageLogHandler).Methods("GET")
	router.HandleFunc("/json/triagelog/undo", handlers.JsonTriageUndoHandler).Methods("POST")
	router.HandleFunc("/json/failure", handlers.JsonListFailureHandler).Methods("GET")
	router.HandleFunc("/json/failure/clear", handlers.JsonClearFailureHandler).Methods("POST")
	router.HandleFunc("/json/cleardigests", handlers.JsonClearDigests).Methods("POST")
	router.HandleFunc("/json/search", handlers.JsonSearchHandler).Methods("GET")
	router.HandleFunc("/json/export", handlers.JsonExportHandler).Methods("GET")
	router.HandleFunc("/json/tryjob", handlers.JsonTryjobListHandler).Methods("GET")
	router.HandleFunc("/json/tryjob/{id}", handlers.JsonTryjobSummaryHandler).Methods("GET")

	// Retrieving that baseline for master and an Gerrit issue are handled the same way
	router.HandleFunc(web.BASELINE_ROUTE, handlers.JsonBaselineHandler).Methods("GET")
	router.HandleFunc(web.BASELINE_ISSUE_ROUTE, handlers.JsonBaselineHandler).Methods("GET")
	router.HandleFunc("/json/refresh/{id}", handlers.JsonRefreshIssue).Methods("GET")

	// Only expose these endpoints if login is enforced across the app or this an open site.
	if openSite {
		router.HandleFunc("/json/ignores", handlers.JsonIgnoresHandler).Methods("GET")
		router.HandleFunc("/json/ignores/add/", handlers.JsonIgnoresAddHandler).Methods("POST")
		router.HandleFunc("/json/ignores/del/{id}", handlers.JsonIgnoresDeleteHandler).Methods("POST")
		router.HandleFunc("/json/ignores/save/{id}", handlers.JsonIgnoresUpdateHandler).Methods("POST")
	}

	// For everything else serve the same markup.
	indexFile := *resourcesDir + "/index.html"
	indexTemplate := template.Must(template.New("").ParseFiles(indexFile)).Lookup("index.html")

	// appConfig is injected into the header of the index file.
	appConfig := &struct {
		BaseRepoURL        string   `json:"baseRepoURL"`
		DefaultCorpus      string   `json:"defaultCorpus"`
		DefaultMatchFields []string `json:"defaultMatchFields"`
		ShowBotProgress    bool     `json:"showBotProgress"`
		Title              string   `json:"title"`
		IsPublic           bool     `json:"isPublic"` // If true this is not open but restrictions apply.
	}{
		BaseRepoURL:        *gitRepoURL,
		DefaultCorpus:      *defaultCorpus,
		DefaultMatchFields: strings.Split(*defaultMatchFields, ","),
		ShowBotProgress:    *showBotProgress,
		Title:              *appTitle,
		IsPublic:           !openSite,
	}

	router.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")

		// Reload the template if we are running locally.
		if *local {
			indexTemplate = template.Must(template.New("").ParseFiles(indexFile)).Lookup("index.html")
		}
		if err := indexTemplate.Execute(w, appConfig); err != nil {
			sklog.Errorf("Failed to expand template: %s", err)
			return
		}
	})

	// set up a router that logs for all URLs except the status and the health endpoints.
	appRouter := mux.NewRouter()
	appRouter.HandleFunc("/json/trstatus", handlers.JsonStatusHandler)
	appRouter.HandleFunc("/ready", httputils.ReadyHandleFunc)

	// Wrap all other routes in in logging middleware.
	appRouter.PathPrefix("/").Handler(httputils.LoggingGzipRequestResponse(router))

	// Set up the external handler to enforce authentication if necessary.
	externalHandler := http.Handler(appRouter)
	if *forceLogin {
		externalHandler = login.ForceAuth(appRouter, OAUTH2_CALLBACK_PATH)
	}

	// Start the internal server on the internal port if requested.
	if *internalPort != "" {
		// Add the profiling endpoints to the internal router.
		internalRouter := mux.NewRouter()
		// Register pprof handlers
		internalRouter.HandleFunc("/debug/pprof/", netpprof.Index)
		internalRouter.HandleFunc("/debug/pprof/cmdline", netpprof.Cmdline)
		internalRouter.HandleFunc("/debug/pprof/profile", netpprof.Profile)
		internalRouter.HandleFunc("/debug/pprof/symbol", netpprof.Symbol)
		internalRouter.HandleFunc("/debug/pprof/trace", netpprof.Trace)

		// Add the rest of the application.
		internalRouter.PathPrefix("/").Handler(appRouter)

		go func() {
			sklog.Infof("Internal server on  http://127.0.0.1" + *internalPort)
			sklog.Fatal(http.ListenAndServe(*internalPort, internalRouter))
		}()
	}

	// Start the server
	sklog.Infof("Serving on http://127.0.0.1" + *port)
	sklog.Fatal(http.ListenAndServe(*port, externalHandler))
}
