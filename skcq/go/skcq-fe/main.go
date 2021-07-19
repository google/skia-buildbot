/*
	Skia Commit Queue frontend server
*/

package main

import (
	"context"
	"encoding/json"
	"flag"
	"html/template"
	"net/http"
	"path/filepath"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"
	"github.com/unrolled/secure"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skcq/go/db"
	"go.skia.org/infra/skcq/go/types"
)

var (
	// Flags
	host        = flag.String("host", "skcq.skia.org", "HTTP service host")
	fsNamespace = flag.String("fs_namespace", "", "The namespace this instance should operate in. e.g. staging or prod")
	fsProjectID = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")
	internal    = flag.Bool("internal", false, "Whether this instance should display changes from internal repos.")
)

// See baseapp.Constructor.
func New() (baseapp.App, error) {
	ctx := context.Background()
	login.SimpleInitWithAllow(*baseapp.Port, *baseapp.Local, nil, nil, nil)
	ts, err := auth.NewDefaultTokenSource(*baseapp.Local, auth.SCOPE_USERINFO_EMAIL, datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatal("Could not create token source: %s", err)
	}

	// Instantiate DB client.
	dbClient, err := db.New(ctx, ts, *fsNamespace, *fsProjectID)
	if err != nil {
		sklog.Fatalf("Could not init DB: %s", err)
	}

	srv := &Server{
		dbClient: dbClient,
	}
	srv.loadTemplates()

	return srv, nil
}

// Server is the state of the server.
type Server struct {
	dbClient  db.DB
	templates *template.Template
}

func (srv *Server) loadTemplates() {
	srv.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*baseapp.ResourcesDir, "index.html"),
		filepath.Join(*baseapp.ResourcesDir, "verifiers_detail.html"),
	))
}

// See baseapp.App.
func (srv *Server) AddHandlers(r *mux.Router) {
	// For login/logout.
	r.HandleFunc(login.DEFAULT_OAUTH2_CALLBACK, login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)

	// SkCQ handlers.
	r.HandleFunc("/", srv.indexHandler).Methods("GET")
	r.HandleFunc("/verifiers_detail/{change_id:[0-9]+}/{patchset_id:[0-9]+}", srv.verifiersDetailHandler).Methods("GET")
	r.HandleFunc("/_/get_current_changes", srv.getCurrentChangesHandler).Methods("POST")
	r.HandleFunc("/_/get_change_attempts", srv.getChangeAttemptsHandler).Methods("POST")
}

func (srv *Server) verifiersDetailHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	changeID := mux.Vars(r)["change_id"]
	patchsetID := mux.Vars(r)["patchset_id"]
	internalText := ""
	if *internal {
		internalText = "(Internal-only)"
	}
	if err := srv.templates.ExecuteTemplate(w, "verifiers_detail.html", map[string]string{
		// Look in webpack.config.js for where the nonce templates are injected.
		"Nonce":      secure.CSPNonce(r.Context()),
		"Internal":   internalText,
		"ChangeID":   changeID,
		"PatchsetID": patchsetID,
	}); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
		return
	}
	return
}

func (srv *Server) getChangeAttemptsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	changeAttemptsRequest := types.GetChangeAttemptsRequest{}
	if err := json.NewDecoder(r.Body).Decode(&changeAttemptsRequest); err != nil {
		httputils.ReportError(w, err, "Failed to decode change attempts request", http.StatusBadRequest)
		return
	}

	changeAttempts, err := srv.dbClient.GetChangeAttempts(r.Context(), changeAttemptsRequest.ChangeID, changeAttemptsRequest.PatchsetID, db.GetChangesCol(*internal))
	if err != nil {
		httputils.ReportError(w, err, "Failed to get change attempts", http.StatusInternalServerError)
		return
	}

	resp := &types.GetChangeAttemptsResponse{
		ChangeAttempts: changeAttempts,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) getCurrentChangesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	currentChangesRquest := types.GetCurrentChangesRequest{}
	if err := json.NewDecoder(r.Body).Decode(&currentChangesRquest); err != nil {
		httputils.ReportError(w, err, "Failed to decode current changes request", http.StatusBadRequest)
		return
	}

	changes, err := srv.dbClient.GetCurrentChanges(r.Context())
	if err != nil {
		httputils.ReportError(w, err, "Failed to get current changes", http.StatusInternalServerError)
		return
	}

	respChanges := []*types.CurrentlyProcessingChange{}
	for _, ch := range changes {
		if ch.DryRun == currentChangesRquest.IsDryRun {
			respChanges = append(respChanges, ch)
		}
	}

	resp := &types.GetCurrentChangesResponse{
		Changes: respChanges,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	internalText := ""
	if *internal {
		internalText = "(Internal-only)"
	}
	if err := srv.templates.ExecuteTemplate(w, "index.html", map[string]string{
		// Look in webpack.config.js for where the nonce templates are injected.
		"Nonce":    secure.CSPNonce(r.Context()),
		"Internal": internalText,
	}); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
		return
	}
	return
}

// See baseapp.App.
func (srv *Server) AddMiddleware() []mux.MiddlewareFunc {
	return []mux.MiddlewareFunc{}
}

func main() {
	// Parse flags to be able to send *host to baseapp.Serve
	flag.Parse()
	baseapp.Serve(New, []string{*host})
}
