// The goldfrontend executable is the process that exposes a RESTful API used by the JS frontend.
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
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4/pgxpool"
	"golang.org/x/oauth2"
	gstorage "google.golang.org/api/storage/v1"
	"google.golang.org/grpc"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/clstore/sqlclstore"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/code_review/gerrit_crs"
	"go.skia.org/infra/golden/go/code_review/github_crs"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/ignore/sqlignorestore"
	"go.skia.org/infra/golden/go/publicparams"
	"go.skia.org/infra/golden/go/search2"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/tracing"
	"go.skia.org/infra/golden/go/web"
	"go.skia.org/infra/golden/go/web/frontend"
)

const (
	// callbackPath is callback endpoint used for the OAuth2 flow
	callbackPath = "/oauth2callback/"

	// Arbitrarily picked.
	maxSQLConnections = 32
)

type frontendServerConfig struct {
	config.Common

	// A list of email addresses or domains that can log into this instance.
	AuthorizedUsers []string `json:"authorized_users"`

	// Client secret file for OAuth2 authentication.
	ClientSecretFile string `json:"client_secret_file"`

	// Force the user to be authenticated for all requests.
	ForceLogin bool `json:"force_login"`

	// Configuration settings that will get passed to the frontend (see modules/settings.ts)
	FrontendConfig frontendConfig `json:"frontend"`

	// If this instance is simply a mirror of another instance's data.
	IsPublicView bool `json:"is_public_view"`

	// MaterializedViewCorpora is the optional list of corpora that should have a materialized
	// view created and refreshed to speed up search results.
	MaterializedViewCorpora []string `json:"materialized_view_corpora" optional:"true"`

	// If non empty, this map of rules will be applied to traces to see if they can be showed on
	// this instance.
	PubliclyAllowableParams publicparams.MatchingRules `json:"publicly_allowed_params" optional:"true"`

	// Path to a directory with static assets that should be served to the frontend (JS, CSS, etc.).
	ResourcesPath string `json:"resources_path"`
}

// IsAuthoritative indicates that this instance can write to known_hashes, update CL statuses, etc.
func (fsc *frontendServerConfig) IsAuthoritative() bool {
	return !fsc.Local && !fsc.IsPublicView
}

type frontendConfig struct {
	BaseRepoURL   string `json:"baseRepoURL"`
	DefaultCorpus string `json:"defaultCorpus"`
	Title         string `json:"title"`
	IsPublic      bool   `json:"isPublic"`
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

	// Load configuration from common and instance-specific JSON files.
	fsc := mustLoadFrontendServerConfig(commonInstanceConfig, thisConfig)

	// Speculative memory usage fix? https://github.com/googleapis/google-cloud-go/issues/375
	grpc.EnableTracing = false

	if err := tracing.Initialize(0.01); err != nil {
		sklog.Fatalf("Could not initialize tracing: %s", err)
	}

	// Needed to use TimeSortableKey(...) which relies on an RNG. See docs there.
	rand.Seed(time.Now().UnixNano())
	// Initialize service.
	_, appName := filepath.Split(os.Args[0])
	common.InitWithMust(appName, common.PrometheusOpt(&fsc.PromPort))

	ctx := context.Background()

	mustStartDebugServer(fsc)

	mustSetUpOAuth2Login(fsc)

	client := mustMakeAuthenticatedHTTPClient(fsc.Local)

	sqlDB := mustInitSQLDatabase(ctx, fsc)

	gsClient := mustMakeGCSClient(ctx, fsc, client)

	publiclyViewableParams := mustMakePubliclyViewableParams(fsc)

	ignoreStore := mustMakeIgnoreStore(ctx, sqlDB)

	reviewSystems := mustInitializeReviewSystems(fsc, client, sqlDB)

	s2a := mustLoadSearchAPI(ctx, fsc, sqlDB, publiclyViewableParams, reviewSystems)

	handlers := mustMakeWebHandlers(ctx, fsc, sqlDB, gsClient, ignoreStore, reviewSystems, s2a)

	rootRouter := mustMakeRootRouter(fsc, handlers)

	// Start the server
	sklog.Infof("Serving on http://127.0.0.1" + fsc.ReadyPort)
	sklog.Fatal(http.ListenAndServe(fsc.ReadyPort, rootRouter))
}

