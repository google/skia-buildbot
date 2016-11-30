package main

import (
	"encoding/json"
	"flag"
	"fmt"
	ehtml "html"
	"html/template"
	"math/rand"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	storage "google.golang.org/api/storage/v1"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/tiling"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/activitylog"
	"go.skia.org/infra/perf/go/alerting"
	"go.skia.org/infra/perf/go/annotate"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/clustering"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	idb "go.skia.org/infra/perf/go/db"
	"go.skia.org/infra/perf/go/parser"
	_ "go.skia.org/infra/perf/go/ptraceingest"
	"go.skia.org/infra/perf/go/ptracestore"
	"go.skia.org/infra/perf/go/quartiles"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/shortcut"
	"go.skia.org/infra/perf/go/shortcut2"
	"go.skia.org/infra/perf/go/stats"
	"go.skia.org/infra/perf/go/tilestats"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/vec"
)

var (
	// TODO(jcgregorio) Make into a flag.
	BEGINNING_OF_TIME = time.Date(2014, time.June, 18, 0, 0, 0, 0, time.UTC)
)

var (
	// indexTemplate is the main index.html page we serve.
	indexTemplate *template.Template = nil

	// clusterTemplate is the /clusters/ page we serve.
	clusterTemplate *template.Template = nil

	alertsTemplate *template.Template = nil

	clTemplate *template.Template = nil

	activityTemplate *template.Template = nil

	helpTemplate *template.Template = nil

	// compareTemplate is the /compare/ page we serve.
	compareTemplate *template.Template = nil

	jsonHandlerPath = regexp.MustCompile(`/json/([a-z]*)$`)

	shortcutHandlerPath = regexp.MustCompile(`/shortcuts/([0-9]*)$`)

	// The three capture groups are dataset, tile scale, and tile number.
	tileHandlerPath = regexp.MustCompile(`/tiles/([0-9]*)/([-0-9]*)/$`)

	// The optional capture group is a githash.
	singleHandlerPath = regexp.MustCompile(`/single/([0-9a-f]+)?$`)

	// The three capture groups are tile scale, tile number, and an optional 'trace.
	queryHandlerPath = regexp.MustCompile(`/query/([0-9]*)/([-0-9]*)/(traces/)?$`)

	clHandlerPath = regexp.MustCompile(`/cl/([0-9]*)$`)

	activityHandlerPath = regexp.MustCompile(`/activitylog/([0-9]*)$`)

	git *gitinfo.GitInfo = nil

	cidl *cid.CommitIDLookup = nil

	commitLinkifyRe = regexp.MustCompile("(?m)^commit (.*)$")
)

// flags
var (
	configFilename = flag.String("config_filename", "default.toml", "Configuration file in TOML format.")
	clusterQueries = flag.String("cluster_queries", "source_type=skp&sub_result=min_ms source_type=svg&sub_result=min_ms", "A space separated list of queries we want to cluster over.")
	gitRepoDir     = flag.String("git_repo_dir", "../../../skia", "Directory location for the Skia repo.")
	gitRepoURL     = flag.String("git_repo_url", "https://skia.googlesource.com/skia", "The URL to pass to git clone for the source repository.")
	influxDatabase = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")
	influxHost     = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxPassword = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxUser     = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	newonly        = flag.Bool("newonly", false, "Only run with the new UI, don't load tracedb stuff.")
	port           = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	ptraceStoreDir = flag.String("ptrace_store_dir", "/tmp/ptracestore", "The directory where the ptracestore tiles are stored.")
	resourcesDir   = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	tileSize       = flag.Int("tile_size", 100, "The size of Tiles.")
	traceservice   = flag.String("trace_service", "localhost:9090", "The address of the traceservice endpoint.")
)

var (
	evt *eventbus.EventBus

	masterTileBuilder tracedb.MasterTileBuilder

	branchTileBuilder tracedb.BranchTileBuilder

	templates *template.Template

	tileStats *tilestats.TileStats

	freshDataFrame *dataframe.Refresher

	frameRequests *dataframe.RunningFrameRequests

	clusterRequests *clustering2.RunningClusterRequests

	regStore *regression.Store
)

func loadTemplates() {
	templates = template.Must(template.New("").ParseFiles(
		filepath.Join(*resourcesDir, "templates/index.html"),
		filepath.Join(*resourcesDir, "templates/clusters.html"),
		filepath.Join(*resourcesDir, "templates/alerting.html"),
		filepath.Join(*resourcesDir, "templates/cl.html"),
		filepath.Join(*resourcesDir, "templates/activitylog.html"),
		filepath.Join(*resourcesDir, "templates/compare.html"),
		filepath.Join(*resourcesDir, "templates/help.html"),
		filepath.Join(*resourcesDir, "templates/frame.html"),
		filepath.Join(*resourcesDir, "templates/percommit.html"),

		// ptracestore pages go here.
		filepath.Join(*resourcesDir, "templates/newindex.html"),
		filepath.Join(*resourcesDir, "templates/clusters2.html"),
		filepath.Join(*resourcesDir, "templates/triage.html"),

		// Sub templates used by other templates.
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
}

func templateHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if *local {
			loadTemplates()
		}
		if err := templates.ExecuteTemplate(w, name, struct{}{}); err != nil {
			glog.Errorln("Failed to expand template:", err)
		}
	}
}

func Init() {
	rand.Seed(time.Now().UnixNano())
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}

	loadTemplates()

	var err error
	git, err = gitinfo.CloneOrUpdate(*gitRepoURL, *gitRepoDir, false)
	if err != nil {
		glog.Fatal(err)
	}
	ptracestore.Init(*ptraceStoreDir)

	freshDataFrame, err = dataframe.NewRefresher(git, ptracestore.Default, time.Minute)
	if err != nil {
		glog.Fatalf("Failed to build the dataframe Refresher: %s", err)
	}

	initIngestion()
	rietveldAPI := rietveld.New(rietveld.RIETVELD_SKIA_URL, httputils.NewTimeoutClient())
	// TODO(stephana): Add gerrit url as a flag and pick correct cookie configs.
	gerritAPI, err := gerrit.NewGerrit(gerrit.GERRIT_SKIA_URL, "", httputils.NewTimeoutClient())
	if err != nil {
		glog.Fatalf("Failed to create Gerrit client: %s", err)
	}
	cidl = cid.New(git, rietveldAPI)

	frameRequests = dataframe.NewRunningFrameRequests(git)
	clusterRequests = clustering2.NewRunningClusterRequests(git, cidl)
	dataframe.StartWarmer(git)
	regStore = regression.NewStore()

	// Start running continuous clustering looking for regressions.
	queries := strings.Split(*clusterQueries, " ")
	go regression.NewContinuous(git, cidl, queries, regStore).Run()

	if !*newonly {
		evt := eventbus.New(nil)
		tileStats = tilestats.New(evt)
		// Connect to traceDB and create the builders.
		db, err := tracedb.NewTraceServiceDBFromAddress(*traceservice, types.PerfTraceBuilder)
		if err != nil {
			glog.Fatalf("Failed to connect to tracedb: %s", err)
		}

		masterTileBuilder, err = tracedb.NewMasterTileBuilder(db, git, *tileSize, evt)
		if err != nil {
			glog.Fatalf("Failed to build trace/db.DB: %s", err)
		}

		branchTileBuilder = tracedb.NewBranchTileBuilder(db, git, rietveldAPI, gerritAPI, evt)
	}
}

