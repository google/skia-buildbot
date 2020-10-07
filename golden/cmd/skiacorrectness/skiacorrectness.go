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
	"go.skia.org/infra/go/metrics2"
	"golang.org/x/oauth2"
	gstorage "google.golang.org/api/storage/v1"
	"google.golang.org/grpc"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo/bt_vcs"
	"go.skia.org/infra/golden/go/baseline/simple_baseliner"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/clstore/fs_clstore"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/code_review/commenter"
	"go.skia.org/infra/golden/go/code_review/gerrit_crs"
	"go.skia.org/infra/golden/go/code_review/github_crs"
	"go.skia.org/infra/golden/go/code_review/updater"
	"go.skia.org/infra/golden/go/config"
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
)

var (
	templates *template.Template
)

type frontendServerConfig struct {
	config.Common

	// A list of email addresses or domains that can log into this instance.
	AuthorizedUsers []string `json:"authorized_users"`

	// A string with placeholders for generating a comment message. See
	// commenter.commentTemplateContext for the exact fields.
	CLCommentTemplate string `json:"cl_comment_template"`

	// Client secret file for OAuth2 authentication.
	ClientSecretFile string `json:"client_secret_file"`

	// If true, Gold will only log comments, it won't actually comment on the CRSes.
	DisableCLComments bool `json:"disable_cl_comments"`

	// The grpc port of the diff server.
	DiffServerGRPC string `json:"diff_server_grpc"`

	// The images serving address of the diff server.
	DiffServerHTTP string `json:"diff_server_http"`

	// If the frontend shouldn't track any CLs. For example, if we are tracking a repo that doesn't
	// have a CQ.
	DisableCLTracking bool `json:"disable_changelist_tracking"`

	// If a trace has more unique digests than this, it will be considered flaky. If this number is
	// greater than NumCommits, then no trace can ever be flaky.
	FlakyTraceThreshold int `json:"flaky_trace_threshold"`

	// Force the user to be authenticated for all requests.
	ForceLogin bool `json:"force_login"`

	// Configuration settings that will get passed to the frontend (see modules/settings.js)
	FrontendConfig frontendConfig `json:"frontend"`

	// If this instance is simply a mirror of another instance's data.
	IsPublicView bool `json:"is_public_view"`

	// File path to built lit-html files that should be served as part of the frontend.
	LitHTMLPath string `json:"lit_html_path"`

	// The longest time negative expectations can go unused before being purged. (0 means infinity)
	NegativesMaxAge config.Duration `json:"negatives_max_age" optional:"true"`

	// Number of recent commits to include in the slideing window of data analysis. Also called the
	// tile size.
	NumCommits int `json:"num_commits"`

	// HTTP service address (e.g., ':9000')
	Port string `json:"port"`

	// The longest time positive expectations can go unused before being purged. (0 means infinity)
	PositivesMaxAge config.Duration `json:"positives_max_age" optional:"true"`

	// Metrics service address (e.g., ':20000')
	PromPort string `json:"prom_port"`

	// If non empty, this map of rules will be applied to traces to see if they can be showed on
	// this instance.
	PubliclyAllowableParams publicparams.MatchingRules `json:"publicly_allowed_params" optional:"true"`

	// This can be used in a CL comment to direct users to the public instance for triaging.
	PublicSiteURL string `json:"public_site_url" optional:"true"`

	// The path to the directory that contains Polymer templates, JS, and CSS files.
	ResourcesPath string `json:"resources_path"`

	// URL where this app is hosted.
	SiteURL string `json:"site_url"`

	// How often to re-fetch the tile, compute the index, and report metrics about the index.
	TileFreshness config.Duration `json:"tile_freshness"`

	// BigTable table ID for the traces.
	TraceBTTable string `json:"trace_bt_table"`

	// If true, Gold will serve the lit-html version of the search page on /search, otherwise it will
	// serve the legacy, Polymer-based version of said page.
	// TODO(lovisolo): Delete this after the legacy search page has been removed.
	UseLitHTMLSearchPage bool `json:"use_lit_html_search_page"`
}

