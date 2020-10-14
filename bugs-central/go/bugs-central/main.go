/*
	Server that collects and displays bug data for Skia from different issue frameworks
*/

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/unrolled/secure"

	"go.skia.org/infra/bugs-central/go/bugs"
	"go.skia.org/infra/bugs-central/go/mail"
	"go.skia.org/infra/bugs-central/go/types"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	// Flags
	host    = flag.String("host", "bugs-central.skia.org", "HTTP service host")
	workdir = flag.String("workdir", ".", "Directory to use for scratch work.")

	emailClientSecretFile = flag.String("email_client_secret_file", "/etc/bugs-central-email-secrets/client_secret.json", "OAuth client secret JSON file for sending email.")
	emailTokenCacheFile   = flag.String("email_token_cache_file", "/etc/bugs-central-email-secrets/client_token.json", "OAuth token cache file for sending email.")
	serviceAccountFile    = flag.String("service_account_file", "/var/secrets/google/key.json", "Service account JSON file.")
	authAllowList         = flag.String("auth_allowlist", "google.com", "White space separated list of domains and email addresses that are allowed to login.")

	// TODO(rmistry): This needs to be much higher (maybe 15 mins)? 1m is only for testing.
	pollInterval = flag.Duration("poll_interval", 1*time.Minute, "How often the server will poll the different issue frameworks.")
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

	// Initialize mailing library.
	var cfg ClientSecretJSON
	err := util.WithReadFile(*emailClientSecretFile, func(f io.Reader) error {
		return json.NewDecoder(f).Decode(&cfg)
	})
	if err != nil {
		sklog.Fatalf("Failed to read client secrets from %q: %s", *emailClientSecretFile, err)
	}
	// Create a copy of the token cache file since mounted secrets are read-only
	// and the access token will need to be updated for the oauth2 flow.
	if !*baseapp.Local {
		fout, err := ioutil.TempFile("", "")
		if err != nil {
			sklog.Fatalf("Unable to create temp file %q: %s", fout.Name(), err)
		}
		err = util.WithReadFile(*emailTokenCacheFile, func(fin io.Reader) error {
			_, err := io.Copy(fout, fin)
			if err != nil {
				err = fout.Close()
			}
			return err
		})
		if err != nil {
			sklog.Fatalf("Failed to write token cache file from %q to %q: %s", *emailTokenCacheFile, fout.Name(), err)
		}
		*emailTokenCacheFile = fout.Name()
	}
	if err := mail.MailInit(cfg.Installed.ClientID, cfg.Installed.ClientSecret, *emailTokenCacheFile); err != nil {
		sklog.Fatalf("Failed to init mail library: %s", err)
	}

	var allow allowed.Allow
	if !*baseapp.Local {
		allow = allowed.NewAllowedFromList([]string{*authAllowList})
	} else {
		allow = allowed.NewAllowedFromList([]string{"fred@example.org", "barney@example.org", "wilma@example.org"})
	}
	login.SimpleInitWithAllow(*baseapp.Port, *baseapp.Local, nil, nil, allow)

	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(*baseapp.Local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_FULL_CONTROL)

	pollerClient, err := bugs.InitPoller(ctx, ts, *serviceAccountFile)
	if err != nil {
		sklog.Fatal("Could not init poller: %s", err)
	}
	pollerClient.StartPoll(*pollInterval)
	fmt.Println(pollerClient)

	srv := &Server{
		pollerClient: pollerClient,
	}
	srv.loadTemplates()

	return srv, nil
}

// Server is the state of the server.
type Server struct {
	pollerClient *bugs.IssuesPoller
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
	appRouter.HandleFunc("/_/get_issues_data", srv.getIssuesData).Methods("GET")

	// Use the appRouter as a handler and wrap it into middleware that enforces authentication.
	appHandler := http.Handler(appRouter)
	if !*baseapp.Local {
		appHandler = login.ForceAuth(appRouter, login.DEFAULT_REDIRECT_URL)
	}

	r.PathPrefix("/").Handler(appHandler)
}

func (srv *Server) indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	// Needed?
	w.Header().Add("Access-Control-Allow-Origin", "style-src 'self' 'unsafe-inline'")

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

	// user := srv.user(r)
	// if !srv.modify.Member(user) {
	// 	httputils.ReportError(w, nil, "You do not have access to set the tree status.", http.StatusInternalServerError)
	// 	return
	// }

	clients, err := srv.pollerClient.GetClients(r.Context())
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

func (srv *Server) getIssuesData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	client := getStringParam("client", r)
	source := getStringParam("source", r)
	query := getStringParam("query", r)
	dataType := getStringParam("type", r)

	fmt.Println(client)
	fmt.Println(source)
	fmt.Println(query)
	fmt.Println(dataType)

	// user := srv.user(r)
	// if !srv.modify.Member(user) {
	// 	httputils.ReportError(w, nil, "You do not have access to set the tree status.", http.StatusInternalServerError)
	// 	return
	// }

	// Will need new method for query snapshot from DB here.
	// o, u, l, err := srv.pollerClient.GetCounts(r.Context(), types.Recclient, source, query)
	// if err != nil {
	// 	httputils.ReportError(w, err, "Failed to get issue counts", http.StatusInternalServerError)
	// 	return
	// }

	// fmt.Printf("%s %s %s", o, u, l)

	data := [][]interface{}{}
	data = append(data, []interface{}{"Date", "P0", "P1", "P2", "P3"})
	data = append(data, []interface{}{"2020-10-01", 1, 9, 10, 12})
	data = append(data, []interface{}{"2020-10-02", 11, 19, 100, 42})

	// resp := map[string]map[string]int{}
	// resp["2020-10-01"] = map[string]int{
	// 	"pri_01": 1,
	// 	"pri_2":  9,
	// 	"pri_3":  10,
	// }
	// resp["2020-10-02"] = map[string]int{
	// 	"pri_01": 11,
	// 	"pri_2":  19,
	// 	"pri_3":  100,
	// }

	// resp := struct {
	// 	OpenCount       int    `json:"open_count"`
	// 	UnassignedCount int    `json:"unassigned_count"`
	// 	QueryLink       string `json:"query_link"`
	// }{
	// 	OpenCount:       o,
	// 	UnassignedCount: u,
	// 	QueryLink:       l,
	// }
	if err := json.NewEncoder(w).Encode(&data); err != nil {
		sklog.Errorf("Failed to send response: %s", err)

	}
}

func (srv *Server) getIssueCountsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// user := srv.user(r)
	// if !srv.modify.Member(user) {
	// 	httputils.ReportError(w, nil, "You do not have access to set the tree status.", http.StatusInternalServerError)
	// 	return
	// }

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

	countsData, l, err := srv.pollerClient.GetCounts(r.Context(), q.Client, q.Source, q.Query)
	if err != nil {
		httputils.ReportError(w, err, "Failed to get issue counts", http.StatusInternalServerError)
		return
	}

	resp := struct {
		OpenCount       int    `json:"open_count"`
		UnassignedCount int    `json:"unassigned_count"`
		QueryLink       string `json:"query_link"`
	}{
		OpenCount:       countsData.OpenCount,
		UnassignedCount: countsData.UnassignedCount,
		QueryLink:       l,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
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
