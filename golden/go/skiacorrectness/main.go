package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/pprof"
	"time"

	"strconv"

	"github.com/fiorix/go-web/autogzip"
	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	"skia.googlesource.com/buildbot.git/go/auth"
	"skia.googlesource.com/buildbot.git/go/common"
	"skia.googlesource.com/buildbot.git/go/database"
	"skia.googlesource.com/buildbot.git/go/login"
	"skia.googlesource.com/buildbot.git/go/metadata"
	"skia.googlesource.com/buildbot.git/go/redisutil"
	"skia.googlesource.com/buildbot.git/go/util"
	"skia.googlesource.com/buildbot.git/golden/go/analysis"
	"skia.googlesource.com/buildbot.git/golden/go/db"
	"skia.googlesource.com/buildbot.git/golden/go/expstorage"
	"skia.googlesource.com/buildbot.git/golden/go/filediffstore"
	"skia.googlesource.com/buildbot.git/golden/go/types"
	"skia.googlesource.com/buildbot.git/perf/go/filetilestore"
)

// Command line flags.
var (
	graphiteServer = flag.String("graphite_server", "skia-monitoring:2003", "Where is Graphite metrics ingestion server running.")
	port           = flag.String("port", ":9000", "HTTP service address (e.g., ':9000')")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	staticDir      = flag.String("static_dir", "./app", "Directory with static content to serve")
	tileStoreDir   = flag.String("tile_store_dir", "/tmp/tileStore", "What directory to look for tiles in.")
	imageDir       = flag.String("image_dir", "/tmp/imagedir", "What directory to store test and diff images in.")
	gsBucketName   = flag.String("gs_bucket", "chromium-skia-gm", "Name of the google storage bucket that holds uploaded images.")
	doOauth        = flag.Bool("oauth", true, "Run through the OAuth 2.0 flow on startup, otherwise use a GCE service account.")
	oauthCacheFile = flag.String("oauth_cache_file", "/home/perf/google_storage_token.data", "Path to the file where to cache cache the oauth credentials.")
	memProfile     = flag.Duration("memprofile", 0, "Duration for which to profile memory. After this duration the program writes the memory profile and exits.")
	resourcesDir   = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the directory relative to the source code files will be used.")
	redisHost      = flag.String("redis_host", "", "The host and port (e.g. 'localhost:6379') of the Redis data store that will be used for caching.")
	redisDB        = flag.Int("redis_db", 0, "The index of the Redis database we should use. Default will work fine in most cases.")
)

var (
	// indexTemplate is the main index.html page we serve.
	indexTemplate *template.Template = nil

	// ignoresTemplate is the page for setting up ignore filters.
	ignoresTemplate *template.Template = nil
)

const (
	IMAGE_URL_PREFIX = "/img/"
)

// ResponseEnvelope wraps all responses. Some fields might be empty depending
// on context or whether there was an error or not.
type ResponseEnvelope struct {
	Data   *interface{} `json:"data"`
	Err    *string      `json:"err"`
	Status int          `json:"status"`
}

var analyzer *analysis.Analyzer = nil

// *****************************************************************************
// *****************************************************************************
// New polymer based UI code begin.
// *****************************************************************************
// *****************************************************************************

// polyMainHandler is the main page for the Polymer based frontend.
func polyMainHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Poly Main Handler: %q\n", r.URL.Path)
	w.Header().Set("Content-Type", "text/html")
	if *local {
		loadTemplates()
	}
	if err := indexTemplate.Execute(w, struct{}{}); err != nil {
		glog.Errorln("Failed to expand template:", err)
	}
}

func loadTemplates() {
	indexTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/index.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
	ignoresTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/ignores.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
}

// polyListTestsHandler returns a JSON list with high level information about
// each test.
//
// The return format looks like:
//
//  [
//    {
//      "name": "01-original",
//      "diameter": 123242,
//      "untriaged": 2,
//      "num": 2
//    },
//    ...
//  ]
//
func polyListTestsHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		util.ReportError(w, r, err, "Failed to parse form data.")
		return
	}
	res, err := analyzer.PolyListTestSimple(r.Form)
	if err != nil {
		util.ReportError(w, r, err, "Failed to load test information")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(res); err != nil {
		util.ReportError(w, r, err, "Failed to encode result")
	}
}

// IgnoreRule is the GUI struct for dealing with Ignore rules.
type IgnoreRule struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Expires time.Time `json:"expires"`
	Query   string    `json:"query"`
	Count   int       `json:"count"`
}

// ignores is an in memory database of ignore rules.
//
// TODO replace with a database.
var ignores = []*IgnoreRule{
	&IgnoreRule{
		ID:      "1",
		Name:    "jcgregorio@google.com",
		Expires: time.Now().Add(time.Hour),
		Query:   "config=gpu",
		Count:   354,
	},
	&IgnoreRule{
		ID:      "2",
		Name:    "jcgregorio@google.com",
		Expires: time.Now().Add(2 * time.Hour * 24),
		Query:   "arch=x86&bench_type=playback&config=8888&extra_config=GDI&os=Android",
		Count:   12,
	},
}

