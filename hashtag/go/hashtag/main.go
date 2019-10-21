package main

import (
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"text/template"

	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/codesearch"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/hashtag/go/codesearchsource"
	"go.skia.org/infra/hashtag/go/gerritsource"
	"go.skia.org/infra/hashtag/go/monorailsource"
	"go.skia.org/infra/hashtag/go/source"
)

type server struct {
	templates        *template.Template
	gerritSource     source.Source
	monorailSource   source.Source
	codesearchSource source.Source
}

func newServer() (baseapp.App, error) {
	// Setup auth.
	var allow allowed.Allow
	if !*baseapp.Local {
		allow = allowed.NewAllowedFromList([]string{"google.com"})
	} else {
		allow = allowed.NewAllowedFromList([]string{"fred@example.org", "barney@example.org", "wilma@example.org"})
	}
	login.SimpleInitWithAllow(*baseapp.Port, *baseapp.Local, nil, nil, allow)

	// Create our Sources.
	gs, err := gerritsource.New()
	if err != nil {
		log.Fatal(err)
	}

	ms, err := monorailsource.New()
	if err != nil {
		log.Fatal(err)
	}

	cs := codesearchsource.New(codesearch.SkiaAllMarkdown)

	ret := &server{
		gerritSource:     gs,
		monorailSource:   ms,
		codesearchSource: cs,
	}
	ret.loadTemplates()

	return ret, nil
}

func (srv *server) loadTemplates() {
	srv.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*baseapp.ResourcesDir, "index.html"),
	))
}

type indexContext struct {
	IsSearch       bool
	Hashtags       []string
	Nonce          string
	GerritList     []source.Artifact
	MonorailList   []source.Artifact
	CodeSearchList []source.Artifact
	Hashtag        string
}

func (srv *server) indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *baseapp.Local {
		srv.loadTemplates()
	}

	templateContext := indexContext{
		// Look in webpack.config.js for where the nonce templates are injected.
		Nonce:    secure.CSPNonce(r.Context()),
		Hashtags: []string{"Forklift", "Hashtag"},
	}

	hashtag := strings.TrimSpace(r.FormValue("hashtag"))
	if hashtag != "" {
		templateContext.IsSearch = true
		// Do searches in parallel.
		var wg sync.WaitGroup

		gerritArtifacts := []source.Artifact{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for artifact := range srv.gerritSource.ByHashtag(hashtag) {
				gerritArtifacts = append(gerritArtifacts, artifact)
			}
		}()

		monorailArtifacts := []source.Artifact{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for artifact := range srv.monorailSource.ByHashtag(hashtag) {
				monorailArtifacts = append(monorailArtifacts, artifact)
			}
		}()

		codesearchArtifacts := []source.Artifact{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for artifact := range srv.codesearchSource.ByHashtag(hashtag) {
				codesearchArtifacts = append(codesearchArtifacts, artifact)
			}
		}()
		wg.Wait()

		// Look in webpack.config.js for where the nonce templates are injected.
		templateContext.GerritList = gerritArtifacts
		templateContext.MonorailList = monorailArtifacts
		templateContext.CodeSearchList = codesearchArtifacts
		templateContext.Hashtag = hashtag
	}

	if err := srv.templates.ExecuteTemplate(w, "index.html", templateContext); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

// See baseapp.App.
func (srv *server) AddHandlers(r *mux.Router) {
	r.HandleFunc("/", srv.indexHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler).Methods("GET")
}

// See baseapp.App.
func (srv *server) AddMiddleware() []mux.MiddlewareFunc {
	ret := []mux.MiddlewareFunc{}
	if !*baseapp.Local {
		ret = append(ret, login.ForceAuthMiddleware(login.DEFAULT_REDIRECT_URL), login.RestrictViewer)
	}
	return ret
}

func main() {
	baseapp.Serve(newServer, []string{"hashtag.skia.org"})
}
