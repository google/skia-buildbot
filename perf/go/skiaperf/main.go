package main

import (
	"encoding/json"
	"flag"
	"fmt"
	ehtml "html"
	"html/template"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"time"
)

import (
	"github.com/fiorix/go-web/autogzip"
	"github.com/golang/glog"
	"github.com/rcrowley/go-metrics"
)

import (
	"skia.googlesource.com/buildbot.git/perf/go/alerting"
	"skia.googlesource.com/buildbot.git/perf/go/clustering"
	"skia.googlesource.com/buildbot.git/perf/go/config"
	"skia.googlesource.com/buildbot.git/perf/go/db"
	"skia.googlesource.com/buildbot.git/perf/go/filetilestore"
	"skia.googlesource.com/buildbot.git/perf/go/gitinfo"
	"skia.googlesource.com/buildbot.git/perf/go/gs"
	"skia.googlesource.com/buildbot.git/perf/go/human"
	"skia.googlesource.com/buildbot.git/perf/go/shortcut"
	"skia.googlesource.com/buildbot.git/perf/go/stats"
	"skia.googlesource.com/buildbot.git/perf/go/types"
	"skia.googlesource.com/buildbot.git/perf/go/util"
)

var (
	// indexTemplate is the main index.html page we serve.
	indexTemplate *template.Template = nil

	// clusterTemplate is the /clusters/ page we serve.
	clusterTemplate *template.Template = nil

	alertsTemplate *template.Template = nil

	clTemplate *template.Template = nil

	// compareTemplate is the /compare/ page we serve.
	compareTemplate *template.Template = nil

	jsonHandlerPath = regexp.MustCompile(`/json/([a-z]*)$`)

	trybotsHandlerPath = regexp.MustCompile(`/trybots/([0-9A-Za-z-/]*)$`)

	shortcutHandlerPath = regexp.MustCompile(`/shortcuts/([0-9]*)$`)

	// The three capture groups are dataset, tile scale, and tile number.
	tileHandlerPath = regexp.MustCompile(`/tiles/([0-9]*)/([-0-9]*)/$`)

	// The three capture groups are tile scale, tile number, and an optional 'trace.
	queryHandlerPath = regexp.MustCompile(`/query/([0-9]*)/([-0-9]*)/(traces/)?$`)

	clHandlerPath = regexp.MustCompile(`/cl/([0-9]*)$`)

	git *gitinfo.GitInfo = nil

	commitLinkifyRe = regexp.MustCompile("(?m)^commit (.*)$")
)

// flags
var (
	port           = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	doOauth        = flag.Bool("oauth", true, "Run through the OAuth 2.0 flow on startup, otherwise use a GCE service account.")
	gitRepoDir     = flag.String("git_repo_dir", "../../../skia", "Directory location for the Skia repo.")
	tileStoreDir   = flag.String("tile_store_dir", "/tmp/tileStore", "What directory to look for tilebuilder tiles in.")
	graphiteServer = flag.String("graphite_server", "skia-monitoring-b:2003", "Where is Graphite metrics ingestion server running.")
	apikey         = flag.String("apikey", "", "The API Key used to make issue tracker requests. Only for local testing.")
)

var (
	nanoTileStore types.TileStore
)

const (
	// Recent number of days to look for trybot data.
	TRYBOT_DAYS_BACK = 7
)

