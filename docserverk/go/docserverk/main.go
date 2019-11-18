// docserver is a super simple Markdown (CommonMark) server.
//
// Every directory has an index.md and the first line of index.md is used as
// the display name for that directory. The directory name itself is used in
// the URL path. See README.md for more design details.
package main

import (
	"context"
	"flag"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/fiorix/go-web/autogzip"
	"github.com/gorilla/mux"
	"github.com/russross/blackfriday/v2"
	"go.skia.org/infra/docserverk/go/docset"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/sklog"
)

var (
	indexTemplate *template.Template

	primary *docset.DocSet
)

// flags
var (
	docRepo      = flag.String("doc_repo", "https://skia.googlesource.com/skia", "The repo to check out.")
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port         = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	preview      = flag.Bool("preview", false, "Preview markdown changes to a local repo. Doesn't do pulls.")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	refresh      = flag.Duration("refresh", 5*time.Minute, "The duration between doc git repo refreshes.")
	resourcesDir = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	workDir      = flag.String("work_dir", "/tmp", "The directory to check out the doc repo into.")
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
	if err = docset.Init(); err != nil {
		sklog.Fatalf("Failed to initialize docset: %s", err)
	}
	if *preview {
		primary, err = docset.NewPreviewDocSet()
	} else {
		primary, err = docset.NewDocSet(context.Background(), *workDir, *docRepo)
	}
	if err != nil {
		sklog.Fatalf("Failed to load the docset: %s", err)
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
	sklog.Infof("Main Handler: %q\n", r.URL.Path)

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

	// Images and other assets won't include the ?cl=[issueid], which means if we
	// add a new image in a CL it won't normally show up in the preview, but we
	// can extract the issue id from the Referer header if present.
	ref := r.Header.Get("Referer")
	if cl == "" && ref != "" {
		if refParsed, err := url.Parse(ref); err == nil && (refParsed.Host == r.Host || refParsed.Host == "skia.org") {
			cl = refParsed.Query().Get("cl")
		}
	}
	if cl != "" {
		issue, err := strconv.ParseInt(cl, 10, 64)
		if err != nil {
			httputils.ReportError(w, fmt.Errorf("Not a valid integer id for an issue."), "The CL given is not valid.", http.StatusInternalServerError)
			return
		}
		d, err = docset.NewDocSetForIssue(context.Background(), *workDir, *docRepo, issue)
		if err == docset.IssueCommittedErr {
			httputils.ReportError(w, err, "Failed to load the given CL, that issue is closed.", http.StatusInternalServerError)
			return
		}
		if err != nil {
			httputils.ReportError(w, err, "Failed to load the given CL.", http.StatusInternalServerError)
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
	if r.URL.Path == "/sitemap.txt" {
		w.Header().Set("Content-Type", "text/plain")
		if _, err := w.Write([]byte(d.SiteMap())); err != nil {
			sklog.Errorf("Failed to write sitemap.txt: %s", err)
		}
		return
	}

	filename, raw, err := d.RawFilename(r.URL.Path)
	if err != nil {
		sklog.Infof("Request for unknown path: %s", r.URL.Path)
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
		httputils.ReportError(w, err, "Failed to load file", http.StatusInternalServerError)
		return
	}
	if raw {
		if _, err := w.Write(b); err != nil {
			sklog.Errorf("Failed to write output: %s", err)
			return
		}
	} else {
		body := blackfriday.Run(b)

		if bodyOnly {
			if _, err := w.Write(body); err != nil {
				sklog.Errorf("Failed to write output: %s", err)
				return
			}
		} else {
			content := &Content{
				Body: string(body),
				Nav:  d.Navigation(),
			}
			if err := indexTemplate.Execute(w, content); err != nil {
				sklog.Error("Failed to expand template:", err)
			}
		}
	}
}

func makeResourceHandler() func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(*resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "max-age=300")
		fileServer.ServeHTTP(w, r)
	}
}

func main() {
	common.InitWithMust(
		"docserver",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)

	if !*local {
		login.SimpleInitMust(*port, *local)
	}

	Init()

	router := mux.NewRouter()
	// Resources are served directly.
	router.PathPrefix("/res/").HandlerFunc(makeResourceHandler())

	if !*local {
		router.HandleFunc("/logout/", login.LogoutHandler)
		router.HandleFunc("/loginstatus/", login.StatusHandler)
		router.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	}

	router.PathPrefix("/").HandlerFunc(autogzip.HandleFunc(mainHandler))

	var h http.Handler = router
	if !*local {
		h = httputils.HealthzAndHTTPS(h)
	}
	http.Handle("/", h)

	sklog.Info("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
