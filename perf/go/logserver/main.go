// Application that serves up the contents of /tmp/glog via HTTP, giving access
// to logs w/o needing to SSH into the server.
package main

import (
	"flag"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/rcrowley/go-metrics"
	"skia.googlesource.com/buildbot.git/go/common"

	"github.com/golang/glog"
)

var port = flag.String("port", ":10115", "HTTP service address (e.g., ':10115')")
var dir = flag.String("dir", "/tmp/glog", "Directory to serve log files from.")
var graphiteServer = flag.String("graphite_server", "skiamonitoring:2003", "Where is Graphite metrics ingestion server running.")

// FileServer returns a handler that serves HTTP requests
// with the contents of the file system rooted at root.
//
// To use the operating system's file system implementation,
// use http.Dir:
//
//     http.Handle("/", FileServer(http.Dir("/tmp")))
//
// Differs from net/http FileServer by making directory listings better.
func FileServer(root http.FileSystem) http.Handler {
	return &fileHandler{root}
}

type fileHandler struct {
	root http.FileSystem
}

func (f *fileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upath := r.URL.Path
	if !strings.HasPrefix(upath, "/") {
		upath = "/" + upath
		r.URL.Path = upath
	}
	serveFile(w, r, f.root, path.Clean(upath))
}

// FileInfoSlice is for sorting.
type FileInfoSlice []os.FileInfo

func (p FileInfoSlice) Len() int           { return len(p) }
func (p FileInfoSlice) Less(i, j int) bool { return p[i].Name() < p[j].Name() }
func (p FileInfoSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// dirList write the directory list to the HTTP response.
//
// glog convention is that log files are created in the following format:
// "ingest.skia-testing-b.perf.log.ERROR.20141015-133007.3273"
// where the first word is the name of the app.
// glog also creates symlinks that look like "ingest.ERROR". These
// symlinks point to the latest log type.
// This method displays sorted symlinks first and then displays sorted sections for
// all apps. Files and directories not in the glog format are bucketed into an
// "unknown" app.
func dirList(w http.ResponseWriter, f http.File) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<pre>\n")
	// Datastructures to populate and output.
	topLevelSymlinks := make([]os.FileInfo, 0)
	appToLogs := make(map[string][]os.FileInfo)
	for {
		fileInfos, err := f.Readdir(10000)
		if err != nil || len(fileInfos) == 0 {
			break
		}
		// Prepopulate the datastructures.
		for _, fileInfo := range fileInfos {
			name := fileInfo.Name()
			nameTokens := strings.Split(name, ".")
			if len(nameTokens) == 2 {
				topLevelSymlinks = append(topLevelSymlinks, fileInfo)
			} else if len(nameTokens) > 1 {
				appToLogs[nameTokens[0]] = append(appToLogs[nameTokens[0]], fileInfo)
			} else {
				// File all directories or files created by something other than
				// glog under "unknown" app.
				appToLogs["unknown"] = append(appToLogs["unknown"], fileInfo)
			}
		}
		// First output the top level symlinks.
		sort.Sort(FileInfoSlice(topLevelSymlinks))
		for _, fileInfo := range topLevelSymlinks {
			writeFileInfo(w, fileInfo)
		}
		// Then output the logs of all the different apps.
		var keys []string
		for k := range appToLogs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, app := range keys {
			appFileInfos := appToLogs[app]
			sort.Sort(FileInfoSlice(appFileInfos))
			fmt.Fprintf(w, "\n===== %s =====\n\n", template.HTMLEscapeString(app))
			for _, fileInfo := range appFileInfos {
				writeFileInfo(w, fileInfo)
			}
		}
	}
	fmt.Fprintf(w, "</pre>\n")
}

func writeFileInfo(w http.ResponseWriter, fileInfo os.FileInfo) {
	name := fileInfo.Name()
	if fileInfo.IsDir() {
		name += "/"
	}
	url := url.URL{Path: name}
	fmt.Fprintf(w, "%s <a href=\"%s\">%s</a>\n", fileInfo.ModTime(), url.String(), template.HTMLEscapeString(name))
}

func serveFile(w http.ResponseWriter, r *http.Request, fs http.FileSystem, name string) {
	f, err := fs.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	d, err1 := f.Stat()
	if err1 != nil {
		http.NotFound(w, r)
		return
	}

	url := r.URL.Path
	if d.IsDir() {
		if url[len(url)-1] != '/' {
			w.Header().Set("Location", path.Base(url)+"/")
			w.WriteHeader(http.StatusMovedPermanently)
			return
		}
	}

	if d.IsDir() {
		glog.Infof("Dir List: %s", name)
		dirList(w, f)
		return
	}

	http.ServeContent(w, r, d.Name(), d.ModTime(), f)
}

func main() {
	common.Init()

	metrics.RegisterRuntimeMemStats(metrics.DefaultRegistry)
	go metrics.CaptureRuntimeMemStats(metrics.DefaultRegistry, 1*time.Minute)
	addr, _ := net.ResolveTCPAddr("tcp", *graphiteServer)
	hostname, err := os.Hostname()
	if err != nil {
		glog.Fatalf("Failed to get Hostname: %s", err)
	}
	go metrics.Graphite(metrics.DefaultRegistry, 1*time.Minute, "skialogserver."+hostname, addr)

	if err := os.MkdirAll(*dir, 0777); err != nil {
		glog.Fatalf("Failed to create dir for log files: %s", err)
	}

	http.Handle("/", http.StripPrefix("/", FileServer(http.Dir(*dir))))
	glog.Fatal(http.ListenAndServe(*port, nil))
}