func Init() {
	rand.Seed(time.Now().UnixNano())

	metrics.RegisterRuntimeMemStats(metrics.DefaultRegistry)
	go metrics.CaptureRuntimeMemStats(metrics.DefaultRegistry, 1*time.Minute)
	addr, _ := net.ResolveTCPAddr("tcp", *graphiteServer)
	go metrics.Graphite(metrics.DefaultRegistry, 1*time.Minute, "skiaperf", addr)

	// Change the current working directory to two directories up from this source file so that we
	// can read templates and serve static (res/) files.
	_, filename, _, _ := runtime.Caller(0)
	cwd := filepath.Join(filepath.Dir(filename), "../..")
	if err := os.Chdir(cwd); err != nil {
		glog.Fatalln(err)
	}

	indexTemplate = template.Must(template.ParseFiles(
		filepath.Join(cwd, "templates/index.html"),
		filepath.Join(cwd, "templates/titlebar.html"),
		filepath.Join(cwd, "templates/header.html"),
	))
	clusterTemplate = template.Must(template.ParseFiles(
		filepath.Join(cwd, "templates/clusters.html"),
		filepath.Join(cwd, "templates/titlebar.html"),
		filepath.Join(cwd, "templates/header.html"),
	))
	alertsTemplate = template.Must(template.ParseFiles(
		filepath.Join(cwd, "templates/alerting.html"),
		filepath.Join(cwd, "templates/titlebar.html"),
		filepath.Join(cwd, "templates/header.html"),
	))
	clTemplate = template.Must(template.ParseFiles(
		filepath.Join(cwd, "templates/cl.html"),
		filepath.Join(cwd, "templates/titlebar.html"),
		filepath.Join(cwd, "templates/header.html"),
	))
	compareTemplate = template.Must(template.ParseFiles(
		filepath.Join(cwd, "templates/compare.html"),
		filepath.Join(cwd, "templates/titlebar.html"),
		filepath.Join(cwd, "templates/header.html"),
	))

	nanoTileStore = filetilestore.NewFileTileStore(*tileStoreDir, "nano", 2*time.Minute)

	var err error
	git, err = gitinfo.NewGitInfo(*gitRepoDir, true)
	if err != nil {
		glog.Fatal(err)
	}
}

// reportError formats an HTTP error response and also logs the detailed error message.
func reportError(w http.ResponseWriter, r *http.Request, err error, message string) {
	glog.Errorln(message, err)
	w.Header().Set("Content-Type", "text/plain")
	http.Error(w, fmt.Sprintf("%s %s", message, err), 500)
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
			reportError(w, r, fmt.Errorf("Error: received %s", ct), "Invalid content type.")
			return
		}
		defer r.Body.Close()
		id, err := shortcut.Insert(r.Body)
		if err != nil {
			reportError(w, r, err, "Error inserting shortcut.")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		if err := enc.Encode(map[string]string{"id": id}); err != nil {
			reportError(w, r, err, "Error while encoding response.")
		}
	} else {
		http.NotFound(w, r)
	}
}

// trybotHandler handles the GET for trybot data.
func trybotHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Trybot Handler: %q\n", r.URL.Path)
	match := trybotsHandlerPath.FindStringSubmatch(r.URL.Path)
	if match == nil {
		http.NotFound(w, r)
		return
	}
	if len(match) != 2 {
		reportError(w, r, fmt.Errorf("Trybot Handler regexp wrong number of matches: %#v", match), "Not Found")
		return
	}
	if r.Method == "GET" {
		daysBack, err := strconv.ParseInt(r.FormValue("daysback"), 10, 64)
		if err != nil {
			glog.Warningln("No valid daysback given; using the default.")
			daysBack = TRYBOT_DAYS_BACK
		}
		endTS, err := strconv.ParseInt(r.FormValue("end"), 10, 64)
		if err != nil {
			glog.Warningln("No valid end ts given; using the default.")
			endTS = time.Now().Unix()
		}
		w.Header().Set("Content-Type", "application/json")
		results, err := gs.GetTryResults(match[1], endTS, int(daysBack))
		if err != nil {
			reportError(w, r, err, "Error getting storage results.")
			return
		}
		w.Write(results)
	}
}

// alertsHandler serves the HTML for the /alerts/ page.
//
// See alertingHandler for the JSON it uses.
func alertsHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Alerts Handler: %q\n", r.URL.Path)
	w.Header().Set("Content-Type", "text/html")
	if err := alertsTemplate.Execute(w, nil); err != nil {
		glog.Errorln("Failed to expand template:", err)
	}
}

// alertingHandler returns the currently untriaged clusters.
//
// The return format is the same as clusteringHandler.
func alertingHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Alerting Handler: %q\n", r.URL.Path)
	w.Header().Set("Content-Type", "application/json")
	tile, err := nanoTileStore.Get(0, -1)
	if err != nil {
		reportError(w, r, err, fmt.Sprintf("Failed to load tile."))
		return
	}

	alerts, err := alerting.ListFrom(tile.Commits[0].CommitTime)
	if err != nil {
		reportError(w, r, err, "Error retrieving cluster summaries.")
		return
	}
	enc := json.NewEncoder(w)
	if err = enc.Encode(map[string][]*types.ClusterSummary{"Clusters": alerts}); err != nil {
		reportError(w, r, err, "Error while encoding response.")
	}
}