// showcutHandler handles the POST requests of the shortcut page.
//
// Shortcuts are of the form:
//
//    {
//       "scale": 0,
//       "tiles": [-1],
//       "hash": "a1092123890...",
//       "ids": [
//            "x86:...",
//            "x86:...",
//            "x86:...",
//       ]
//    }
//
// hash - The git hash of where a step was detected. Can be null.
//
func shortcutHandler(w http.ResponseWriter, r *http.Request) {
	// TODO(jcgregorio): Add unit tests.
	match := shortcutHandlerPath.FindStringSubmatch(r.URL.Path)
	if match == nil {
		http.NotFound(w, r)
		return
	}
	if r.Method == "POST" {
		// check header
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			httputils.ReportError(w, r, fmt.Errorf("Error: received %s", ct), "Invalid content type.")
			return
		}
		defer util.Close(r.Body)
		id, err := shortcut.Insert(r.Body)
		if err != nil {
			httputils.ReportError(w, r, err, "Error inserting shortcut.")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		if err := enc.Encode(map[string]string{"id": id}); err != nil {
			glog.Errorf("Failed to write or encode output: %s", err)
		}
	} else {
		http.NotFound(w, r)
	}
}

// alertingHandler returns the currently untriaged clusters.
//
// The return format is the same as clusteringHandler.
func alertingHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Alerting Handler: %q\n", r.URL.Path)
	w.Header().Set("Content-Type", "application/json")
	tile := masterTileBuilder.GetTile()
	alerts, err := alerting.ListFrom(tile.Commits[0].CommitTime)
	if err != nil {
		httputils.ReportError(w, r, err, "Error retrieving cluster summaries.")
		return
	}
	enc := json.NewEncoder(w)
	if err = enc.Encode(map[string][]*types.ClusterSummary{"Clusters": alerts}); err != nil {
		glog.Errorf("Failed to write or encode output: %s", err)
	}
}

// alertResetHandler deletes all the non-Bug alerts.
//
func alertResetHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("AlertResetHandler: %q\n", r.URL.Path)
	if login.LoggedInAs(r) == "" {
		httputils.ReportError(w, r, fmt.Errorf("Not logged in."), "You must be logged in to change an alert status.")
		return
	}
	if r.Method != "POST" {
		http.NotFound(w, r)
		return
	}
	if err := alerting.Reset(); err != nil {
		glog.Errorln("Failed to delete all non-Bug alerts:", err)
	}
	http.Redirect(w, r, "/alerts/", 303)
}

// clHandler serves the HTML for the /cl/<id> page.
//
// These are shortcuts to individual clusters.
//
// See alertingHandler for the JSON it uses.
//
func clHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *local {
		loadTemplates()
	}

	match := clHandlerPath.FindStringSubmatch(r.URL.Path)
	if r.Method != "GET" || match == nil || len(match) != 2 {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.ParseInt(match[1], 10, 0)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed parsing ID.")
		return
	}
	cl, err := alerting.Get(id)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to find cluster with that ID.")
		return
	}
	if err := templates.ExecuteTemplate(w, "cl.html", cl); err != nil {
		glog.Errorln("Failed to expand template:", err)
	}
}

// activityHandler serves the HTML for the /activitylog/ page.
//
// If an optional number n is appended to the path, returns the most recent n
// activities. Otherwise returns the most recent 100 results.
//
func activityHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *local {
		loadTemplates()
	}

	match := activityHandlerPath.FindStringSubmatch(r.URL.Path)
	if r.Method != "GET" || match == nil || len(match) != 2 {
		http.NotFound(w, r)
		return
	}
	n := 100
	if len(match[1]) > 0 {
		num, err := strconv.ParseInt(match[1], 10, 0)
		if err != nil {
			httputils.ReportError(w, r, err, "Failed parsing the given number.")
			return
		}
		n = int(num)
	}
	a, err := activitylog.GetRecent(n)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to retrieve activity.")
		return
	}
	if err := templates.ExecuteTemplate(w, "activitylog.html", a); err != nil {
		glog.Errorln("Failed to expand template:", err)
	}
}

// writeClusterSummaries writes out a ClusterSummaries instance as a JSON response.
func writeClusterSummaries(summary *clustering.ClusterSummaries, w http.ResponseWriter, r *http.Request) {
	enc := json.NewEncoder(w)
	if err := enc.Encode(summary); err != nil {
		glog.Errorf("Failed to write or encode output: %s", err)
	}
}

// perCommitJSONHandler returns the data needed for displaying statistics about a single commit, either
// a real commit, or a trybot run.
//
// The return format is a serialized kmlabel.Description:
//
//  {
//    percent: 0.041025641025641026,
//    centers: [
//      {
//        ids: [
//          "Arm7:GCC:GPU:Adreno330:Nexus5:Android:GM_multipicturedraw_rectclip_simple_180_286:msaa4",
//          ...
//        ],
//        size: 21,
//        wordcloud: [
//          [{Value: "Android", Weight: 26}],
//          [{Value: "Qualcomm", Weight: 25}, {Value: "ARM", Weight: 12}]
//          ...
//        ],
//      },
//      {
//        ids: [
//          "Arm7:GCC:GPU:Adreno330:Nexus5:Android:GM_convex_poly_clip_870_540:gpu",
//          "Arm7:GCC:GPU:Adreno330:Nexus5:Android:GM_radial_gradient3_500_500:gpu"
//          ...
//        ],
//        size: 13,
//        wordcloud: [
//          [{Value: "Arm7", Weight: 21}, {Value: "x86_64", Weight: 16}],
//          [{Value: "GCC", Weight: 21}, {Value: "Clang", Weight: 16}],
//          [{Value: "Qualcomm", Weight: 21}, {Value: "Intel Inc.", Weight: 15}],
//        ]
//      }
//    ]
//  }
//
// Takes the following query parameters:
//
//   ref_id     - The reference commit id.
//   ref_source - The reference commit value.
//   ref_ts     - The reference commit timestamp.
//   query      - A paramset in URI query format used to filter the results at each commit.
//
func perCommitJSONHandler(w http.ResponseWriter, r *http.Request) {
	ref_ts, err := strconv.Atoi(r.FormValue("ref_ts"))
	if err != nil {
		httputils.ReportError(w, r, fmt.Errorf("Failed to parse value."), "Invalid ref_ts.")
		return
	}
	commits := []*tracedb.CommitID{
		&tracedb.CommitID{
			ID:        r.FormValue("ref_id"),
			Source:    r.FormValue("ref_source"),
			Timestamp: int64(ref_ts),
		},
	}
	glog.Infof("%#v", *commits[0])
	tile, err := branchTileBuilder.CachedTileFromCommits(commits)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to create tile from query.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	query := r.FormValue("query")
	q, err := url.ParseQuery(query)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to parse query parameters."))
		return
	}
	ret := quartiles.FromTile(tile, tileStats, q)
	enc := json.NewEncoder(w)
	if err := enc.Encode(ret); err != nil {
		glog.Errorf("Failed to write or encode output: %s", err)
		return
	}
}

