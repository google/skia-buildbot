package main

import (
	"context"
	"log"
	"net/http"
	"path/filepath"
	"text/template"

	"github.com/gorilla/mux"
	"github.com/skia-dev/secure"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
)

type server struct {
	templates    *template.Template
	filesService *drive.FilesService
}

func newServer() (baseapp.App, error) {
	var allow allowed.Allow
	if !*baseapp.Local {
		allow = allowed.NewAllowedFromList([]string{"@google.com"})
	} else {
		allow = allowed.NewAllowedFromList([]string{"fred@example.org", "barney@example.org", "wilma@example.org"})
	}

	login.SimpleInitWithAllow(*baseapp.Port, *baseapp.Local, nil, nil, allow)

	ctx := context.Background()
	client, err := google.DefaultClient(ctx, drive.DriveReadonlyScope)
	if err != nil {
		log.Fatal(err)
	}

	driveService, err := drive.New(client)

	ret := &server{
		filesService: driveService.Files,
	}
	ret.loadTemplates()

	return ret, nil
}

func (srv *server) loadTemplates() {
	srv.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*baseapp.ResourcesDir, "index.html"),
	))
}

func (srv *server) indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *baseapp.Local {
		srv.loadTemplates()
	}
	if err := srv.templates.ExecuteTemplate(w, "index.html", map[string]string{
		// Look in webpack.config.js for where the nonce templates are injected.
		"nonce": secure.CSPNonce(r.Context()),
	}); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

func (srv *server) searchByHashtag(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *baseapp.Local {
		srv.loadTemplates()
	}
	if err := srv.templates.ExecuteTemplate(w, "searchByHashtag.html", map[string]string{
		// Look in webpack.config.js for where the nonce templates are injected.
		"nonce": secure.CSPNonce(r.Context()),
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
	baseapp.Serve(newServer, []string{"hashtag.skia.org"})
}
