package main

import (
	"bytes"
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
	"skia.googlesource.com/buildbot.git/perf/go/config"
	"skia.googlesource.com/buildbot.git/perf/go/db"
	"skia.googlesource.com/buildbot.git/perf/go/filetilestore"
	"skia.googlesource.com/buildbot.git/perf/go/gitinfo"
	"skia.googlesource.com/buildbot.git/perf/go/gs"
	"skia.googlesource.com/buildbot.git/perf/go/human"
	"skia.googlesource.com/buildbot.git/perf/go/shortcut"
	"skia.googlesource.com/buildbot.git/perf/go/types"
	"skia.googlesource.com/buildbot.git/perf/go/util"
)

var (
	// indexTemplate is the main index.html page we serve.
	indexTemplate *template.Template = nil

	// clusterTemplate is the /clusters/ page we serve.
	clusterTemplate *template.Template = nil

	jsonHandlerPath = regexp.MustCompile(`/json/([a-z]*)$`)

	trybotsHandlerPath = regexp.MustCompile(`/trybots/([0-9A-Za-z-/]*)$`)

	clustersHandlerPath = regexp.MustCompile(`/clusters/([a-z]*)$`)

	shortcutHandlerPath = regexp.MustCompile(`/shortcuts/([0-9]*)$`)

	// The three capture groups are dataset, tile scale, and tile number.
	tileHandlerPath = regexp.MustCompile(`/tiles/([0-9]*)/([-0-9]*)/$`)

	// The three capture groups are tile scale, tile number, and an optional 'trace.
	queryHandlerPath = regexp.MustCompile(`/query/([0-9]*)/([-0-9]*)/(traces/)?$`)

	git *gitinfo.GitInfo = nil

	commitLinkifyRe = regexp.MustCompile("(?m)^commit (.*)$")
)

// flags
var (
	port         = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	doOauth      = flag.Bool("oauth", true, "Run through the OAuth 2.0 flow on startup, otherwise use a GCE service account.")
	gitRepoDir   = flag.String("git_repo_dir", "../../../skia", "Directory location for the Skia repo.")
	tileDir      = flag.String("tile_dir", "/tmp/tiles", "What directory to look for tiles in.")
	tileStoreDir = flag.String("tile_store_dir", "/tmp/tileStore", "What directory to look for tilebuilder tiles in.")
)

var (
	nanoTileStore types.TileStore
)

const (
	// Maximum allowed data POST size.
	MAX_POST_SIZE = 64000

	// Recent number of days to look for trybot data.
	TRYBOT_DAYS_BACK = 7
)

func Init() {
	rand.Seed(time.Now().UnixNano())

	metrics.RegisterRuntimeMemStats(metrics.DefaultRegistry)
	go metrics.CaptureRuntimeMemStats(metrics.DefaultRegistry, 1*time.Minute)
	addr, _ := net.ResolveTCPAddr("tcp", "skia-monitoring-b:2003")
	go metrics.Graphite(metrics.DefaultRegistry, 1*time.Minute, "skiaperf", addr)

	// Change the current working directory to two directories up from this source file so that we
	// can read templates and serve static (res/) files.
	_, filename, _, _ := runtime.Caller(0)
	cwd := filepath.Join(filepath.Dir(filename), "../..")
	if err := os.Chdir(cwd); err != nil {
		glog.Fatalln(err)
	}

	indexTemplate = template.Must(template.ParseFiles(filepath.Join(cwd, "templates/index.html")))
	clusterTemplate = template.Must(template.ParseFiles(filepath.Join(cwd, "templates/clusters.html")))

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
	http.Error(w, message, 500)
}

// showcutHandler handles the POST requests of the shortcut page.
//
// Shortcuts are of the form:
//
//    {
//       "scale": 0,
//       "tiles": [-1],
//       "ids": [
//            "x86:...",
//            "x86:...",
//            "x86:...",
//       ]
//    }
//
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

