// skiacorrectness implements the process that exposes a RESTful API used by the JS frontend.
package main

import (
	"context"
	"encoding/json"
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

	"cloud.google.com/go/pubsub"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/golden/go/types"
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
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/go/vcsinfo/bt_vcs"
	"go.skia.org/infra/golden/go/baseline/simple_baseliner"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/clstore/dualclstore"
	"go.skia.org/infra/golden/go/clstore/fs_clstore"
	"go.skia.org/infra/golden/go/clstore/sqlclstore"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/code_review/commenter"
	"go.skia.org/infra/golden/go/code_review/gerrit_crs"
	"go.skia.org/infra/golden/go/code_review/github_crs"
	"go.skia.org/infra/golden/go/code_review/updater"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/expectations/cleanup"
	"go.skia.org/infra/golden/go/expectations/fs_expectationstore"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/ignore/fs_ignorestore"
	"go.skia.org/infra/golden/go/ignore/sqlignorestore"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/publicparams"
	"go.skia.org/infra/golden/go/search"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/status"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/tilesource"
	"go.skia.org/infra/golden/go/tjstore"
	"go.skia.org/infra/golden/go/tjstore/dualtjstore"
	"go.skia.org/infra/golden/go/tjstore/fs_tjstore"
	"go.skia.org/infra/golden/go/tjstore/sqltjstore"
	"go.skia.org/infra/golden/go/tracestore/bt_tracestore"
	"go.skia.org/infra/golden/go/warmer"
	"go.skia.org/infra/golden/go/web"
)

