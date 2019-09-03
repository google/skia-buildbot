// skiacorrectness implements the process that exposes a RESTful API used by the JS frontend.
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

	"cloud.google.com/go/datastore"
	"github.com/flynn/json5"
	"github.com/gorilla/mux"
	"google.golang.org/api/option"
	gstorage "google.golang.org/api/storage/v1"
	"google.golang.org/grpc"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gevent"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/go/vcsinfo/bt_vcs"
	"go.skia.org/infra/golden/go/baseline/simple_baseliner"
	"go.skia.org/infra/golden/go/clstore/fs_clstore"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore"
	"go.skia.org/infra/golden/go/expstorage/fs_expstore"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/ignore/ds_ignorestore"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/search"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/status"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/tilesource"
	"go.skia.org/infra/golden/go/tjstore/fs_tjstore"
	"go.skia.org/infra/golden/go/tracestore/bt_tracestore"
	"go.skia.org/infra/golden/go/tryjobs/gerrit_tryjob_monitor"
	"go.skia.org/infra/golden/go/tryjobstore/ds_tryjobstore"
	"go.skia.org/infra/golden/go/warmer"
	"go.skia.org/infra/golden/go/web"
)

const (
	// IMAGE_URL_PREFIX is path prefix used for all images (digests and diffs)
	IMAGE_URL_PREFIX = "/img/"

	// OAUTH2_CALLBACK_PATH is callback endpoint used for the Oauth2 flow
	OAUTH2_CALLBACK_PATH = "/oauth2callback/"

	// EVERYTHING_PUBLIC can be provided as the value for the whitelist file to whitelist all configurations
	EVERYTHING_PUBLIC = "all"
)

