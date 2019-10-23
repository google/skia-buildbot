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

// sourceDescriptor describes a single source.Source.
type sourceDescriptor struct {
	displayName string
	source      source.Source
}
type server struct {
	templates *template.Template
	sources   []sourceDescriptor
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
		sources: []sourceDescriptor{
			sourceDescriptor{
				displayName: "Documents",
				source:      cs,
			},
			sourceDescriptor{
				displayName: "Bugs",
				source:      ms,
			},
			sourceDescriptor{
				displayName: "CLs",
				source:      gs,
			},
		},
	}
	ret.loadTemplates()

	return ret, nil
}

func (srv *server) loadTemplates() {
	srv.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*baseapp.ResourcesDir, "index.html"),
	))
}

// result of a singe source search.
type result struct {
	DisplayName string
	Artifacts   []source.Artifact
}

// TemplateContext is the context for the index.html template.
type TemplateContext struct {
	// Nonce is the CSP Nonce. Look in webpack.config.js for where the nonce
	// templates are injected.
	Nonce string

	// IsSearch is true if we contain search results.
	IsSearch bool

	// HashTag is the search query made is IsSearch is true.
	Hashtag string

	// Hashtags is the list of "official" hashtags.
	Hashtags []string

	// Results of the search.
	Results []result
}

func (srv *server) indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *baseapp.Local {
		srv.loadTemplates()
	}

	templateContext := TemplateContext{
		// Look in webpack.config.js for where the nonce templates are injected.
		Nonce:    secure.CSPNonce(r.Context()),
		Hashtags: []string{"Forklift", "Hashtag", "Particles", "Skottie"},
	}

	hashtag := strings.TrimSpace(r.FormValue("hashtag"))
	if hashtag != "" {
		templateContext.Hashtag = hashtag
		templateContext.IsSearch = true
		templateContext.Results = make([]result, len(srv.sources))

		// Do searches in parallel.
		var wg sync.WaitGroup
		var mutex sync.Mutex
		for i, s := range srv.sources {
			results := []source.Artifact{}
			wg.Add(1)
			go func(i int, s sourceDescriptor) {
				defer wg.Done()
				for artifact := range s.source.ByHashtag(hashtag) {
					results = append(results, artifact)
				}
				mutex.Lock()
				defer mutex.Unlock()
				templateContext.Results[i] = result{
					DisplayName: s.displayName,
					Artifacts:   results,
				}
			}(i, s)
		}
		wg.Wait()
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
