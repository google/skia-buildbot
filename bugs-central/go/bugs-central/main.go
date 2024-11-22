/*
	Server that collects and displays bug data for Skia's clients from different issue frameworks
*/

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/go-chi/chi/v5"
	"github.com/unrolled/secure"
	"golang.org/x/oauth2/google"

	"go.skia.org/infra/bugs-central/go/db"
	"go.skia.org/infra/bugs-central/go/poller"
	"go.skia.org/infra/bugs-central/go/types"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/alogin/proxylogin"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

var (
	// Flags
	host         = flag.String("host", "bugs-central.skia.org", "HTTP service host")
	workdir      = flag.String("workdir", ".", "Directory to use for scratch work.")
	fsNamespace  = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'bugs-central'")
	fsProjectID  = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")
	pollInterval = flag.Duration("poll_interval", 2*time.Hour, "How often the server will poll the different issue frameworks for open issues.")

	// Cache of clients to charts data. Used for displaying charts in the UI.
	clientsToChartsDataCache = map[string]map[string]*types.IssueCountsData{}
	// mtx to control access to the above charts data cache.
	mtxChartsData sync.RWMutex
)

const (
	secretProject     = "skia-infra-public"
	secretGithubToken = "bugs-central-github-token"
)

// See baseapp.Constructor.
func New() (baseapp.App, error) {
	ctx := context.Background()

	// Create workdir if it does not exist.
	if err := os.MkdirAll(*workdir, 0755); err != nil {
		sklog.Fatalf("Could not create %s: %s", *workdir, err)
	}

	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail, auth.ScopeFullControl, datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatalf("Could not init token source: %s", err)
	}
	dbClient, err := db.New(ctx, ts, *fsNamespace, *fsProjectID)
	if err != nil {
		sklog.Fatalf("Could not init DB: %s", err)
	}

	// Get github token from secrets to send to poller.
	secretClient, err := secret.NewClient(ctx)
	if err != nil {
		sklog.Fatalf("Could not init secrets manager: %s", err)
	}
	githubToken, err := secretClient.Get(ctx, secretProject, secretGithubToken, secret.VersionLatest)
	if err != nil {
		sklog.Fatalf("Failed to retrieve secret %s: %s", secretGithubToken, err)
	}
	githubToken = strings.TrimSpace(githubToken)
	githubTokenFile, err := os.CreateTemp("", "github-token-")
	if err != nil {
		sklog.Fatalf("Could not create tmp file for github token: %s", err)
	}
	defer githubTokenFile.Close()
	if _, err := githubTokenFile.Write([]byte(githubToken)); err != nil {
		sklog.Fatalf("Could not write github token to tmp file: %s", err)
	}

	// Instantiate poller and turn it on.
	pollerClient, err := poller.New(ctx, ts, githubTokenFile.Name(), dbClient)
	if err != nil {
		sklog.Fatalf("Could not init poller: %s", err)
	}
	if err := pollerClient.Start(ctx, *pollInterval); err != nil {
		sklog.Fatalf("Could not start poller: %s", err)
	}

	srv := &Server{
		pollerClient: pollerClient,
		dbClient:     dbClient,
	}
	srv.loadTemplates()

	// Populate the charts data cache on startup and then periodically.
	cleanup.Repeat(*pollInterval, func(ctx context.Context) {
		if err := srv.populateChartsDataCache(ctx); err != nil {
			sklog.Errorf("Could not populate the charts data cache: %s", err)
			return
		}
	}, nil)

	return srv, nil
}

// Server is the state of the server.
type Server struct {
	pollerClient *poller.IssuesPoller
	dbClient     types.BugsDB
	templates    *template.Template
}

func (srv *Server) loadTemplates() {
	srv.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*baseapp.ResourcesDir, "index.html"),
	))
}

// See baseapp.App.
func (srv *Server) AddHandlers(r chi.Router) {
	// Endpoint that status will use to get client counts.
	r.Get("/get_client_counts", httputils.CorsHandler(srv.getClientCounts))

	plogin := proxylogin.NewWithDefaults()

	// All other endpoints must be logged in.
	appRouter := chi.NewRouter()
	appRouter.HandleFunc("/", srv.indexHandler)
	appRouter.Post("/_/get_issue_counts", srv.getIssueCountsHandler)
	appRouter.Post("/_/get_clients_sources_queries", srv.getClients)
	appRouter.Post("/_/get_charts_data", srv.getChartsData)
	appRouter.Post("/_/get_issues_outside_slo", srv.getIssuesOutsideSLO)

	appRouter.Get("/_/login/status", alogin.LoginStatusHandler(plogin))

	// Use the appRouter as a handler and wrap it into middleware that enforces authentication.
	appHandler := http.Handler(appRouter)
	if !*baseapp.Local {
		appHandler = alogin.ForceRole(appRouter, plogin, roles.Viewer)
	}

	r.Handle("/*", appHandler)
}

func (srv *Server) indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	if err := srv.templates.ExecuteTemplate(w, "index.html", map[string]string{

		"Nonce": secure.CSPNonce(r.Context()),
	}); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
		return
	}
}

func (srv *Server) getClients(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	clients, err := srv.dbClient.GetClientsFromDB(r.Context())
	if err != nil {
		httputils.ReportError(w, err, "Failed to get clients", http.StatusInternalServerError)
		return
	}

	resp := types.GetClientsResponse{
		Clients: clients,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to send response: %s", err)

	}
}

// StatusData is used in the response of the get_client_counts endpoint.
type StatusData struct {
	UntriagedCount int    `json:"untriaged_count"`
	Link           string `json:"link"`
}