func mustLoadSearchAPI(ctx context.Context, fsc *frontendServerConfig, sqlDB *pgxpool.Pool, publiclyViewableParams publicparams.Matcher, systems []clstore.ReviewSystem) *search2.Impl {
	templates := map[string]string{}
	for _, crs := range systems {
		templates[crs.ID] = crs.URLTemplate
	}

	s2a := search2.New(sqlDB, fsc.WindowSize)
	s2a.SetReviewSystemTemplates(templates)
	sklog.Infof("SQL Search loaded with CRS templates %s", templates)
	err := s2a.StartCacheProcess(ctx, 5*time.Minute, fsc.WindowSize)
	if err != nil {
		sklog.Fatalf("Cannot load caches for search2 backend: %s", err)
	}
	if err := s2a.StartMaterializedViews(ctx, fsc.MaterializedViewCorpora, 5*time.Minute); err != nil {
		sklog.Fatalf("Cannot create materialized views %s: %s", fsc.MaterializedViewCorpora, err)
	}
	if fsc.IsPublicView {
		if err := s2a.StartApplyingPublicParams(ctx, publiclyViewableParams, 5*time.Minute); err != nil {
			sklog.Fatalf("Could not apply public params: %s", err)
		}
		sklog.Infof("Public params applied to search2")
	}

	return s2a
}

// mustLoadFrontendServerConfig parses the common and instance-specific JSON configuration files.
func mustLoadFrontendServerConfig(commonInstanceConfig *string, thisConfig *string) *frontendServerConfig {
	var fsc frontendServerConfig
	if err := config.LoadFromJSON5(&fsc, commonInstanceConfig, thisConfig); err != nil {
		sklog.Fatalf("Reading config: %s", err)
	}
	sklog.Infof("Loaded config %#v", fsc)
	return &fsc
}

// mustStartDebugServer starts an internal HTTP server for debugging purposes if requested.
func mustStartDebugServer(fsc *frontendServerConfig) {
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
}

// mustSetUpOAuth2Login initializes the OAuth 2.0 login system.
func mustSetUpOAuth2Login(fsc *frontendServerConfig) {
	// Set up login
	redirectURL := fsc.SiteURL + "/oauth2callback/"
	if fsc.Local {
		redirectURL = fmt.Sprintf("http://localhost%s/oauth2callback/", fsc.ReadyPort)
	}
	sklog.Infof("The allowed list of users is: %q", fsc.AuthorizedUsers)
	if err := login.Init(redirectURL, strings.Join(fsc.AuthorizedUsers, " "), fsc.ClientSecretFile); err != nil {
		sklog.Fatalf("Failed to initialize the login system: %s", err)
	}
}

// mustMakeAuthenticatedHTTPClient returns an http.Client with the credentials required by the
// services that Gold communicates with.
func mustMakeAuthenticatedHTTPClient(local bool) *http.Client {
	// Get the token source for the service account with access to the services
	// we need to operate.
	tokenSource, err := auth.NewDefaultTokenSource(local, auth.ScopeUserinfoEmail, gstorage.CloudPlatformScope, auth.ScopeGerrit)
	if err != nil {
		sklog.Fatalf("Failed to authenticate service account: %s", err)
	}
	return httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client()
}

// mustInitSQLDatabase initializes a SQL database. If there are any errors, it will panic via
// sklog.Fatal.
func mustInitSQLDatabase(ctx context.Context, fsc *frontendServerConfig) *pgxpool.Pool {
	if fsc.SQLDatabaseName == "" {
		sklog.Fatalf("Must have SQL Database Information")
	}
	url := sql.GetConnectionURL(fsc.SQLConnection, fsc.SQLDatabaseName)
	conf, err := pgxpool.ParseConfig(url)
	if err != nil {
		sklog.Fatalf("error getting postgres config %s: %s", url, err)
	}

	conf.MaxConns = maxSQLConnections
	db, err := pgxpool.ConnectConfig(ctx, conf)
	if err != nil {
		sklog.Fatalf("error connecting to the database: %s", err)
	}
	sklog.Infof("Connected to SQL database %s", fsc.SQLDatabaseName)
	return db
}