func main() {
	// Command line flags.
	var (
		commonInstanceConfig = flag.String("common_instance_config", "", "Path to the json5 file containing the configuration that needs to be the same across all services for a given instance.")
		thisConfig           = flag.String("config", "", "Path to the json5 file containing the configuration specific to baseline server.")
		hang                 = flag.Bool("hang", false, "Stop and do nothing after reading the flags. Good for debugging containers.")
	)
	// Parse the flags, so we can load the configuration files.
	flag.Parse()

	if *hang {
		sklog.Info("Hanging")
		select {}
	}

	var fsc frontendServerConfig
	if err := config.LoadFromJSON5(&fsc, commonInstanceConfig, thisConfig); err != nil {
		sklog.Fatalf("Reading config: %s", err)
	}
	sklog.Infof("Loaded config %#v", fsc)

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
		common.PrometheusOpt(&fsc.PromPort),
	}

	_, appName := filepath.Split(os.Args[0])
	common.InitWithMust(appName, logOpts...)

	ctx := context.Background()

	// Start the internal server on the internal port if requested.
	if fsc.DebugPort != "" {
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
			sklog.Infof("Internal server on http://127.0.0.1" + fsc.DebugPort)
			sklog.Fatal(http.ListenAndServe(fsc.DebugPort, internalRouter))
		}()
	}

	// Set up login
	redirectURL := fsc.SiteURL + "/oauth2callback/"
	if fsc.Local {
		redirectURL = fmt.Sprintf("http://localhost%s/oauth2callback/", fsc.Port)
	}
	sklog.Infof("The allowed list of users is: %q", fsc.AuthorizedUsers)
	if err := login.Init(redirectURL, strings.Join(fsc.AuthorizedUsers, " "), fsc.ClientSecretFile); err != nil {
		sklog.Fatalf("Failed to initialize the login system: %s", err)
	}

	// Get the token source for the service account with access to the services
	// we need to operate.
	tokenSource, err := auth.NewDefaultTokenSource(fsc.Local, auth.SCOPE_USERINFO_EMAIL, gstorage.CloudPlatformScope, auth.SCOPE_GERRIT)
	if err != nil {
		sklog.Fatalf("Failed to authenticate service account: %s", err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client()

	// Create the client connection and connect to the server.
	conn, err := grpc.Dial(fsc.DiffServerGRPC,
		grpc.WithInsecure(),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallSendMsgSize(diffstore.MAX_MESSAGE_SIZE),
			grpc.MaxCallRecvMsgSize(diffstore.MAX_MESSAGE_SIZE)))
	if err != nil {
		sklog.Fatalf("Unable to connect to grpc service: %s", err)
	}

	diffStore, err := diffstore.NewNetDiffStore(ctx, conn, fsc.DiffServerHTTP)
	if err != nil {
		sklog.Fatalf("Unable to initialize NetDiffStore: %s", err)
	}
	sklog.Infof("DiffStore: NetDiffStore initiated.")

	if fsc.Local {
		appName = bt.TestingAppProfile
	}
	btConf := &bt_gitstore.BTConfig{
		InstanceID: fsc.BTInstance,
		ProjectID:  fsc.BTProjectID,
		TableID:    fsc.GitBTTable,
		AppProfile: appName,
	}

	gitStore, err := bt_gitstore.New(ctx, btConf, fsc.GitRepoURL)
	if err != nil {
		sklog.Fatalf("Error instantiating gitstore: %s", err)
	}

	// TODO(kjlubick): remove gitilesRepo and the GetFile() from vcsinfo (unused and
	//  leaky abstraction).
	gitilesRepo := gitiles.NewRepo("", nil)
	vcs, err := bt_vcs.New(ctx, gitStore, git.DefaultBranch, gitilesRepo)
	if err != nil {
		sklog.Fatalf("Error creating BT-backed VCS instance: %s", err)
	}

	btc := bt_tracestore.BTConfig{
		InstanceID: fsc.BTInstance,
		ProjectID:  fsc.BTProjectID,
		TableID:    fsc.TraceBTTable,
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

	// Indicates that this instance can write to known_hashes, update changelist statuses, etc
	isAuthoritative := !fsc.Local && !fsc.IsPublicView

	gsClientOpt := storage.GCSClientOptions{
		KnownHashesGCSPath: fsc.KnownHashesGCSPath,
		Dryrun:             !isAuthoritative,
	}

	gsClient, err := storage.NewGCSClient(ctx, client, gsClientOpt)
	if err != nil {
		sklog.Fatalf("Unable to create GCSClient: %s", err)
	}

	// Auth note: the underlying firestore.NewClient looks at the
	// GOOGLE_APPLICATION_CREDENTIALS env variable, so we don't need to supply
	// a token source.
	fsClient, err := firestore.NewClient(ctx, fsc.FirestoreProjectID, "gold", fsc.FirestoreNamespace, nil)
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
	if len(fsc.PubliclyAllowableParams) > 0 {
		if publiclyViewableParams, err = publicparams.MatcherFromRules(fsc.PubliclyAllowableParams); err != nil {
			sklog.Fatalf("Could not load list of public params: %s", err)
		}
	}

	// Check if this is public instance. If so, make sure we have a non-nil Matcher.
	if fsc.IsPublicView && publiclyViewableParams == nil {
		sklog.Fatal("A non-empty map of publiclyViewableParams must be provided if is public view.")
	}

	ignoreStore := fs_ignorestore.New(ctx, fsClient)

	if err := ignore.StartMetrics(ctx, ignoreStore, fsc.TileFreshness.Duration); err != nil {
		sklog.Fatalf("Failed to start monitoring for expired ignore rules: %s", err)
	}

	tjs := fs_tjstore.New(fsClient)
	reviewSystems, err := initializeReviewSystems(fsc.CodeReviewSystems, fsClient, client)
	if err != nil {
		sklog.Fatalf("Could not initialize CRS: %s", err)
	}

	var clUpdater code_review.ChangeListLandedUpdater
	if isAuthoritative && !fsc.DisableCLTracking {
		clUpdater = updater.New(expStore, reviewSystems)
	}

	ctc := tilesource.CachedTileSourceConfig{
		CLUpdater:              clUpdater,
		IgnoreStore:            ignoreStore,
		NCommits:               fsc.NumCommits,
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
		ChangeListener:    expChangeHandler,
		DiffStore:         diffStore,
		ExpectationsStore: expStore,
		GCSClient:         gsClient,
		ReviewSystems:     reviewSystems,
		TileSource:        tileSource,
		TryJobStore:       tjs,
		Warmer:            warmer.New(),
	}

	// Rebuild the index every few minutes.
	sklog.Infof("Starting indexer to run every %s", fsc.TileFreshness)
	ixr, err := indexer.New(ctx, ic, fsc.TileFreshness.Duration)
	if err != nil {
		sklog.Fatalf("Failed to create indexer: %s", err)
	}
	sklog.Infof("Indexer created.")

	// TODO(kjlubick) include non-nil comment.Store when it is implemented.
	searchAPI := search.New(diffStore, expStore, expChangeHandler, ixr, reviewSystems, tjs, nil, publiclyViewableParams, fsc.FlakyTraceThreshold)

	sklog.Infof("Search API created")

	if isAuthoritative && !fsc.DisableCLTracking {
		for _, rs := range reviewSystems {
			clCommenter, err := commenter.New(rs, searchAPI, fsc.CLCommentTemplate, fsc.SiteURL, fsc.PublicSiteURL, fsc.DisableCLComments)
			if err != nil {
				sklog.Fatalf("Could not initialize commenter: %s", err)
			}
			startCommenter(ctx, clCommenter)
		}
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

	if isAuthoritative {
		policy := cleanup.Policy{
			PositiveMaxLastUsed: fsc.PositivesMaxAge.Duration,
			NegativeMaxLastUsed: fsc.NegativesMaxAge.Duration,
		}
		if err := cleanup.Start(ctx, ixr, expStore, exp, policy); err != nil {
			sklog.Fatalf("Could not start expectation cleaning process %s", err)
		}
	}

	handlers, err := web.NewHandlers(web.HandlersConfig{
		Baseliner:         baseliner,
		DiffStore:         diffStore,
		ExpectationsStore: expStore,
		GCSClient:         gsClient,
		IgnoreStore:       ignoreStore,
		Indexer:           ixr,
		ReviewSystems:     reviewSystems,
		SearchAPI:         searchAPI,
		StatusWatcher:     statusWatcher,
		TileSource:        tileSource,
		TryJobStore:       tjs,
		VCS:               vcs,
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
	loggedRouter.PathPrefix("/res/").HandlerFunc(web.MakeResourceHandler(fsc.ResourcesPath))
	// lit-html based UI endpoint.
	loggedRouter.PathPrefix("/dist/").HandlerFunc(web.MakeResourceHandler(fsc.LitHTMLPath))
	loggedRouter.HandleFunc(callbackPath, login.OAuth2CallbackHandler)

	loggedRouter.HandleFunc("/loginstatus/", login.StatusHandler)
	loggedRouter.HandleFunc("/logout/", login.LogoutHandler)

	// Set up a subrouter for the '/json' routes which make up the Gold API.
	// This makes routing faster, but also returns a failure when an /json route is
	// requested that doesn't exist. If we did this differently a call to a non-existing endpoint
	// would be handled by the route that handles the returning the index template and make
	// debugging confusing.
	jsonRouter := loggedRouter.PathPrefix("/json").Subrouter()

	v1JSON := func(rpcRoute string, handlerFunc http.HandlerFunc) *mux.Route {
		return versionedRPC(rpcRoute, "v1", handlerFunc, jsonRouter, false /*=isAuthenticated*/)
	}

	v2JSON := func(rpcRoute string, handlerFunc http.HandlerFunc) *mux.Route {
		return versionedRPC(rpcRoute, "v2", handlerFunc, jsonRouter, false /*=isAuthenticated*/)
	}

	v0("/byblame", handlers.ByBlameHandler, jsonRouter).Methods("GET")
	v1JSON("/byblame", handlers.ByBlameHandler).Methods("GET")
	v0("/changelists", handlers.ChangeListsHandler, jsonRouter).Methods("GET")
	v1JSON("/changelists", handlers.ChangeListsHandler).Methods("GET")
	v0("/clusterdiff", handlers.ClusterDiffHandler, jsonRouter).Methods("GET")
	v1JSON("/clusterdiff", handlers.ClusterDiffHandler).Methods("GET")
	v0("/commits", handlers.CommitsHandler, jsonRouter).Methods("GET")
	v1JSON("/commits", handlers.CommitsHandler).Methods("GET")
	v0("/debug/digestsbytestname/{corpus}/{testName}", handlers.GetPerTraceDigestsByTestName, jsonRouter).Methods("GET")
	v1JSON("/debug/digestsbytestname/{corpus}/{testName}", handlers.GetPerTraceDigestsByTestName).Methods("GET")
	v0("/debug/flakytraces/{minUniqueDigests}", handlers.GetFlakyTracesData, jsonRouter).Methods("GET")
	v1JSON("/debug/flakytraces/{minUniqueDigests}", handlers.GetFlakyTracesData).Methods("GET")
	v0("/details", handlers.DetailsHandler, jsonRouter).Methods("GET")
	v1JSON("/details", handlers.DetailsHandler).Methods("GET")
	v0("/diff", handlers.DiffHandler, jsonRouter).Methods("GET")
	v1JSON("/diff", handlers.DiffHandler).Methods("GET")
	v0("/digests", handlers.DigestListHandler, jsonRouter).Methods("GET")
	v1JSON("/digests", handlers.DigestListHandler).Methods("GET")
	v0("/export", handlers.ExportHandler, jsonRouter).Methods("GET")
	v1JSON("/export", handlers.ExportHandler).Methods("GET")
	v0("/latestpositivedigest/{traceId}", handlers.LatestPositiveDigestHandler, jsonRouter).Methods("GET")
	v1JSON("/latestpositivedigest/{traceId}", handlers.LatestPositiveDigestHandler).Methods("GET")
	v0("/list", handlers.ListTestsHandler, jsonRouter).Methods("GET")
	v1JSON("/list", handlers.ListTestsHandler).Methods("GET")
	v0("/paramset", handlers.ParamsHandler, jsonRouter).Methods("GET")
	v1JSON("/paramset", handlers.ParamsHandler).Methods("GET")
	v0("/search", handlers.SearchHandler, jsonRouter).Methods("GET")
	v1JSON("/search", handlers.SearchHandler).Methods("GET")
	v0("/triage", handlers.TriageHandler, jsonRouter).Methods("POST")
	v1JSON("/triage", handlers.TriageHandler).Methods("POST")
	v0("/triagelog", handlers.TriageLogHandler, jsonRouter).Methods("GET")
	v1JSON("/triagelog", handlers.TriageLogHandler).Methods("GET")
	v0("/triagelog/undo", handlers.TriageUndoHandler, jsonRouter).Methods("POST")
	v1JSON("/triagelog/undo", handlers.TriageUndoHandler).Methods("POST")
	v0("/whoami", handlers.Whoami, jsonRouter).Methods("GET")
	v1JSON("/whoami", handlers.Whoami).Methods("GET")

	// We trim the fulled shared route to fit the pattern of the rest of these routes.
	trim := func(r string) string {
		r = strings.TrimPrefix(r, "/json")
		r = strings.TrimPrefix(r, "/v1")
		r = strings.TrimPrefix(r, "/v2")
		return r
	}
	// Routes shared with the baseline server. These usually don't see traffic because the envoy
	// routing directs these requests to the baseline servers, if there are some.
	v0(trim(shared.KnownHashesRoute), handlers.TextKnownHashesProxy, jsonRouter).Methods("GET")
	v1JSON(trim(shared.KnownHashesRouteV1), handlers.TextKnownHashesProxy).Methods("GET")
	// Retrieving that baseline for master and an Gerrit issue are handled the same way
	// These routes can be served with baseline_server for higher availability.
	v0(trim(shared.ExpectationsRoute), handlers.BaselineHandlerV1, jsonRouter).Methods("GET")
	v1JSON(trim(shared.ExpectationsRouteV1), handlers.BaselineHandlerV1).Methods("GET")
	v2JSON(trim(shared.ExpectationsRouteV2), handlers.BaselineHandlerV2).Methods("GET")
	// TODO(lovisolo): Remove the below route once goldctl is fully migrated.
	v0(trim(shared.ExpectationsLegacyRoute), handlers.BaselineHandlerV1, jsonRouter).Methods("GET")

	// Only expose these endpoints if this instance is not a public view. The reason we want to hide
	// ignore rules is so that we don't leak params that might be in them.
	if !fsc.IsPublicView {
		v0("/ignores", handlers.ListIgnoreRules, jsonRouter).Methods("GET")
		v1JSON("/ignores", handlers.ListIgnoreRules).Methods("GET")
		v0("/ignores/add/", handlers.AddIgnoreRule, jsonRouter).Methods("POST")
		v1JSON("/ignores/add/", handlers.AddIgnoreRule).Methods("POST")
		v0("/ignores/del/{id}", handlers.DeleteIgnoreRule, jsonRouter).Methods("POST")
		v1JSON("/ignores/del/{id}", handlers.DeleteIgnoreRule).Methods("POST")
		v0("/ignores/save/{id}", handlers.UpdateIgnoreRule, jsonRouter).Methods("POST")
		v1JSON("/ignores/save/{id}", handlers.UpdateIgnoreRule).Methods("POST")
	}

	// Make sure we return a 404 for anything that starts with /json and could not be found.
	jsonRouter.HandleFunc("/{ignore:.*}", http.NotFound)
	loggedRouter.HandleFunc("/json", http.NotFound)

	loadTemplates := func() {
		templates = template.Must(template.New("").ParseFiles(filepath.Join(fsc.ResourcesPath, "index.html")))
		templates = template.Must(templates.ParseGlob(filepath.Join(fsc.LitHTMLPath, "dist", "*.html")))
	}

	loadTemplates()

	fsc.FrontendConfig.BaseRepoURL = fsc.GitRepoURL
	fsc.FrontendConfig.IsPublic = fsc.IsPublicView

	templateHandler := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			httputils.AddOriginTrialHeader(w, fsc.Local)

			// Reload the template if we are running locally.
			if fsc.Local {
				loadTemplates()
			}
			if err := templates.ExecuteTemplate(w, name, fsc.FrontendConfig); err != nil {
				sklog.Errorf("Failed to expand template %s : %s", name, err)
				return
			}
		}
	}

	// These are the new lit-html pages.
	loggedRouter.HandleFunc("/", templateHandler("byblame.html"))
	loggedRouter.HandleFunc("/changelists", templateHandler("changelists.html"))
	loggedRouter.HandleFunc("/cluster", templateHandler("cluster.html"))
	loggedRouter.HandleFunc("/triagelog", templateHandler("triagelog.html"))
	loggedRouter.HandleFunc("/ignores", templateHandler("ignorelist.html"))
	loggedRouter.HandleFunc("/diff", templateHandler("diff.html"))
	loggedRouter.HandleFunc("/detail", templateHandler("details.html"))
	loggedRouter.HandleFunc("/details", templateHandler("details.html"))
	loggedRouter.HandleFunc("/list", templateHandler("by_test_list.html"))
	loggedRouter.HandleFunc("/help", templateHandler("help.html"))

	// The lit-html search page is gated behind a feature flag so we can roll it out slowly.
	// TODO(lovisolo): Delete this after the legacy search page has been removed.
	searchPagePath := "/newsearch"
	if fsc.UseLitHTMLSearchPage {
		searchPagePath = "/search"
	}
	loggedRouter.HandleFunc(searchPagePath, templateHandler("search.html"))

	loggedRouter.HandleFunc("/cl/{system}/{id}", handlers.ChangeListSearchRedirect)

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
	if fsc.ForceLogin {
		appHandler = login.ForceAuth(appRouter, callbackPath)
	}

	// The appHandler contains all application specific routes that are have logging and
	// authentication configured. Now we wrap it into the router that is exposed to the host
	// (aka the K8s container) which requires that some routes are never logged or authenticated.
	rootRouter := mux.NewRouter()

	v1Root := func(rpcRoute string, handlerFunc http.HandlerFunc) *mux.Route {
		return versionedRPC(rpcRoute, "v1", handlerFunc, rootRouter, true /*=isAuthenticated*/)
	}

	rootRouter.HandleFunc("/healthz", httputils.ReadyHandleFunc)
	v0("/json/changelist/{system}/{id}/{patchset}/untriaged", httputils.CorsHandler(handlers.ChangeListUntriagedHandler), rootRouter).Methods("GET")
	v1Root("/changelist/{system}/{id}/{patchset}/untriaged", httputils.CorsHandler(handlers.ChangeListUntriagedHandler)).Methods("GET")
	v0("/json/trstatus", httputils.CorsHandler(handlers.StatusHandler), rootRouter).Methods("GET")
	v1Root("/trstatus", httputils.CorsHandler(handlers.StatusHandler)).Methods("GET")
	v0("/json/changelist/{system}/{id}", httputils.CorsHandler(handlers.ChangeListSummaryHandler), rootRouter).Methods("GET")
	v1Root("/changelist/{system}/{id}", httputils.CorsHandler(handlers.ChangeListSummaryHandler)).Methods("GET")

	rootRouter.PathPrefix("/").Handler(appHandler)

	// Start the server
	sklog.Infof("Serving on http://127.0.0.1" + fsc.Port)
	sklog.Fatal(http.ListenAndServe(fsc.Port, rootRouter))
}

// v0 sets up a route on the given router with a wrapper to count the number of calls to the given
// endpoint. A version zero route refers to the legacy, unversioned routes.
func v0(rpcRoute string, handlerFunc http.HandlerFunc, router *mux.Router) *mux.Route {
	counter := metrics2.GetCounter(web.RPCCallCounterMetric, map[string]string{
		// For consistency, we remove the /json from all routes when adding them in the metrics.
		"route":   strings.TrimPrefix(rpcRoute, "/json"),
		"version": "v0",
	})
	return router.HandleFunc(rpcRoute, func(w http.ResponseWriter, r *http.Request) {
		counter.Inc(1)
		handlerFunc(w, r)
	})
}

// versionedRPC sets up a route on the given router with a wrapper to count the number of calls to
// the given versioned endpoint. A versioned endpoint looks like /json/v1/what/ever.
// rpcRoute is expected to be the part after /json (including a slash). If isUnauthenticated is
// true, "/json" will be added as a prefix to the final route.
func versionedRPC(rpcRoute, version string, handlerFunc http.HandlerFunc, router *mux.Router, isUnauthenticated bool) *mux.Route {
	counter := metrics2.GetCounter(web.RPCCallCounterMetric, map[string]string{
		"route":   rpcRoute,
		"version": version,
	})
	fullRoute := "/" + version + rpcRoute
	if isUnauthenticated {
		fullRoute = "/json" + fullRoute
	}
	return router.HandleFunc(fullRoute, func(w http.ResponseWriter, r *http.Request) {
		counter.Inc(1)
		handlerFunc(w, r)
	})
}

func initializeReviewSystems(configs []config.CodeReviewSystem, fc *firestore.Client, hc *http.Client) ([]clstore.ReviewSystem, error) {
	rs := make([]clstore.ReviewSystem, 0, len(configs))
	for _, cfg := range configs {
		var crs code_review.Client
		if cfg.Flavor == "gerrit" {
			if cfg.GerritURL == "" {
				return nil, skerr.Fmt("You must specify gerrit_url")
			}
			gerritClient, err := gerrit.NewGerrit(cfg.GerritURL, hc)
			if err != nil {
				return nil, skerr.Fmt("Could not create gerrit client for %s", cfg.GerritURL)
			}
			crs = gerrit_crs.New(gerritClient)
		} else if cfg.Flavor == "github" {
			if cfg.GitHubRepo == "" || cfg.GitHubCredPath == "" {
				return nil, skerr.Fmt("You must specify github_repo and github_cred_path")
			}
			gBody, err := ioutil.ReadFile(cfg.GitHubCredPath)
			if err != nil {
				return nil, skerr.Fmt("Couldn't find githubToken in %s: %s", cfg.GitHubCredPath, err)
			}
			gToken := strings.TrimSpace(string(gBody))
			githubTS := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: gToken})
			c := httputils.DefaultClientConfig().With2xxOnly().WithTokenSource(githubTS).Client()
			crs = github_crs.New(c, cfg.GitHubRepo)
		} else {
			return nil, skerr.Fmt("CRS flavor %s not supported.", cfg.Flavor)
		}

		rs = append(rs, clstore.ReviewSystem{
			ID:          cfg.ID,
			Client:      crs,
			Store:       fs_clstore.New(fc, cfg.ID),
			URLTemplate: cfg.URLTemplate,
		})
	}
	return rs, nil
}

type frontendConfig struct {
	BaseRepoURL   string `json:"baseRepoURL"`
	DefaultCorpus string `json:"defaultCorpus"`
	Title         string `json:"title"`
	IsPublic      bool   `json:"isPublic"`
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
