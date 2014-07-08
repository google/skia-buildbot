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
        "sort"
	"strconv"
	"time"
)

import (
	"github.com/fiorix/go-web/autogzip"
	"github.com/golang/glog"
	"github.com/rcrowley/go-metrics"
)

import (
	"config"
        "db"
)

var (
	// indexTemplate is the main index.html page we serve.
	indexTemplate *template.Template = nil

	// clusterTemplate is the /clusters/ page we serve.
	clusterTemplate *template.Template = nil

	jsonHandlerPath = regexp.MustCompile(`/json/([a-z]*)$`)

	clustersHandlerPath = regexp.MustCompile(`/clusters/([a-z]*)$`)

        shortcutHandlerPath = regexp.MustCompile(`/shortcuts/([0-9]*)$`)
)

// flags
var (
	port       = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	doOauth    = flag.Bool("oauth", true, "Run through the OAuth 2.0 flow on startup, otherwise use a GCE service account.")
	gitRepoDir = flag.String("git_repo_dir", "../../../skia", "Directory location for the Skia repo.")
	tileDir    = flag.String("tile_dir", "/tmp/tiles", "What directory to look for tiles in.")
)

var (
	data *Data
)

const (
	// Maximum allowed data POST size.
	MAX_POST_SIZE = 64000
)

func init() {
	flag.Parse()

	rand.Seed(time.Now().UnixNano())

	metrics.RegisterRuntimeMemStats(metrics.DefaultRegistry)
	go metrics.CaptureRuntimeMemStats(metrics.DefaultRegistry, 1*time.Minute)
	addr, _ := net.ResolveTCPAddr("tcp", "skia-monitoring-b:2003")
	go metrics.Graphite(metrics.DefaultRegistry, 1*time.Minute, "skiaperf", addr)

	// Change the current working directory to the directory of the executable.
	var err error
	cwd, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		glog.Fatalln(err)
	}
	if err := os.Chdir(cwd); err != nil {
		glog.Fatalln(err)
	}

	indexTemplate = template.Must(template.ParseFiles(filepath.Join(cwd, "templates/index.html")))
	clusterTemplate = template.Must(template.ParseFiles(filepath.Join(cwd, "templates/clusters.html")))

}

// reportError formats an HTTP error response and also logs the detailed error message.
func reportError(w http.ResponseWriter, r *http.Request, err error, message string) {
	glog.Errorln(message, err)
	w.Header().Set("Content-Type", "text/plain")
	http.Error(w, message, 500)
}

type TracesShortcut struct {
    Keys []string     `json:"keys"`
}

type ShortcutResponse struct {
    Id      int64       `json:"id"`
}

// showcutHandler handles the POST and GET requests of the shortcut page.
func shortcutHandler(w http.ResponseWriter, r *http.Request) {
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
            responseBytes, err := json.Marshal(ShortcutResponse { Id: id })
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
		if err := applyAnnotation(buf); err != nil {
			reportError(w, r, fmt.Errorf("Annotation update error: %s", err), "Failed to change annotation in db.")
		}
	}
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

func main() {
	flag.Parse()

	glog.Infoln("Begin loading data.")
	var err error
	data, err = NewData(*doOauth, *gitRepoDir, *tileDir)
	if err != nil {
		glog.Fatalln("Failed initial load of data from BigQuery: ", err)
	}

	// Resources are served directly.
	http.Handle("/res/", autogzip.Handle(http.FileServer(http.Dir("./"))))

	http.HandleFunc("/", autogzip.HandleFunc(mainHandler))
	http.HandleFunc("/json/", jsonHandler) // We pre-gzip this ourselves.
        http.HandleFunc("/shortcuts/", shortcutHandler)
	http.HandleFunc("/clusters/", autogzip.HandleFunc(clusterHandler))
	http.HandleFunc("/annotations/", autogzip.HandleFunc(annotationsHandler))

	glog.Infoln("Ready to serve.")
	glog.Fatal(http.ListenAndServe(*port, nil))
}