// mustMakeGCSClient returns a storage.GCSClient that uses the given http.Client. If the Gold
// instance is not authoritative (e.g. when running locally) the client won't actually write any
// files.
func mustMakeGCSClient(ctx context.Context, fsc *frontendServerConfig, client *http.Client) storage.GCSClient {
	gsClientOpt := storage.GCSClientOptions{
		Bucket:             fsc.GCSBucket,
		KnownHashesGCSPath: fsc.KnownHashesGCSPath,
		Dryrun:             !fsc.IsAuthoritative(),
	}

	gsClient, err := storage.NewGCSClient(ctx, client, gsClientOpt)
	if err != nil {
		sklog.Fatalf("Unable to create GCSClient: %s", err)
	}

	return gsClient
}

// mustMakePubliclyViewableParams validates and computes a publicparams.Matcher from the publicly
// allowed params specified in the JSON configuration files.
func mustMakePubliclyViewableParams(fsc *frontendServerConfig) publicparams.Matcher {
	var publiclyViewableParams publicparams.Matcher
	var err error

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

	return publiclyViewableParams
}

// mustMakeIgnoreStore returns a new ignore.Store and starts a monitoring routine that counts the
// the number of expired ignore rules and exposes this as a metric.
func mustMakeIgnoreStore(ctx context.Context, db *pgxpool.Pool) ignore.Store {
	ignoreStore := sqlignorestore.New(db)

	if err := ignore.StartMetrics(ctx, ignoreStore, 5*time.Minute); err != nil {
		sklog.Fatalf("Failed to start monitoring for expired ignore rules: %s", err)
	}
	return ignoreStore
}

// mustInitializeReviewSystems validates and instantiates one clstore.ReviewSystem for each CRS
// specified via the JSON configuration files.
func mustInitializeReviewSystems(fsc *frontendServerConfig, hc *http.Client, sqlDB *pgxpool.Pool) []clstore.ReviewSystem {
	rs := make([]clstore.ReviewSystem, 0, len(fsc.CodeReviewSystems))
	for _, cfg := range fsc.CodeReviewSystems {
		var crs code_review.Client
		if cfg.Flavor == "gerrit" {
			if cfg.GerritURL == "" {
				sklog.Fatal("You must specify gerrit_url")
				return nil
			}
			gerritClient, err := gerrit.NewGerrit(cfg.GerritURL, hc)
			if err != nil {
				sklog.Fatalf("Could not create gerrit client for %s", cfg.GerritURL)
				return nil
			}
			crs = gerrit_crs.New(gerritClient)
		} else if cfg.Flavor == "github" {
			if cfg.GitHubRepo == "" || cfg.GitHubCredPath == "" {
				sklog.Fatal("You must specify github_repo and github_cred_path")
				return nil
			}
			gBody, err := ioutil.ReadFile(cfg.GitHubCredPath)
			if err != nil {
				sklog.Fatalf("Couldn't find githubToken in %s: %s", cfg.GitHubCredPath, err)
				return nil
			}
			gToken := strings.TrimSpace(string(gBody))
			githubTS := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: gToken})
			c := httputils.DefaultClientConfig().With2xxOnly().WithTokenSource(githubTS).Client()
			crs = github_crs.New(c, cfg.GitHubRepo)
		} else {
			sklog.Fatalf("CRS flavor %s not supported.", cfg.Flavor)
			return nil
		}
		sqlCS := sqlclstore.New(sqlDB, cfg.ID)
		rs = append(rs, clstore.ReviewSystem{
			ID:          cfg.ID,
			Client:      crs,
			Store:       sqlCS,
			URLTemplate: cfg.URLTemplate,
		})
	}
	return rs
}