// deleteIgnoreRule deletes an Ignore rule.
//
// TODO replace with database action.
func deleteIgnoreRule(id string) {
	for i, r := range ignores {
		if r.ID == id {
			ignores = append(ignores[:i], ignores[i+1:]...)
			break
		}
	}
}

// addIgnoreRule adds the IgnoreRule to the database.
//
// TODO replace with database action.
func addIgnoreRule(ig *IngoreRule) {
	ignores = append(ignores, ig)
}

// polyIgnoresJSONHandler returns the current ignore rules in JSON format.
func polyIgnoresJSONHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(ignores); err != nil {
		util.ReportError(w, r, err, "Failed to encode result")
	}
}

func polyIgnoresDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	deleteIgnoreRule(id)
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(ignores); err != nil {
		util.ReportError(w, r, err, "Failed to encode result")
	}
}

type IngoresAddRequest struct {
	Duration string `json:"duration"`
	Filter   string `json:"filter"`
}

var durationRe = regexp.MustCompile("([0-9]+)([smhdw])")

// polyIgnoresAddHandler is for adding a new ignore rule.
func polyIgnoresAddHandler(w http.ResponseWriter, r *http.Request) {
	user := login.LoggedInAs(r)
	if user == "" {
		util.ReportError(w, r, fmt.Errorf("Not logged in."), "You must be logged in to add an ignore rule.")
		return
	}
	req := &IngoresAddRequest{}
	if err := parseJson(r, req); err != nil {
		util.ReportError(w, r, err, "Failed to decode result")
		return
	}
	parsed := durationRe.FindStringSubmatch(req.Duration)
	if len(parsed) != 3 {
		util.ReportError(w, r, fmt.Errorf("Rejected duration: %s", req.Duration), "Failed to parse duration")
		return
	}
	// TODO break out the following into its own func, add tests.
	n, err := strconv.ParseInt(parsed[1], 10, 32)
	if err != nil {
		util.ReportError(w, r, err, "Failed to parse duration")
		return
	}
	d := time.Second
	switch parsed[2][0] {
	case 's':
		d = time.Duration(n) * time.Second
	case 'm':
		d = time.Duration(n) * time.Minute
	case 'h':
		d = time.Duration(n) * time.Hour
	case 'd':
		d = time.Duration(n) * 24 * time.Hour
	case 'w':
		d = time.Duration(n) * 7 * 24 * time.Hour
	}
	ig := &IgnoreRule{
		ID:      "foo",
		Name:    user,
		Expires: time.Now().Add(d),
		Query:   req.Filter,
	}
	addIgnoreRule(id)
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(ignores); err != nil {
		util.ReportError(w, r, err, "Failed to encode result")
	}
}

// polyIgnoresHandler is for setting up ignores rules.
func polyIgnoresHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Poly Ignores Handler: %q\n", r.URL.Path)
	w.Header().Set("Content-Type", "text/html")
	if *local {
		loadTemplates()
	}
	if err := ignoresTemplate.Execute(w, struct{}{}); err != nil {
		glog.Errorln("Failed to expand template:", err)
	}
}

func polyParamsHandler(w http.ResponseWriter, r *http.Request) {
	res, err := analyzer.ParamSet()
	if err != nil {
		util.ReportError(w, r, err, "Failed to load ParamSet")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(res); err != nil {
		util.ReportError(w, r, err, "Failed to encode result")
	}
}

// makeResourceHandler creates a static file handler that sets a caching policy.
func makeResourceHandler() func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(*resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", string(300))
		fileServer.ServeHTTP(w, r)
	}
}

// Init figures out where the resources are and then loads the templates.
func Init() {
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}
	loadTemplates()
}

// *****************************************************************************
// *****************************************************************************
// New polymer based UI code end.
// *****************************************************************************
// *****************************************************************************

// tileCountsHandler handles GET requests for the classification counts over
// all tests and digests of a tile.
func tileCountsHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	result, err := analyzer.GetTileCounts(query)
	if err != nil {
		sendErrorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sendResponse(w, result, http.StatusOK)
}

func listTestDetailsHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	result, err := analyzer.ListTestDetails(query)
	if err != nil {
		sendErrorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sendResponse(w, result, http.StatusOK)
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
	sendResponse(w, result, http.StatusOK)
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

	sendResponse(w, result, http.StatusOK)
}

// statusHandler returns the current status with respect to HEAD.
func statusHandler(w http.ResponseWriter, r *http.Request) {
	result := analyzer.GetStatus()
	sendResponse(w, result, http.StatusOK)
}

// sendErrorResponse wraps an error in a response envelope and sends it to
// the client.
func sendErrorResponse(w http.ResponseWriter, errorMsg string, status int) {
	resp := ResponseEnvelope{nil, &errorMsg, status}
	sendJson(w, &resp)
}