// clusteringHandler handles doing the actual k-means clustering.
//
// The return format is JSON of the form:
//
// {
//   "Clusters": [
//     {
//       "Keys": [
//          "x86:GeForce320M:MacMini4.1:Mac10.8:GM_varied_text_clipped_no_lcd_640_480:8888",...],
//       "ParamSummaries": [
//           [{"Value": "Win8", "Weight": 15}, {"Value": "Android", "Weight": 14}, ...]
//       ],
//       "StepFit": {
//          "LeastSquares":0.0006582442047814354,
//          "TurningPoint":162,
//          "StepSize":0.023272272692293046,
//          "Regression": 35.3
//       }
//       Traces: [[[0, -0.00007967326606768456], [1, 0.011877665949459049], [2, 0.012158129176717419],...]]
//     },
//     ...
//   ],
//   "K": 5,
//   "StdDevThreshhold": 0.1
// }
//
// Note that Keys contains all the keys, while Traces only contains traces of
// the N closest cluster members and the centroid.
//
// Takes the following query parameters:
//
//   _k      - The K to use for k-means clustering.
//   _stddev - The standard deviation to use when normalize traces
//             during k-means clustering.
//   _issue  - The Rietveld issue ID with trybot results to include.
//
// Additionally the rest of the query parameters as returned from
// sk.Query.selectionsAsQuery().
func clusteringHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Clustering Handler: %q\n", r.URL.Path)
	tile := masterTileBuilder.GetTile()
	w.Header().Set("Content-Type", "application/json")
	// If there are no query parameters just return with an empty set of ClusterSummaries.
	if r.FormValue("_k") == "" || r.FormValue("_stddev") == "" {
		writeClusterSummaries(clustering.NewClusterSummaries(), w, r)
		return
	}

	k, err := strconv.ParseInt(r.FormValue("_k"), 10, 32)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("_k parameter must be an integer %s.", r.FormValue("_k")))
		return
	}
	stddev, err := strconv.ParseFloat(r.FormValue("_stddev"), 64)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("_stddev parameter must be a float %s.", r.FormValue("_stddev")))
		return
	}

	delete(r.Form, "_k")
	delete(r.Form, "_stddev")
	delete(r.Form, "_issue")

	// Create a filter function for traces that match the query parameters and
	// optionally tryResults.
	filter := func(key string, tr *types.PerfTrace) bool {
		return tiling.Matches(tr, r.Form)
	}

	summary, err := clustering.CalculateClusterSummaries(tile, int(k), stddev, filter)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to calculate clusters.")
		return
	}
	writeClusterSummaries(summary, w, r)
}

// tileHandler accepts URIs like /tiles/0/1
// where the URI format is /tiles/<tile-scale>/<tile-number>
//
// It returns JSON of the form:
//
//  {
//    tiles: [20],
//    scale: 0,
//    paramset: {
//      "os": ["Android", "ChromeOS", ..],
//      "arch": ["Arm7", "x86", ...],
//    },
//    commits: [
//      {
//        "commit_time": 140329432,
//        "hash": "0e03478100ea",
//        "author": "someone@google.com",
//        "commit_msg": "The subject line of the commit.",
//      },
//      ...
//    ],
//    ticks: [
//      [1.5, "Mon"],
//      [3.5, "Tue"]
//    ],
//    skps: [
//      5, 13, 24
//    ]
//  }
//
//  Where skps are the commit indices where the SKPs were updated.
//
func tileHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Tile Handler: %q\n", r.URL.Path)
	handlerStart := time.Now()
	match := tileHandlerPath.FindStringSubmatch(r.URL.Path)
	if r.Method != "GET" || match == nil || len(match) != 3 {
		http.NotFound(w, r)
		return
	}
	tileScale, err := strconv.ParseInt(match[1], 10, 0)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed parsing tile scale.")
		return
	}
	tileNumber, err := strconv.ParseInt(match[2], 10, 0)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed parsing tile number.")
		return
	}
	glog.Infof("tile: %d %d", tileScale, tileNumber)
	tile := masterTileBuilder.GetTile()
	guiTile := tiling.NewTileGUI(tile.Scale, tile.TileIndex)
	guiTile.Commits = tile.Commits
	guiTile.ParamSet = tile.ParamSet
	// SkpCommits goes out to the git repo, add caching if this turns out to be
	// slow.
	if skps, err := git.SkpCommits(tile); err != nil {
		guiTile.Skps = []int{}
		glog.Errorf("Failed to calculate skps: %s", err)
	} else {
		guiTile.Skps = skps
	}

	ts := []int64{}
	for _, c := range tile.Commits {
		if c.CommitTime != 0 {
			ts = append(ts, c.CommitTime)
		}
	}
	glog.Infof("%#v", ts)
	guiTile.Ticks = human.FlotTickMarks(ts)

	// Marshal and send
	marshaledResult, err := json.Marshal(guiTile)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to marshal JSON.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(marshaledResult)
	if err != nil {
		glog.Errorf("Failed to write or encode output: %s", err)
	}
	glog.Infoln("Total handler time: ", time.Since(handlerStart).Nanoseconds())
}

// QueryResponse is for formatting the JSON output from queryHandler.
type QueryResponse struct {
	Traces []*tiling.TraceGUI `json:"traces"`
	Hash   string             `json:"hash"`
}

// FlatQueryResponse is for formatting the JSON output from calcHandler when the user
// requests flat=true. The output isn't formatted for input into Flot, instead the Values
// are returned as a simple slice, which is easier to work with in IPython.
type FlatQueryResponse struct {
	Traces []*types.PerfTrace
}