// mustMakeWebHandlers returns a new web.Handlers.
func mustMakeWebHandlers(ctx context.Context, fsc *frontendServerConfig, db *pgxpool.Pool, gsClient storage.GCSClient, ignoreStore ignore.Store, reviewSystems []clstore.ReviewSystem, s2a search2.API) *web.Handlers {
	handlers, err := web.NewHandlers(web.HandlersConfig{
		DB:            db,
		GCSClient:     gsClient,
		IgnoreStore:   ignoreStore,
		ReviewSystems: reviewSystems,
		Search2API:    s2a,
		WindowSize:    fsc.WindowSize,
	}, web.FullFrontEnd)
	if err != nil {
		sklog.Fatalf("Failed to initialize web handlers: %s", err)
	}
	handlers.StartCacheWarming(ctx)
	return handlers
}

// mustMakeRootRouter returns a mux.Router that can be used to serve Gold's web UI and JSON API.
func mustMakeRootRouter(fsc *frontendServerConfig, handlers *web.Handlers) *mux.Router {
	rootRouter := mux.NewRouter()
	rootRouter.HandleFunc("/healthz", httputils.ReadyHandleFunc)

	// loggedRouter contains all the endpoints that are logged. See the call below to
	// LoggingGzipRequestResponse.
	loggedRouter := mux.NewRouter()

	// Login endpoints.
	loggedRouter.HandleFunc(callbackPath, login.OAuth2CallbackHandler)
	loggedRouter.HandleFunc("/loginstatus/", login.StatusHandler)
	loggedRouter.HandleFunc("/logout/", login.LogoutHandler)

	// JSON endpoints.
	addAuthenticatedJSONRoutes(loggedRouter, fsc, handlers)
	addUnauthenticatedJSONRoutes(rootRouter, fsc, handlers)

	// Routes to serve the UI, static assets, etc.
	addUIRoutes(loggedRouter, fsc, handlers)

	// set up the app router that might be authenticated and logs almost everything.
	appRouter := mux.NewRouter()
	// Images should not be served gzipped as PNGs typically have zlib compression anyway.
	appRouter.PathPrefix("/img/").HandlerFunc(handlers.ImageHandler).Methods("GET")
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
	rootRouter.PathPrefix("/").Handler(appHandler)

	return rootRouter
}

// addUIRoutes adds the necessary routes to serve Gold's web pages and static assets such as JS and
// CSS bundles, static images (digest and diff images are handled elsewhere), etc.
func addUIRoutes(router *mux.Router, fsc *frontendServerConfig, handlers *web.Handlers) {
	// Serve static assets (JS and CSS Webpack bundles, images, etc.).
	//
	// Note that this includes the raw HTML templates (e.g. /dist/byblame.html) with unpopulated
	// placeholders such as {{.Title}}. These aren't used directly by client code. We should probably
	// unexpose them and only serve the JS/CSS Webpack bundles from this route (and any other static
	// assets such as the favicon).
	router.PathPrefix("/dist/").Handler(http.StripPrefix("/dist/", http.HandlerFunc(makeResourceHandler(fsc.ResourcesPath))))

	var templates *template.Template

	loadTemplates := func() {
		templates = template.Must(template.New("").ParseGlob(filepath.Join(fsc.ResourcesPath, "*.html")))
	}

	loadTemplates()

	fsc.FrontendConfig.BaseRepoURL = fsc.GitRepoURL
	fsc.FrontendConfig.IsPublic = fsc.IsPublicView

	templateHandler := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")

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

	// These routes serve the web UI.
	router.HandleFunc("/", templateHandler("byblame.html"))
	router.HandleFunc("/changelists", templateHandler("changelists.html"))
	router.HandleFunc("/cluster", templateHandler("cluster.html"))
	router.HandleFunc("/triagelog", templateHandler("triagelog.html"))
	router.HandleFunc("/ignores", templateHandler("ignorelist.html"))
	router.HandleFunc("/diff", templateHandler("diff.html"))
	router.HandleFunc("/detail", templateHandler("details.html"))
	router.HandleFunc("/details", templateHandler("details.html"))
	router.HandleFunc("/list", templateHandler("by_test_list.html"))
	router.HandleFunc("/help", templateHandler("help.html"))
	router.HandleFunc("/search", templateHandler("search.html"))
	router.HandleFunc("/cl/{system}/{id}", handlers.ChangelistSearchRedirect)
}