// GetClientCountsResponse is the response used by the get_client_counts endpoint.
type GetClientCountsResponse struct {
	ClientsToStatusData map[types.RecognizedClient]StatusData `json:"clients_to_status_data"`
}

func (srv *Server) getClientCounts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	clientsForStatus := []types.RecognizedClient{types.SkiaClient, types.AndroidClient, types.ChromiumClient, types.OSSFuzzClient}
	clientsToStatusData := map[types.RecognizedClient]StatusData{}
	for _, c := range clientsForStatus {
		countsData, err := srv.dbClient.GetCountsFromDB(r.Context(), c, "", "")
		if err != nil {
			httputils.ReportError(w, err, "Failed to query DB.", http.StatusInternalServerError)
		}
		clientsToStatusData[c] = StatusData{
			UntriagedCount: countsData.UntriagedCount,
			Link:           fmt.Sprintf("http://%s/?client=%s", *host, c),
		}
	}

	resp := GetClientCountsResponse{
		ClientsToStatusData: clientsToStatusData,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) getIssuesOutsideSLO(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse the request.
	var req types.ClientSourceQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode request.", http.StatusInternalServerError)
		return
	}

	priToIssues := srv.pollerClient.GetOpenIssues().GetIssuesOutsideSLO(req.Client, req.Source, req.Query)
	resp := types.IssuesOutsideSLOResponse{
		PriToSLOIssues: priToIssues,
	}
	if err := json.NewEncoder(w).Encode(&resp); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) getChartsData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse the request.
	var req types.ClientSourceQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode request.", http.StatusInternalServerError)
		return
	}
	c := types.RecognizedClient(req.Client)
	s := types.IssueSource(req.Source)
	q := req.Query
	clientKey := getClientsKey(c, s, q)
	sklog.Infof("Retrieving charts data request for %s", clientKey)

	// Read chart data from the cache.
	mtxChartsData.RLock()
	defer mtxChartsData.RUnlock()
	dateToCountsData, ok := clientsToChartsDataCache[clientKey]
	if !ok {
		errorMsg := fmt.Sprintf("Could not find client key: %s", clientKey)
		httputils.ReportError(w, errors.New(errorMsg), errorMsg, http.StatusBadRequest)
		return
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

	resp := types.GetChartsDataResponse{
		OpenData:      openData,
		SloData:       sloData,
		UntriagedData: untriagedData,
	}
	if err := json.NewEncoder(w).Encode(&resp); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

// getDateToCountsData returns counts data for the specified client, source and
// query.
func (srv *Server) getDateToCountsData(ctx context.Context, c types.RecognizedClient, s types.IssueSource, q string) (map[string]*types.IssueCountsData, error) {
	qds, err := srv.dbClient.GetQueryDataFromDB(context.Background(), c, s, q)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	dateToCountsData := map[string]*types.IssueCountsData{}
	validRunIds, err := srv.dbClient.GetAllRecognizedRunIds(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get valid runIds from DB")
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
	return dateToCountsData, nil
}

// populateChartsDataCache populates the charts data cache for
// all supported clients + sources + queries. Also includes data
// for empty-client, empty-source, empty-query.
func (srv *Server) populateChartsDataCache(ctx context.Context) error {
	mtxChartsData.Lock()
	defer mtxChartsData.Unlock()

	sklog.Info("Starting populating charts data cache")
	clientsData, err := srv.dbClient.GetClientsFromDB(ctx)
	if err != nil {
		return skerr.Wrapf(err, "Failed to get clients")
	}
	for c, sources := range clientsData {
		for s, queries := range sources {
			for q := range queries {
				// Add data for client + source + query.
				dateToCountsData, err := srv.getDateToCountsData(ctx, c, s, q)
				if err != nil {
					return skerr.Wrap(err)
				}
				clientsToChartsDataCache[getClientsKey(c, s, q)] = dateToCountsData
			}
			// Add data for client + source + empty-query.
			dateToCountsData, err := srv.getDateToCountsData(ctx, c, s, "")
			if err != nil {
				return skerr.Wrap(err)
			}
			clientsToChartsDataCache[getClientsKey(c, s, "")] = dateToCountsData
		}
		// Add data for client + empty-source + empty-query.
		dateToCountsData, err := srv.getDateToCountsData(ctx, c, "", "")
		if err != nil {
			return skerr.Wrap(err)
		}
		clientsToChartsDataCache[getClientsKey(c, "", "")] = dateToCountsData
	}
	// Add data for empty-client + empty-source + empty-query.
	dateToCountsData, err := srv.getDateToCountsData(ctx, "", "", "")
	if err != nil {
		return skerr.Wrap(err)
	}
	clientsToChartsDataCache[getClientsKey("", "", "")] = dateToCountsData

	sklog.Info("Done populating charts data cache")
	return nil
}

// getClientsKey returns a key that combines the client, source and query.
func getClientsKey(client types.RecognizedClient, source types.IssueSource, query string) string {
	return fmt.Sprintf("%s > %s > %s", client, source, query)
}

func (srv *Server) getIssueCountsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse the request.
	var req types.ClientSourceQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode request.", http.StatusInternalServerError)
		return
	}

	countsData, err := srv.dbClient.GetCountsFromDB(r.Context(), req.Client, req.Source, req.Query)
	if err != nil {
		httputils.ReportError(w, err, "Failed to get issue counts", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(countsData); err != nil {
		sklog.Errorf("Failed to send response: %s", err)

	}
}

// See baseapp.App.
func (srv *Server) AddMiddleware() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{}
}

func main() {
	// Parse flags to be able to send *host to baseapp.Serve
	flag.Parse()
	baseapp.Serve(New, []string{*host})
}
