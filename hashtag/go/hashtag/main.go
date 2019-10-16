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
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v2"
)

type server struct {
	templates *template.Template
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

	ret := &server{}
	ret.loadTemplates()

	return ret, nil
}

func (srv *server) loadTemplates() {
	srv.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*baseapp.ResourcesDir, "index.html"),
		filepath.Join(*baseapp.ResourcesDir, "searchByName.html"),
	))
}

func (srv *server) indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *baseapp.Local {
		srv.loadTemplates()
	}
	if err := srv.templates.ExecuteTemplate(w, "index.html", map[string]string{
		// Look in webpack.config.js for where the nonce templates are injected.
		"Nonce": secure.CSPNonce(r.Context()),
	}); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

type searchContext struct {
	Nonce string          `json:"nonce"`
	List  *drive.FileList `json:"list"`
}

func (srv *server) searchByHashtag(w http.ResponseWriter, r *http.Request) {
	hashtag := strings.TrimSpace(mux.Vars(r)["hashtag"])
	if hashtag == "" {
		http.NotFound(w, r)
		return
	}

	// Check if in the valid set of hashtags.
	/*
		list, err := srv.filesService.List().Corpora("drive").IncludeTeamDriveItems(true).Q(fmt.Sprintf("fullText contains '#%s'", hashtag)).TeamDriveId("0AOGploz136NUUk9PVA").SupportsTeamDrives(true).Do()
		if err != nil {
			httputils.ReportError(w, err, "Failed to make request.", http.StatusInternalServerError)
			return
		}
	*/
	w.Header().Set("Content-Type", "text/html")
	if *baseapp.Local {
		srv.loadTemplates()
	}
	sklog.Infof("Nonce: %q", secure.CSPNonce(r.Context()))
	if err := srv.templates.ExecuteTemplate(w, "searchByName.html", &searchContext{
		// Look in webpack.config.js for where the nonce templates are injected.
		Nonce: secure.CSPNonce(r.Context()),
		List:  nil,
	}); err != nil {
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
	baseapp.Serve(newServer, []string{"hashtag.skia.org", "apis.google.com"})
}