// queryHandler handles queries for and about traces.
//
// Queries look like:
//
//     /query/0/-1/?arch=Arm7&arch=x86&scale=1
//
// Where they keys and values in the query params are from the ParamSet.
// Repeated parameters are matched via OR. I.e. the above query will include
// anything that has an arch of Arm7 or x86.
//
// The first two path paramters are tile scale and tile number, where -1 means
// the last tile at the given scale.
//
// The normal response is JSON of the form:
//
// {
//   "matches": 187,
// }
//
// If the path is:
//
//    /query/0/-1/traces/?arch=Arm7&arch=x86&scale=1
//
// Then the response is the set of traces that match that query.
//
//  {
//    "traces": [
//      {
//        // All of these keys and values should be exactly what Flot consumes.
//        data: [[1, 1.1], [20, 30]],
//        label: "key1",
//        _params: {"os: "Android", ...}
//      },
//      {
//        data: [[1.2, 2.1], [20, 35]],
//        label: "key2",
//        _params: {"os: "Android", ...}
//      }
//    ]
//  }
//
// If the path is:
//
//    /query/0/-1/traces/?__shortcut=11
//
// Then the traces in the shortcut with that ID are returned, along with the
// git hash at the step function, if the shortcut came from an alert.
//
//  {
//    "traces": [
//      {
//        // All of these keys and values should be exactly what Flot consumes.
//        data: [[1, 1.1], [20, 30]],
//        label: "key1",
//        _params: {"os: "Android", ...}
//      },
//      ...
//    ],
//    "hash": "a012334...",
//  }
//
//
// TODO Add ability to query across a range of tiles.
func queryHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Query Handler: %q\n", r.URL.Path)
	match := queryHandlerPath.FindStringSubmatch(r.URL.Path)
	glog.Infof("%#v", match)
	if r.Method != "GET" || match == nil || len(match) != 4 {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		httputils.ReportError(w, r, err, "Failed to parse query params.")
	}
	tileScale, err := strconv.ParseInt(match[1], 10, 0)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed parsing tile scale.")
		return
	}
	tileNumber, err := strconv.ParseInt(match[2], 10, 0)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed parsing tile number.")
		return
	}
	glog.Infof("tile: %d %d", tileScale, tileNumber)
	tile := masterTileBuilder.GetTile()
	w.Header().Set("Content-Type", "application/json")
	ret := &QueryResponse{
		Traces: []*tiling.TraceGUI{},
		Hash:   "",
	}
	if match[3] == "" {
		// We only want the count.
		total := 0
		for _, tr := range tile.Traces {
			if tiling.Matches(tr, r.Form) {
				total++
			}
		}
		glog.Info("Count: ", total)
		inc := json.NewEncoder(w)
		if err := inc.Encode(map[string]int{"matches": total}); err != nil {
			glog.Errorf("Failed to write or encode output: %s", err)
			return
		}
	} else {
		// We want the matching traces.
		shortcutID := r.Form.Get("__shortcut")
		if shortcutID != "" {
			sh, err := shortcut.Get(shortcutID)
			if err != nil {
				http.NotFound(w, r)
				return
			}

			ret.Hash = sh.Hash
			for _, k := range sh.Keys {
				if tr, ok := tile.Traces[k]; ok {
					tg := traceGuiFromTrace(tr.(*types.PerfTrace), k, tile)
					if tg != nil {
						ret.Traces = append(ret.Traces, tg)
					}
				} else if tiling.IsFormulaID(k) {
					// Re-evaluate the formula and add all the results to the response.
					formula := tiling.FormulaFromID(k)
					if err := addCalculatedTraces(ret, tile, formula); err != nil {
						glog.Errorf("Failed evaluating formula (%q) while processing shortcut %s: %s", formula, shortcutID, err)
					}
				} else if strings.HasPrefix(k, "!") {
					glog.Errorf("A calculated trace is slipped through: (%s) in shortcut %s: %s", k, shortcutID, err)
				}
			}
		} else {
			for key, tr := range tile.Traces {
				if tiling.Matches(tr, r.Form) {
					tg := traceGuiFromTrace(tr.(*types.PerfTrace), key, tile)
					if tg != nil {
						ret.Traces = append(ret.Traces, tg)
					}
				}
			}
		}
		enc := json.NewEncoder(w)
		if err := enc.Encode(ret); err != nil {
			glog.Errorf("Failed to write or encode output: %s", err)
			return
		}
	}
}

// SingleTrace is used in SingleResponse.
type SingleTrace struct {
	Val    float64           `json:"val"`
	Params map[string]string `json:"params"`
}

// SingleResponse is for formatting the JSON output from singleHandler.
// Hash is the commit hash whose data are used in Traces.
type SingleResponse struct {
	Traces []*SingleTrace `json:"traces"`
	Hash   string         `json:"hash"`
}

// singleHandler is similar to /query/0/-1/traces?<param filters>, but takes an
// optional commit hash and returns a single value for each trace at that commit,
// or the latest value if a hash is not given or found. The resulting JSON is in
// SingleResponse format that looks like:
//
//  {
//    "traces": [
//      {
//        val: 1.1,
//        params: {"os: "Android", ...}
//      },
//      ...
//    ],
//    "hash": "abc123",
//  }
//
func singleHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Single Handler: %q\n", r.URL.Path)
	handlerStart := time.Now()
	match := singleHandlerPath.FindStringSubmatch(r.URL.Path)
	if r.Method != "GET" || match == nil || len(match) != 2 {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		httputils.ReportError(w, r, err, "Failed to parse query params.")
	}
	hash := match[1]

	tileNum, idx, err := git.TileAddressFromHash(hash, BEGINNING_OF_TIME)
	if err != nil {
		glog.Infof("Did not find hash '%s', use latest: %q.\n", hash, err)
		tileNum = -1
		idx = -1
	}
	glog.Infof("Hash: %s tileNum: %d, idx: %d\n", hash, tileNum, idx)
	tile := masterTileBuilder.GetTile()

	if idx < 0 {
		idx = len(tile.Commits) - 1 // Defaults to the last slice element.
	}
	glog.Infof("Tile: %d; Idx: %d\n", tileNum, idx)

	ret := SingleResponse{
		Traces: []*SingleTrace{},
		Hash:   tile.Commits[idx].Hash,
	}
	for _, tr := range tile.Traces {
		if tiling.Matches(tr, r.Form) {
			v, err := vec.FillAt(tr.(*types.PerfTrace).Values, idx)
			if err != nil {
				httputils.ReportError(w, r, err, "Error while getting value at slice index.")
				return
			}
			t := &SingleTrace{
				Val:    v,
				Params: tr.Params(),
			}
			ret.Traces = append(ret.Traces, t)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(ret); err != nil {
		glog.Errorf("Failed to write or encode output: %s", err)
	}
	glog.Infoln("Total handler time: ", time.Since(handlerStart).Nanoseconds())
}

// traceGuiFromTrace returns a populated TraceGUI from the given trace.
func traceGuiFromTrace(trace *types.PerfTrace, key string, tile *tiling.Tile) *tiling.TraceGUI {
	newTraceData := make([][2]float64, 0)
	for i, v := range trace.Values {
		if v != config.MISSING_DATA_SENTINEL && tile.Commits[i] != nil && tile.Commits[i].CommitTime > 0 {
			//newTraceData = append(newTraceData, [2]float64{float64(tile.Commits[i].CommitTime), v})
			newTraceData = append(newTraceData, [2]float64{float64(i), v})
		}
	}
	if len(newTraceData) >= 0 {
		return &tiling.TraceGUI{
			Data:   newTraceData,
			Label:  key,
			Params: trace.Params(),
		}
	} else {
		return nil
	}
}

// addCalculatedTraces adds the traces returned from evaluating the given
// formula over the given tile to the QueryResponse.
func addCalculatedTraces(qr *QueryResponse, tile *tiling.Tile, formula string) error {
	ctx := parser.NewContext(tile)
	traces, err := ctx.Eval(formula)
	if err != nil {
		return fmt.Errorf("Failed to evaluate formula %q: %s", formula, err)
	}
	hasFormula := false
	for _, tr := range traces {
		if tiling.IsFormulaID(tr.Params()["id"]) {
			hasFormula = true
		}
		tg := traceGuiFromTrace(tr, tr.Params()["id"], tile)
		qr.Traces = append(qr.Traces, tg)
	}
	if !hasFormula {
		// If we haven't added the formula trace to the response yet, add it in now.
		f := types.NewPerfTraceN(len(tile.Commits))
		tg := traceGuiFromTrace(f, tiling.AsFormulaID(formula), tile)
		qr.Traces = append(qr.Traces, tg)
	}
	return nil
}

// addFlatCalculatedTraces adds the traces returned from evaluating the given
// formula over the given tile to the FlatQueryResponse. Doesn't include an empty
// formula trace. Useful for pulling data into IPython.
func addFlatCalculatedTraces(qr *FlatQueryResponse, tile *tiling.Tile, formula string) error {
	ctx := parser.NewContext(tile)
	traces, err := ctx.Eval(formula)
	if err != nil {
		return fmt.Errorf("Failed to evaluate formula %q: %s", formula, err)
	}
	for _, tr := range traces {
		qr.Traces = append(qr.Traces, tr)
	}
	return nil
}

// calcHandler handles requests for the form:
//
//    /calc/?formula=filter("config=8888")
//
// Where the formula is any formula that parser.Eval() accepts.
//
// The response is the same format as queryHandler.
func calcHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Calc Handler: %q\n", r.URL.Path)
	w.Header().Set("Content-Type", "application/json")
	tile := masterTileBuilder.GetTile()
	formula := r.FormValue("formula")

	var data interface{} = nil
	if r.FormValue("flat") == "true" {
		resp := &FlatQueryResponse{
			Traces: []*types.PerfTrace{},
		}
		if err := addFlatCalculatedTraces(resp, tile, formula); err != nil {
			httputils.ReportError(w, r, err, fmt.Sprintf("Failed in /calc/ to evaluate formula."))
			return
		}
		data = resp
	} else {
		resp := &QueryResponse{
			Traces: []*tiling.TraceGUI{},
			Hash:   "",
		}
		if err := addCalculatedTraces(resp, tile, formula); err != nil {
			httputils.ReportError(w, r, err, fmt.Sprintf("Failed in /calc/ to evaluate formula."))
			return
		}
		data = resp
	}
	enc := json.NewEncoder(w)
	if err := enc.Encode(data); err != nil {
		glog.Errorf("Failed to write or encode output: %s", err)
		return
	}
}