// clusterHandler handles the GET of the clusters page.
func clusterHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Cluster Handler: %q\n", r.URL.Path)
	match := clustersHandlerPath.FindStringSubmatch(r.URL.Path)
	if match == nil {
		http.NotFound(w, r)
		return
	}
	if len(match) != 2 {
		reportError(w, r, fmt.Errorf("Clusters Handler regexp wrong number of matches: %#v", match), "Not Found")
		return
	}

	tile, err := nanoTileStore.Get(0, -1)
	if err != nil {
		reportError(w, r, err, fmt.Sprintf("Failed to load tile."))
		return
	}
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "text/html")
		// If there are no query parameters just return with an empty set of ClusterSummaries.
		if r.FormValue("_k") == "" {
			if err := clusterTemplate.Execute(w, NewClusterSummaries()); err != nil {
				glog.Errorln("Failed to expand template:", err)
			}
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
		if err := clusterTemplate.Execute(w, calculateClusterSummaries(tile, int(k), stddev, filter)); err != nil {
			glog.Errorln("Failed to expand template:", err)
		}
	}
}

// annotationsHandler handles POST requests to the annotations page.
func annotationsHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Annotations Handler: %q\n", r.URL.Path)
	if r.Method == "POST" {
		buf := bytes.NewBuffer(make([]byte, 0, MAX_POST_SIZE))
		n, err := buf.ReadFrom(r.Body)
		if err != nil {
			reportError(w, r, err, "Failed to read annotation request body to buffer.")
			return
		}
		if n == MAX_POST_SIZE {
			reportError(w, r, err, fmt.Sprintf("Annotation request size >= max post size %d.", MAX_POST_SIZE))
			return
		}
		if err := db.ApplyAnnotation(buf); err != nil {
			reportError(w, r, fmt.Errorf("Annotation update error: %s", err), "Failed to change annotation in db.")
		}
	} else if r.Method == "GET" {
		startTS, err := strconv.ParseInt(r.FormValue("start"), 10, 64)
		if err != nil {
			reportError(w, r, fmt.Errorf("Error parsing startTS: %s", err), "Failed to get startTS for querying annotations.")
			return
		}
		endTS, err := strconv.ParseInt(r.FormValue("end"), 10, 64)
		if err != nil {
			reportError(w, r, fmt.Errorf("Error parsing endTS: %s", err), "Failed to get endTS for querying annotations.")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		annotations, err := db.GetAnnotations(startTS, endTS)
		if err != nil {
			reportError(w, r, fmt.Errorf("Error getting annotations: %s", err), "Failed to read and return annotations from db.")
			return
		}
		w.Write(annotations)
	}
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
	if skps, err := git.SkpCommits(tile.Commits); err != nil {
		guiTile.Skps = []int{}
		glog.Errorf("Failed to calculate skps: %s", err)
	} else {
		guiTile.Skps = skps
	}
	// DO NOT SUBMIT
	// Turns out there hasn't been an SKP update in the last two weeks.
	// Mock some skp changes for testing.
	guiTile.Skps = []int{5, 25, 50}

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
// Then the traces in the shortcut with that ID are returned.
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
		ret := []*types.TraceGUI{}

		shortcutID := r.Form.Get("__shortcut")
		if shortcutID != "" {
			sh, err := shortcut.Get(shortcutID)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			for _, k := range sh.Keys {
				if tr, ok := tile.Traces[k]; ok {
					tg := traceGuiFromTrace(tr, k, tile)
					if tg != nil {
						ret = append(ret, tg)
					}
				}
			}
		} else {
			for key, tr := range tile.Traces {
				if traceMatches(tr, r.Form) {
					tg := traceGuiFromTrace(tr, key, tile)
					if tg != nil {
						ret = append(ret, tg)
					}
				}
			}
		}
		inc := json.NewEncoder(w)
		if err := inc.Encode(map[string][]*types.TraceGUI{"traces": ret}); err != nil {
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
	glog.Infoln("Begin loading data.")

	// Resources are served directly.
	http.HandleFunc("/res/", autogzip.HandleFunc(makeResourceHandler()))

	http.HandleFunc("/", autogzip.HandleFunc(mainHandler))
	http.HandleFunc("/shortcuts/", shortcutHandler)
	http.HandleFunc("/tiles/", tileHandler)
	http.HandleFunc("/query/", queryHandler)
	http.HandleFunc("/commits/", commitsHandler)
	http.HandleFunc("/trybots/", autogzip.HandleFunc(trybotHandler))
	http.HandleFunc("/clusters/", autogzip.HandleFunc(clusterHandler))
	http.HandleFunc("/annotations/", autogzip.HandleFunc(annotationsHandler))

	glog.Infoln("Ready to serve.")
	glog.Fatal(http.ListenAndServe(*port, nil))
}
