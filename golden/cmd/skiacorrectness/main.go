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
	"net/http/pprof"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/oauth2"
	gstorage "google.golang.org/api/storage/v1"
	"google.golang.org/grpc"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/go/vcsinfo/bt_vcs"
	"go.skia.org/infra/golden/go/baseline/simple_baseliner"
	"go.skia.org/infra/golden/go/clstore/fs_clstore"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/code_review/commenter"
	"go.skia.org/infra/golden/go/code_review/gerrit_crs"
	"go.skia.org/infra/golden/go/code_review/github_crs"
	"go.skia.org/infra/golden/go/code_review/updater"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/expectations/cleanup"
	"go.skia.org/infra/golden/go/expectations/fs_expectationstore"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/ignore/fs_ignorestore"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/publicparams"
	"go.skia.org/infra/golden/go/search"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/status"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/tilesource"
	"go.skia.org/infra/golden/go/tjstore/fs_tjstore"
	"go.skia.org/infra/golden/go/tracestore/bt_tracestore"
	"go.skia.org/infra/golden/go/warmer"
	"go.skia.org/infra/golden/go/web"
)

const (
	// imgURLPrefix is path prefix used for all images (digests and diffs)
	imgURLPrefix = "/img/"

	// callbackPath is callback endpoint used for the OAuth2 flow
	callbackPath = "/oauth2callback/"

	// everythingPublic can be provided as the value for the publicly viewable file
	// allow all configurations to be visible.
	everythingPublic = "all"
)

var (
	templates *template.Template
)

