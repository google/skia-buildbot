package main

import (
	"context"
	"flag"
	"net/http"
	"net/url"

	"github.com/fiorix/go-web/autogzip"
	"github.com/gorilla/mux"
	"go.skia.org/infra/docsyserver/go/codereview"
	"go.skia.org/infra/docsyserver/go/codereview/gerrit"
	"go.skia.org/infra/docsyserver/go/docset"
	"go.skia.org/infra/docsyserver/go/docsy"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// flags
var (
	docPath   = flag.String("doc_path", "site", "The relative directory, from the top of the repo, where the documents are located.")
	docRepo   = flag.String("doc_repo", "https://skia.googlesource.com/skia", "The repo to check out.")
	docsyDir  = flag.String("docsy_dir", "../../docsy-example", "The directory where docsy is found.")
	gerritURL = flag.String("gerrit_url", "https://skia-review.googlesource.com", "The gerrit URL.")
	hugoExe   = flag.String("hugo", "hugo", "The absolute path to the hugo executable.")
	local     = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	dologin   = flag.Bool("do_login", true, "Also handle login requests for other sites.")
	port      = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort  = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	workDir   = flag.String("work_dir", "/tmp", "The directory to check out the doc repo into.")
)

type server struct {
	docset docset.DocSet
}

func new() (*server, error) {
	codeReview, err := gerrit.New(*local, *gerritURL, *docRepo)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	docsy := docsy.New(*hugoExe, *docsyDir, *docPath)

	docset := docset.New(*workDir, *docPath, *docsyDir, *docRepo, codeReview, docsy)
	if err := docset.Start(context.Background()); err != nil {
		return nil, skerr.Wrap(err)
	}

	return &server{
		docset: docset,
	}, nil
}

// mainHandler handles the GET of the main page.
//
// Handles servering all the processed Markdown documents
// and other assetts in the doc repo.
func (s *server) mainHandler(w http.ResponseWriter, r *http.Request) {
	sklog.Infof("Main Handler: %q\n", r.URL.Path)

	issue := r.FormValue("cl")

	// Images and other assets won't include the ?cl=[issueid], which means if we
	// add a new image in a CL it won't normally show up in the preview, but we
	// can extract the issue id from the Referer header if present.
	ref := r.Header.Get("Referer")
	if issue == "" && ref != "" {
		if refParsed, err := url.Parse(ref); err == nil && (refParsed.Host == r.Host || refParsed.Host == "skia.org") {
			issue = refParsed.Query().Get("cl")
		}
	}
	if issue == "" {
		issue = string(codereview.MainIssue)
	}
	fs, err := s.docset.FileSystem(r.Context(), codereview.Issue(issue))
	if err != nil {
		if err == docset.IssueClosedErr {
			http.Error(w, "This issue has been merged or abandoned.", http.StatusNotFound)
			return
		}
		// If the documentation failed to render then err.Error() will contain
		// the details on why, i.e. the hugo error output.
		httputils.ReportError(w, err, "Failed to load."+err.Error(), http.StatusInternalServerError)
		return
	}
	http.FileServer(fs).ServeHTTP(w, r)
}

func main() {
	common.InitWithMust(
		"docserver",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)

	server, err := new()
	if err != nil {
		sklog.Fatal(err)
	}

	router := mux.NewRouter()
	router.PathPrefix("/").HandlerFunc(autogzip.HandleFunc(server.mainHandler))
	if !*local && *dologin {
		login.SimpleInitMust(*port, *local)
		router.HandleFunc("/logout/", login.LogoutHandler)
		router.HandleFunc("/loginstatus/", login.StatusHandler)
		router.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	}

	var h http.Handler = router
	if !*local {
		h = httputils.HealthzAndHTTPS(h)
	}
	http.Handle("/", h)

	sklog.Info("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
