package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime/pprof"
	"time"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/redisutil"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/analysis"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/db"
	"go.skia.org/infra/golden/go/digeststore"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/filediffstore"
	"go.skia.org/infra/golden/go/history"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/status"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/summary"
	"go.skia.org/infra/golden/go/tally"
	"go.skia.org/infra/golden/go/types"
	pconfig "go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/filetilestore"
)

// Command line flags.
var (
	graphiteServer    = flag.String("graphite_server", "skia-monitoring:2003", "Where is Graphite metrics ingestion server running.")
	port              = flag.String("port", ":9000", "HTTP service address (e.g., ':9000')")
	local             = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	staticDir         = flag.String("static_dir", "./app", "Directory with static content to serve")
	tileStoreDir      = flag.String("tile_store_dir", "/tmp/tileStore", "What directory to look for tiles in.")
	imageDir          = flag.String("image_dir", "/tmp/imagedir", "What directory to store test and diff images in.")
	gsBucketName      = flag.String("gs_bucket", "chromium-skia-gm", "Name of the google storage bucket that holds uploaded images.")
	doOauth           = flag.Bool("oauth", true, "Run through the OAuth 2.0 flow on startup, otherwise use a GCE service account.")
	oauthCacheFile    = flag.String("oauth_cache_file", "/home/perf/google_storage_token.data", "Path to the file where to cache cache the oauth credentials.")
	memProfile        = flag.Duration("memprofile", 0, "Duration for which to profile memory. After this duration the program writes the memory profile and exits.")
	nCommits          = flag.Int("n_commits", 50, "Number of recent commits to include in the analysis.")
	resourcesDir      = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the directory relative to the source code files will be used.")
	redirectURL       = flag.String("redirect_url", "https://gold.skia.org/oauth2callback/", "OAuth2 redirect url. Only used when local=false.")
	redisHost         = flag.String("redis_host", "", "The host and port (e.g. 'localhost:6379') of the Redis data store that will be used for caching.")
	redisDB           = flag.Int("redis_db", 0, "The index of the Redis database we should use. Default will work fine in most cases.")
	startAnalyzer     = flag.Bool("start_analyzer", true, "Create an instance of the analyzer and start it running.")
	startExperimental = flag.Bool("start_experimental", true, "Start experimental features.")
	cpuProfile        = flag.Duration("cpu_profile", 0, "Duration for which to profile the CPU usage. After this duration the program writes the CPU profile and exits.")
	forceLogin        = flag.Bool("force_login", false, "Force the user to be authenticated for all requests.")
	authWhiteList     = flag.String("auth_whitelist", login.DEFAULT_DOMAIN_WHITELIST, "White space separated list of domains and email addresses that are allowed to login.")
	nTilesToBackfill  = flag.Int("backfill_tiles", 0, "Number of tiles to backfill in our history of tiles.")
	storageDir        = flag.String("storage_dir", "/tmp/gold-storage", "Directory to store reproducible application data.")
)

const (
	IMAGE_URL_PREFIX = "/img/"

	// OAUTH2_CALLBACK_PATH is callback endpoint used for the Oauth2 flow.
	OAUTH2_CALLBACK_PATH = "/oauth2callback/"
)

// TODO(stephana): Once the analyzer related code is removed, simplify
// the ResponseEnvelope and use it solely to wrap JSON arrays.
// Remove sendResponse and sendErrorResponse in favor of sendJsonResponse
// and util.ReportError.

// ResponseEnvelope wraps all responses. Some fields might be empty depending
// on context or whether there was an error or not.
type ResponseEnvelope struct {
	Data       *interface{}             `json:"data"`
	Err        *string                  `json:"err"`
	Status     int                      `json:"status"`
	Pagination *util.ResponsePagination `json:"pagination"`
}

// CommonEnv captures shared that affect the frontend as well as the backend.
// It is used in setting up endpoints as well as rendering HTML.
type CommonEnv struct {
	// BaseURL is the base path of the application.
	BaseURL string
}