func main() {
	// Command line flags.
	var (
		appTitle               = flag.String("app_title", "Skia Gold", "Title of the deployed up on the front end.")
		authoritative          = flag.Bool("authoritative", false, "Indicates that this instance can write to known_hashes, update changelist statuses, etc")
		authorizedUsers        = flag.String("auth_users", login.DEFAULT_DOMAIN_WHITELIST, "White space separated list of domains and email addresses that are allowed to login.")
		btInstanceID           = flag.String("bt_instance", "production", "ID of the BigTable instance that contains Git metadata")
		btProjectID            = flag.String("bt_project_id", "skia-public", "project id with BigTable instance")
		changeListTracking     = flag.Bool("changelist_tracking", true, "Should gold track ChangeLists looking for ChangeListExpectations")
		clCommentDryRun        = flag.Bool("cl_comment_dryrun", true, "If we should only log comments")
		clCommentTemplateIndex = flag.Int("cl_comment_template_idx", 0, "An index into a hard-coded list of templates for making comments on CLs.")
		clientSecretFile       = flag.String("client_secret", "", "Client secret file for OAuth2 authentication.")
		crsURLTemplate         = flag.String("crs_url_template", "", "A URL with %s where a CL ID should be placed to complete it.")
		defaultCorpus          = flag.String("default_corpus", "gm", "The corpus identifier shown by default on the frontend.")
		diffServerGRPCAddr     = flag.String("diff_server_grpc", "", "The grpc port of the diff server. 'diff_server_http also needs to be set.")
		diffServerImageAddr    = flag.String("diff_server_http", "", "The images serving address of the diff server. 'diff_server_grpc has to be set as well.")
		flakyTraceThreshold    = flag.Int("flaky_trace_threshold", 10000, "If a trace has more unique digests than this, it will be considered flaky. The default value is high enough such that no trace will be considered flaky.")
		forceLogin             = flag.Bool("force_login", true, "Force the user to be authenticated for all requests.")
		fsNamespace            = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'flutter', 'skia', etc")
		fsProjectID            = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")
		gerritURL              = flag.String("gerrit_url", gerrit.GERRIT_SKIA_URL, "URL of the Gerrit instance where we retrieve CL metadata.")
		gitBTTableID           = flag.String("git_bt_table", "", "ID of the BigTable table that contains Git metadata")
		githubCredPath         = flag.String("github_cred_path", "", "Filepath to file containing GitHub token")
		githubRepo             = flag.String("github_repo", "", "User and repo of GitHub project to connect to, e.g. google/skia")
		gitRepoURL             = flag.String("git_repo_url", "https://skia.googlesource.com/skia", "The URL to pass to git clone for the source repository.")
		hang                   = flag.Bool("hang", false, "If true, just hang and do nothing.")
		indexInterval          = flag.Duration("idx_interval", 5*time.Minute, "Interval at which the indexer calculates the search index.")
		internalPort           = flag.String("internal_port", "", "HTTP service address for internal pprof data. No authentication on this port.")
		knownHashesGCSPath     = flag.String("known_hashes_gcs_path", "", "GCS path, where the known hashes file should be stored. If empty no file will be written. Format: <bucket>/<path>.")
		litHTMLDir             = flag.String("lit_html_dir", "", "File path to build lit-html files")
		local                  = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
		nCommits               = flag.Int("n_commits", 50, "Number of recent commits to include in the analysis.")
		negativesMaxAge        = flag.Duration("negatives_max_age", 0, "The longest time negative expectations can go unused before being purged. (0 means infinity)")
		noCloudLog             = flag.Bool("no_cloud_log", false, "Disables cloud logging. Primarily for running locally and in K8s.")
		port                   = flag.String("port", ":9000", "HTTP service address (e.g., ':9000')")
		positivesMaxAge        = flag.Duration("positives_max_age", 0, "The longest time positive expectations can go unused before being purged. (0 means infinity)")
		primaryCRS             = flag.String("primary_crs", "gerrit", "Primary CodeReviewSystem (e.g. 'gerrit', 'github'")
		promPort               = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
		pubWhiteList           = flag.String("public_whitelist", "", fmt.Sprintf("File name of a JSON5 file that contains a query with the traces to white list. If set to '%s' everything is included. This is required if force_login is false.", everythingPublic))
		redirectURL            = flag.String("redirect_url", "https://gold.skia.org/oauth2callback/", "OAuth2 redirect url. Only used when local=false.")
		resourcesDir           = flag.String("resources_dir", "", "The directory to find Polymer templates, JS, and CSS files.")
		siteURL                = flag.String("site_url", "https://gold.skia.org", "URL where this app is hosted.")
		tileFreshness          = flag.Duration("tile_freshness", time.Minute, "How often to re-fetch the tile")
		traceBTTableID         = flag.String("trace_bt_table", "", "BigTable table ID for the traces.")
	)
	// Parse the options. So we can configure logging.
	flag.Parse()

	if *hang {
		sklog.Infof("--hang provided; doing nothing.")
		httputils.RunHealthCheckServer(*port)
	}

	// Speculative memory usage fix? https://github.com/googleapis/google-cloud-go/issues/375
	grpc.EnableTracing = false

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

	// Start the internal server on the internal port if requested.
	if *internalPort != "" {
		// Add the profiling endpoints to the internal router.
		internalRouter := mux.NewRouter()

		// Set up the health check endpoint.
		internalRouter.HandleFunc("/healthz", httputils.ReadyHandleFunc)

		// Register pprof handlers
		internalRouter.HandleFunc("/debug/pprof/", pprof.Index)
		internalRouter.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		internalRouter.HandleFunc("/debug/pprof/profile", pprof.Profile)
		internalRouter.HandleFunc("/debug/pprof/{profile}", pprof.Index)

		go func() {
			sklog.Infof("Internal server on http://127.0.0.1" + *internalPort)
			sklog.Fatal(http.ListenAndServe(*internalPort, internalRouter))
		}()
	}

	if *resourcesDir == "" || *litHTMLDir == "" {
		sklog.Fatal("You must specify both --resource_dir and --lit_html_dir")
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

	// Get the token source for the service account with access to the services
	// we need to operate.
	tokenSource, err := auth.NewDefaultTokenSource(*local, auth.SCOPE_USERINFO_EMAIL, gstorage.CloudPlatformScope, auth.SCOPE_GERRIT)
	if err != nil {
		sklog.Fatalf("Failed to authenticate service account: %s", err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client()

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

		diffStore, err = diffstore.NewNetDiffStore(ctx, conn, *diffServerImageAddr)
		if err != nil {
			sklog.Fatalf("Unable to initialize NetDiffStore: %s", err)
		}
		sklog.Infof("DiffStore: NetDiffStore initiated.")
	} else {
		sklog.Fatalf("Must specify --diff_server_http and --diff_server_grpc")
	}

	var vcs vcsinfo.VCS
	if *btInstanceID != "" && *gitBTTableID != "" {
		if *local {
			appName = bt.TestingAppProfile
		}
		btConf := &bt_gitstore.BTConfig{
			ProjectID:  *btProjectID,
			InstanceID: *btInstanceID,
			TableID:    *gitBTTableID,
			AppProfile: appName,
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

		gitilesRepo := gitiles.NewRepo("", nil)
		bvcs, err := bt_vcs.New(ctx, gitStore, "master", gitilesRepo)
		if err != nil {
			sklog.Fatalf("Error creating BT-backed VCS instance: %s", err)
		}
		vcs = bvcs
	} else {
		sklog.Fatal("You must specify --bt_instance and --git_bt_table")
	}

	if *traceBTTableID == "" {
		sklog.Fatal("You must specify --trace_bt_table")
	}

	btc := bt_tracestore.BTConfig{
		ProjectID:  *btProjectID,
		InstanceID: *btInstanceID,
		TableID:    *traceBTTableID,
		VCS:        vcs,
	}

	err = bt_tracestore.InitBT(ctx, btc)
	if err != nil {
		sklog.Fatalf("Could not initialize BigTable tracestore with config %#v: %s", btc, err)
	}

	traceStore, err := bt_tracestore.New(ctx, btc, false)
	if err != nil {
		sklog.Fatalf("Could not instantiate BT tracestore: %s", err)
	}

	gsClientOpt := storage.GCSClientOptions{
		KnownHashesGCSPath: *knownHashesGCSPath,
		Dryrun:             !*authoritative,
	}

	gsClient, err := storage.NewGCSClient(ctx, client, gsClientOpt)
	if err != nil {
		sklog.Fatalf("Unable to create GCSClient: %s", err)
	}

	if *fsNamespace == "" {
		sklog.Fatalf("--fs_namespace must be set")
	}

	// Auth note: the underlying firestore.NewClient looks at the
	// GOOGLE_APPLICATION_CREDENTIALS env variable, so we don't need to supply
	// a token source.
	fsClient, err := firestore.NewClient(ctx, *fsProjectID, "gold", *fsNamespace, nil)
	if err != nil {
		sklog.Fatalf("Unable to configure Firestore: %s", err)
	}

	// Set up the cloud expectations store
	expChangeHandler := expectations.NewEventDispatcher()
	expStore := fs_expectationstore.New(fsClient, expChangeHandler, fs_expectationstore.ReadWrite)
	if err := expStore.Initialize(ctx); err != nil {
		sklog.Fatalf("Unable to initialize fs_expstore: %s", err)
	}

	baseliner := simple_baseliner.New(expStore)

	var publiclyViewableParams publicparams.Matcher
	// Load the publiclyViewable params if configured and disable querying for issues.
	if *pubWhiteList != "" && *pubWhiteList != everythingPublic {
		if publiclyViewableParams, err = loadParamFile(*pubWhiteList); err != nil {
			sklog.Fatalf("Could not load list of public params: %s", err)
		}
	}

	// Check if this is public instance. If so, make sure a list of public params
	// has been specified - can be everythingPublic.
	if !*forceLogin && *pubWhiteList == "" {
		sklog.Fatalf("Empty whitelist file. A non-empty white list must be provided if force_login=false.")
	}

	// openSite indicates whether this can expose all end-points. The user still has to be authenticated.
	openSite := (*pubWhiteList == everythingPublic) || *forceLogin

	ignoreStore := fs_ignorestore.New(ctx, fsClient)

	if err := ignore.StartMetrics(ctx, ignoreStore, *tileFreshness); err != nil {
		sklog.Fatalf("Failed to start monitoring for expired ignore rules: %s", err)
	}

	cls := fs_clstore.New(fsClient, *primaryCRS)
	tjs := fs_tjstore.New(fsClient)

	var crs code_review.Client
	if *primaryCRS == "gerrit" {
		gerritClient, err := gerrit.NewGerrit(*gerritURL, client)
		if err != nil {
			sklog.Fatalf("Could not create gerrit client for %s", *gerritURL)
		}
		crs = gerrit_crs.New(gerritClient)
	} else if *primaryCRS == "github" {
		if *githubRepo == "" || *githubCredPath == "" {
			sklog.Fatalf("You must specify --github_repo and --github_cred_path")
		}
		gBody, err := ioutil.ReadFile(*githubCredPath)
		if err != nil {
			sklog.Fatalf("Couldn't find githubToken in %s: %s", *githubCredPath, err)
		}
		gToken := strings.TrimSpace(string(gBody))
		githubTS := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: gToken})
		c := httputils.DefaultClientConfig().With2xxOnly().WithTokenSource(githubTS).Client()
		crs = github_crs.New(c, *githubRepo)
	} else {
		sklog.Warningf("CRS %s not supported, tracking ChangeLists is disabled", *primaryCRS)
	}

	var clUpdater code_review.ChangeListLandedUpdater
	if *authoritative && crs != nil && *changeListTracking {
		clUpdater = updater.New(crs, expStore, cls)
	}

	ctc := tilesource.CachedTileSourceConfig{
		CLUpdater:              clUpdater,
		IgnoreStore:            ignoreStore,
		NCommits:               *nCommits,
		PubliclyViewableParams: publiclyViewableParams,
		TraceStore:             traceStore,
		VCS:                    vcs,
	}

	tileSource := tilesource.New(ctc)
	sklog.Infof("Fetching tile")
	// Blocks until tile is fetched
	err = tileSource.StartUpdater(ctx, 2*time.Minute)
	if err != nil {
		sklog.Fatalf("Could not fetch initial tile: %s", err)
	}

	ic := indexer.IndexerConfig{
		DiffStore:         diffStore,
		ChangeListener:    expChangeHandler,
		ExpectationsStore: expStore,
		GCSClient:         gsClient,
		TileSource:        tileSource,
		Warmer:            warmer.New(),
		TryJobStore:       tjs,
		CLStore:           cls,
	}

	// Rebuild the index every few minutes.
	sklog.Infof("Starting indexer to run every %s", *indexInterval)
	ixr, err := indexer.New(ctx, ic, *indexInterval)
	if err != nil {
		sklog.Fatalf("Failed to create indexer: %s", err)
	}
	sklog.Infof("Indexer created.")

	// TODO(kjlubick) include non-nil comment.Store when it is implemented.
	searchAPI := search.New(diffStore, expStore, expChangeHandler, ixr, cls, tjs, nil, publiclyViewableParams, *flakyTraceThreshold)

	sklog.Infof("Search API created")

	if *authoritative && crs != nil && *changeListTracking {
		clCommenter := commenter.New(crs, cls, searchAPI, clTemplates[*clCommentTemplateIndex], *siteURL, *clCommentDryRun)
		startCommenter(ctx, clCommenter)
	}

	swc := status.StatusWatcherConfig{
		VCS:               vcs,
		ChangeListener:    expChangeHandler,
		TileSource:        tileSource,
		ExpectationsStore: expStore,
	}

	statusWatcher, err := status.New(ctx, swc)
	if err != nil {
		sklog.Fatalf("Failed to initialize status watcher: %s", err)
	}
	sklog.Infof("statusWatcher created")

	// reminder: this exp will be updated whenever expectations change.
	exp, err := expStore.Get(ctx)
	if err != nil {
		sklog.Fatalf("Failed to get master-branch expectations: %s", err)
	}

	if *authoritative {
		policy := cleanup.Policy{
			PositiveMaxLastUsed: *positivesMaxAge,
			NegativeMaxLastUsed: *negativesMaxAge,
		}
		if err := cleanup.Start(ctx, ixr, expStore, exp, policy); err != nil {
			sklog.Fatalf("Could not start expectation cleaning process %s", err)
		}
	}

	handlers, err := web.NewHandlers(web.HandlersConfig{
		Baseliner:             baseliner,
		ChangeListStore:       cls,
		CodeReviewURLTemplate: *crsURLTemplate,
		DiffStore:             diffStore,
		ExpectationsStore:     expStore,
		GCSClient:             gsClient,
		IgnoreStore:           ignoreStore,
		Indexer:               ixr,
		SearchAPI:             searchAPI,
		StatusWatcher:         statusWatcher,
		TileSource:            tileSource,
		TryJobStore:           tjs,
		VCS:                   vcs,
	}, web.FullFrontEnd)
	if err != nil {
		sklog.Fatalf("Failed to initialize web handlers: %s", err)
	}

	mainTimer.Stop()

	// loggedRouter contains all the endpoints that are logged. See the call below to
	// LoggingGzipRequestResponse.
	loggedRouter := mux.NewRouter()

	// Set up the resource to serve the image files.
	imgHandler, err := diffStore.ImageHandler(imgURLPrefix)
	if err != nil {
		sklog.Fatalf("Unable to get image handler: %s", err)
	}

	// Legacy Polymer based UI endpoint
	loggedRouter.PathPrefix("/res/").HandlerFunc(web.MakeResourceHandler(*resourcesDir))
	// lit-html based UI endpoint.
	loggedRouter.PathPrefix("/dist/").HandlerFunc(web.MakeResourceHandler(*litHTMLDir))
	loggedRouter.HandleFunc(callbackPath, login.OAuth2CallbackHandler)

	loggedRouter.HandleFunc("/json/version", skiaversion.JsonHandler)
	loggedRouter.HandleFunc("/loginstatus/", login.StatusHandler)
	loggedRouter.HandleFunc("/logout/", login.LogoutHandler)

	// Set up a subrouter for the '/json' routes which make up the Gold API.
	// This makes routing faster, but also returns a failure when an /json route is
	// requested that doesn't exist. If we did this differently a call to a non-existing endpoint
	// would be handled by the route that handles the returning the index template and make
	// debugging confusing.
	jsonRouter := loggedRouter.PathPrefix("/json").Subrouter()
	trim := func(r string) string { return strings.TrimPrefix(r, "/json") }

	jsonRouter.HandleFunc(trim(shared.KnownHashesRoute), handlers.TextKnownHashesProxy).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/byblame"), handlers.ByBlameHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/clusterdiff"), handlers.ClusterDiffHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/commits"), handlers.CommitsHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/details"), handlers.DetailsHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/diff"), handlers.DiffHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/export"), handlers.ExportHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/list"), handlers.ListTestsHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/list2"), handlers.ListTestsHandler2).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/paramset"), handlers.ParamsHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/search"), handlers.SearchHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/triage"), handlers.TriageHandler).Methods("POST")
	jsonRouter.HandleFunc(trim("/json/triagelog"), handlers.TriageLogHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/triagelog/undo"), handlers.TriageUndoHandler).Methods("POST")
	jsonRouter.HandleFunc(trim("/json/changelists"), handlers.ChangeListsHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/changelist/{system}/{id}"), handlers.ChangeListSummaryHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/changelist/{system}/{id}/{patchset}/untriaged"), handlers.ChangeListUntriagedHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/digests"), handlers.DigestListHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/whoami"), handlers.Whoami).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/latestpositivedigest/{traceId}"), handlers.LatestPositiveDigestHandler).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/debug/digestsbytestname/{corpus}/{testName}"), handlers.GetPerTraceDigestsByTestName).Methods("GET")
	jsonRouter.HandleFunc(trim("/json/debug/flakytraces/{minUniqueDigests}"), handlers.GetFlakyTracesData).Methods("GET")

	// Retrieving that baseline for master and an Gerrit issue are handled the same way
	// These routes can be served with baseline_server for higher availability.
	jsonRouter.HandleFunc(trim(shared.ExpectationsRoute), handlers.BaselineHandler).Methods("GET")
	// TODO(lovisolo): Remove the below route once goldctl is fully migrated.
	jsonRouter.HandleFunc(trim(shared.ExpectationsLegacyRoute), handlers.BaselineHandler).Methods("GET")

	// Only expose these endpoints if login is enforced across the app or this an open site.
	if openSite {
		jsonRouter.HandleFunc(trim("/json/ignores"), handlers.ListIgnoreRules).Methods("GET")
		jsonRouter.HandleFunc(trim("/json/ignores/add/"), handlers.AddIgnoreRule).Methods("POST")
		jsonRouter.HandleFunc(trim("/json/ignores/del/{id}"), handlers.DeleteIgnoreRule).Methods("POST")
		jsonRouter.HandleFunc(trim("/json/ignores/save/{id}"), handlers.UpdateIgnoreRule).Methods("POST")
	}

	// Make sure we return a 404 for anything that starts with /json and could not be found.
	jsonRouter.HandleFunc("/{ignore:.*}", http.NotFound)
	loggedRouter.HandleFunc("/json", http.NotFound)

	loadTemplates := func() {
		templates = template.Must(template.New("").ParseFiles(filepath.Join(*resourcesDir, "index.html")))
		templates = template.Must(templates.ParseGlob(filepath.Join(*litHTMLDir, "dist", "*.html")))
	}

	loadTemplates()

	// appConfig is injected into the header of the index file.
	appConfig := &struct {
		BaseRepoURL        string `json:"baseRepoURL"`
		CodeReviewTemplate string `json:"crsTemplate"`
		DefaultCorpus      string `json:"defaultCorpus"`
		Title              string `json:"title"`
		IsPublic           bool   `json:"isPublic"` // If true this is not open but restrictions apply.
	}{
		BaseRepoURL:        *gitRepoURL,
		CodeReviewTemplate: *crsURLTemplate,
		DefaultCorpus:      *defaultCorpus,
		Title:              *appTitle,
		IsPublic:           !openSite,
	}

	templateHandler := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			httputils.AddOriginTrialHeader(w, *local)

			// Reload the template if we are running locally.
			if *local {
				loadTemplates()
			}
			if err := templates.ExecuteTemplate(w, name, appConfig); err != nil {
				sklog.Errorf("Failed to expand template %s : %s", name, err)
				return
			}
		}
	}

	// These are the new lit-html pages.
	loggedRouter.HandleFunc("/", templateHandler("byblame.html"))
	loggedRouter.HandleFunc("/changelists", templateHandler("changelists.html"))
	loggedRouter.HandleFunc("/triagelog", templateHandler("triagelog.html"))
	loggedRouter.HandleFunc("/ignores", templateHandler("ignorelist.html"))
	loggedRouter.HandleFunc("/diff", templateHandler("diff.html"))
	loggedRouter.HandleFunc("/detail", templateHandler("details.html"))
	loggedRouter.HandleFunc("/details", templateHandler("details.html"))
	loggedRouter.HandleFunc("/cl/{system}/{id}", handlers.ChangeListSearchRedirect)
	loggedRouter.HandleFunc("/list", templateHandler("by_test_list.html"))

	// This route handles the legacy polymer "single page" app model
	loggedRouter.PathPrefix("/").Handler(templateHandler("index.html"))

	// set up the app router that might be authenticated and logs almost everything.
	appRouter := mux.NewRouter()
	// Images should not be served gzipped, which can sometimes have issues
	// when serving an image from a NetDiffstore with HTTP2. Additionally, is wasteful
	// given PNGs typically have zlib compression anyway.
	appRouter.PathPrefix(imgURLPrefix).Handler(imgHandler)
	appRouter.PathPrefix("/").Handler(httputils.LoggingGzipRequestResponse(loggedRouter))

	// Use the appRouter as a handler and wrap it into middleware that enforces authentication if
	// necessary it was requested via the force_login flag.
	appHandler := http.Handler(appRouter)
	if *forceLogin {
		appHandler = login.ForceAuth(appRouter, callbackPath)
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

// startCommenter begins the background process that comments on CLs.
func startCommenter(ctx context.Context, cmntr code_review.ChangeListCommenter) {
	go func() {
		// TODO(kjlubick): tune this time, maybe make it a flag
		util.RepeatCtx(ctx, 3*time.Minute, func(ctx context.Context) {
			err := cmntr.CommentOnChangeListsWithUntriagedDigests(ctx)
			if err != nil {
				sklog.Errorf("Could not comment on CLs with Untriaged Digests: %s", err)
			}
		})
	}()
}

// loadParamFile loads the given JSON5 file that defines the query to
// make traces publicly viewable. If the given file is empty or otherwise
// cannot be parsed an error will be returned.
func loadParamFile(fName string) (publicparams.Matcher, error) {
	b, err := ioutil.ReadFile(fName)
	if err != nil {
		return nil, skerr.Wrapf(err, "reading file %s", fName)
	}
	sklog.Infof("publicly viewable params loaded from %s", fName)
	sklog.Debugf("%s", string(b))

	return publicparams.MatcherFromJSON(b)
}

const basicCLTemplate = `Gold has detected about %d untriaged digest(s) on patchset %d.
View them at %s/cl/%s/%s`

const chromeCLTemplate = `Gold has detected about %d untriaged digest(s) on patchset %d.

Please triage them at %s/cl/%s/%s before submitting.

If this is due to a failure in a gtest-based test, this functionality is currently experimental and won't block your CL.
If all the trybots passed and you don't expect your CL to have any effect on browser UI, you can likely ignore this message.
See the FAQ for more information: https://docs.google.com/document/d/1BnwcxzhT8FFvY3YF-6BT4Mqgrb9U40t0HMfEVSSEpNs/edit?usp=sharing
`

// TODO(kjlubick) When this stops being practical (i.e. more than 2 custom messages), add a
//  config file that is read in on boot. This can include the public params, authorized users and
//  maybe other keys as well. I think perf uses some third-party library for this.
var clTemplates = []string{basicCLTemplate, chromeCLTemplate}