func main() {
	// Command line flags.
	var (
		appTitle            = flag.String("app_title", "Skia Gold", "Title of the deployed up on the front end.")
		authoritative       = flag.Bool("authoritative", false, "Indicates that this instance should write changes that could be triggered on multiple instances running in parallel.")
		authorizedUsers     = flag.String("auth_users", login.DEFAULT_DOMAIN_WHITELIST, "White space separated list of domains and email addresses that are allowed to login.")
		btInstanceID        = flag.String("bt_instance", "production", "ID of the BigTable instance that contains Git metadata")
		btProjectID         = flag.String("bt_project_id", "skia-public", "project id with BigTable instance")
		clientSecretFile    = flag.String("client_secret", "", "Client secret file for OAuth2 authentication.")
		cpuProfile          = flag.Duration("cpu_profile", 0, "Duration for which to profile the CPU usage. After this duration the program writes the CPU profile and exits.")
		defaultCorpus       = flag.String("default_corpus", "gm", "The corpus identifier shown by default on the frontend.")
		defaultMatchFields  = flag.String("match_fields", "name", "A comma separated list of fields that need to match when finding closest images.")
		diffServerGRPCAddr  = flag.String("diff_server_grpc", "", "The grpc port of the diff server. 'diff_server_http also needs to be set.")
		diffServerImageAddr = flag.String("diff_server_http", "", "The images serving address of the diff server. 'diff_server_grpc has to be set as well.")
		dsNamespace         = flag.String("ds_namespace", "", "Cloud datastore namespace to be used by this instance.")
		dsProjectID         = flag.String("ds_project_id", "", "Project id that houses the datastore instance.")
		eventTopic          = flag.String("event_topic", "", "The pubsub topic to use for distributed events.")
		forceLogin          = flag.Bool("force_login", true, "Force the user to be authenticated for all requests.")
		fsLegacyAuth        = flag.Bool("fs_legacy_auth", false, "use legacy credentials to auth Firestore")
		fsNamespace         = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'flutter', 'skia', etc")
		fsProjectID         = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")
		gerritURL           = flag.String("gerrit_url", gerrit.GERRIT_SKIA_URL, "URL of the Gerrit instance where we retrieve CL metadata.")
		gitBTTableID        = flag.String("git_bt_table", "", "ID of the BigTable table that contains Git metadata")
		gitRepoDir          = flag.String("git_repo_dir", "../../../skia", "Directory location for the Skia repo.")
		gitRepoURL          = flag.String("git_repo_url", "https://skia.googlesource.com/skia", "The URL to pass to git clone for the source repository.")
		hashesGSPath        = flag.String("hashes_gs_path", "", "GS path, where the known hashes file should be stored. If empty no file will be written. Format: <bucket>/<path>.")
		indexInterval       = flag.Duration("idx_interval", 5*time.Minute, "Interval at which the indexer calculates the search index.")
		internalPort        = flag.String("internal_port", "", "HTTP service address for internal clients, e.g. probers. No authentication on this port.")
		local               = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
		memProfile          = flag.Duration("memprofile", 0, "Duration for which to profile memory. After this duration the program writes the memory profile and exits.")
		nCommits            = flag.Int("n_commits", 50, "Number of recent commits to include in the analysis.")
		noCloudLog          = flag.Bool("no_cloud_log", false, "Disables cloud logging. Primarily for running locally and in K8s.")
		port                = flag.String("port", ":9000", "HTTP service address (e.g., ':9000')")
		promPort            = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
		pubWhiteList        = flag.String("public_whitelist", "", fmt.Sprintf("File name of a JSON5 file that contains a query with the traces to white list. If set to '%s' everything is included. This is required if force_login is false.", EVERYTHING_PUBLIC))
		pubsubProjectID     = flag.String("pubsub_project_id", "", "Project ID that houses the pubsub topics (e.g. for ingestion).")
		redirectURL         = flag.String("redirect_url", "https://gold.skia.org/oauth2callback/", "OAuth2 redirect url. Only used when local=false.")
		resourcesDir        = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the directory relative to the source code files will be used.")
		showBotProgress     = flag.Bool("show_bot_progress", true, "Query status.skia.org for the progress of bot results.")
		siteURL             = flag.String("site_url", "https://gold.skia.org", "URL where this app is hosted.")
		tileFreshness       = flag.Duration("tile_freshness", time.Minute, "How often to re-fetch the tile")
		traceBTTableID      = flag.String("trace_bt_table", "", "BigTable table ID for the traces.")
	)
	// Parse the options. So we can configure logging.
	flag.Parse()

	var err error

	// Needed to use TimeSortableKey(...) which relies on an RNG. See docs there.
	rand.Seed(time.Now().UnixNano())

	mainTimer := timer.New("main init")

	// If we are running this, we really don't want to talk to the emulator.
	firestore.EnsureNotEmulator()

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
	skiaversion.MustLogVersion()

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
		f, err := ioutil.TempFile("./", "cpu-profile")
		if err != nil {
			sklog.Fatalf("Unable to create cpu profile file: %s", err)
		}
		sklog.Infof("Writing CPU Profile to %s", f.Name())

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

	// Start the internal server on the internal port if requested.
	if *internalPort != "" {
		// Add the profiling endpoints to the internal router.
		internalRouter := mux.NewRouter()

		// Set up the health check endpoint.
		internalRouter.HandleFunc("/healthz", httputils.ReadyHandleFunc)

		// Register pprof handlers
		internalRouter.HandleFunc("/debug/pprof/", netpprof.Index)
		internalRouter.HandleFunc("/debug/pprof/symbol", netpprof.Symbol)
		internalRouter.HandleFunc("/debug/pprof/profile", netpprof.Profile)
		internalRouter.HandleFunc("/debug/pprof/{profile}", netpprof.Index)

		go func() {
			sklog.Infof("Internal server on  http://127.0.0.1" + *internalPort)
			sklog.Fatal(http.ListenAndServe(*internalPort, internalRouter))
		}()
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
	sklog.Infof("The allowed list of users is: %q", *authorizedUsers)
	if err := login.Init(useRedirectURL, *authorizedUsers, *clientSecretFile); err != nil {
		sklog.Fatalf("Failed to initialize the login system: %s", err)
	}

	// Get the token source for the service account with access to GCS, the Monorail issue tracker,
	// cloud pubsub, and datastore.
	tokenSource, err := auth.NewDefaultTokenSource(*local, datastore.ScopeDatastore, gstorage.CloudPlatformScope, "https://www.googleapis.com/auth/userinfo.email")
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
		sklog.Fatalf("Must specify --diff_server_http and --diff_server_grpc")
	}

	// Set up the event bus which can either be in-process or distributed
	// depending whether an PubSub topic was defined.
	var evt eventbus.EventBus = nil
	if *eventTopic != "" {
		evt, err = gevent.New(*pubsubProjectID, *eventTopic, nodeName, option.WithTokenSource(tokenSource))
		if err != nil {
			sklog.Fatalf("Unable to create global event client. Got error: %s", err)
		}
		sklog.Infof("Global eventbus for topic '%s' and subscriber '%s' created.", *eventTopic, nodeName)
	} else {
		evt = eventbus.New()
	}

	var vcs vcsinfo.VCS
	if *btInstanceID != "" && *gitBTTableID != "" {
		btConf := &bt_gitstore.BTConfig{
			ProjectID:  *btProjectID,
			InstanceID: *btInstanceID,
			TableID:    *gitBTTableID,
		}

		// If the repoURL is numeric then it is treated like the numeric ID of a repository and
		// we look up the corresponding repo URL.
		useRepoURL := *gitRepoURL
		if foundRepoURL, ok := bt_gitstore.RepoURLFromID(ctx, btConf, *gitRepoURL); ok {
			useRepoURL = foundRepoURL
		}
		gitStore, err := bt_gitstore.New(ctx, btConf, useRepoURL)
		if err != nil {
			sklog.Fatalf("Error instantiating gitstore: %s", err)
		}

		gitilesRepo := gitiles.NewRepo("", "", nil)
		bvcs, err := bt_vcs.New(ctx, gitStore, "master", gitilesRepo)
		if err != nil {
			sklog.Fatalf("Error creating BT-backed VCS instance: %s", err)
		}

		bvcs.StartTracking(ctx, evt)
		vcs = bvcs
	} else {
		vcs, err = gitinfo.CloneOrUpdate(ctx, *gitRepoURL, *gitRepoDir, false)
		if err != nil {
			sklog.Fatalf("Error creating on-disk VCS instance: %s", err)
		}
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

	btc := bt_tracestore.BTConfig{
		ProjectID:  *btProjectID,
		InstanceID: *btInstanceID,
		TableID:    *traceBTTableID,
		VCS:        vcs,
	}

	err = bt_tracestore.InitBT(context.Background(), btc)
	if err != nil {
		sklog.Fatalf("Could not initialize BigTable tracestore with config %#v: %s", btc, err)
	}

	traceStore, err := bt_tracestore.New(context.Background(), btc, false)
	if err != nil {
		sklog.Fatalf("Could not instantiate BT tracestore: %s", err)
	}

	gsClientOpt := storage.GCSClientOptions{
		HashesGSPath: *hashesGSPath,
		Dryrun:       *local,
	}

	gsClient, err := storage.NewGCSClient(client, gsClientOpt)
	if err != nil {
		sklog.Fatalf("Unable to create GCSClient: %s", err)
	}

	if err := ds.InitWithOpt(*dsProjectID, *dsNamespace, option.WithTokenSource(tokenSource)); err != nil {
		sklog.Fatalf("Unable to configure cloud datastore: %s", err)
	}

	if *fsNamespace == "" {
		sklog.Fatalf("--fs_namespace must be set")
	}

	var fsClient *firestore.Client
	if *fsLegacyAuth {
		fsClient, err = firestore.NewClient(context.Background(), *fsProjectID, "gold", *fsNamespace, tokenSource)
		if err != nil {
			sklog.Fatalf("Unable to configure Firestore: %s", err)
		}
	} else {
		// Auth note: the underlying firestore.NewClient looks at the
		// GOOGLE_APPLICATION_CREDENTIALS env variable, so we don't need to supply
		// a token source.
		fsClient, err = firestore.NewClient(context.Background(), *fsProjectID, "gold", *fsNamespace, nil)
		if err != nil {
			sklog.Fatalf("Unable to configure Firestore: %s", err)
		}
	}

	// Set up the cloud expectations store
	expStore, err := fs_expstore.New(fsClient, evt, fs_expstore.ReadWrite)
	if err != nil {
		sklog.Fatalf("Unable to initialize fs_expstore: %s", err)
	}

	deprecatedTJS, err := ds_tryjobstore.New(ds.DS, evt)
	if err != nil {
		sklog.Fatalf("Unable to instantiate tryjob store: %s", err)
	}
	tryjobMonitor := gerrit_tryjob_monitor.New(deprecatedTJS, expStore, gerritAPI, *siteURL, evt, *authoritative)

	baseliner := simple_baseliner.New(expStore)

	publiclyViewableParams := paramtools.ParamSet{}
	// Load the publiclyViewable params if configured and disable querying for issues.
	if *pubWhiteList != "" && *pubWhiteList != EVERYTHING_PUBLIC {
		if publiclyViewableParams, err = loadParamFile(*pubWhiteList); err != nil {
			sklog.Fatalf("Could not load list of public params: %s", err)
		}
	}

	// Check if this is public instance. If so, make sure a list of public params
	// has been specified - can be EVERYTHING_PUBLIC.
	if !*forceLogin && (*pubWhiteList == "") {
		sklog.Fatalf("Empty whitelist file. A non-empty white list must be provided if force_login=false.")
	}

	// openSite indicates whether this can expose all end-points. The user still has to be authenticated.
	openSite := (*pubWhiteList == EVERYTHING_PUBLIC) || *forceLogin

	ignoreStore, err := ds_ignorestore.New(ds.DS)
	if err != nil {
		sklog.Fatalf("Unable to create ignorestore: %s", err)
	}

	if err := ignore.StartMonitoring(ignoreStore, *tileFreshness); err != nil {
		sklog.Fatalf("Failed to start monitoring for expired ignore rules: %s", err)
	}

	ctc := tilesource.CachedTileSourceConfig{
		EventBus:               evt,
		GerritAPI:              gerritAPI,
		IgnoreStore:            ignoreStore,
		NCommits:               *nCommits,
		PubliclyViewableParams: publiclyViewableParams,
		TraceStore:             traceStore,
		TryjobMonitor:          tryjobMonitor,
		VCS:                    vcs,
	}

	tileSource := tilesource.New(ctc)
	sklog.Infof("Fetching tile")
	// Blocks until tile is fetched
	err = tileSource.StartUpdater(context.Background(), 2*time.Minute)
	if err != nil {
		sklog.Fatalf("Could not fetch initial tile: %s", err)
	}

	ic := indexer.IndexerConfig{
		DiffStore:         diffStore,
		EventBus:          evt,
		ExpectationsStore: expStore,
		GCSClient:         gsClient,
		TileSource:        tileSource,
		Warmer:            warmer.New(),
	}

	// Rebuild the index every few minutes.
	sklog.Infof("Starting indexer to run every %s", *indexInterval)
	ixr, err := indexer.New(ic, *indexInterval)
	if err != nil {
		sklog.Fatalf("Failed to create indexer: %s", err)
	}
	sklog.Infof("Indexer created.")

	cls := fs_clstore.New(fsClient, "gerrit")
	tjs := fs_tjstore.New(fsClient, "buildbucket")

	searchAPI := search.New(diffStore, expStore, ixr, deprecatedTJS, cls, tjs, publiclyViewableParams)

	sklog.Infof("Search API created")

	swc := status.StatusWatcherConfig{
		VCS:               vcs,
		EventBus:          evt,
		TileSource:        tileSource,
		ExpectationsStore: expStore,
	}

	statusWatcher, err := status.New(swc)
	if err != nil {
		sklog.Fatalf("Failed to initialize status watcher: %s", err)
	}
	sklog.Infof("statusWatcher created")

	handlers := web.WebHandlers{
		Baseliner:               baseliner,
		DeprecatedTryjobMonitor: tryjobMonitor,
		DeprecatedTryjobStore:   deprecatedTJS,
		DiffStore:               diffStore,
		ExpectationsStore:       expStore,
		GCSClient:               gsClient,
		IgnoreStore:             ignoreStore,
		Indexer:                 ixr,
		SearchAPI:               searchAPI,
		StatusWatcher:           statusWatcher,
		TileSource:              tileSource,
		VCS:                     vcs,
	}

	mainTimer.Stop()

	// loggedRouter contains all the endpoints that are logged. See the call below to
	// LoggingGzipRequestResponse.
	loggedRouter := mux.NewRouter()

	// Set up the resource to serve the image files.
	imgHandler, err := diffStore.ImageHandler(IMAGE_URL_PREFIX)
	if err != nil {
		sklog.Fatalf("Unable to get image handler: %s", err)
	}

	// New Polymer based UI endpoints.
	loggedRouter.PathPrefix("/res/").HandlerFunc(web.MakeResourceHandler(*resourcesDir))
	loggedRouter.HandleFunc(OAUTH2_CALLBACK_PATH, login.OAuth2CallbackHandler)

	// TODO(stephana): remove "/_/hashes" in favor of "/json/hashes" once all clients have switched.

	// /_/hashes is used by the bots to find hashes it does not need to upload.
	loggedRouter.HandleFunc(shared.LEGACY_KNOWN_HASHES_ROUTE, handlers.TextKnownHashesProxy).Methods("GET")
	loggedRouter.HandleFunc("/json/version", skiaversion.JsonHandler)
	loggedRouter.HandleFunc("/loginstatus/", login.StatusHandler)
	loggedRouter.HandleFunc("/logout/", login.LogoutHandler)

	// Set up a subrouter for the '/json' routes which make up the Gold API.
	// This makes routing faster, but also returns a failure when an /json route is
	// requested that doesn't exist. If we did this differently a call to a non-existing endpoint
	// would be handled by the route that handles the returning the index template and make debugging
	// confusing.
	jsonRouter := loggedRouter.PathPrefix("/json").Subrouter()
	trim := func(r string) string { return strings.TrimPrefix(r, "/json") }

	jsonRouter.HandleFunc(trim(shared.KNOWN_HASHES_ROUTE), handlers.TextKnownHashesProxy).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/byblame"), handlers.ByBlameHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/cleardigests"), handlers.ClearDigests).Methods("POST")
	jsonRouter.HandleFunc(trim("/json/clusterdiff"), handlers.ClusterDiffHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/cmp"), handlers.DigestTableHandler).Methods("POST")
	jsonRouter.HandleFunc(trim("/json/commits"), handlers.CommitsHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/details"), handlers.DetailsHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/diff"), handlers.DiffHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/export"), handlers.ExportHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/failure"), handlers.ListFailureHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/failure/clear"), handlers.ClearFailureHandler).Methods("POST")
	jsonRouter.HandleFunc(trim("/json/gitlog"), handlers.GitLogHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/list"), handlers.ListTestsHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/paramset"), handlers.ParamsHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/search"), handlers.SearchHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/triage"), handlers.TriageHandler).Methods("POST")
	jsonRouter.HandleFunc(trim("/json/triagelog"), handlers.TriageLogHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/triagelog/undo"), handlers.TriageUndoHandler).Methods("POST")
	jsonRouter.HandleFunc(trim("/json/tryjob"), handlers.DeprecatedTryjobListHandler).Methods("GET")
	// FIXME(kjlubick): The following will not work until the new ChangeListStore/TryJobStore etc
	// is piped into web.go
	jsonRouter.HandleFunc(trim("/json/changelists"), handlers.ChangeListsHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/changelist/{system}/{id}"), handlers.ChangeListSummaryHandler).Methods("GET")

	// Retrieving that baseline for master and an Gerrit issue are handled the same way
	// These routes can be served with baseline_server for higher availability.
	jsonRouter.HandleFunc(trim(shared.EXPECTATIONS_ROUTE), handlers.BaselineHandler).Methods("GET")
	jsonRouter.HandleFunc(trim(shared.EXPECTATIONS_ISSUE_ROUTE), handlers.BaselineHandler).Methods("GET")

	jsonRouter.HandleFunc(trim("/json/refresh/{id}"), handlers.RefreshIssue).Methods("GET")

	// Only expose these endpoints if login is enforced across the app or this an open site.
	if openSite {
		jsonRouter.HandleFunc(trim("/json/ignores"), handlers.IgnoresHandler).Methods("GET")
		jsonRouter.HandleFunc(trim("/json/ignores/add/"), handlers.IgnoresAddHandler).Methods("POST")
		jsonRouter.HandleFunc(trim("/json/ignores/del/{id}"), handlers.IgnoresDeleteHandler).Methods("POST")
		jsonRouter.HandleFunc(trim("/json/ignores/save/{id}"), handlers.IgnoresUpdateHandler).Methods("POST")
	}

	// Make sure we return a 404 for anything that starts with /json and could not be found.
	jsonRouter.HandleFunc("/{ignore:.*}", http.NotFound)
	loggedRouter.HandleFunc("/json", http.NotFound)

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

	loggedRouter.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	// set up the app router that might be authenticated and logs almost everything.
	appRouter := mux.NewRouter()
	// Images should not be served gzipped, which can sometimes have issues
	// when serving an image from a NetDiffstore with HTTP2. Additionally, is wasteful
	// given PNGs typically have zlib compression anyway.
	appRouter.PathPrefix(IMAGE_URL_PREFIX).Handler(imgHandler)
	appRouter.PathPrefix("/").Handler(httputils.LoggingGzipRequestResponse(loggedRouter))

	// Use the appRouter as a handler and wrap it into middleware that enforces authentication if
	// necessary it was requested via the force_login flag.
	appHandler := http.Handler(appRouter)
	if *forceLogin {
		appHandler = login.ForceAuth(appRouter, OAUTH2_CALLBACK_PATH)
	}

	// The appHandler contains all application specific routes that are have logging and
	// authentication configured. Now we wrap it into the router that is exposed to the host
	// (aka the K8s container) which requires that some routes are never logged or authenticated.
	rootRouter := mux.NewRouter()
	rootRouter.HandleFunc("/healthz", httputils.ReadyHandleFunc)
	rootRouter.HandleFunc("/json/trstatus", httputils.CorsHandler(handlers.StatusHandler))

	rootRouter.PathPrefix("/").Handler(appHandler)

	// Start the server
	sklog.Infof("Serving on http://127.0.0.1" + *port)
	sklog.Fatal(http.ListenAndServe(*port, rootRouter))
}

// loadParamFile loads the given JSON5 file that defines the query to
// make traces publicly viewable. If the given file is empty or otherwise
// cannot be parsed an error will be returned.
func loadParamFile(fName string) (paramtools.ParamSet, error) {
	params := paramtools.ParamSet{}

	f, err := os.Open(fName)
	if err != nil {
		return params, skerr.Fmt("unable open file %s: %s", fName, err)
	}
	defer util.Close(f)

	if err := json5.NewDecoder(f).Decode(&params); err != nil {
		return params, skerr.Fmt("invalid JSON5 in %s: %s", fName, err)
	}

	// Make sure the param file is not empty.
	empty := true
	for _, values := range params {
		if empty = len(values) == 0; !empty {
			break
		}
	}
	if empty {
		return params, fmt.Errorf("publicly viewable params in %s cannot be empty.", fName)
	}
	sklog.Infof("publicly viewable params loaded from %s", fName)
	sklog.Debugf("%#v", params)
	return params, nil
}