// commitsHandler handles requests for commits.
//
// Queries look like:
//
//     /commits/?begin=hash1&end=hash2
//
//  or if there is only one hash:
//
//     /commits/?begin=hash
//
// The response is HTML of the form:
//
//  <pre>
//    commit <a href="http://skia.googlesource....">5bdbd13d8833d23e0da552f6817ae0b5a4e849e5</a>
//    Author: Joe Gregorio <jcgregorio@google.com>
//    Date:   Wed Aug 6 16:16:18 2014 -0400
//
//        Back to plotting lines.
//
//        perf/go/skiaperf/perf.go
//        perf/go/types/types.go
//        perf/res/js/logic.js
//
//    commit <a
//    ...
//  </pre>
//
func commitsHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Query Handler: %q\n", r.URL.Path)
	if r.Method != "GET" {
		http.NotFound(w, r)
		return
	}
	begin := r.FormValue("begin")
	if len(begin) != 40 {
		httputils.ReportError(w, r, fmt.Errorf("Invalid hash format: %s", begin), "Error while looking up hashes.")
		return
	}
	end := r.FormValue("end")
	body, err := git.Log(begin, end)
	if err != nil {
		httputils.ReportError(w, r, err, "Error while looking up hashes.")
		return
	}
	escaped := ehtml.EscapeString(body)
	linkified := commitLinkifyRe.ReplaceAllString(escaped, "<span class=subject>commit <a href=\"https://skia.googlesource.com/skia/+/${1}\" target=\"_blank\">${1}</a></span>")

	w.Header().Set("Content-Type", "text/plain")
	if _, err := w.Write([]byte(fmt.Sprintf("<pre>%s</pre>", linkified))); err != nil {
		glog.Errorf("Failed to write or encode output: %s", err)
		return
	}
}

// shortCommitsHandler returns basic info of a range of commits.
//
// Queries look like:
//
//     /commits/?begin=hash1&end=hash2
//
// Response is JSON of ShortCommits format that looks like:
//
// {
//   "commits": [
//     {
//       hash: "123abc",
//       author: "bensong",
//       subject: "Adds short commits."
//     },
//     ...
//   ]
// }
//
func shortCommitsHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Query Handler: %q\n", r.URL.Path)
	if r.Method != "GET" {
		http.NotFound(w, r)
		return
	}
	begin := r.FormValue("begin")
	if len(begin) != 40 {
		httputils.ReportError(w, r, fmt.Errorf("Invalid begin hash format: %s", begin), "Error while looking up hashes.")
		return
	}
	end := r.FormValue("end")
	if len(end) != 40 {
		httputils.ReportError(w, r, fmt.Errorf("Invalid end hash format: %s", end), "Error while looking up hashes.")
		return
	}
	commits, err := git.ShortList(begin, end)
	if err != nil {
		httputils.ReportError(w, r, err, "Error while looking up hashes.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(commits); err != nil {
		glog.Errorf("Failed to write or encode output: %s", err)
	}
}

//
type CommitJSONResponse struct {
	Commits  []*tracedb.CommitIDLong `json:"commits"`
	ParamSet map[string][]string     `json:"paramset"`
}

// commitsJSONHandler returns JSON info of a range of commits.
//
// Queries look like:
//
//     /_/commits/?begin=h1&dur=h2&source=master
//
// Where:
//  - h1 and h2 are human time values, 2w, 3h, 1d, etc. Defaults
//    to begin=2w, dur=now(), where dur is the duration, i.e. the size
//    of the window.
//  - source is the source of the commits, like "master". Defaults
//    to "" which means include all sources.
//
//
// Response is a JSON serialization of CommitJSONResponse:
//
//   {
//     commits:[
//        {
//          ts: 14070203,
//          id: "123abc",
//          source: "master",
//          author: "name@example.org",
//          desc: "Adds short commits."
//        },
//        ...
//     ],
//     paramset: {
//     },
//   ]
//
func commitsJSONHandler(w http.ResponseWriter, r *http.Request) {
	// Convert query params to time.Times.
	beginStr := r.FormValue("begin")
	if beginStr == "" {
		beginStr = "2w"
	}
	begin, err := human.ParseDuration(beginStr)
	if err != nil {
		httputils.ReportError(w, r, fmt.Errorf("Failed to parse duration."), "Invalid value for begin.")
		return
	}
	durStr := r.FormValue("dur")
	if durStr == "" {
		durStr = beginStr
	}
	dur, err := human.ParseDuration(durStr)
	if err != nil {
		httputils.ReportError(w, r, fmt.Errorf("Failed to parse duration."), "Invalid value for end.")
		return
	}
	if dur > begin {
		dur = begin
	}

	beginTime := time.Now().Add(-begin)
	endTime := beginTime.Add(dur)
	commits, err := branchTileBuilder.ListLong(beginTime, endTime, r.FormValue("source"))
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to load commits")
		return
	}

	tile := masterTileBuilder.GetTile()
	body := CommitJSONResponse{
		Commits:  commits,
		ParamSet: tile.ParamSet,
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(body); err != nil {
		glog.Errorf("Failed to write or encode output: %s", err)
	}
}

// helpHandler handles the GET of the main page.
func helpHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Help Handler: %q\n", r.URL.Path)
	if *local {
		loadTemplates()
	}
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "text/html")
		ctx := parser.NewContext(nil)
		if err := templates.ExecuteTemplate(w, "help.html", ctx); err != nil {
			glog.Errorln("Failed to expand template:", err)
		}
	}
}