// clHandler serves the HTML for the /cl/<id> page.
//
// These are shortcuts to individual clusters.
//
// See alertingHandler for the JSON it uses.
//
func clHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	match := clHandlerPath.FindStringSubmatch(r.URL.Path)
	if r.Method != "GET" || match == nil || len(match) != 2 {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.ParseInt(match[1], 10, 0)
	if err != nil {
		reportError(w, r, err, "Failed parsing ID.")
		return
	}
	cl, err := alerting.Get(id)
	if err != nil {
		reportError(w, r, err, "Failed to find cluster with that ID.")
		return
	}
	if err := clTemplate.Execute(w, cl); err != nil {
		glog.Errorln("Failed to expand template:", err)
	}
}

// compareHandler handles the GET of the compare page.
func compareHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Compare Handler: %q\n", r.URL.Path)
	w.Header().Set("Content-Type", "text/html")
	if err := compareTemplate.Execute(w, nil); err != nil {
		glog.Errorln("Failed to expand template:", err)
	}
}

// annotateHandler serves the /annotate/ endpoint for changing the status of an
// alert cluster.
//
// Expects a POST'd form with the following values:
//
//   id - The id of the alerting cluster.
//   status - The new Status value.
//   message - The new Messge value.
//
func annotateHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Annotate Handler: %q\n", r.URL.Path)

	if r.Method != "POST" {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		reportError(w, r, err, "Failed to parse query params.")
		return
	}

	// Load the form data.
	id, err := strconv.ParseInt(r.FormValue("id"), 10, 32)
	if err != nil {
		reportError(w, r, err, fmt.Sprintf("id parameter must be an integer %s.", r.FormValue("id")))
		return
	}
	newStatus := r.FormValue("status")
	message := r.FormValue("message")
	if !util.In(newStatus, types.ValidStatusValues) {
		reportError(w, r, fmt.Errorf("Invalid status value: %s", newStatus), "Unknown value.")
		return
	}

	// Store the updated values in the ClusterSummary.
	c, err := alerting.Get(id)
	if err != nil {
		reportError(w, r, err, "Failed to load cluster summary.")
		return
	}
	c.Status = newStatus
	c.Message = message
	if err := alerting.Write(c); err != nil {
		reportError(w, r, err, "Failed to save cluster summary.")
		return
	}

	if newStatus != "Bug" {
		http.Redirect(w, r, "/alerts/", 303)
	} else {
		q := url.Values{
			"labels": []string{"FromSkiaPerf,Type-Defect,Priority-Medium"},
			"comment": []string{fmt.Sprintf(`This bug was found via SkiaPerf.

Visit this URL to see the details of the suspicious cluster:

      http://skiaperf.com/cl/%d.

Don't remove the above URL, it is used to match bugs to alerts.
    `, id)},
		}
		codesiteURL := "https://code.google.com/p/skia/issues/entry?" + q.Encode()
		http.Redirect(w, r, codesiteURL, http.StatusTemporaryRedirect)
	}
}

// clustersHandler handles the GET of the clusters page.
func clustersHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Cluster Handler: %q\n", r.URL.Path)
	w.Header().Set("Content-Type", "text/html")
	if err := clusterTemplate.Execute(w, nil); err != nil {
		glog.Errorln("Failed to expand template:", err)
	}
}

