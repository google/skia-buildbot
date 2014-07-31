// Copyright (c) 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
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
	"skia.googlesource.com/buildbot.git/perf/go/gs"
	"skia.googlesource.com/buildbot.git/perf/go/types"
)

var (
	// indexTemplate is the main index.html page we serve.
	indexTemplate *template.Template = nil

	// index2Template is the main index.html page we serve.
	index2Template *template.Template = nil

	// clusterTemplate is the /clusters/ page we serve.
	clusterTemplate *template.Template = nil

	jsonHandlerPath = regexp.MustCompile(`/json/([a-z]*)$`)

	trybotsHandlerPath = regexp.MustCompile(`/trybots/([0-9A-Za-z-/]*)$`)

	clustersHandlerPath = regexp.MustCompile(`/clusters/([a-z]*)$`)

	shortcutHandlerPath = regexp.MustCompile(`/shortcuts/([0-9]*)$`)

	// The three capture groups are dataset, tile scale, and tile number.
	tileHandlerPath = regexp.MustCompile(`/tiles/([a-z]*)/([0-9]*)/([-0-9,]*)$`)
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
	data *Data

	tileStores map[config.DatasetName]types.TileStore
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
	index2Template = template.Must(template.ParseFiles(filepath.Join(cwd, "templates/index2.html")))
	clusterTemplate = template.Must(template.ParseFiles(filepath.Join(cwd, "templates/clusters.html")))

	tileStores = make(map[config.DatasetName]types.TileStore)
	for _, name := range config.ALL_DATASET_NAMES {
		tileStores[name] = filetilestore.NewFileTileStore(*tileStoreDir, string(name), 10*time.Minute)
	}
}

// reportError formats an HTTP error response and also logs the detailed error message.
func reportError(w http.ResponseWriter, r *http.Request, err error, message string) {
	glog.Errorln(message, err)
	w.Header().Set("Content-Type", "text/plain")
	http.Error(w, message, 500)
}

type TracesShortcut struct {
	Keys    []string `json:"keys"`
	Dataset string   `json:"dataset"`
	Tiles   []int    `json:"tiles"`
	Scale   int      `json:"scale"`
}

type ShortcutResponse struct {
	Id int64 `json:"id"`
}