func initpageHandler(w http.ResponseWriter, r *http.Request) {
	df := freshDataFrame.Get()
	resp, err := dataframe.ResponseFromDataFrame(&dataframe.DataFrame{
		Header:   df.Header,
		ParamSet: df.ParamSet,
		TraceSet: ptracestore.TraceSet{},
	}, git, false)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to load init data.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		glog.Errorf("Failed to encode paramset: %s", err)
	}
}

// RangeRequest is used in cidRangeHandler and is used to query for a range of
// cid.CommitIDs that include the range between [begin, end) and include the
// explicit CommitID of "Source, Offset".
type RangeRequest struct {
	Source string `json:"source"`
	Offset int    `json:"offset"`
	Begin  int64  `json:"begin"`
	End    int64  `json:"end"`
}

// cidRangeHandler accepts a POST'd JSON serialized RangeRequest
// and returns a serialized JSON slice of cid.CommitDetails.
func cidRangeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	rr := &RangeRequest{}
	defer util.Close(r.Body)
	if err := json.NewDecoder(r.Body).Decode(rr); err != nil {
		httputils.ReportError(w, r, err, "Failed to decode JSON.")
		return
	}

	df := freshDataFrame.Get()
	begin := df.Header[0].Timestamp
	end := df.Header[len(df.Header)-1].Timestamp
	var err error
	if rr.Begin != 0 || rr.End != 0 {
		if rr.Begin != 0 {
			begin = rr.Begin
		}
		if rr.End != 0 {
			end = rr.End
		}
		df = dataframe.NewHeaderOnly(git, time.Unix(begin, 0), time.Unix(end, 0), false)
	}

	found := false
	cids := []*cid.CommitID{}
	for _, h := range df.Header {
		cids = append(cids, &cid.CommitID{
			Offset: int(h.Offset),
			Source: h.Source,
		})
		if int(h.Offset) == rr.Offset && h.Source == rr.Source {
			found = true
		}
	}
	if !found && rr.Source != "" && rr.Offset != -1 {
		cids = append(cids, &cid.CommitID{
			Offset: rr.Offset,
			Source: rr.Source,
		})
	}

	resp, err := cidl.Lookup(cids)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to lookup all commit ids")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		glog.Errorf("Failed to encode paramset: %s", err)
	}
}

// FrameStartResponse is serialized as JSON for the response in
// frameStartHandler.
type FrameStartResponse struct {
	ID string `json:"id"`
}

// frameStartHandler starts a FrameRequest running and returns the ID
// of the Go routine doing the work.
//
// Building a DataFrame can take a long time to complete, so we run the request
// in a Go routine and break the building of DataFrames into three separate
// requests:
//  * Start building the DataFrame (_/frame/start), which returns an identifier of the long
//    running request, {id}.
//  * Query the status of the running request (_/frame/status/{id}).
//  * Finally return the constructed DataFrame (_/frame/results/{id}).
func frameStartHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fr := &dataframe.FrameRequest{}
	defer util.Close(r.Body)
	if err := json.NewDecoder(r.Body).Decode(fr); err != nil {
		httputils.ReportError(w, r, err, "Failed to decode JSON.")
		return
	}

	resp := FrameStartResponse{
		ID: frameRequests.Add(fr),
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		glog.Errorf("Failed to encode paramset: %s", err)
	}
}

// FrameStatus is used to serialize a JSON response in frameStatusHandler.
type FrameStatus struct {
	State   dataframe.ProcessState `json:"state"`
	Message string                 `json:"message"`
	Percent float32                `json:"percent"`
}

// frameStatusHandler returns the status of a pending FrameRequest.
//
// See frameStartHandler for more details.
func frameStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]
	state, message, percent, err := frameRequests.Status(id)
	if err != nil {
		httputils.ReportError(w, r, err, message)
		return
	}

	resp := FrameStatus{
		State:   state,
		Message: message,
		Percent: percent,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		glog.Errorf("Failed to encode response: %s", err)
	}
}

// frameResultsHandler returns the results of a pending FrameRequest.
//
// See frameStatusHandler for more details.
func frameResultsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]
	df, err := frameRequests.Response(id)
	if err != nil {
		httputils.ReportError(w, r, err, "Async processing of frame failed.")
		return
	}

	if err := json.NewEncoder(w).Encode(df); err != nil {
		glog.Errorf("Failed to encode response: %s", err)
	}
}

// countHandler takes the POST'd query and runs that against the current
// dataframe and returns how many traces match the query.
func countHandler(w http.ResponseWriter, r *http.Request) {
	if *local {
		loadTemplates()
	}
	w.Header().Set("Content-Type", "application/json")
	if err := r.ParseForm(); err != nil {
		httputils.ReportError(w, r, err, "Invalid URL query.")
		return
	}
	var err error
	var q *query.Query
	if q, err = query.New(r.Form); err != nil {
		httputils.ReportError(w, r, err, "Invalid query.")
		return
	}
	count := 0
	for key, _ := range freshDataFrame.Get().TraceSet {
		if q.Matches(key) {
			count += 1
		}
	}
	if err := json.NewEncoder(w).Encode(struct {
		Count int `json:"count"`
	}{Count: count}); err != nil {
		glog.Errorf("Failed to encode paramset: %s", err)
	}
}

// cidHandler takes the POST'd list of dataframe.ColumnHeaders,
// and returns a serialized slice of vcsinfo.ShortCommit's.
func cidHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	cids := []*cid.CommitID{}
	if err := json.NewDecoder(r.Body).Decode(&cids); err != nil {
		httputils.ReportError(w, r, err, "Could not decode POST body.")
		return
	}
	resp, err := cidl.Lookup(cids)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to lookup all commit ids")
		return
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		glog.Errorf("Failed to encode paramset: %s", err)
	}
}

// ClusterStartResponse is serialized as JSON for the response in
// clusterStartHandler.
type ClusterStartResponse struct {
	ID string `json:"id"`
}

// clusterStartHandler takes a POST'd ClusterRequest and starts a long
// running Go routine to do the actual clustering. The ID of the long
// running routine is returned to be used in subsequent calls to
// clusterStatusHandler to check on the status of the work.
func clusterStartHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	req := &clustering2.ClusterRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, r, err, "Could not decode POST body.")
		return
	}
	resp := ClusterStartResponse{
		ID: clusterRequests.Add(req),
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		glog.Errorf("Failed to encode paramset: %s", err)
	}
}

// ClusterStatus is used to serialize a JSON response in clusterStatusHandler.
type ClusterStatus struct {
	State   clustering2.ProcessState     `json:"state"`
	Message string                       `json:"message"`
	Value   *clustering2.ClusterResponse `json:"value"`
}

// clusterStatusHandler is used to check on the status of a long
// running cluster request. The ID of the routine is passed in via
// the URL path. A JSON serialized ClusterStatus is returned, with
// ClusterStatus.Value being populated only when the clustering
// process has finished.
func clusterStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]

	status := &ClusterStatus{}
	state, msg, err := clusterRequests.Status(id)
	if err != nil {
		httputils.ReportError(w, r, err, msg)
		return
	}
	status.State = state
	status.Message = msg
	if state == clustering2.PROCESS_SUCCESS {
		value, err := clusterRequests.Response(id)
		if err != nil {
			httputils.ReportError(w, r, err, "Failed to retrieve results.")
			return
		}
		status.Value = value
	}

	if err := json.NewEncoder(w).Encode(status); err != nil {
		glog.Errorf("Failed to encode paramset: %s", err)
	}
}