// writeClusterSummaries writes out a ClusterSummaries instance as a JSON response.
func writeClusterSummaries(summary *clustering.ClusterSummaries, w http.ResponseWriter, r *http.Request) {
	enc := json.NewEncoder(w)
	if err := enc.Encode(summary); err != nil {
		reportError(w, r, err, "Error while encoding ClusterSummaries response.")
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
// the 10 closest cluster members and the centroid.
func clusteringHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Clustering Handler: %q\n", r.URL.Path)
	tile, err := nanoTileStore.Get(0, -1)
	if err != nil {
		reportError(w, r, err, fmt.Sprintf("Failed to load tile."))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	// If there are no query parameters just return with an empty set of ClusterSummaries.
	if r.FormValue("_k") == "" || r.FormValue("_stddev") == "" {
		writeClusterSummaries(clustering.NewClusterSummaries(), w, r)
		return
	}

	k, err := strconv.ParseInt(r.FormValue("_k"), 10, 32)
	if err != nil {
		reportError(w, r, err, fmt.Sprintf("_k parameter must be an integer %s.", r.FormValue("_k")))
		return
	}
	stddev, err := strconv.ParseFloat(r.FormValue("_stddev"), 64)
	if err != nil {
		reportError(w, r, err, fmt.Sprintf("_stddev parameter must be a float %s.", r.FormValue("_stddev")))
		return
	}

	// Create a filter function for traces that match the query parameters.
	delete(r.Form, "_k")
	delete(r.Form, "_stddev")
	filter := func(tr *types.Trace) bool {
		return traceMatches(tr, r.Form)
	}
	summary, err := clustering.CalculateClusterSummaries(tile, int(k), stddev, filter)
	if err != nil {
		reportError(w, r, err, "Failed to calculate clusters.")
		return
	}
	writeClusterSummaries(summary, w, r)
}

// getTile retrieves a tile from the disk
func getTile(tileScale, tileNumber int) (*types.Tile, error) {
	start := time.Now()
	tile, err := nanoTileStore.Get(int(tileScale), int(tileNumber))
	glog.Infoln("Time for tile load: ", time.Since(start).Nanoseconds())
	if err != nil || tile == nil {
		return nil, fmt.Errorf("Unable to get tile from tilestore: ", err)
	}
	return tile, nil
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
		reportError(w, r, err, "Failed parsing tile scale.")
		return
	}
	tileNumber, err := strconv.ParseInt(match[2], 10, 0)
	if err != nil {
		reportError(w, r, err, "Failed parsing tile number.")
		return
	}
	glog.Infof("tile: %d %d", tileScale, tileNumber)
	tile, err := getTile(int(tileScale), int(tileNumber))
	if err != nil {
		reportError(w, r, err, "Failed retrieving tile.")
		return
	}

	guiTile := types.NewTileGUI(tile.Scale, tile.TileIndex)
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
		reportError(w, r, err, "Failed to marshal JSON.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(marshaledResult)
	if err != nil {
		reportError(w, r, err, "Error while marshalling results.")
	}
	glog.Infoln("Total handler time: ", time.Since(handlerStart).Nanoseconds())
}

// traceMatches returns true if a trace has Params that match the given query.
func traceMatches(trace *types.Trace, query url.Values) bool {
	for k, values := range query {
		if _, ok := trace.Params[k]; !ok || !util.In(trace.Params[k], values) {
			return false
		}
	}
	return true
}

// QueryResponse is for formatting the JSON output from queryHandler.
type QueryResponse struct {
	Traces []*types.TraceGUI `json:"traces"`
	Hash   string            `json:"hash"`
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
		reportError(w, r, err, "Failed to parse query params.")
	}
	tileScale, err := strconv.ParseInt(match[1], 10, 0)
	if err != nil {
		reportError(w, r, err, "Failed parsing tile scale.")
		return
	}
	tileNumber, err := strconv.ParseInt(match[2], 10, 0)
	if err != nil {
		reportError(w, r, err, "Failed parsing tile number.")
		return
	}
	glog.Infof("tile: %d %d", tileScale, tileNumber)
	tile, err := getTile(int(tileScale), int(tileNumber))
	if err != nil {
		reportError(w, r, err, "Failed retrieving tile.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	ret := QueryResponse{
		Traces: []*types.TraceGUI{},
		Hash:   "",
	}
	if match[3] == "" {
		// We only want the count.
		total := 0
		for _, tr := range tile.Traces {
			if traceMatches(tr, r.Form) {
				total++
			}
		}
		glog.Info("Count: ", total)
		inc := json.NewEncoder(w)
		if err := inc.Encode(map[string]int{"matches": total}); err != nil {
			reportError(w, r, err, "Error while encoding query response.")
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
					tg := traceGuiFromTrace(tr, k, tile)
					if tg != nil {
						ret.Traces = append(ret.Traces, tg)
					}
				}
			}
		} else {
			for key, tr := range tile.Traces {
				if traceMatches(tr, r.Form) {
					tg := traceGuiFromTrace(tr, key, tile)
					if tg != nil {
						ret.Traces = append(ret.Traces, tg)
					}
				}
			}
		}
		inc := json.NewEncoder(w)
		if err := inc.Encode(ret); err != nil {
			reportError(w, r, err, "Error while encoding query response.")
			return
		}
	}
}

