/*
	Server that collects and displays bug data for Skia's clients from different issue frameworks
*/

package main

import (
	"context"
	"encoding/json"
	"flag"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"
	"github.com/unrolled/secure"

	"go.skia.org/infra/bugs-central/go/db"
	"go.skia.org/infra/bugs-central/go/poller"
	"go.skia.org/infra/bugs-central/go/types"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/sklog"
)

var (
	// Flags
	host        = flag.String("host", "bugs-central.skia.org", "HTTP service host")
	workdir     = flag.String("workdir", ".", "Directory to use for scratch work.")
	fsNamespace = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'bugs-central'")
	fsProjectID = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")

	serviceAccountFile = flag.String("service_account_file", "/var/secrets/google/key.json", "Service account JSON file.")
	authAllowList      = flag.String("auth_allowlist", "google.com", "White space separated list of domains and email addresses that are allowed to login.")

	pollInterval = flag.Duration("poll_interval", 2*time.Hour, "How often the server will poll the different issue frameworks.")
)

type ClientConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type ClientSecretJSON struct {
	Installed ClientConfig `json:"installed"`
}

// See baseapp.Constructor.
func New() (baseapp.App, error) {
	// Create workdir if it does not exist.
	if err := os.MkdirAll(*workdir, 0755); err != nil {
		sklog.Fatalf("Could not create %s: %s", *workdir, err)
	}

	var allow allowed.Allow
	if !*baseapp.Local {
		allow = allowed.NewAllowedFromList([]string{*authAllowList})
	} else {
		allow = allowed.NewAllowedFromList([]string{"fred@example.org", "barney@example.org", "wilma@example.org"})
	}
	login.SimpleInitWithAllow(*baseapp.Port, *baseapp.Local, nil, nil, allow)

	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(*baseapp.Local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_FULL_CONTROL, datastore.ScopeDatastore)
	dbClient, err := db.New(ctx, ts, *fsNamespace, *fsProjectID)
	if err != nil {
		sklog.Fatal("Could not init DB: %s", err)
	}

	pollerClient, err := poller.New(ctx, ts, *serviceAccountFile, dbClient)
	if err != nil {
		sklog.Fatal("Could not init poller: %s", err)
	}
	if err := pollerClient.Start(ctx, *pollInterval); err != nil {
		sklog.Fatal("Could not start poller: %s", err)
	}

	srv := &Server{
		pollerClient: pollerClient,
		dbClient:     dbClient,
	}
	srv.loadTemplates()

	return srv, nil
}

// Server is the state of the server.
type Server struct {
	pollerClient *poller.IssuesPoller
	dbClient     *db.FirestoreDB
	templates    *template.Template
}

func (srv *Server) loadTemplates() {
	srv.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*baseapp.ResourcesDir, "index.html"),
	))
}

// user returns the currently logged in user, or a placeholder if running locally.
func (srv *Server) user(r *http.Request) string {
	user := "barney@example.org"
	if !*baseapp.Local {
		user = login.LoggedInAs(r)
	}
	return user
}

// See baseapp.App.
func (srv *Server) AddHandlers(r *mux.Router) {
	// For login/logout.
	r.HandleFunc(login.DEFAULT_OAUTH2_CALLBACK, login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)

	// All endpoints that require authentication should be added to this router.
	appRouter := mux.NewRouter()
	appRouter.HandleFunc("/", srv.indexHandler)
	appRouter.HandleFunc("/_/get_issue_counts", srv.getIssueCountsHandler).Methods("POST")
	appRouter.HandleFunc("/_/get_clients_sources_queries", srv.getClients).Methods("POST")
	appRouter.HandleFunc("/_/get_charts_data", srv.getChartsData).Methods("POST")

	// Use the appRouter as a handler and wrap it into middleware that enforces authentication.
	appHandler := http.Handler(appRouter)
	if !*baseapp.Local {
		appHandler = login.ForceAuth(appRouter, login.DEFAULT_REDIRECT_URL)
	}

	r.PathPrefix("/").Handler(appHandler)
}

func (srv *Server) indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	if err := srv.templates.ExecuteTemplate(w, "index.html", map[string]string{
		// Look in webpack.config.js for where the nonce templates are injected.
		"Nonce": secure.CSPNonce(r.Context()),
	}); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
		return
	}
	return
}

func (srv *Server) getClients(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	clients, err := srv.dbClient.GetClientsFromDB(r.Context())
	if err != nil {
		httputils.ReportError(w, err, "Failed to get clients", http.StatusInternalServerError)
		return
	}

	resp := struct {
		Clients map[types.RecognizedClient]map[types.IssueSource]map[string]bool `json:"clients"`
	}{
		Clients: clients,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to send response: %s", err)

	}
}