// showcutHandler handles the POST and GET requests of the shortcut page.
func shortcutHandler(w http.ResponseWriter, r *http.Request) {
	// TODO(kelvinly/jcgregorio?): Add unit testing later
	match := shortcutHandlerPath.FindStringSubmatch(r.URL.Path)
	if match == nil {
		http.NotFound(w, r)
		return
	}
	if r.Method == "GET" {
		var traces string
		err := db.DB.QueryRow(`SELECT traces FROM shortcuts WHERE id =?`, match[1]).Scan(&traces)
		if err != nil {
			reportError(w, r, err, "Error while looking up shortcut.")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(traces))
	} else if r.Method == "POST" {
		r.ParseForm()
		if traces := r.Form.Get("data"); len(traces) <= 0 {
			reportError(w, r, fmt.Errorf("Unable to extract list of traces."), "Unable to process request.")
			return
		} else {
			// Validate by successfully marshalling and unmarshalling
			var marshalledShortcuts TracesShortcut
			err := json.Unmarshal([]byte(traces), &marshalledShortcuts)
			if err != nil {
				reportError(w, r, err, "Error while validating input.")
				return
			}
			// Sort them so any set of traces will always result in the same
			// JSON
			if len(marshalledShortcuts.Keys) <= 0 {
				reportError(w, r, fmt.Errorf("Invalid input."), "Unable to process request.")
				return
			}
			sort.Strings(marshalledShortcuts.Keys)
			formattedKeys, err := json.Marshal(marshalledShortcuts)
			if err != nil {
				reportError(w, r, err, "Error while validating input.")
				return
			}
			result, err := db.DB.Exec(`INSERT INTO shortcuts (traces) VALUES (?)`,
				string(formattedKeys))
			if err != nil {
				reportError(w, r, err, fmt.Sprintf("Error while inserting traces %s", traces))
				return
			}
			id, err := result.LastInsertId()
			if err != nil {
				reportError(w, r, err, "Error while looking at ID of new traces.")
				return
			}
			w.Header().Set("Content-Type", "application/text")
			responseBytes, err := json.Marshal(ShortcutResponse{Id: id})
			if err != nil {
				reportError(w, r, err, "Error while marshalling response.")
				return
			}
			_, err = w.Write(responseBytes)
			if err != nil {
				reportError(w, r, err, "Error while writing result.")
				return
			}
		}
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
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "text/html")
		if err := clusterTemplate.Execute(w, data.ClusterSummaries(config.DatasetName(match[1]))); err != nil {
			glog.Errorln("Failed to expand template:", err)
		}
	} else if r.Method == "POST" { // POST for now, move to GET later for custom clusters.
		k, err := strconv.ParseInt(r.FormValue("k"), 10, 32)
		if err != nil {
			reportError(w, r, err, fmt.Sprintf("k parameter must be an integer %s.", r.FormValue("k")))
		}
		stddev, err := strconv.ParseFloat(r.FormValue("stddev"), 64)
		if err != nil {
			reportError(w, r, err, fmt.Sprintf("stddev parameter must be a float %s.", r.FormValue("stddev")))
		}
		w.Header().Set("Content-Type", "text/html")
		if err := clusterTemplate.Execute(w, data.ClusterSummariesFor(config.DatasetName(match[1]), int(k), stddev)); err != nil {
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
func getTile(dataset config.DatasetName, tileScale, tileNumber int) (*types.Tile, error) {
	// TODO: Use some sort of cache
	tileStore, ok := tileStores[dataset]
	if !ok {
		return nil, fmt.Errorf("Unable to access dataset store for %s", dataset)
	}
	start := time.Now()
	tile, err := tileStore.Get(int(tileScale), int(tileNumber))
	glog.Infoln("Time for tile load: ", time.Since(start).Nanoseconds())
	if err != nil || tile == nil {
		return nil, fmt.Errorf("Unable to get tile from tilestore: ", err)
	}
	return tile, nil
}

type TilesResponse struct {
	Tiles []*types.TileGUI `json:"tiles"`
}

// tileHandler accepts URIs like /tiles/skps/0/1?traces=Some:long:trace:here&omit_commits=true
// where the URI format is /tiles/<dataset-name>/<tile-scale>/<tile-number>
// It accepts a comma-delimited string of keys as traces, and
// also omit_commits, omit_traces, and omit_names, which each cause the corresponding
// section (described more thoroughly in types.go) to be omitted from the JSON
func tileHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Tile Handler: %q\n", r.URL.Path)
	handlerStart := time.Now()
	match := tileHandlerPath.FindStringSubmatch(r.URL.Path)
	if r.Method != "GET" || match == nil || len(match) != 4 {
		http.NotFound(w, r)
		return
	}
	dataset := config.DatasetName(match[1])
	if dataset != config.DATASET_SKP && dataset != config.DATASET_MICRO {
		reportError(w, r, fmt.Errorf("Invalid dataset specified: %s", dataset), "")
		return
	}
	tileScale, err := strconv.ParseInt(match[2], 10, 0)
	if err != nil {
		reportError(w, r, err, "Failed parsing tile scale.")
		return
	}
	tileNumberStrings := strings.Split(match[3], ",")
	tileNumbers := make([]int, 0, len(tileNumberStrings))
	for _, tileNumberStr := range tileNumberStrings {
		tileNumber, err := strconv.ParseInt(tileNumberStr, 10, 0)
		if err != nil {
			glog.Warningf("Failed parsing tile number %s", tileNumberStr)
			continue
		}
		tileNumbers = append(tileNumbers, int(tileNumber))
	}
	if len(tileNumbers) <= 0 {
		reportError(w, r, err, "No valid tile numbers passed in.")
		return
	}

	tracesRequested := strings.Split(r.FormValue("traces"), ",")
	omitCommits := r.FormValue("omit_commits") != ""
	omitParams := r.FormValue("omit_params") != ""
	omitNames := r.FormValue("omit_names") != ""
	paramList, ok := config.KEY_PARAM_ORDER[dataset]

	allTiles := TilesResponse{Tiles: make([]*types.TileGUI, 0, len(tileNumbers))}
	for _, tileNumber := range tileNumbers {
		tile, err := getTile(dataset, int(tileScale), int(tileNumber))
		if err != nil {
			glog.Warning("Failed to retrieve tile:", err)
			continue
		}
		guiTile := types.NewGUITile(tile.Scale, tile.TileIndex)
		if !ok {
			reportError(w, r, err, "Unable to read parameter list for dataset: ")
			return
		}
		for _, keyName := range tracesRequested {
			if len(keyName) <= 0 {
				continue
			}
			var rawTrace *types.Trace
			if rawTrace, ok = tile.Traces[types.TraceKey(keyName)]; !ok {
				glog.Infof("Missing trace data in tile %d for %s\n", tileNumber, keyName)
				continue
			}
			newTraceData := make([][2]float64, 0)
			for i, traceVal := range rawTrace.Values {
				// Make sure it's not an empty tile by seeing
				// if the CommitTime is greater that the default of zero.
				if traceVal != config.MISSING_DATA_SENTINEL && tile.Commits[i] != nil && tile.Commits[i].CommitTime > 0 {
					newTraceData = append(newTraceData, [2]float64{
						float64(tile.Commits[i].CommitTime),
						traceVal,
						// We should have 53 significand bits, so this should work correctly basically forever
					})
				}
			}
			if len(newTraceData) > 0 {
				guiTile.Traces = append(guiTile.Traces, types.TraceGUI{
					Data: newTraceData,
					Key:  keyName,
				})
			}
		}
		if !omitCommits {
			guiTile.Commits = make([]*types.Commit, 0, config.TILE_SIZE)
			for i := range guiTile.Commits {
				// Make sure it's not an empty tile by seeing
				// if the CommitTime is greater that the default of zero.
				if tile.Commits[i] != nil && tile.Commits[i].CommitTime > 0 {
					guiTile.Commits = append(guiTile.Commits, tile.Commits[i])
				}
			}
		}
		if !omitNames {
			for key, _ := range tile.Traces {
				guiTile.NameList = append(guiTile.NameList, key)
			}
		}
		if !omitParams {
			guiTile.ParamSet = make([][]string, len(paramList))
			for i := range guiTile.ParamSet {
				if readableName, ok := config.HUMAN_READABLE_PARAM_NAMES[paramList[i]]; !ok {
					glog.Warningln(fmt.Sprintf("%s does not exist in the readable parameter names list", paramList[i]))
					guiTile.ParamSet[i] = []string{paramList[i]}
				} else {
					guiTile.ParamSet[i] = []string{readableName}
				}
			}
			for i := range guiTile.ParamSet {
				paramValues, ok := tile.ParamSet[paramList[i]]
				if !ok {
					reportError(w, r, fmt.Errorf("Unable to find param set for %s in tile %d", paramList[i], tileNumber), "")
					return
				}
				guiTile.ParamSet[i] = append(guiTile.ParamSet[i], []string(paramValues)...)
			}
		}
		allTiles.Tiles = append(allTiles.Tiles, guiTile)
	}
	// Marshal and send
	marshaledResult, err := json.Marshal(allTiles)
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

// jsonHandler handles the GET for the JSON requests.
func jsonHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("JSON Handler: %q\n", r.URL.Path)
	match := jsonHandlerPath.FindStringSubmatch(r.URL.Path)
	if match == nil {
		http.NotFound(w, r)
		return
	}
	if len(match) != 2 {
		reportError(w, r, fmt.Errorf("JSON Handler regexp wrong number of matches: %#v", match), "Not Found")
		return
	}
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "application/json")
		// TODO(jcgregorio) Detect if they didn't send Accept-Encoding. But really,
		// they want to use gzip.
		w.Header().Set("Content-Encoding", "gzip")
		data.AsGzippedJSON(*tileDir, config.DatasetName(match[1]), w)
	}
}