// keysHandler handles the POST requests of a list of keys.
//
//    {
//       "keys": [
//            ",arch=x86,...",
//            ",arch=x86,...",
//       ]
//    }
//
// And returns the ID of the new shortcut to that list of keys:
//
//   {
//     "id": 123456,
//   }
func keysHandler(w http.ResponseWriter, r *http.Request) {
	defer util.Close(r.Body)
	id, err := shortcut2.Insert(r.Body)
	if err != nil {
		httputils.ReportError(w, r, err, "Error inserting shortcut.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"id": id}); err != nil {
		glog.Errorf("Failed to write or encode output: %s", err)
	}
}

// gotoHandler handles redirecting from a git hash to either the explore,
// clustering, or triage page.
//
// Sets begin and end to a range of commits on either side of the selected
// commit.
func gotoHandler(w http.ResponseWriter, r *http.Request) {
	hash := mux.Vars(r)["hash"]
	dest := mux.Vars(r)["dest"]
	index, err := git.IndexOf(hash)
	if err != nil {
		httputils.ReportError(w, r, err, "Could not look up git hash.")
		return
	}
	last := git.LastN(1)
	if len(last) != 1 {
		httputils.ReportError(w, r, fmt.Errorf("gitinfo.LastN(1) returned 0 hashes."), "Failed to find last hash.")
		return
	}
	lastIndex, err := git.IndexOf(last[0])
	if err != nil {
		httputils.ReportError(w, r, err, "Could not look up last git hash.")
		return
	}
	begin := index - config.GOTO_RANGE
	if begin < 0 {
		begin = 0
	}
	end := index + config.GOTO_RANGE
	if end > lastIndex {
		end = lastIndex
	}
	details, err := cidl.Lookup([]*cid.CommitID{
		&cid.CommitID{
			Offset: begin,
			Source: "master",
		},
		&cid.CommitID{
			Offset: end,
			Source: "master",
		},
	})
	if err != nil {
		httputils.ReportError(w, r, err, "Could not convert indices to hashes.")
		return
	}
	beginTime := details[0].Timestamp
	endTime := details[1].Timestamp + 1

	if dest == "e" {
		http.Redirect(w, r, fmt.Sprintf("/e/?begin=%d&end=%d", beginTime, endTime), http.StatusFound)
	} else if dest == "c" {
		http.Redirect(w, r, fmt.Sprintf("/c/?begin=%d&end=%d&offset=%d&source=master", beginTime, endTime, index), http.StatusFound)
	} else if dest == "t" {
		http.Redirect(w, r, fmt.Sprintf("/t/?begin=%d&end=%d", beginTime, endTime), http.StatusFound)
	}
}

// TriageRequest is used in triageHandler.
type TriageRequest struct {
	Cid         *cid.CommitID           `json:"cid"`
	Query       string                  `json:"query"`
	Triage      regression.TriageStatus `json:"triage"`
	ClusterType string                  `json:"cluster_type"`
}

// TriageResponse is used in triageHandler.
type TriageResponse struct {
	Bug string `json:"bug"` // URL to bug reporting page.
}

// triageHandler takes a POST'd TriageRequest serialized as JSON
// and performs the triage.
//
// If succesful it returns a 200, or an HTTP status code of 500 otherwise.
func triageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if login.LoggedInAs(r) == "" {
		httputils.ReportError(w, r, fmt.Errorf("Not logged in."), "You must be logged in to triage.")
		return
	}
	if r.Method != "POST" {
		http.NotFound(w, r)
		return
	}
	tr := &TriageRequest{}
	defer util.Close(r.Body)
	if err := json.NewDecoder(r.Body).Decode(tr); err != nil {
		httputils.ReportError(w, r, err, "Failed to decode JSON.")
		return
	}
	detail, err := cidl.Lookup([]*cid.CommitID{tr.Cid})
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to find CommitID.")
		return
	}

	if tr.ClusterType == "low" {
		err = regStore.TriageLow(detail[0], tr.Query, tr.Triage)
	} else {
		err = regStore.TriageHigh(detail[0], tr.Query, tr.Triage)
	}

	if err != nil {
		httputils.ReportError(w, r, err, "Failed to triage.")
		return
	}

	link := fmt.Sprintf("%s/t/?begin=%d&end=%d", r.Header.Get("Origin"), detail[0].Timestamp, detail[0].Timestamp+1)
	a := &types.Activity{
		UserID: login.LoggedInAs(r),
		Action: fmt.Sprintf("Perf Triage: %q %q %q %q", tr.Query, detail[0].URL, tr.Triage.Status, tr.Triage.Message),
		URL:    link,
	}
	if err := activitylog.Write(a); err != nil {
		glog.Errorf("Failed to log activity: %s", err)
	}

	resp := &TriageResponse{}

	if tr.Triage.Status == regression.NEGATIVE {

		comment := fmt.Sprintf(`This bug was found via SkiaPerf.

Visit this URL to see the details of the suspicious cluster:

  %s

The suspect commit is:

  %s
  `, link, detail[0].URL)
		q := url.Values{
			"labels":  []string{"FromSkiaPerf,Type-Defect,Priority-Medium"},
			"comment": []string{comment},
		}
		resp.Bug = "https://bugs.chromium.org/p/skia/issues/entry?" + q.Encode()
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		glog.Errorf("Failed to write or encode output: %s", err)
	}
}

// RegressionRangeRequest is used in regressionRangeHandler and is used to query for a range of
// of Regressions.
//
// Begin and End are Unix timestamps in seconds.
type RegressionRangeRequest struct {
	Begin int64 `json:"begin"`
	End   int64 `json:"end"`
}

// RegressionRow are all the Regression's for a specific commit. It is used in
// RegressionRangeResponse.
//
// The Columns have the same order as RegressionRangeResponse.Header.
type RegressionRow struct {
	Id      *cid.CommitDetail        `json:"cid"`
	Columns []*regression.Regression `json:"columns"`
}

// RegressionRangeResponse is the response from regressionRangeHandler.
type RegressionRangeResponse struct {
	Header []string         `json:"header"`
	Table  []*RegressionRow `json:"table"`
}