// sendResponse wraps the data of a succesful response in a response envelope
// and sends it to the client.
func sendResponse(w http.ResponseWriter, data interface{}, status int) {
	resp := ResponseEnvelope{&data, nil, status}
	sendJson(w, &resp)
}

// sendJson serializes the response envelope and sends ito the client.
func sendJson(w http.ResponseWriter, resp *ResponseEnvelope) {
	jsonBytes, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonBytes)
}

// parseJson extracts the body from the request and parses it into the
// provided interface.
func parseJson(r *http.Request, v interface{}) error {
	// TODO (stephana): validate the JSON against a schema. Might not be necessary !
	defer r.Body.Close()
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
	// Setup DB flags.
	database.SetupFlags(db.PROD_DB_HOST, db.PROD_DB_PORT, database.USER_RW, db.PROD_DB_NAME)

	// Global init to initialize
	common.InitWithMetrics("skiacorrectness", graphiteServer)

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
			pprof.WriteHeapProfile(f)
			f.Close()
			glog.Infof("Memory profile written to %s", f.Name())

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
	var redirectURL = fmt.Sprintf("http://localhost%s/oauth2callback/", *port)
	if !*local {
		cookieSalt = metadata.Must(metadata.ProjectGet(metadata.COOKIESALT))
		clientID = metadata.Must(metadata.ProjectGet(metadata.CLIENT_ID))
		clientSecret = metadata.Must(metadata.ProjectGet(metadata.CLIENT_SECRET))
		redirectURL = "https://gold.skia.org/oauth2callback/"
	}
	login.Init(clientID, clientSecret, redirectURL, cookieSalt)

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
	conf, err := database.ConfigFromFlagsAndMetadata(*local, db.MigrationSteps())
	if err != nil {
		glog.Fatal(err)
	}
	vdb := database.NewVersionedDB(conf)
	expStore := expstorage.NewCachingExpectationStore(expstorage.NewSQLExpectationStore(vdb))
	tileStore := filetilestore.NewFileTileStore(*tileStoreDir, "golden", -1)

	// Initialize the Analyzer
	imgFS := NewURLAwareFileServer(*imageDir, IMAGE_URL_PREFIX)
	analyzer = analysis.NewAnalyzer(expStore, tileStore, diffStore, imgFS.GetURL, 10*time.Minute)

	router := mux.NewRouter()

	// Wire up the resources. We use the 'rest' prefix to avoid any name
	// clashes witht the static files being served.
	// TODO (stephana): Wrap the handlers in autogzip unless we defer that to
	// the front-end proxy.
	router.HandleFunc("/rest/counts", tileCountsHandler).Methods("GET")
	router.HandleFunc("/rest/triage", listTestDetailsHandler).Methods("GET")
	router.HandleFunc("/rest/triage/{testname}", testDetailsHandler).Methods("GET")
	router.HandleFunc("/rest/triage", triageDigestsHandler).Methods("POST")
	router.HandleFunc("/rest/status", statusHandler).Methods("GET")

	// Set up the login related resources.
	// TODO (stephana): Clean up the URLs so they have the same prefix.
	http.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	http.HandleFunc("/rest/logout", login.LogoutHandler)
	http.HandleFunc("/rest/loginstatus", login.StatusHandler)

	// Set up the resource to serve the image files.
	router.PathPrefix(IMAGE_URL_PREFIX).Handler(imgFS.Handler)

	// New Polymer based UI endpoints.
	router.PathPrefix("/res/").Handler(autogzip.HandleFunc(makeResourceHandler()))
	// All the handlers will be prefixed with poly to differentiate it from the
	// angular code until the angular code is removed.
	http.HandleFunc("/loginstatus/", login.StatusHandler)
	http.HandleFunc("/logout/", login.LogoutHandler)
	router.HandleFunc("/2/", polyMainHandler).Methods("GET")
	router.HandleFunc("/2/ignores", polyIgnoresHandler).Methods("GET")
	router.HandleFunc("/2/_/list", polyListTestsHandler).Methods("GET")
	router.HandleFunc("/2/_/paramset", polyParamsHandler).Methods("GET")
	router.HandleFunc("/2/_/ignores", polyIgnoresJSONHandler).Methods("GET")
	router.HandleFunc("/2/_/ignores/del/{id}", polyIgnoresDeleteHandler).Methods("POST")
	router.HandleFunc("/2/_/ignores/add/", polyIgnoresAddHandler).Methods("POST")

	// Everything else is served out of the static directory.
	router.PathPrefix("/").Handler(http.FileServer(http.Dir(*staticDir)))

	// Send all requests to the router
	http.Handle("/", router)

	// Start the server
	glog.Infoln("Serving on http://127.0.0.1" + *port)
	glog.Fatal(http.ListenAndServe(*port, nil))
}