// addAuthenticatedJSONRoutes populates the given router with the subset of Gold's JSON RPC routes
// that require authentication.
func addAuthenticatedJSONRoutes(router *mux.Router, fsc *frontendServerConfig, handlers *web.Handlers) {
	// Set up a subrouter for the '/json' routes which make up the Gold API.
	// This makes routing faster, but also returns a failure when an /json route is
	// requested that doesn't exist. If we did this differently a call to a non-existing endpoint
	// would be handled by the route that handles the returning the index template and make
	// debugging confusing.
	pathPrefix := "/json"
	jsonRouter := router.PathPrefix(pathPrefix).Subrouter()

	add := func(jsonRoute string, handlerFunc http.HandlerFunc, method string) {
		addJSONRoute(jsonRoute, handlerFunc, jsonRouter, pathPrefix).Methods(method)
	}

	add("/json/v2/byblame", handlers.ByBlameHandler, "GET")
	add("/json/v2/changelists", handlers.ChangelistsHandler, "GET")
	add("/json/v2/clusterdiff", handlers.ClusterDiffHandler, "GET")
	add("/json/v2/commits", handlers.CommitsHandler, "GET")
	add("/json/v1/positivedigestsbygrouping/{groupingID}", handlers.PositiveDigestsByGroupingIDHandler, "GET")
	add("/json/v2/details", handlers.DetailsHandler, "GET")
	add("/json/v2/diff", handlers.DiffHandler, "GET")
	add("/json/v2/digests", handlers.DigestListHandler, "GET")
	add("/json/v2/latestpositivedigest/{traceID}", handlers.LatestPositiveDigestHandler, "GET")
	add("/json/v2/list", handlers.ListTestsHandler, "GET")
	add("/json/v2/paramset", handlers.ParamsHandler, "GET")
	add("/json/v2/search", handlers.SearchHandler, "GET")
	add("/json/v2/triage", handlers.TriageHandler, "POST")
	add("/json/v2/triagelog", handlers.TriageLogHandler, "GET")
	add("/json/v2/triagelog/undo", handlers.TriageUndoHandler, "POST")
	add("/json/whoami", handlers.Whoami, "GET")
	add("/json/v1/whoami", handlers.Whoami, "GET")

	// Routes shared with the baseline server. These usually don't see traffic because the envoy
	// routing directs these requests to the baseline servers, if there are some.
	add(frontend.KnownHashesRoute, handlers.TextKnownHashesProxy, "GET")
	add(frontend.KnownHashesRouteV1, handlers.TextKnownHashesProxy, "GET")
	// Retrieving a baseline for the primary branch and a Gerrit issue are handled the same way.
	// These routes can be served with baseline_server for higher availability.
	add(frontend.ExpectationsRouteV2, handlers.BaselineHandlerV2, "GET")

	// Only expose these endpoints if this instance is not a public view. The reason we want to hide
	// ignore rules is so that we don't leak params that might be in them.
	if !fsc.IsPublicView {
		add("/json/v2/ignores", handlers.ListIgnoreRules2, "GET")
		add("/json/ignores/add/", handlers.AddIgnoreRule, "POST")
		add("/json/v1/ignores/add/", handlers.AddIgnoreRule, "POST")
		add("/json/ignores/del/{id}", handlers.DeleteIgnoreRule, "POST")
		add("/json/v1/ignores/del/{id}", handlers.DeleteIgnoreRule, "POST")
		add("/json/ignores/save/{id}", handlers.UpdateIgnoreRule, "POST")
		add("/json/v1/ignores/save/{id}", handlers.UpdateIgnoreRule, "POST")
	}

	// Make sure we return a 404 for anything that starts with /json and could not be found.
	jsonRouter.HandleFunc("/{ignore:.*}", http.NotFound)
	router.HandleFunc(pathPrefix, http.NotFound)
}