// main2Handler handles the GET of the main page.
func main2Handler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Main2 Handler: %q\n", r.URL.Path)
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "text/html")
		if err := index2Template.Execute(w, struct{}{}); err != nil {
			glog.Errorln("Failed to expand template:", err)
		}
	}
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
	var err error
	data, err = NewData(*doOauth, *gitRepoDir, *tileDir)
	if err != nil {
		glog.Fatalln("Failed initial load of data from BigQuery: ", err)
	}

	// Resources are served directly.
	http.HandleFunc("/res/", autogzip.HandleFunc(makeResourceHandler()))

	http.HandleFunc("/", autogzip.HandleFunc(mainHandler))
	http.HandleFunc("/index2", autogzip.HandleFunc(main2Handler))
	http.HandleFunc("/json/", jsonHandler) // We pre-gzip this ourselves.
	http.HandleFunc("/shortcuts/", shortcutHandler)
	http.HandleFunc("/tiles/", tileHandler)
	http.HandleFunc("/trybots/", autogzip.HandleFunc(trybotHandler))
	http.HandleFunc("/clusters/", autogzip.HandleFunc(clusterHandler))
	http.HandleFunc("/annotations/", autogzip.HandleFunc(annotationsHandler))

	glog.Infoln("Ready to serve.")
	glog.Fatal(http.ListenAndServe(*port, nil))
}
