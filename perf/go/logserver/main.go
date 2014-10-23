// Application that serves up the contents of /tmp/glog via HTTP, giving access
// to logs w/o needing to SSH into the server.
package main

import (
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"

	"skia.googlesource.com/buildbot.git/perf/go/flags"

	"github.com/golang/glog"
)

var port = flag.String("port", ":10115", "HTTP service address (e.g., ':10115')")
var dir = flag.String("dir", "/tmp/glog", "Directory to serve log files from.")

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

func dirList(w http.ResponseWriter, f http.File) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<pre>\n")
	for {
		dirs, err := f.Readdir(10000)
		sort.Sort(FileInfoSlice(dirs))
		if err != nil || len(dirs) == 0 {
			break
		}
		for _, d := range dirs {
			name := d.Name()
			if d.IsDir() {
				name += "/"
			}
			url := url.URL{Path: name}
			fmt.Fprintf(w, "%s <a href=\"%s\">%s</a>\n", d.ModTime(), url.String(), template.HTMLEscapeString(name))
		}
	}
	fmt.Fprintf(w, "</pre>\n")
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
	flag.Parse()
	defer glog.Flush()

	if err := os.MkdirAll(*dir, 0777); err != nil {
		glog.Fatalf("Failed to create dir for log files: %s", err)
	}

	flags.Log()

	http.Handle("/", http.StripPrefix("/", FileServer(http.Dir(*dir))))
	glog.Fatal(http.ListenAndServe(*port, nil))
}