// addUnauthenticatedJSONRoutes populates the given router with the subset of Gold's JSON RPC routes
// that do not require authentication.
func addUnauthenticatedJSONRoutes(router *mux.Router, _ *frontendServerConfig, handlers *web.Handlers) {
	add := func(jsonRoute string, handlerFunc http.HandlerFunc) {
		addJSONRoute(jsonRoute, httputils.CorsHandler(handlerFunc), router, "").Methods("GET")
	}

	add("/json/v2/trstatus", handlers.StatusHandler)
	add("/json/v2/changelist/{system}/{id}", handlers.PatchsetsAndTryjobsForCL2)
	add("/json/v1/changelist_summary/{system}/{id}", handlers.ChangelistSummaryHandler)
}

var (
	unversionedJSONRouteRegexp = regexp.MustCompile(`/json/(?P<path>.+)`)
	versionedJSONRouteRegexp   = regexp.MustCompile(`/json/v(?P<version>\d+)/(?P<path>.+)`)
)

// addJSONRoute adds a handler function to a router for the given JSON RPC route, which must be of
// the form "/json/<path>" or "/json/v<n>/<path>", and increases a counter to track RPC and version
// usage every time the RPC is invoked.
//
// If the given routerPathPrefix is non-empty, it will be removed from the JSON RPC route before the
// handler function is added to the router (useful with subrouters for path prefixes, e.g. "/json").
//
// It panics if jsonRoute does not start with '/json', or if the routerPathPrefix is not a prefix of
// the jsonRoute, or if the jsonRoute uses version 0 (e.g. /json/v0/foo), which is reserved for
// unversioned RPCs.
//
// This function has been designed to take the full JSON RPC route as an argument, including the
// RPC version number and the subrouter path prefix, if any (e.g. "/json/v2/my/rpc" vs. "/my/rpc").
// This results in clearer code at the callsite because the reader can immediately see what the
// final RPC route will look like from outside the HTTP server.
func addJSONRoute(jsonRoute string, handlerFunc http.HandlerFunc, router *mux.Router, routerPathPrefix string) *mux.Route {
	// Make sure the jsonRoute agrees with the router path prefix (which can be the empty string).
	if !strings.HasPrefix(jsonRoute, routerPathPrefix) {
		panic(fmt.Sprintf(`Prefix "%s" not found in JSON RPC route: %s`, routerPathPrefix, jsonRoute))
	}

	// Parse the JSON RPC route, which can be of the form "/json/v<n>/<path>" or "/json/<path>", and
	// extract <path> and <n>, defaulting to 0 for the unversioned case.
	var path string
	version := 0 // Default value is used for unversioned JSON RPCs.
	if matches := versionedJSONRouteRegexp.FindStringSubmatch(jsonRoute); matches != nil {
		var err error
		version, err = strconv.Atoi(matches[1])
		if err != nil {
			// Should never happen.
			panic("Failed to convert RPC version to integer (indicates a bug in the regexp): " + jsonRoute)
		}
		if version == 0 {
			// Disallow /json/v0/* because we indicate unversioned RPCs with version 0.
			panic("JSON RPC version cannot be 0: " + jsonRoute)
		}
		path = matches[2]
	} else if matches := unversionedJSONRouteRegexp.FindStringSubmatch(jsonRoute); matches != nil {
		path = matches[1]
	} else {
		// The path is neither a versioned nor an unversioned JSON RPC route. This is a coding error.
		panic("Unrecognized JSON RPC route format: " + jsonRoute)
	}

	counter := metrics2.GetCounter(web.RPCCallCounterMetric, map[string]string{
		"route":   "/" + path,
		"version": fmt.Sprintf("v%d", version),
	})

	return router.HandleFunc(strings.TrimPrefix(jsonRoute, routerPathPrefix), func(w http.ResponseWriter, r *http.Request) {
		counter.Inc(1)
		handlerFunc(w, r)
	})
}

// makeResourceHandler creates a static file handler that sets a caching policy.
func makeResourceHandler(resourceDir string) func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(resourceDir))
	return func(w http.ResponseWriter, r *http.Request) {
		// No limit for anon users - this should be fast enough to handle a large load.
		w.Header().Add("Cache-Control", "max-age=300")
		fileServer.ServeHTTP(w, r)
	}
}
