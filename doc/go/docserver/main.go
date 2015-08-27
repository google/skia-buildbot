// docserver is a super simple Markdown (CommonMark) server.
//
// Every directory has an index.md and the first line of index.md is used as
// the display name for that directory. The directory name itself is used in
// the URL path. See README.md for more design details.
package main

import (
	"flag"
	"mime"
	"net/http"
	"path/filepath"
	"runtime"
	"text/template"
	"time"

	"strconv"
	"strings"

	"github.com/fiorix/go-web/autogzip"
	"github.com/russross/blackfriday"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/doc/go/docset"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/util"
)

var (
	indexTemplate *template.Template

	primary *docset.DocSet
)

// flags
var (
	port           = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	graphiteServer = flag.String("graphite_server", "", "Where is Graphite metrics ingestion server running.")
	resourcesDir   = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	workDir        = flag.String("work_dir", "/tmp", "The directory to check out the doc repo into.")
	docRepo        = flag.String("doc_repo", "https://skia.googlesource.com/skia", "The directory to check out the doc repo into.")
	refresh        = flag.Duration("refresh", 5*time.Minute, "The duration between doc git repo refreshes.")
	preview        = flag.Bool("preview", false, "Preview markdown changes to a local repo. Doesn't do pulls.")
)

func loadTemplates() {
	indexTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/index.html"),
	))
}

func Init() {
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}
	loadTemplates()

	var err error
	if *preview {
		primary, err = docset.NewPreviewDocSet()
	} else {
		primary, err = docset.NewDocSet(*workDir, *docRepo)
	}
	if err != nil {
		glog.Fatalf("Failed to load the docset: %s", err)
	}

	if !*preview {
		go docset.StartCleaner(*workDir)
	}
}

type Content struct {
	Body string
	Nav  string
}

// mainHandler handles the GET of the main page.
//
// Handles servering all the processed Markdown documents
// and other assetts in the doc repo.
func mainHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Main Handler: %q\n", r.URL.Path)

	// If the request begins with /_/ then it is an XHR request and we only need
	// to return the content and not the surrounding markup.
	bodyOnly := false
	if strings.HasPrefix(r.URL.Path, "/_/") {
		bodyOnly = true
		r.URL.Path = r.URL.Path[2:]
	}

	d := primary
	// If there is a cl={issue_number} query parameter supplied then
	// clone a new copy of the docs repo and patch that issue into it,
	// and then serve this reqeust from that patched repo.
	cl := r.FormValue("cl")
	if cl != "" {
		issue, err := strconv.ParseInt(cl, 10, 64)
		if err != nil {
			util.ReportError(w, r, err, "The CL given is not valid.")
			return
		}
		d, err = docset.NewDocSetForIssue(filepath.Join(*workDir, "patches"), filepath.Join(*workDir, "primary"), issue)
		if err != nil {
			util.ReportError(w, r, err, "Failed to load the given CL")
			return
		}
	}

	// When running in local mode reload all templates and rebuild the navigation
	// menu on every request, so we don't have to start and stop the server while
	// developing.
	if *local {
		d.BuildNavigation()
		loadTemplates()
	}

	filename, raw, err := d.RawFilename(r.URL.Path)
	if err != nil {
		glog.Infof("Request for unknown path: %s", r.URL.Path)
		http.NotFound(w, r)
		return
	}

	// Set the content type.
	mimetype := "text/html"
	if raw {
		mimetype = mime.TypeByExtension(filepath.Ext(filename))
	}
	w.Header().Set("Content-Type", mimetype)

	// Write the response.
	b, err := d.Body(filename)
	if err != nil {
		util.ReportError(w, r, err, "Failed to load file")
		return
	}
	if raw {
		if _, err := w.Write(b); err != nil {
			glog.Errorf("Failed to write output: %s", err)
			return
		}
	} else {
		body := blackfriday.MarkdownCommon(b)
		if bodyOnly {
			if _, err := w.Write(body); err != nil {
				glog.Errorf("Failed to write output: %s", err)
				return
			}
		} else {
			content := &Content{
				Body: string(body),
				Nav:  d.Navigation(),
			}
			if err := indexTemplate.Execute(w, content); err != nil {
				glog.Errorln("Failed to expand template:", err)
			}
		}
	}
}

func makeResourceHandler() func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(*resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", string(300))
		fileServer.ServeHTTP(w, r)
	}
}

func main() {
	defer common.LogPanic()
	common.InitWithMetrics("docserver", graphiteServer)
	Init()

	// Resources are served directly.
	http.HandleFunc("/res/", autogzip.HandleFunc(makeResourceHandler()))
	http.HandleFunc("/", autogzip.HandleFunc(mainHandler))

	glog.Infoln("Ready to serve.")
	glog.Fatal(http.ListenAndServe(*port, nil))
}