// regressionRangeHandler accepts a POST'd JSON serialized RegressionRangeRequest
// and returns a serialized JSON RegressionRangeResponse:
//
//    {
//      header: [ "query1", "query2", "query3", ...],
//      table: [
//        { cid: cid1, columns: [ Regression, Regression, Regression, ...], },
//        { cid: cid2, columns: [ Regression, null,       Regression, ...], },
//        { cid: cid3, columns: [ Regression, Regression, Regression, ...], },
//      ]
//    }
//
// Note that there will be nulls in the columns slice where no Regression have been found.
func regressionRangeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	rr := &RegressionRangeRequest{}
	defer util.Close(r.Body)
	if err := json.NewDecoder(r.Body).Decode(rr); err != nil {
		httputils.ReportError(w, r, err, "Failed to decode JSON.")
		return
	}

	// Get a list of commits for the range.
	indexCommits := git.Range(time.Unix(rr.Begin, 0), time.Unix(rr.End, 0))
	ids := make([]*cid.CommitID, 0, len(indexCommits))
	for _, indexCommit := range indexCommits {
		ids = append(ids, &cid.CommitID{
			Source: "master",
			Offset: indexCommit.Index,
		})
	}

	// Convert the CommitIDs to CommitDetails.
	cids, err := cidl.Lookup(ids)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to look up commit details")
		return
	}

	// Query for Regressions in the range.
	regMap, err := regStore.Range(rr.Begin, rr.End)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to retrieve clusters.")
		return
	}

	// Build a list of all queries we are currently clustering over, joined with
	// the queries that are present in the set of Regressions we just loaded.
	headers := strings.Split(*clusterQueries, " ")
	for _, reg := range regMap {
		for q, _ := range reg.ByQuery {
			headers = append(headers, q)
		}
	}
	headers = util.NewStringSet(headers).Keys()
	sort.Sort(sort.StringSlice(headers))

	// Reverse the order of the cids, so the latest
	// commit shows up first in the UI display.
	revCids := make([]*cid.CommitDetail, len(cids), len(cids))
	for i, c := range cids {
		revCids[len(cids)-1-i] = c
	}

	// Build the RegressionRangeResponse.
	ret := RegressionRangeResponse{
		Header: headers,
		Table:  []*RegressionRow{},
	}

	for _, cid := range revCids {
		row := &RegressionRow{
			Id:      cid,
			Columns: make([]*regression.Regression, len(headers), len(headers)),
		}
		if r, ok := regMap[cid.ID()]; ok {
			for i, h := range headers {
				if reg, ok := r.ByQuery[h]; ok {
					row.Columns[i] = reg
				}
			}
		}
		ret.Table = append(ret.Table, row)
	}
	if err := json.NewEncoder(w).Encode(ret); err != nil {
		glog.Errorf("Failed to write or encode output: %s", err)
	}
}

func makeResourceHandler() func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(*resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "max-age=300")
		fileServer.ServeHTTP(w, r)
	}
}

func initIngestion() {
	evt := eventbus.New(nil)

	// Initialize oauth client and start the ingesters.
	client, err := auth.NewDefaultJWTServiceAccountClient(storage.CloudPlatformScope)
	if err != nil {
		glog.Fatalf("Failed to auth: %s", err)
	}

	// Start the ingesters.
	config, err := sharedconfig.ConfigFromTomlFile(*configFilename)
	if err != nil {
		glog.Fatalf("Unable to read config file %s. Got error: %s", *configFilename, err)
	}

	ingesters, err := ingestion.IngestersFromConfig(config, client, evt)
	if err != nil {
		glog.Fatalf("Unable to instantiate ingesters: %s", err)
	}
	for _, oneIngester := range ingesters {
		oneIngester.Start()
	}
}

func main() {
	defer common.LogPanic()
	// Setup DB flags.
	dbConf := idb.DBConfigFromFlags()

	common.InitWithMetrics2("skiaperf", influxHost, influxUser, influxPassword, influxDatabase, local)

	Init()
	if !*local {
		if err := dbConf.GetPasswordFromMetadata(); err != nil {
			glog.Fatal(err)
		}
	}
	if err := dbConf.InitDB(); err != nil {
		glog.Fatal(err)
	}

	if !*newonly {
		stats.Start(masterTileBuilder, git)
		alerting.Start(masterTileBuilder)
	}

	var redirectURL = fmt.Sprintf("http://localhost%s/oauth2callback/", *port)
	if !*local {
		redirectURL = "https://perf.skia.org/oauth2callback/"
	}
	if err := login.InitFromMetadataOrJSON(redirectURL, login.DEFAULT_SCOPE, login.DEFAULT_DOMAIN_WHITELIST); err != nil {
		glog.Fatalf("Failed to initialize the login system: %s", err)
	}

	// Resources are served directly.
	router := mux.NewRouter()

	router.PathPrefix("/res/").HandlerFunc(makeResourceHandler())

	router.HandleFunc("/", templateHandler("index.html"))

	// New endpoints that use ptracestore will go here.
	router.HandleFunc("/e/", templateHandler("newindex.html"))
	router.HandleFunc("/c/", templateHandler("clusters2.html"))
	router.HandleFunc("/t/", templateHandler("triage.html"))
	router.HandleFunc("/g/{dest:[ect]}/{hash:[a-zA-Z0-9]+}", gotoHandler)
	router.HandleFunc("/_/initpage/", initpageHandler)
	router.HandleFunc("/_/cidRange/", cidRangeHandler)
	router.HandleFunc("/_/count/", countHandler)
	router.HandleFunc("/_/cid/", cidHandler)
	router.HandleFunc("/_/keys/", keysHandler)
	router.HandleFunc("/_/frame/start", frameStartHandler)
	router.HandleFunc("/_/frame/status/{id:[a-zA-Z0-9]+}", frameStatusHandler)
	router.HandleFunc("/_/frame/results/{id:[a-zA-Z0-9]+}", frameResultsHandler)
	router.HandleFunc("/_/cluster/start", clusterStartHandler)
	router.HandleFunc("/_/cluster/status/{id:[a-zA-Z0-9]+}", clusterStatusHandler)
	router.HandleFunc("/_/reg/", regressionRangeHandler)
	router.HandleFunc("/_/triage/", triageHandler)

	router.HandleFunc("/frame/", templateHandler("frame.html"))
	router.HandleFunc("/shortcuts/", shortcutHandler)
	router.PathPrefix("/tiles/").HandlerFunc(tileHandler)
	router.PathPrefix("/single/").HandlerFunc(singleHandler)
	router.PathPrefix("/query/").HandlerFunc(queryHandler)
	router.HandleFunc("/commits/", commitsHandler)
	router.HandleFunc("/_/commits/", commitsJSONHandler)
	router.HandleFunc("/shortcommits/", shortCommitsHandler)
	router.HandleFunc("/clusters/", templateHandler("clusters.html"))
	router.HandleFunc("/clustering/", clusteringHandler)
	router.PathPrefix("/cl/").HandlerFunc(clHandler)
	router.PathPrefix("/activitylog/").HandlerFunc(activityHandler)
	router.HandleFunc("/alerts/", templateHandler("alerting.html"))
	router.HandleFunc("/alerting/", alertingHandler)
	router.HandleFunc("/alert_reset/", alertResetHandler)
	router.HandleFunc("/annotate/", annotate.Handler)
	router.HandleFunc("/compare/", templateHandler("compare.html"))
	router.HandleFunc("/per/", templateHandler("percommit.html"))
	router.HandleFunc("/_/per/", perCommitJSONHandler)
	router.HandleFunc("/calc/", calcHandler)
	router.HandleFunc("/help/", helpHandler)
	router.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	router.HandleFunc("/logout/", login.LogoutHandler)
	router.HandleFunc("/loginstatus/", login.StatusHandler)

	http.Handle("/", httputils.LoggingGzipRequestResponse(router))

	glog.Infoln("Ready to serve.")
	glog.Fatal(http.ListenAndServe(*port, nil))
}