const (
	// imgURLPrefix is path prefix used for all images (digests and diffs)
	imgURLPrefix = "/img/"

	// callbackPath is callback endpoint used for the OAuth2 flow
	callbackPath = "/oauth2callback/"

	// Arbitrarily picked.
	maxSQLConnections = 20
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

	// Path to a directory with static assets that should be served to the frontend (JS, CSS, etc.).
	ResourcesPath string `json:"resources_path"`

	// URL where this app is hosted.
	SiteURL string `json:"site_url"`

	// How often to re-fetch the tile, compute the index, and report metrics about the index.
	TileFreshness config.Duration `json:"tile_freshness"`

	// BigTable table ID for the traces.
	TraceBTTable string `json:"trace_bt_table"`
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

	// Needed to use TimeSortableKey(...) which relies on an RNG. See docs there.
	rand.Seed(time.Now().UnixNano())

	// If we are running this, we really don't want to talk to the emulator.
	firestore.EnsureNotEmulator()

	// Initialize service.
	_, appName := filepath.Split(os.Args[0])
	common.InitWithMust(appName, common.PrometheusOpt(&fsc.PromPort))

	ctx := context.Background()

	mustStartDebugServer(fsc)

	mustSetUpOAuth2Login(fsc)

	client := mustMakeAuthenticatedHTTPClient(fsc.Local)

	sqlDB := mustInitSQLDatabase(ctx, fsc)

	diffStore := mustMakeDiffStore(ctx, fsc)

	gitStore := mustMakeGitStore(ctx, fsc, appName)

	vcs := mustMakeVCS(ctx, fsc, gitStore)

	traceStore := mustMakeTraceStore(ctx, fsc, vcs)

	gsClient := mustMakeGCSClient(ctx, fsc, client)

	fsClient := mustMakeFirestoreClient(ctx, fsc)

	expStore, expChangeHandler := mustMakeExpectationsStore(ctx, fsClient)

	publiclyViewableParams := mustMakePubliclyViewableParams(fsc)

	ignoreStore := mustMakeIgnoreStore(ctx, fsc, fsClient, sqlDB)

	tjs := mustMakeTryJobStore(fsClient, sqlDB)

	reviewSystems := mustInitializeReviewSystems(fsc, fsClient, client, sqlDB)

	tileSource := mustMakeTileSource(ctx, fsc, expStore, ignoreStore, traceStore, vcs, publiclyViewableParams, reviewSystems)

	ixr := mustMakeIndexer(ctx, fsc, expStore, expChangeHandler, diffStore, gsClient, reviewSystems, tileSource, tjs)

	// TODO(kjlubick) include non-nil comment.Store when it is implemented.
	searchAPI := search.New(diffStore, expStore, expChangeHandler, ixr, reviewSystems, tjs, nil, publiclyViewableParams, fsc.FlakyTraceThreshold)
	sklog.Infof("Search API created")

	mustStartCommenters(ctx, fsc, reviewSystems, searchAPI)

	statusWatcher := mustMakeStatusWatcher(ctx, vcs, expStore, expChangeHandler, tileSource)

	mustStartExpectationsCleanupProcess(ctx, fsc, expStore, ixr)

	handlers := mustMakeWebHandlers(diffStore, expStore, gsClient, ignoreStore, ixr, reviewSystems, searchAPI, statusWatcher, tileSource, tjs, vcs)

	rootRouter := mustMakeRootRouter(fsc, handlers, diffStore)

	// Start the server
	sklog.Infof("Serving on http://127.0.0.1" + fsc.Port)
	sklog.Fatal(http.ListenAndServe(fsc.Port, rootRouter))
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
		redirectURL = fmt.Sprintf("http://localhost%s/oauth2callback/", fsc.Port)
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
	tokenSource, err := auth.NewDefaultTokenSource(local, auth.SCOPE_USERINFO_EMAIL, gstorage.CloudPlatformScope, auth.SCOPE_GERRIT)
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

// mustMakeDiffStore returns a diff.DiffStore that speaks to a remote diff server via gRPC.
func mustMakeDiffStore(ctx context.Context, fsc *frontendServerConfig) diff.DiffStore {
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

	return diffStore
}

// mustMakeGitStore instantiates a BigTable-backed gitstore.GitStore using the BigTable specified
// via the JSON configuration files.
func mustMakeGitStore(ctx context.Context, fsc *frontendServerConfig, appName string) *bt_gitstore.BigTableGitStore {
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

	return gitStore
}

// mustMakeVCS returns a vcsinfo.VCS that wraps the given BigTable-backed GitStore.
func mustMakeVCS(ctx context.Context, fsc *frontendServerConfig, gitStore *bt_gitstore.BigTableGitStore) *bt_vcs.BigTableVCS {
	// TODO(kjlubick): remove gitilesRepo and the GetFile() from vcsinfo (unused and
	//  leaky abstraction).
	gitilesRepo := gitiles.NewRepo("", nil)
	vcs, err := bt_vcs.New(ctx, gitStore, fsc.GitRepoBranch, gitilesRepo)
	if err != nil {
		sklog.Fatalf("Error creating BT-backed VCS instance: %s", err)
	}
	return vcs
}

// mustMakeTraceStore returns a BigTable-backed tracestore.TraceStore.
func mustMakeTraceStore(ctx context.Context, fsc *frontendServerConfig, vcs *bt_vcs.BigTableVCS) *bt_tracestore.BTTraceStore {
	btc := bt_tracestore.BTConfig{
		InstanceID: fsc.BTInstance,
		ProjectID:  fsc.BTProjectID,
		TableID:    fsc.TraceBTTable,
		VCS:        vcs,
	}

	if err := bt_tracestore.InitBT(ctx, btc); err != nil {
		sklog.Fatalf("Could not initialize BigTable tracestore with config %#v: %s", btc, err)
	}

	traceStore, err := bt_tracestore.New(ctx, btc, false)
	if err != nil {
		sklog.Fatalf("Could not instantiate BT tracestore: %s", err)
	}

	return traceStore
}

// mustMakeGCSClient returns a storage.GCSClient that uses the given http.Client. If the Gold
// instance is not authoritative (e.g. when running locally) the client won't actually write any
// files.
func mustMakeGCSClient(ctx context.Context, fsc *frontendServerConfig, client *http.Client) storage.GCSClient {
	gsClientOpt := storage.GCSClientOptions{
		KnownHashesGCSPath: fsc.KnownHashesGCSPath,
		Dryrun:             !fsc.IsAuthoritative(),
	}

	gsClient, err := storage.NewGCSClient(ctx, client, gsClientOpt)
	if err != nil {
		sklog.Fatalf("Unable to create GCSClient: %s", err)
	}

	return gsClient
}

// mustMakeFirestoreClient returns a firestore.Client using the settings from the JSON configuration
// files.
func mustMakeFirestoreClient(ctx context.Context, fsc *frontendServerConfig) *firestore.Client {
	// Auth note: the underlying firestore.NewClient looks at the
	// GOOGLE_APPLICATION_CREDENTIALS env variable, so we don't need to supply
	// a token source.
	fsClient, err := firestore.NewClient(ctx, fsc.FirestoreProjectID, "gold", fsc.FirestoreNamespace, nil)
	if err != nil {
		sklog.Fatalf("Unable to configure Firestore: %s", err)
	}
	return fsClient
}

// mustMakeExpectationsStore returns a Firestore-backed expectations.Store and a corresponding
// change event dispatcher.
func mustMakeExpectationsStore(ctx context.Context, fsClient *firestore.Client) (*fs_expectationstore.Store, *expectations.ChangeEventDispatcher) {
	// Set up the cloud expectations store
	expChangeHandler := expectations.NewEventDispatcher()
	expStore := fs_expectationstore.New(fsClient, expChangeHandler, fs_expectationstore.ReadWrite)
	if err := expStore.Initialize(ctx); err != nil {
		sklog.Fatalf("Unable to initialize fs_expstore: %s", err)
	}
	return expStore, expChangeHandler
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
func mustMakeIgnoreStore(ctx context.Context, fsc *frontendServerConfig, fsClient *firestore.Client, db *pgxpool.Pool) ignore.Store {
	var ignoreStore ignore.Store
	if db != nil {
		ignoreStore = sqlignorestore.New(db)
		sklog.Info("Using new SQL Ignore store")
	} else {
		ignoreStore = fs_ignorestore.New(ctx, fsClient)
		sklog.Info("Using deprecated Firestore Ignore store")
	}

	if err := ignore.StartMetrics(ctx, ignoreStore, fsc.TileFreshness.Duration); err != nil {
		sklog.Fatalf("Failed to start monitoring for expired ignore rules: %s", err)
	}
	return ignoreStore
}

// mustMakeTryJobStore returns a new tjstore.Store
func mustMakeTryJobStore(client *firestore.Client, db *pgxpool.Pool) tjstore.Store {
	fireTS := fs_tjstore.New(client)
	sqlTS := sqltjstore.New(db)
	return dualtjstore.New(sqlTS, fireTS)
}

// mustInitializeReviewSystems validates and instantiates one clstore.ReviewSystem for each CRS
// specified via the JSON configuration files.
func mustInitializeReviewSystems(fsc *frontendServerConfig, fc *firestore.Client, hc *http.Client, sqlDB *pgxpool.Pool) []clstore.ReviewSystem {
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
		fireCS := fs_clstore.New(fc, cfg.ID)
		sqlCS := sqlclstore.New(sqlDB, cfg.ID)
		rs = append(rs, clstore.ReviewSystem{
			ID:          cfg.ID,
			Client:      crs,
			Store:       dualclstore.New(sqlCS, fireCS),
			URLTemplate: cfg.URLTemplate,
		})
	}
	return rs
}

// mustMakeTileSource returns a new tilesource.TileSource.
func mustMakeTileSource(ctx context.Context, fsc *frontendServerConfig, expStore expectations.Store, ignoreStore ignore.Store, traceStore *bt_tracestore.BTTraceStore, vcs vcsinfo.VCS, publiclyViewableParams publicparams.Matcher, reviewSystems []clstore.ReviewSystem) tilesource.TileSource {
	var clUpdater code_review.ChangelistLandedUpdater
	if fsc.IsAuthoritative() && !fsc.DisableCLTracking {
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
	if err := tileSource.StartUpdater(ctx, 2*time.Minute); err != nil {
		sklog.Fatalf("Could not fetch initial tile: %s", err)
	}

	return tileSource
}

// mustMakeIndexer makes a new indexer.Indexer.
func mustMakeIndexer(ctx context.Context, fsc *frontendServerConfig, expStore expectations.Store, expChangeHandler expectations.ChangeEventRegisterer, diffStore diff.DiffStore, gsClient storage.GCSClient, reviewSystems []clstore.ReviewSystem, tileSource tilesource.TileSource, tjs tjstore.Store) *indexer.Indexer {
	psc, err := pubsub.NewClient(ctx, fsc.PubsubProjectID)
	if err != nil {
		sklog.Fatalf("initializing pubsub client for project %s: %s", fsc.PubsubProjectID, err)
	}

	ic := indexer.IndexerConfig{
		DiffWorkPublisher: &pubsubDiffPublisher{client: psc, topic: fsc.DiffWorkTopic},
		DiffStore:         diffStore,
		ExpChangeListener: expChangeHandler,
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

	return ixr
}

type pubsubDiffPublisher struct {
	client *pubsub.Client
	topic  string
}

// CalculateDiffs publishes a WorkerMessage to the configured PubSub topic so that a worker
// (see diffcalculator) can pick it up and calculate the diffs.
func (p *pubsubDiffPublisher) CalculateDiffs(ctx context.Context, grouping paramtools.Params, left, right []types.Digest) error {
	body, err := json.Marshal(diff.WorkerMessage{
		Version:         diff.WorkerMessageVersion,
		Grouping:        grouping,
		AdditionalLeft:  left,
		AdditionalRight: right,
	})
	if err != nil {
		return skerr.Wrap(err) // should never happen because JSON input is well-formed.
	}
	pr := p.client.Topic(p.topic).Publish(ctx, &pubsub.Message{
		Data: body,
	})
	// Blocks until message actual sent
	_, err = pr.Get(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// mustStartCommenters starts a background process that comments on CLs for each of the review
// systems specified in the JSON configuration files, unless the Gold instance is not authoritative
// (e.g. when running locally) or when CL tracking is disabled via the JSON configuration.
func mustStartCommenters(ctx context.Context, fsc *frontendServerConfig, reviewSystems []clstore.ReviewSystem, searchAPI search.SearchAPI) {
	if fsc.IsAuthoritative() && !fsc.DisableCLTracking {
		for _, rs := range reviewSystems {
			clCommenter, err := commenter.New(rs, searchAPI, fsc.CLCommentTemplate, fsc.SiteURL, fsc.PublicSiteURL, fsc.DisableCLComments)
			if err != nil {
				sklog.Fatalf("Could not initialize commenter: %s", err)
			}
			startCommenter(ctx, clCommenter)
		}
	}
}

// startCommenter begins the background process that comments on CLs.
func startCommenter(ctx context.Context, cmntr code_review.ChangelistCommenter) {
	go func() {
		// TODO(kjlubick): tune this time, maybe make it a flag
		util.RepeatCtx(ctx, 3*time.Minute, func(ctx context.Context) {
			if err := cmntr.CommentOnChangelistsWithUntriagedDigests(ctx); err != nil {
				sklog.Errorf("Could not comment on CLs with Untriaged Digests: %s", err)
			}
		})
	}()
}

// mustMakeStatusWatcher returns a new status.StatusWatcher.
func mustMakeStatusWatcher(ctx context.Context, vcs vcsinfo.VCS, expStore expectations.Store, expChangeHandler expectations.ChangeEventRegisterer, tileSource tilesource.TileSource) *status.StatusWatcher {
	swc := status.StatusWatcherConfig{
		ExpChangeListener: expChangeHandler,
		ExpectationsStore: expStore,
		TileSource:        tileSource,
		VCS:               vcs,
	}

	statusWatcher, err := status.New(ctx, swc)
	if err != nil {
		sklog.Fatalf("Failed to initialize status watcher: %s", err)
	}
	sklog.Infof("statusWatcher created")

	return statusWatcher
}

// mustStartExpectationsCleanupProcess starts a process that will garbage-collect any stale
// expectations, unless the Gold instance is not authoritative (e.g. when running locally).
func mustStartExpectationsCleanupProcess(ctx context.Context, fsc *frontendServerConfig, expStore *fs_expectationstore.Store, ixr *indexer.Indexer) {
	// reminder: this exp will be updated whenever expectations change.
	exp, err := expStore.Get(ctx)
	if err != nil {
		sklog.Fatalf("Failed to get master-branch expectations: %s", err)
	}

	if fsc.IsAuthoritative() {
		policy := cleanup.Policy{
			PositiveMaxLastUsed: fsc.PositivesMaxAge.Duration,
			NegativeMaxLastUsed: fsc.NegativesMaxAge.Duration,
		}
		if err := cleanup.Start(ctx, ixr, expStore, exp, policy); err != nil {
			sklog.Fatalf("Could not start expectation cleaning process %s", err)
		}
	}
}

// mustMakeWebHandlers returns a new web.Handlers.
func mustMakeWebHandlers(diffStore diff.DiffStore, expStore expectations.Store, gsClient storage.GCSClient, ignoreStore ignore.Store, ixr *indexer.Indexer, reviewSystems []clstore.ReviewSystem, searchAPI search.SearchAPI, statusWatcher *status.StatusWatcher, tileSource tilesource.TileSource, tjs tjstore.Store, vcs vcsinfo.VCS) *web.Handlers {
	handlers, err := web.NewHandlers(web.HandlersConfig{
		Baseliner:         simple_baseliner.New(expStore),
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
	return handlers
}

// mustMakeRootRouter returns a mux.Router that can be used to serve Gold's web UI and JSON API.
func mustMakeRootRouter(fsc *frontendServerConfig, handlers *web.Handlers, diffStore diff.DiffStore) *mux.Router {
	rootRouter := mux.NewRouter()
	rootRouter.HandleFunc("/healthz", httputils.ReadyHandleFunc)

	// loggedRouter contains all the endpoints that are logged. See the call below to
	// LoggingGzipRequestResponse.
	loggedRouter := mux.NewRouter()

	// Set up the resource to serve the image files.
	imgHandler, err := diffStore.ImageHandler(imgURLPrefix)
	if err != nil {
		sklog.Fatalf("Unable to get image handler: %s", err)
	}

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
	router.PathPrefix("/dist/").Handler(http.StripPrefix("/dist/", http.HandlerFunc(web.MakeResourceHandler(fsc.ResourcesPath))))

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

	add("/json/byblame", handlers.ByBlameHandler, "GET")
	add("/json/v1/byblame", handlers.ByBlameHandler, "GET")
	add("/json/changelists", handlers.ChangelistsHandler, "GET")
	add("/json/v1/changelists", handlers.ChangelistsHandler, "GET")
	add("/json/clusterdiff", handlers.ClusterDiffHandler, "GET")
	add("/json/v1/clusterdiff", handlers.ClusterDiffHandler, "GET")
	add("/json/commits", handlers.CommitsHandler, "GET")
	add("/json/v1/commits", handlers.CommitsHandler, "GET")
	add("/json/debug/digestsbytestname/{corpus}/{testName}", handlers.GetPerTraceDigestsByTestName, "GET")
	add("/json/v1/debug/digestsbytestname/{corpus}/{testName}", handlers.GetPerTraceDigestsByTestName, "GET")
	add("/json/debug/flakytraces/{minUniqueDigests}", handlers.GetFlakyTracesData, "GET")
	add("/json/v1/debug/flakytraces/{minUniqueDigests}", handlers.GetFlakyTracesData, "GET")
	add("/json/details", handlers.DetailsHandler, "GET")
	add("/json/v1/details", handlers.DetailsHandler, "GET")
	add("/json/diff", handlers.DiffHandler, "GET")
	add("/json/v1/diff", handlers.DiffHandler, "GET")
	add("/json/digests", handlers.DigestListHandler, "GET")
	add("/json/v1/digests", handlers.DigestListHandler, "GET")
	add("/json/export", handlers.ExportHandler, "GET")
	add("/json/v1/export", handlers.ExportHandler, "GET")
	add("/json/latestpositivedigest/{traceId}", handlers.LatestPositiveDigestHandler, "GET")
	add("/json/v1/latestpositivedigest/{traceId}", handlers.LatestPositiveDigestHandler, "GET")
	add("/json/list", handlers.ListTestsHandler, "GET")
	add("/json/v1/list", handlers.ListTestsHandler, "GET")
	add("/json/paramset", handlers.ParamsHandler, "GET")
	add("/json/v1/paramset", handlers.ParamsHandler, "GET")
	add("/json/search", handlers.SearchHandler, "GET")
	add("/json/v1/search", handlers.SearchHandler, "GET")
	add("/json/triage", handlers.TriageHandler, "POST")
	add("/json/v1/triage", handlers.TriageHandler, "POST")
	add("/json/triagelog", handlers.TriageLogHandler, "GET")
	add("/json/v1/triagelog", handlers.TriageLogHandler, "GET")
	add("/json/triagelog/undo", handlers.TriageUndoHandler, "POST")
	add("/json/v1/triagelog/undo", handlers.TriageUndoHandler, "POST")
	add("/json/whoami", handlers.Whoami, "GET")
	add("/json/v1/whoami", handlers.Whoami, "GET")

	// Routes shared with the baseline server. These usually don't see traffic because the envoy
	// routing directs these requests to the baseline servers, if there are some.
	add(shared.KnownHashesRoute, handlers.TextKnownHashesProxy, "GET")
	add(shared.KnownHashesRouteV1, handlers.TextKnownHashesProxy, "GET")
	// Retrieving that baseline for master and an Gerrit issue are handled the same way
	// These routes can be served with baseline_server for higher availability.
	add(shared.ExpectationsRoute, handlers.BaselineHandlerV1, "GET")
	add(shared.ExpectationsRouteV1, handlers.BaselineHandlerV1, "GET")
	add(shared.ExpectationsRouteV2, handlers.BaselineHandlerV2, "GET")

	// Only expose these endpoints if this instance is not a public view. The reason we want to hide
	// ignore rules is so that we don't leak params that might be in them.
	if !fsc.IsPublicView {
		add("/json/ignores", handlers.ListIgnoreRules, "GET")
		add("/json/v1/ignores", handlers.ListIgnoreRules, "GET")
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

	add("/json/changelist/{system}/{id}/{patchset}/untriaged", handlers.ChangelistUntriagedHandler)
	add("/json/v1/changelist/{system}/{id}/{patchset}/untriaged", handlers.ChangelistUntriagedHandler)
	add("/json/trstatus", handlers.StatusHandler)
	add("/json/v1/trstatus", handlers.StatusHandler)
	add("/json/changelist/{system}/{id}", handlers.ChangelistSummaryHandler)
	add("/json/v1/changelist/{system}/{id}", handlers.ChangelistSummaryHandler)
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
