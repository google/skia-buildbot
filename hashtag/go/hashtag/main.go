package main

import (
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"text/template"

	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"github.com/unrolled/secure"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/hashtag/go/codesearchsource"
	"go.skia.org/infra/hashtag/go/drivesource"
	"go.skia.org/infra/hashtag/go/gerritsource"
	"go.skia.org/infra/hashtag/go/monorailsource"
	"go.skia.org/infra/hashtag/go/source"
)

// sourceDescriptor describes a single source.Source.
type sourceDescriptor struct {
	displayName string
	source      source.Source
}

// server implements baseapp.App.
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

	viper.SetConfigName("config") // name of config file (without extension)
	viper.AddConfigPath(*baseapp.ResourcesDir)
	err := viper.ReadInConfig()
	if err != nil {
		return nil, err
	}

	// Create our Sources.
	gs, err := gerritsource.New()
	if err != nil {
		return nil, err
	}
	ms, err := monorailsource.New()
	if err != nil {
		return nil, err
	}
	cs, err := codesearchsource.New()
	if err != nil {
		return nil, err
	}
	ds, err := drivesource.New()
	if err != nil {
		return nil, err
	}

	ret := &server{
		sources: []sourceDescriptor{
			{
				displayName: "Drive",
				source:      ds,
			},
			{
				displayName: "Documents",
				source:      cs,
			},
			{
				displayName: "Bugs",
				source:      ms,
			},
			{
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

	// Query is the search query made if IsSearch is true.
	Query source.Query

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
		Hashtags: viper.GetStringSlice("hashtags"),
		Query:    source.Query{},
	}

	hashtag := strings.TrimSpace(r.FormValue("hashtag"))
	if hashtag != "" {
		templateContext.Query.Type = source.HashtagQuery
		templateContext.Query.Value = hashtag
		templateContext.IsSearch = true
		templateContext.Results = make([]result, len(srv.sources))

		// Do searches in parallel.
		var wg sync.WaitGroup
		for i, s := range srv.sources {
			wg.Add(1)
			go func(i int, s sourceDescriptor) {
				defer wg.Done()
				results := []source.Artifact{}
				for artifact := range s.source.Search(r.Context(), templateContext.Query) {
					results = append(results, artifact)
				}
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
