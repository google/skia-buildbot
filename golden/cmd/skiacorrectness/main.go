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
	"sync"
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
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/issues"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/go/vcsinfo/bt_vcs"
	"go.skia.org/infra/golden/go/baseline/gcs_baseliner"
	"go.skia.org/infra/golden/go/db"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/search"
	"go.skia.org/infra/golden/go/shared"
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

func main() {
	// Command line flags.
	var (
		appTitle            = flag.String("app_title", "Skia Gold", "Title of the deployed up on the front end.")
		authoritative       = flag.Bool("authoritative", false, "Indicates that this instance should write changes that could be triggered on multiple instances running in parallel.")
		authorizedUsers     = flag.String("auth_users", login.DEFAULT_DOMAIN_WHITELIST, "White space separated list of domains and email addresses that are allowed to login.")
		baselineGSPath      = flag.String("baseline_gs_path", "", "GS path, where the baseline file should be stored. If empty no file will be written. Format: <bucket>/<path>.")
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
		gerritURL           = flag.String("gerrit_url", gerrit.GERRIT_SKIA_URL, "URL of the Gerrit instance where we retrieve CL metadata.")
		gitBTInstanceID     = flag.String("git_bt_instance", "", "ID of the BigTable instance that contains Git metadata")
		gitBTTableID        = flag.String("git_bt_table", "", "ID of the BigTable table that contains Git metadata")
		gitRepoDir          = flag.String("git_repo_dir", "../../../skia", "Directory location for the Skia repo.")
		gitRepoURL          = flag.String("git_repo_url", "https://skia.googlesource.com/skia", "The URL to pass to git clone for the source repository.")
		gsBucketNames       = flag.String("gs_buckets", "", "Comma-separated list of google storage bucket that hold uploaded images.")
		hashesGSPath        = flag.String("hashes_gs_path", "", "GS path, where the known hashes file should be stored. If empty no file will be written. Format: <bucket>/<path>.")
		imageDir            = flag.String("image_dir", "/tmp/imagedir", "What directory to store test and diff images in.")
		indexInterval       = flag.Duration("idx_interval", 5*time.Minute, "Interval at which the indexer calculates the search index.")
		internalPort        = flag.String("internal_port", "", "HTTP service address for internal clients, e.g. probers. No authentication on this port.")
		local               = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
		memProfile          = flag.Duration("memprofile", 0, "Duration for which to profile memory. After this duration the program writes the memory profile and exits.")
		nCommits            = flag.Int("n_commits", 50, "Number of recent commits to include in the analysis.")
		noCloudLog          = flag.Bool("no_cloud_log", false, "Disables cloud logging. Primarily for running locally and in K8s.")
		port                = flag.String("port", ":9000", "HTTP service address (e.g., ':9000')")
		projectID           = flag.String("project_id", common.PROJECT_ID, "GCP project ID.")
		promPort            = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
		pubWhiteList        = flag.String("public_whitelist", "", fmt.Sprintf("File name of a JSON5 file that contains a query with the traces to white list. If set to '%s' everything is included. This is required if force_login is false.", WHITELIST_ALL))
		redirectURL         = flag.String("redirect_url", "https://gold.skia.org/oauth2callback/", "OAuth2 redirect url. Only used when local=false.")
		resourcesDir        = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the directory relative to the source code files will be used.")
		serviceAccountFile  = flag.String("service_account_file", "", "Credentials file for service account.")
		showBotProgress     = flag.Bool("show_bot_progress", true, "Query status.skia.org for the progress of bot results.")
		siteURL             = flag.String("site_url", "https://gold.skia.org", "URL where this app is hosted.")
		sparseInput         = flag.Bool("sparse", false, "Sparse input expected. Filter out 'empty' commits.")
		storageDir          = flag.String("storage_dir", "", "Directory to store reproducible application data. [DEPRECATED]")
		traceservice        = flag.String("trace_service", "localhost:10000", "The address of the traceservice endpoint.")
	)

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
	skiaversion.MustLogVersion()

	// TODO(stephana): Running the setup process in the a parallel go-routine is an ugly hack and
	// should be removed as soon as the setup process is fast enough to complete within 10 seconds.

	// Run the setup process in the background so we can quickly expose a URL for health checks.
	// This is currently necessary to ease deployment on Kubernetes. It renders the initial
	// readyness check meaningless, but not subsequent health checks once the setup is completed.
	var openSite bool
	var handlers web.WebHandlers
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

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
		sklog.Infof("The allowed list of users is: %q", *authorizedUsers)
		if err := login.Init(useRedirectURL, *authorizedUsers, *clientSecretFile); err != nil {
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
			if *gsBucketNames == "" {
				sklog.Fatalf("Must specify --gs_buckets or (--diff_server_http and --diff_server_grpc)")
			}
			mapper := diffstore.NewGoldDiffStoreMapper(&diff.DiffMetrics{})
			diffStore, err = diffstore.NewMemDiffStore(client, *imageDir, strings.Split(*gsBucketNames, ","), diffstore.DEFAULT_GCS_IMG_DIR_NAME, *cacheSize, mapper)
			if err != nil {
				sklog.Fatalf("Allocating local DiffStore failed: %s", err)
			}
			sklog.Infof("DiffStore: MemDiffStore initiated.")
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

		var vcs vcsinfo.VCS
		if *gitBTInstanceID != "" && *gitBTTableID != "" {
			btConf := &bt_gitstore.BTConfig{
				ProjectID:  *projectID,
				InstanceID: *gitBTInstanceID,
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

			trackNCommits := *nCommits
			if *sparseInput {
				// If the input is sparse we watch a magnitude more commits to make sure we don't miss any
				// commit.
				trackNCommits *= 10
			}
			vcs, err = bt_vcs.New(gitStore, "master", gitilesRepo, evt, trackNCommits)
		} else {
			vcs, err = gitinfo.CloneOrUpdate(ctx, *gitRepoURL, *gitRepoDir, false)
		}
		if err != nil {
			sklog.Fatalf("Error creating VCS instance: %s", err)
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

		// TODO(stephana): All dependencies on storageDir should be removed once we have landed
		// all instances in K8s.

		// If a storage directory was provided we can use it to cache tiles.
		mtbCache := ""
		if *storageDir != "" {
			mtbCache = filepath.Join(*storageDir, "cached-last-tile")
		}

		masterTileBuilder, err := tracedb.NewMasterTileBuilder(ctx, db, vcs, *nCommits, evt, mtbCache)
		if err != nil {
			sklog.Fatalf("Failed to build trace/db.DB: %s", err)
		}

		gsClientOpt := storage.GCSClientOptions{
			HashesGSPath:   *hashesGSPath,
			BaselineGSPath: *baselineGSPath,
		}

		gsClient, err := storage.NewGCSClient(client, gsClientOpt)
		if err != nil {
			sklog.Fatalf("Unable to create GCSClient: %s", err)
		}

		if err := ds.InitWithOpt(*projectID, *dsNamespace, option.WithTokenSource(tokenSource)); err != nil {
			sklog.Fatalf("Unable to configure cloud datastore: %s", err)
		}

		// Set up the cloud expectations store, since at least the issue portion
		// will be used even if we use MySQL.
		var expStore expstorage.DEPRECATED_ExpectationsStore
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

			// This uses MySQL to manage expectations for the master branch.
			expStore = expstorage.NewSQLExpectationStore(vdb)
		}
		expStore = expstorage.NewCachingExpectationStore(expStore, evt)

		tryjobStore, err := tryjobstore.NewCloudTryjobStore(ds.DS, issueExpStoreFactory, evt)
		if err != nil {
			sklog.Fatalf("Unable to instantiate tryjob store: %s", err)
		}
		tryjobMonitor := tryjobs.NewTryjobMonitor(tryjobStore, expStore, issueExpStoreFactory, gerritAPI, *siteURL, evt, *authoritative)

		// Initialize the Baseliner instance from the values set above.
		baseliner, err := gcs_baseliner.New(gsClient, expStore, issueExpStoreFactory, tryjobStore, vcs)
		if err != nil {
			sklog.Fatalf("Error initializing baseliner: %s", err)
		}

		// Extract the site URL
		storages := &storage.Storage{
			DiffStore:            diffStore,
			ExpectationsStore:    expStore,
			IssueExpStoreFactory: issueExpStoreFactory,
			TraceDB:              db,
			MasterTileBuilder:    masterTileBuilder,
			NCommits:             *nCommits,
			EventBus:             evt,
			TryjobStore:          tryjobStore,
			TryjobMonitor:        tryjobMonitor,
			GerritAPI:            gerritAPI,
			GCSClient:            gsClient,
			VCS:                  vcs,
			IsAuthoritative:      *authoritative,
			SiteURL:              *siteURL,
			IsSparseTile:         *sparseInput,
			Baseliner:            baseliner,
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
		openSite = (*pubWhiteList == WHITELIST_ALL) || *forceLogin

		// TODO(stephana): Remove this workaround to avoid circular dependencies once the 'storage' module is cleaned up.

		// If MySQL is configured we use it to store the ignore rules.
		if useMySQL {
			storages.IgnoreStore = ignore.NewSQLIgnoreStore(vdb, storages.ExpectationsStore, storages.GetTileStreamNow(time.Minute, "gold-ignore-store"))
		} else if storages.IgnoreStore, err = ignore.NewCloudIgnoreStore(ds.DS, storages.ExpectationsStore, storages.GetTileStreamNow(time.Minute, "gold-ignore-store")); err != nil {
			sklog.Fatalf("Unable to create ignorestore: %s", err)
		}

		if err := ignore.Init(storages.IgnoreStore); err != nil {
			sklog.Fatalf("Failed to start monitoring for expired ignore rules: %s", err)
		}

		// Rebuild the index every few minutes.
		sklog.Infof("Starting indexer to run every %s", *indexInterval)
		ixr, err := indexer.New(storages, *indexInterval)
		if err != nil {
			sklog.Fatalf("Failed to create indexer: %s", err)
		}
		sklog.Infof("Indexer created.")

		searchAPI, err := search.NewSearchAPI(storages, ixr)
		if err != nil {
			sklog.Fatalf("Failed to create instance of search API: %s", err)
		}
		sklog.Infof("Search API created")

		issueTracker := issues.NewMonorailIssueTracker(client, issues.PROJECT_SKIA)

		statusWatcher, err := status.New(storages)
		if err != nil {
			sklog.Fatalf("Failed to initialize status watcher: %s", err)
		}
		sklog.Infof("statusWatcher created")

		handlers = web.WebHandlers{
			Storages:      storages,
			StatusWatcher: statusWatcher,
			Indexer:       ixr,
			IssueTracker:  issueTracker,
			SearchAPI:     searchAPI,
		}

		mainTimer.Stop()
	}()

	// TODO(stephana): Running the setup process in the a parallel go-routine is an ugly hack and the
	// code below to set up a temporary server should also be removed asap.

	// Set up a server that indicates that the server is ready until the
	// startup work is finished.
	readyRouter := mux.NewRouter()
	readyRouter.HandleFunc("/healthz", httputils.ReadyHandleFunc)
	srv := http.Server{Addr: *port, Handler: readyRouter}
	go func() {
		sklog.Infof("Start serving ready endpoint.")
		err := srv.ListenAndServe()
		if err != http.ErrServerClosed {
			sklog.Fatalf("Unexpected error closing ready server: %s", err)
		}
		sklog.Infof("Finished serving ready endpoint.")
	}()

	// Wait until the various storage objects are set up correctly.
	wg.Wait()
	if err := srv.Shutdown(context.Background()); err != nil {
		sklog.Fatalf("Error shutting down ready server: %s", err)
	}

	// loggedRouter contains all the endpoints that are logged. See the call below to
	// LoggingGzipRequestResponse.
	loggedRouter := mux.NewRouter()

	// Set up the resource to serve the image files.
	imgHandler, err := handlers.Storages.DiffStore.ImageHandler(IMAGE_URL_PREFIX)
	if err != nil {
		sklog.Fatalf("Unable to get image handler: %s", err)
	}
	loggedRouter.PathPrefix(IMAGE_URL_PREFIX).Handler(imgHandler)

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
	// requested that doesn't exit. If we did this differently a call to a non-existing endpoint
	// would be handled by the route that handles the returning the index template and make debugging
	// confusing.
	jsonRouter := loggedRouter.PathPrefix("/json").Subrouter()
	trim := func(r string) string { return strings.TrimPrefix(r, "/json") }

	jsonRouter.HandleFunc(trim(shared.KNOWN_HASHES_ROUTE), handlers.TextKnownHashesProxy).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/byblame"), handlers.JsonByBlameHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/cleardigests"), handlers.JsonClearDigests).Methods("POST")
	jsonRouter.HandleFunc(trim("/json/clusterdiff"), handlers.JsonClusterDiffHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/cmp"), handlers.JsonCompareTestHandler).Methods("POST")
	jsonRouter.HandleFunc(trim("/json/commits"), handlers.JsonCommitsHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/details"), handlers.JsonDetailsHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/diff"), handlers.JsonDiffHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/export"), handlers.JsonExportHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/failure"), handlers.JsonListFailureHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/failure/clear"), handlers.JsonClearFailureHandler).Methods("POST")
	jsonRouter.HandleFunc(trim("/json/gitlog"), handlers.JsonGitLogHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/list"), handlers.JsonListTestsHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/paramset"), handlers.JsonParamsHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/search"), handlers.JsonSearchHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/triage"), handlers.JsonTriageHandler).Methods("POST")
	jsonRouter.HandleFunc(trim("/json/triagelog"), handlers.JsonTriageLogHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/triagelog/undo"), handlers.JsonTriageUndoHandler).Methods("POST")
	jsonRouter.HandleFunc(trim("/json/tryjob"), handlers.JsonTryjobListHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/tryjob/{id}"), handlers.JsonTryjobSummaryHandler).Methods("GET")

	// Retrieving that baseline for master and an Gerrit issue are handled the same way
	// These routes can be served with baseline_server for higher availability.
	jsonRouter.HandleFunc(trim(shared.EXPECTATIONS_ROUTE), handlers.JsonBaselineHandler).Methods("GET")
	jsonRouter.HandleFunc(trim(shared.EXPECTATIONS_ISSUE_ROUTE), handlers.JsonBaselineHandler).Methods("GET")

	jsonRouter.HandleFunc(trim("/json/refresh/{id}"), handlers.JsonRefreshIssue).Methods("GET")

	// Only expose these endpoints if login is enforced across the app or this an open site.
	if openSite {
		jsonRouter.HandleFunc(trim("/json/ignores"), handlers.JsonIgnoresHandler).Methods("GET")
		jsonRouter.HandleFunc(trim("/json/ignores/add/"), handlers.JsonIgnoresAddHandler).Methods("POST")
		jsonRouter.HandleFunc(trim("/json/ignores/del/{id}"), handlers.JsonIgnoresDeleteHandler).Methods("POST")
		jsonRouter.HandleFunc(trim("/json/ignores/save/{id}"), handlers.JsonIgnoresUpdateHandler).Methods("POST")
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

	// set up the app router that might be authenticated and logs everything except for the status
	// endpoint which is polled a lot.
	appRouter := mux.NewRouter()
	appRouter.HandleFunc("/json/trstatus", handlers.JsonStatusHandler)
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
	rootRouter.PathPrefix("/").Handler(appHandler)

	// Start the internal server on the internal port if requested.
	if *internalPort != "" {
		// Add the profiling endpoints to the internal router.
		internalRouter := mux.NewRouter()

		// Set up the health check endpoint.
		internalRouter.HandleFunc("/healthz", httputils.ReadyHandleFunc)

		// Register pprof handlers
		internalRouter.HandleFunc("/debug/pprof/", netpprof.Index)
		internalRouter.HandleFunc("/debug/pprof/cmdline", netpprof.Cmdline)
		internalRouter.HandleFunc("/debug/pprof/profile", netpprof.Profile)
		internalRouter.HandleFunc("/debug/pprof/symbol", netpprof.Symbol)
		internalRouter.HandleFunc("/debug/pprof/trace", netpprof.Trace)

		// Add the rest of the application without any authentication that was configured.
		internalRouter.PathPrefix("/").Handler(appRouter)

		go func() {
			sklog.Infof("Internal server on  http://127.0.0.1" + *internalPort)
			sklog.Fatal(http.ListenAndServe(*internalPort, internalRouter))
		}()
	}

	// Start the server
	sklog.Infof("Serving on http://127.0.0.1" + *port)
	sklog.Fatal(http.ListenAndServe(*port, rootRouter))
}
