// Copyright (c) 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package main

import (
	"bytes"
	"database/sql"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

import (
	"github.com/fiorix/go-web/autogzip"
	_ "github.com/go-sql-driver/mysql"
	"github.com/golang/glog"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rcrowley/go-metrics"
)

import (
	"config"
)

var (
	// indexTemplate is the main index.html page we serve.
	indexTemplate *template.Template = nil

	// clusterTemplate is the /clusters/ page we serve.
	clusterTemplate *template.Template = nil

	// db is the database, nil if we don't have an SQL database to store data into.
	db *sql.DB = nil

	jsonHandlerPath = regexp.MustCompile(`/json/([a-z]*)$`)

	clustersHandlerPath = regexp.MustCompile(`/clusters/([a-z]*)$`)
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

	// Connect to MySQL server. First, get the password from the metadata server.
	// See https://developers.google.com/compute/docs/metadata#custom.
	req, err := http.NewRequest("GET", "http://metadata/computeMetadata/v1/instance/attributes/readwrite", nil)
	if err != nil {
		glog.Fatalln(err)
	}
	client := http.Client{}
	req.Header.Add("X-Google-Metadata-Request", "True")
	if resp, err := client.Do(req); err == nil {
		password, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			glog.Fatalln("Failed to read password from metadata server:", err)
		}
		// The IP address of the database is found here:
		//    https://console.developers.google.com/project/31977622648/sql/instances/skiaperf/overview
		// And 3306 is the default port for MySQL.
		db, err = sql.Open("mysql", fmt.Sprintf("readwrite:%s@tcp(173.194.104.24:3306)/skia?parseTime=true", password))
		if err != nil {
			glog.Fatalln("Failed to open connection to SQL server:", err)
		}
	} else {
		glog.Infoln("Failed to find metadata, unable to connect to MySQL server (Expected when running locally):", err)
		// Fallback to sqlite for local use.
		db, err = sql.Open("sqlite3", "./perf.db")
		if err != nil {
			glog.Fatalln("Failed to open:", err)
		}
		sql := `CREATE TABLE notes (
	     id     INT       NOT NULL PRIMARY KEY,
	     type   TINYINT,
	     author TEXT,
	     notes  TEXT      NOT NULL
	     )`
		_, err = db.Exec(sql)
		glog.Infoln("Status creating sqlite table for notes:", err)
		sql = `CREATE TABLE githash (
	     githash   VARCHAR(40)  NOT NULL PRIMARY KEY,
	     ts        TIMESTAMP    NOT NULL,
	     gitnumber INT          NOT NULL,
	     author    TEXT         NOT NULL,
	     message   TEXT         NOT NULL
	     )`

		_, err = db.Exec(sql)
		glog.Infoln("Status creating sqlite table for githash:", err)

		sql = `CREATE TABLE githashnotes (
	     githash VARCHAR(40)  NOT NULL,
	     ts      TIMESTAMP    NOT NULL,
	     id      INT          NOT NULL,
	     FOREIGN KEY (githash) REFERENCES githash(githash),
	     FOREIGN KEY (id) REFERENCES notes(id)
	     )`

		_, err = db.Exec(sql)
		glog.Infoln("Status creating sqlite table for githashnotes:", err)
	}

	// Ping the database to keep the connection fresh.
	go func() {
		c := time.Tick(1 * time.Minute)
		for _ = range c {
			if err := db.Ping(); err != nil {
				glog.Warningln("Database failed to respond:", err)
			}
		}
	}()
}

// reportError formats an HTTP error response and also logs the detailed error message.
func reportError(w http.ResponseWriter, r *http.Request, err error, message string) {
	glog.Errorln(message, err)
	w.Header().Set("Content-Type", "text/plain")
	http.Error(w, message, 500)
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
	http.HandleFunc("/clusters/", autogzip.HandleFunc(clusterHandler))
	http.HandleFunc("/annotations/", autogzip.HandleFunc(annotationsHandler))

	glog.Infoln("Ready to serve.")
	glog.Fatal(http.ListenAndServe(*port, nil))
}