func getStringParam(name string, r *http.Request) string {
	raw, ok := r.URL.Query()[name]
	if !ok {
		return ""
	}
	return raw[0]
}

func (srv *Server) getChartsData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse the request.
	q := struct {
		Client types.RecognizedClient `json:"client"`
		Source types.IssueSource      `json:"source"`
		Query  string                 `json:"query"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		httputils.ReportError(w, err, "Failed to decode request.", http.StatusInternalServerError)
		return
	}

	qds, err := srv.dbClient.GetQueryDataFromDB(context.Background(), types.RecognizedClient(q.Client), types.IssueSource(q.Source), q.Query)
	if err != nil {
		sklog.Fatal(err)
	}

	dateToCountsData := map[string]*types.IssueCountsData{}
	validRunIds, err := srv.dbClient.GetAllRecognizedRunIds(r.Context())
	if err != nil {
		httputils.ReportError(w, err, "Failed to get valid runIds from DB", http.StatusInternalServerError)
		return
	}

	for _, qd := range qds {
		if _, ok := validRunIds[qd.RunId]; !ok {
			// Ignore this query data since runId was not found.
			continue
		}

		d := qd.RunId
		if _, ok := dateToCountsData[d]; !ok {
			dateToCountsData[d] = &types.IssueCountsData{}
		}
		dateToCountsData[d].Merge(qd.CountsData)
	}

	// Sort the dates in dateToCountsData.
	dates := []string{}
	for d := range dateToCountsData {
		dates = append(dates, d)
	}
	sort.Slice(dates, func(i, j int) bool {
		ts1, err := time.Parse(time.RFC1123, dates[i])
		if err != nil {
			sklog.Errorf("Could not time.Parse %s", dates[i])
		}
		ts2, err := time.Parse(time.RFC1123, dates[j])
		if err != nil {
			sklog.Errorf("Could not time.Parse %s", dates[j])
		}
		return ts1.Before(ts2)
	})

	openData := [][]interface{}{}
	sloData := [][]interface{}{}
	untriagedData := [][]interface{}{}
	// The first row should contain column information.
	openData = append(openData, []interface{}{"Date", "P0/P1", "P2", "P3+"})
	sloData = append(sloData, []interface{}{"Date", "SLO: P0/P1", "SLO: P2", "SLO: P3+"})
	untriagedData = append(untriagedData, []interface{}{"Date", "Untriaged"})
	for _, d := range dates {
		countsData := dateToCountsData[d]
		openData = append(openData, []interface{}{
			d,                                       // Date
			countsData.P0Count + countsData.P1Count, // P0/P1
			countsData.P2Count,                      // P2
			countsData.P3Count + countsData.P4Count + countsData.P5Count + countsData.P6Count, // P3+
		})
		sloData = append(sloData, []interface{}{
			d, // Date
			countsData.P0SLOViolationCount + countsData.P1SLOViolationCount, // SLO: P0/P1
			countsData.P2SLOViolationCount,                                  // SLO: P2
			countsData.P3SLOViolationCount,                                  // SLO: P3+
		})

		// We did not ingest untriaged data before the 1603288800 timestamp.
		// Hack to exclude everything before so we do not see 0s in the charts.
		ts, err := time.Parse(time.RFC1123, d)
		if err != nil {
			sklog.Errorf("Could not time.Parse %s", d)
		}
		if ts.After(time.Unix(1603288800, 0)) {
			untriagedData = append(untriagedData, []interface{}{
				d,                         // Date
				countsData.UntriagedCount, // Untriaged
			})
		}
	}

	resp := struct {
		OpenData      interface{} `json:"open_data"`
		SloData       interface{} `json:"slo_data"`
		UntriagedData interface{} `json:"untriaged_data"`
	}{
		OpenData:      openData,
		SloData:       sloData,
		UntriagedData: untriagedData,
	}
	if err := json.NewEncoder(w).Encode(&resp); err != nil {
		sklog.Errorf("Failed to send response: %s", err)

	}
}

func (srv *Server) getIssueCountsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse the request.
	q := struct {
		Client types.RecognizedClient `json:"client"`
		Source types.IssueSource      `json:"source"`
		Query  string                 `json:"query"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		httputils.ReportError(w, err, "Failed to decode request.", http.StatusInternalServerError)
		return
	}

	countsData, err := srv.dbClient.GetCountsFromDB(r.Context(), q.Client, q.Source, q.Query)
	if err != nil {
		httputils.ReportError(w, err, "Failed to get issue counts", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(countsData); err != nil {
		sklog.Errorf("Failed to send response: %s", err)

	}
}

// See baseapp.App.
func (srv *Server) AddMiddleware() []mux.MiddlewareFunc {
	return []mux.MiddlewareFunc{}
}

func main() {
	baseapp.Serve(New, []string{*host})
}
