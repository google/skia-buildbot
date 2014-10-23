package main

import (
	"encoding/json"
	"flag"
	"net/http"
	"time"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"skia.googlesource.com/buildbot.git/go/database"
	_ "skia.googlesource.com/buildbot.git/go/init"
	"skia.googlesource.com/buildbot.git/golden/go/analysis"
	"skia.googlesource.com/buildbot.git/golden/go/db"
	"skia.googlesource.com/buildbot.git/golden/go/expstorage"
	"skia.googlesource.com/buildbot.git/golden/go/filediffstore"
	"skia.googlesource.com/buildbot.git/perf/go/filetilestore"
)

// Command line flags.
var (
	port         = flag.String("port", ":9000", "HTTP service address (e.g., ':9000')")
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	staticDir    = flag.String("static_dir", "./app", "Directory with static content to serve")
	tileStoreDir = flag.String("tile_store_dir", "/tmp/tileStore", "What directory to look for tiles in.")
	imageDiffDir = flag.String("image_diff_dir", "/tmp/imagediffdir", "What directory to store diff images in.")
	gsBucketName = flag.String("gs_bucket", "chromium-skia-gm", "Name of the google storage bucket that holds uploaded images.")
	mysqlConnStr = flag.String("mysql_conn", "", "MySQL connection string for backend database. If 'local' is false the password in this string will be substituted via the metadata server.")
	sqlitePath   = flag.String("sqlite_path", "./golden.db", "Filepath of the embedded SQLite database. Requires 'local' to be set to true and 'mysql_conn' to be empty to take effect.")
)

// ResponseEnvelope wraps all responses. Some fields might be empty depending
// on context or whether there was an error or not.
type ResponseEnvelope struct {
	Data   *interface{} `json:"data"`
	Err    *string      `json:"err"`
	Status int          `json:"status"`
}

var analyzer *analysis.Analyzer = nil

// tileCountsHandler handles GET requests for the classification counts over
// all tests and digests of a tile.
func tileCountsHandler(w http.ResponseWriter, r *http.Request) {
	result, err := analyzer.GetTileCounts()
	if err != nil {
		sendErrorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sendResponse(w, result, http.StatusOK)
}

// testCountsHandler handles GET requests for the aggregrated classification
// counts for a specific tests.
func testCountsHandler(w http.ResponseWriter, r *http.Request) {
	testName := mux.Vars(r)["testname"]
	result, err := analyzer.GetTestCounts(testName)
	if err != nil {
		sendErrorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}

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

func main() {
	// Get the expecations storage, the filediff storage and the tilestore.
	diffStore := filediffstore.NewFileDiffStore(nil, *imageDiffDir, *gsBucketName)
	vdb := database.NewVersionedDB(db.GetConfig(*mysqlConnStr, *sqlitePath, *local))
	expStore := expstorage.NewSQLExpectationStore(vdb)
	tileStore := filetilestore.NewFileTileStore(*tileStoreDir, "golden", -1)

	// Initialize the Analyzer
	analyzer = analysis.NewAnalyzer(expStore, tileStore, diffStore, 5*time.Minute)

	router := mux.NewRouter()

	// Wire up the resources. We use the 'rest' prefix to avoid any name
	// clashes witht the static files being served.
	// TODO (stephana): Wrap the handlers in autogzip unless we defer that to
	// the front-end proxy.
	router.HandleFunc("/rest/tilecounts", tileCountsHandler)
	router.HandleFunc("/rest/tilecounts/{testname}", testCountsHandler)

	// Everything else is served out of the static directory.
	router.PathPrefix("/").Handler(http.FileServer(http.Dir(*staticDir)))

	// Send all requests to the router
	http.Handle("/", router)

	// Start the server
	glog.Infoln("Serving on http://127.0.0.1" + *port)
	glog.Fatal(http.ListenAndServe(*port, nil))
}