var (
	analyzer *analysis.Analyzer = nil

	// Module level variables that need to be accessible to main2.go.
	storages           *storage.Storage
	pathToURLConverter analysis.PathToURLConverter
	tallies            *tally.Tallies
	summaries          *summary.Summaries
	statusWatcher      *status.StatusWatcher
	commonEnv          CommonEnv
	blamer             *blame.Blamer
)

func listTestDetailsHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	result, err := analyzer.ListTestDetails(query)
	if err != nil {
		sendErrorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendResponse(w, result, http.StatusOK, nil)
}

// testDetailsHandler returns sufficient information about the given
// testName to triage digests.
func testDetailsHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	testName := mux.Vars(r)["testname"]
	result, err := analyzer.GetTestDetails(testName, query)
	if err != nil {
		sendErrorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendResponse(w, result, http.StatusOK, nil)
}

// triageDigestsHandler handles triaging digests. It requires the user
// to be logged in and upon success returns the the test details in the
// same format as testDetailsHandler. That way it can be used by the
// frontend to incrementally triage digests for a specific test
// (or set of tests.)
// TODO (stephana): This is not finished and WIP.
func triageDigestsHandler(w http.ResponseWriter, r *http.Request) {
	// Make sure the user is authenticated.
	userId := login.LoggedInAs(r)
	if userId == "" {
		sendErrorResponse(w, "You must be logged in triage digests.", http.StatusForbidden)
		return
	}

	// Parse input data in the body.
	var tc map[string]types.TestClassification
	if err := parseJson(r, &tc); err != nil {
		sendErrorResponse(w, "Unable to parse JSON. Error: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Update the labeling of the given tests and digests.
	result, err := analyzer.SetDigestLabels(tc, userId)
	if err != nil {
		sendErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	sendResponse(w, result, http.StatusOK, nil)
}

// blameHandler returns the blame list for the given test.
func blameHandler(w http.ResponseWriter, r *http.Request) {
	testName := mux.Vars(r)["testname"]
	result := analyzer.GetBlameList(testName)
	sendResponse(w, result, http.StatusOK, nil)
}

// statusHandler returns the current status with respect to HEAD.
func statusHandler(w http.ResponseWriter, r *http.Request) {
	result := analyzer.GetStatus()
	sendResponse(w, result, http.StatusOK, nil)
}

// sendErrorResponse wraps an error in a response envelope and sends it to
// the client.
func sendErrorResponse(w http.ResponseWriter, errorMsg string, status int) {
	resp := ResponseEnvelope{nil, &errorMsg, status, nil}
	sendJson(w, &resp, status)
}

// sendResponse wraps the data of a succesful response in a response envelope
// and sends it to the client.
func sendResponse(w http.ResponseWriter, data interface{}, status int, pagination *util.ResponsePagination) {
	resp := ResponseEnvelope{&data, nil, status, pagination}
	sendJson(w, &resp, status)
}

// sendJson serializes the response envelope and sends ito the client.
func sendJson(w http.ResponseWriter, resp *ResponseEnvelope, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// parseJson extracts the body from the request and parses it into the
// provided interface.
func parseJson(r *http.Request, v interface{}) error {
	// TODO (stephana): validate the JSON against a schema. Might not be necessary !
	defer util.Close(r.Body)
	decoder := json.NewDecoder(r.Body)
	return decoder.Decode(v)
}

// URLAwareFileServer wraps around a standard file server and allows to generate
// URLs for a given path that is contained in the root.
type URLAwareFileServer struct {
	// baseDir is the root directory for all content served. All paths have to
	// be contained somewhere in the directory tree below this.
	baseDir string

	// baseUrl is the URL prefix that maps to baseDir.
	baseUrl string

	// Handler is a standard FileServer handler.
	Handler http.Handler
}

func NewURLAwareFileServer(baseDir, baseUrl string) *URLAwareFileServer {
	absPath, err := filepath.Abs(baseDir)
	if err != nil {
		glog.Fatalf("Unable to get abs path of %s. Got error: %s", baseDir, err)
	}

	return &URLAwareFileServer{
		baseDir: absPath,
		baseUrl: baseUrl,
		Handler: http.StripPrefix(baseUrl, http.FileServer(http.Dir(absPath))),
	}
}

// converToUrl returns the path component of a URL given the path
// contained within baseDir.
func (ug *URLAwareFileServer) GetURL(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		glog.Errorf("Unable to get absolute path of %s. Got error: %s", path, err)
		return ""
	}

	relPath, err := filepath.Rel(ug.baseDir, absPath)
	if err != nil {
		glog.Errorf("Unable to find subpath got error %s", err)
		return ""
	}

	ret := ug.baseUrl + relPath
	return ret
}

// getOAuthClient returns an oauth client (either from cached credentials or
// via an authentication flow) or nil depending on whether doOauth is false.
func getOAuthClient(doOauth bool, cacheFilePath string) *http.Client {
	if doOauth {
		config := auth.DefaultOAuthConfig(cacheFilePath)
		client, err := auth.RunFlow(config)
		if err != nil {
			glog.Fatalf("Failed to auth: %s", err)
		}
		return client
	}
	return nil
}

func main() {
	var err error

	t := timer.New("main init")
	// Setup DB flags.
	dbConf := database.ConfigFromFlags(db.PROD_DB_HOST, db.PROD_DB_PORT, database.USER_RW, db.PROD_DB_NAME, db.MigrationSteps())

	// Global init to initialize
	common.InitWithMetrics("skiacorrectness", graphiteServer)

	v, err := skiaversion.GetVersion()
	if err != nil {
		glog.Fatalf("Unable to retrieve version: %s", err)
	}
	glog.Infof("Version %s, built at %s", v.Commit, v.Date)

	// Enable the memory profiler if memProfile was set.
	// TODO(stephana): This should be moved to a HTTP endpoint that
	// only responds to internal IP addresses/ports.
	if *memProfile > 0 {
		time.AfterFunc(*memProfile, func() {
			glog.Infof("Writing Memory Profile")
			f, err := ioutil.TempFile("./", "memory-profile")
			if err != nil {
				glog.Fatalf("Unable to create memory profile file: %s", err)
			}
			if err := pprof.WriteHeapProfile(f); err != nil {
				glog.Fatalf("Unable to write memory profile file: %v", err)
			}
			util.Close(f)
			glog.Infof("Memory profile written to %s", f.Name())

			os.Exit(0)
		})
	}

	if *cpuProfile > 0 {
		glog.Infof("Writing CPU Profile")
		f, err := ioutil.TempFile("./", "cpu-profile")
		if err != nil {
			glog.Fatalf("Unable to create cpu profile file: %s", err)
		}

		if err := pprof.StartCPUProfile(f); err != nil {
			glog.Fatalf("Unable to write cpu profile file: %v", err)
		}
		time.AfterFunc(*cpuProfile, func() {
			pprof.StopCPUProfile()
			util.Close(f)
			glog.Infof("CPU profile written to %s", f.Name())
			os.Exit(0)
		})
	}

	// Init this module.
	Init()

	// Initialize submodules.
	filediffstore.Init()

	// Set up login
	// TODO (stephana): Factor out to go/login/login.go and removed hard coded
	// values.
	var cookieSalt = "notverysecret"
	var clientID = "31977622648-ubjke2f3staq6ouas64r31h8f8tcbiqp.apps.googleusercontent.com"
	var clientSecret = "rK-kRY71CXmcg0v9I9KIgWci"
	var useRedirectURL = fmt.Sprintf("http://localhost%s/oauth2callback/", *port)
	if !*local {
		cookieSalt = metadata.Must(metadata.ProjectGet(metadata.COOKIESALT))
		clientID = metadata.Must(metadata.ProjectGet(metadata.CLIENT_ID))
		clientSecret = metadata.Must(metadata.ProjectGet(metadata.CLIENT_SECRET))
		useRedirectURL = *redirectURL
	}
	login.Init(clientID, clientSecret, useRedirectURL, cookieSalt, login.DEFAULT_SCOPE, *authWhiteList, *local)

	// get the Oauthclient if necessary.
	client := getOAuthClient(*doOauth, *oauthCacheFile)

	// Set up the cache implementation to use.
	cacheFactory := filediffstore.MemCacheFactory
	if *redisHost != "" {
		cacheFactory = func(uniqueId string, codec util.LRUCodec) util.LRUCache {
			return redisutil.NewRedisLRUCache(*redisHost, *redisDB, uniqueId, codec)
		}
	}

	// Get the expecations storage, the filediff storage and the tilestore.
	diffStore, err := filediffstore.NewFileDiffStore(client, *imageDir, *gsBucketName, filediffstore.DEFAULT_GS_IMG_DIR_NAME, cacheFactory, filediffstore.RECOMMENDED_WORKER_POOL_SIZE)
	if err != nil {
		glog.Fatalf("Allocating DiffStore failed: %s", err)
	}

	if !*local {
		if err := dbConf.GetPasswordFromMetadata(); err != nil {
			glog.Fatal(err)
		}
	}
	vdb, err := dbConf.NewVersionedDB()
	if err != nil {
		glog.Fatal(err)
	}

	digestStore, err := digeststore.New(*storageDir)
	if err != nil {
		glog.Fatal(err)
	}

	eventBus := eventbus.New()
	storages = &storage.Storage{
		DiffStore:         diffStore,
		ExpectationsStore: expstorage.NewCachingExpectationStore(expstorage.NewSQLExpectationStore(vdb), eventBus),
		IgnoreStore:       ignore.NewSQLIgnoreStore(vdb),
		TileStore:         filetilestore.NewFileTileStore(*tileStoreDir, pconfig.DATASET_GOLD, 2*time.Minute),
		DigestStore:       digestStore,
		NCommits:          *nCommits,
		EventBus:          eventBus,
	}

	if blamer, err = blame.New(storages); err != nil {
		glog.Fatalf("Unable to create blamer: %s", err)
	}

	if err := ignore.Init(storages.IgnoreStore); err != nil {
		glog.Fatalf("Failed to start monitoring for expired ignore rules: %s", err)
	}

	if err := history.Init(storages, *nTilesToBackfill); err != nil {
		glog.Fatalf("Unable to initialize history package: %s", err)
	}

	// Enable the experimental features.
	if *startExperimental {
		tallies, err = tally.New(storages)
		if err != nil {
			glog.Fatalf("Failed to build tallies: %s", err)
		}

		summaries, err = summary.New(storages, tallies)
		if err != nil {
			glog.Fatalf("Failed to build summary: %s", err)
		}

		statusWatcher, err = status.New(storages)
		if err != nil {
			glog.Fatalf("Failed to initialize status watcher: %s", err)
		}
	}

	// Initialize the Analyzer
	imgFS := NewURLAwareFileServer(*imageDir, IMAGE_URL_PREFIX)
	pathToURLConverter = imgFS.GetURL
	if *startAnalyzer {
		analyzer = analysis.NewAnalyzer(storages, imgFS.GetURL, cacheFactory, 5*time.Minute)
	}
	t.Stop()

	router := mux.NewRouter()

	// Wire up the resources. We use the 'rest' prefix to avoid any name
	// clashes witht the static files being served.
	router.HandleFunc("/rest/triage", listTestDetailsHandler).Methods("GET")
	router.HandleFunc("/rest/triage/{testname}", testDetailsHandler).Methods("GET")
	router.HandleFunc("/rest/triage", triageDigestsHandler).Methods("POST")
	router.HandleFunc("/rest/status", util.CorsHandler(statusHandler)).Methods("GET")
	router.HandleFunc("/rest/blame/{testname}", blameHandler).Methods("GET")

	// Set up the login related resources.
	// TODO (stephana): Clean up the URLs so they have the same prefix.
	router.HandleFunc(OAUTH2_CALLBACK_PATH, login.OAuth2CallbackHandler)
	router.HandleFunc("/rest/logout", login.LogoutHandler)
	router.HandleFunc("/rest/loginstatus", login.StatusHandler)

	// Set up the resource to serve the image files.
	router.PathPrefix(IMAGE_URL_PREFIX).Handler(imgFS.Handler)

	// New Polymer based UI endpoints.
	router.PathPrefix("/res/").HandlerFunc(makeResourceHandler())
	// All the handlers will be prefixed with poly to differentiate it from the
	// angular code until the angular code is removed.
	router.HandleFunc("/loginstatus/", login.StatusHandler)
	router.HandleFunc("/logout/", login.LogoutHandler)

	commonEnv.BaseURL = "/2/"
	if !*startAnalyzer {
		commonEnv.BaseURL = "/"
	}

	polyRouter := router.PathPrefix(commonEnv.BaseURL).Subrouter()
	polyRouter.HandleFunc("/", polyMainHandler).Methods("GET")
	polyRouter.HandleFunc("/ignores", polyIgnoresHandler).Methods("GET")
	polyRouter.HandleFunc("/cmp/{test}", polyCompareHandler).Methods("GET")
	polyRouter.HandleFunc("/detail", polySingleDigestHandler).Methods("GET")
	polyRouter.HandleFunc("/diff", polyDiffDigestHandler).Methods("GET")
	polyRouter.HandleFunc("/_/diff", polyDiffJSONDigestHandler).Methods("GET")
	polyRouter.HandleFunc("/_/list", polyListTestsHandler).Methods("GET")
	polyRouter.HandleFunc("/_/paramset", polyParamsHandler).Methods("GET")
	polyRouter.HandleFunc("/_/ignores", polyIgnoresJSONHandler).Methods("GET")
	polyRouter.HandleFunc("/_/ignores/del/{id}", polyIgnoresDeleteHandler).Methods("POST")
	polyRouter.HandleFunc("/_/ignores/add/", polyIgnoresAddHandler).Methods("POST")
	polyRouter.HandleFunc("/_/ignores/save/{id}", polyIgnoresUpdateHandler).Methods("POST")
	polyRouter.HandleFunc("/_/test", polyTestHandler).Methods("POST")
	polyRouter.HandleFunc("/_/details", polyDetailsHandler).Methods("GET")
	polyRouter.HandleFunc("/_/triage", polyTriageHandler).Methods("POST")
	polyRouter.HandleFunc("/_/status/{test}", polyTestStatusHandler).Methods("GET")

	polyRouter.HandleFunc("/triagelog", polyTriageLogView).Methods("GET")
	polyRouter.HandleFunc("/_/triagelog", polyTriageLogHandler).Methods("GET")

	polyRouter.HandleFunc("/_/hashes", polyAllHashesHandler).Methods("GET")

	polyRouter.HandleFunc("/_/status", polyStatusHandler).Methods("GET")

	if *startAnalyzer {
		// Everything else is served out of the static directory.
		router.PathPrefix("/").Handler(http.FileServer(http.Dir(*staticDir)))
	}

	// Add the necessary middleware and have the router handle all requests.
	// By structuring the middleware this way we only log requests that are
	// authenticated.
	rootHandler := util.LoggingGzipRequestResponse(router)
	if *forceLogin {
		rootHandler = login.ForceAuth(rootHandler, OAUTH2_CALLBACK_PATH)
	}
	http.Handle("/", rootHandler)

	// Start the server
	glog.Infoln("Serving on http://127.0.0.1" + *port)
	glog.Fatal(http.ListenAndServe(*port, nil))
}
