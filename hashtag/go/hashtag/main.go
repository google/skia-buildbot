package main

import (
	"context"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/hashtag/go/gerritsource"
	"go.skia.org/infra/hashtag/go/monorailsource"
	"go.skia.org/infra/hashtag/go/source"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v2"
)

type server struct {
	templates      *template.Template
	gerritSource   source.Source
	monorailSource source.Source
}

func newServer() (baseapp.App, error) {
	// Setup auth.
	var allow allowed.Allow
	if !*baseapp.Local {
		allow = allowed.NewAllowedFromList([]string{"@google.com"})
	} else {
		allow = allowed.NewAllowedFromList([]string{"fred@example.org", "barney@example.org", "wilma@example.org"})
	}
	login.SimpleInitWithAllow(*baseapp.Port, *baseapp.Local, nil, nil, allow)

	// Get clients for our Sources.
	ctx := context.Background()
	client, err := google.DefaultClient(ctx, drive.DriveReadonlyScope, drive.DriveScope)
	if err != nil {
		log.Fatal(err)
	}
	resp, err := client.Get("https://www.googleapis.com/drive/v2/teamdrives")
	//resp, err := client.Get("https://www.googleapis.com/drive/v2/teamdrives?trace=email:jcgregorio")

	if err != nil {
		sklog.Error(err)
	} else {
		sklog.Infof("%#v", *resp)
	}

	gs, err := gerritsource.New()
	if err != nil {
		log.Fatal(err)
	}

	ms, err := monorailsource.New()
	if err != nil {
		log.Fatal(err)
	}

	ret := &server{
		gerritSource:   gs,
		monorailSource: ms,
	}
	ret.loadTemplates()

	return ret, nil
}

func (srv *server) loadTemplates() {
	srv.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*baseapp.ResourcesDir, "index.html"),
		filepath.Join(*baseapp.ResourcesDir, "searchByName.html"),
	))
}

type indexContext struct {
	Nonce    string
	Hashtags []string
}

func (srv *server) indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *baseapp.Local {
		srv.loadTemplates()
	}
	if err := srv.templates.ExecuteTemplate(w, "index.html", indexContext{
		// Look in webpack.config.js for where the nonce templates are injected.
		Nonce:    secure.CSPNonce(r.Context()),
		Hashtags: []string{"Forklift", "Skolo"},
	}); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

type searchContext struct {
	Nonce        string            `json:"nonce"`
	GerritList   []source.Artifact `json:"gerrit_list"`
	MonorailList []source.Artifact `json:"monorail_list"`
	Hashtag      string            `json:"hashtag"`
}

func (srv *server) searchByHashtag(w http.ResponseWriter, r *http.Request) {
	hashtag := strings.TrimSpace(mux.Vars(r)["hashtag"])
	if hashtag == "" {
		http.NotFound(w, r)
		return
	}

	// TODO Check if in the valid set of hashtags.

	w.Header().Set("Content-Type", "text/html")
	if *baseapp.Local {
		srv.loadTemplates()
	}
	gerritArtifacts := []source.Artifact{}
	for artifact := range srv.gerritSource.ByHashtag(hashtag) {
		gerritArtifacts = append(gerritArtifacts, artifact)
	}
	monorailArtifacts := []source.Artifact{}
	for artifact := range srv.monorailSource.ByHashtag(hashtag) {
		monorailArtifacts = append(monorailArtifacts, artifact)
	}

	resp := &searchContext{
		// Look in webpack.config.js for where the nonce templates are injected.
		Nonce:        secure.CSPNonce(r.Context()),
		GerritList:   gerritArtifacts,
		MonorailList: monorailArtifacts,
		Hashtag:      hashtag,
	}

	if err := srv.templates.ExecuteTemplate(w, "searchByName.html", resp); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

// See baseapp.App.
func (srv *server) AddHandlers(r *mux.Router) {
	r.HandleFunc("/", srv.indexHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler).Methods("GET")

	// GETs
	r.HandleFunc("/h/{hashtag:.+}", srv.searchByHashtag).Methods("GET")
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