// traceGuiFromTrace returns a populated TraceGUI from the given trace.
func traceGuiFromTrace(trace *types.Trace, key string, tile *types.Tile) *types.TraceGUI {
	newTraceData := make([][2]float64, 0)
	for i, v := range trace.Values {
		if v != config.MISSING_DATA_SENTINEL && tile.Commits[i] != nil && tile.Commits[i].CommitTime > 0 {
			//newTraceData = append(newTraceData, [2]float64{float64(tile.Commits[i].CommitTime), v})
			newTraceData = append(newTraceData, [2]float64{float64(i), v})
		}
	}
	if len(newTraceData) > 0 {
		return &types.TraceGUI{
			Data:   newTraceData,
			Label:  key,
			Params: trace.Params,
		}
	} else {
		return nil
	}
}

// commitsHandler handles requests for commits.
//
// The ParamSet is the set of available parameters and their possible values
// based on the set of traces in a tile.
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
//
// TODO Add ability to query across a range of tiles.
func commitsHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Query Handler: %q\n", r.URL.Path)
	if r.Method != "GET" {
		http.NotFound(w, r)
		return
	}
	begin := r.FormValue("begin")
	if len(begin) != 40 {
		reportError(w, r, fmt.Errorf("Invalid hash format: %s", begin), "Error while looking up hashes.")
		return
	}
	end := r.FormValue("end")
	body, err := git.Log(begin, end)
	if err != nil {
		reportError(w, r, err, "Error while looking up hashes.")
		return
	}
	escaped := ehtml.EscapeString(body)
	linkified := commitLinkifyRe.ReplaceAllString(escaped, "<span class=subject>commit <a href=\"https://skia.googlesource.com/skia/+/${1}\" target=\"_blank\">${1}</a></span>")

	w.Write([]byte(fmt.Sprintf("<pre>%s</pre>", linkified)))
}

// mainHandler handles the GET of the main page.
func mainHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Main Handler: %q\n", r.URL.Path)
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "text/html")
		if err := indexTemplate.Execute(w, struct{}{}); err != nil {
			glog.Errorln("Failed to expand template:", err)
		}
	}
}

func makeResourceHandler() func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir("./"))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", string(300))
		fileServer.ServeHTTP(w, r)
	}
}

func main() {
	flag.Parse()

	Init()
	db.Init()
	stats.Start(nanoTileStore, git)
	alerting.Start(nanoTileStore, *apikey)
	glog.Infoln("Begin loading data.")

	// Resources are served directly.
	http.HandleFunc("/res/", autogzip.HandleFunc(makeResourceHandler()))

	http.HandleFunc("/", autogzip.HandleFunc(mainHandler))
	http.HandleFunc("/shortcuts/", shortcutHandler)
	http.HandleFunc("/tiles/", tileHandler)
	http.HandleFunc("/query/", queryHandler)
	http.HandleFunc("/commits/", commitsHandler)
	http.HandleFunc("/trybots/", autogzip.HandleFunc(trybotHandler))
	http.HandleFunc("/clusters/", autogzip.HandleFunc(clustersHandler))
	http.HandleFunc("/clustering/", autogzip.HandleFunc(clusteringHandler))
	http.HandleFunc("/cl/", autogzip.HandleFunc(clHandler))
	http.HandleFunc("/alerts/", autogzip.HandleFunc(alertsHandler))
	http.HandleFunc("/alerting/", autogzip.HandleFunc(alertingHandler))
	http.HandleFunc("/annotate/", autogzip.HandleFunc(annotateHandler))
	http.HandleFunc("/compare/", autogzip.HandleFunc(compareHandler))

	glog.Infoln("Ready to serve.")
	glog.Fatal(http.ListenAndServe(*port, nil))
}
