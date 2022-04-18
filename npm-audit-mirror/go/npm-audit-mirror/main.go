/*
	Server that brings up NPM mirrors for supported projects, performs
	pre-download security checks, and audits their package.json files.
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/netutils"
	"go.skia.org/infra/go/sklog"
	allowlist "go.skia.org/infra/npm-audit-mirror/go/allowlists"
	audit "go.skia.org/infra/npm-audit-mirror/go/audits"
	"go.skia.org/infra/npm-audit-mirror/go/checks"
	"go.skia.org/infra/npm-audit-mirror/go/config"
	"go.skia.org/infra/npm-audit-mirror/go/db"
	"go.skia.org/infra/npm-audit-mirror/go/mirrors"
	"go.skia.org/infra/npm-audit-mirror/go/types"
	"golang.org/x/oauth2/google"
)

var (
	// Flags
	host               = flag.String("host", "npm.skia.org", "HTTP service host")
	workdir            = flag.String("workdir", ".", "Directory to use for scratch work.")
	fsNamespace        = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'npm-audit-mirror-staging'")
	fsProjectID        = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")
	serviceAccountFile = flag.String("service_account_file", "/var/secrets/google/key.json", "Service account JSON file.")
	authAllowList      = flag.String("auth_allowlist", "google.com", "White space separated list of domains and email addresses that are allowed to login.")
	hang               = flag.Bool("hang", false, "If true, don't spin up the server, just hang without doing anything.")
	auditsInterval     = flag.Duration("audits_interval", 2*time.Hour, "How often the server checks for audit issues.")
)

// See baseapp.Constructor.
func New() (baseapp.App, error) {
	if *hang {
		return &Server{}, nil
	}

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
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail, auth.ScopeGerrit, auth.ScopeFullControl, datastore.ScopeDatastore, "https://www.googleapis.com/auth/devstorage.read_only")
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Instantiate DB.
	dbClient, err := db.New(ctx, ts, *fsNamespace, *fsProjectID)
	if err != nil {
		sklog.Fatalf("Could not init DB: %s", err)
	}

	// Get the NPM audit mirror config.
	cfg, err := config.GetConfig()
	if err != nil {
		sklog.Fatalf("Could not parse the config file: %s", err)
	}
	// Get the mirror config template.
	mirrorCfgTemplate, err := config.GetMirrorConfigTmpl()
	if err != nil {
		sklog.Fatalf("Could not parse the mirror config template: %s", err)
	}

	// Loop through all supported projects and start their mirrors and
	// their audits.
	supportedProjectsToInfo := map[string]*ProjectInfo{}
	for projectName, projectCfg := range cfg.SupportedProjects {

		// Create a project specific workdir if it does not already exist.
		projectWorkdir := filepath.Join(*workdir, projectName)
		if err := os.MkdirAll(projectWorkdir, 0755); err != nil {
			sklog.Fatalf("Could not create %s: %s", projectWorkdir, err)
		}

		// Create a file to output rejection reasons to.
		rejectionsLogFilePath := path.Join(projectWorkdir, "rejections-log.txt")
		if _, err := os.OpenFile(rejectionsLogFilePath, os.O_RDONLY|os.O_CREATE, 0644); err != nil {
			sklog.Fatalf("Could not create %s: %s", rejectionsLogFilePath, err)
		}

		// Start the audit for the project.
		a, err := audit.NewNpmProjectAudit(ctx, projectName, projectCfg.RepoURL, projectCfg.GitBranch, projectCfg.PackageJSONDir, projectWorkdir, *serviceAccountFile, httpClient, dbClient, projectCfg.MonorailConfig)
		if err != nil {
			sklog.Fatalf("Could not instantiate audit: %s", err)
		}
		a.StartAudit(ctx, *auditsInterval)

		// Start the mirror for the project.
		host := fmt.Sprintf("https://%s", *host)
		if *baseapp.Local {
			host = fmt.Sprintf("http://localhost%s", *baseapp.Port)
		}
		m, err := mirrors.NewVerdaccioMirror(projectName, projectWorkdir, host, mirrorCfgTemplate)
		if err != nil {
			sklog.Fatalf("Could not create mirror for %s: %s", projectName, err)
		}
		unusedPort := netutils.FindUnusedTCPPort()
		if err := m.StartMirror(ctx, unusedPort); err != nil {
			sklog.Fatalf("Could not start mirror for %s: %s", projectName, err)
		}

		// Get allowlist of all specified packages and all their non-semver dependencies.
		allowlistWithDeps, err := allowlist.GetAllowlistWithDeps(projectCfg.PackagesAllowList, httpClient)
		if err != nil {
			sklog.Fatalf("Could not get allowlist with direct dependencies: %s", err)
		}

		// Populate project info with all artifacts from above.
		projectInfo := ProjectInfo{}
		projectInfo.verdacciPort = unusedPort
		projectInfo.rejectionsLogFilePath = rejectionsLogFilePath
		projectInfo.checksManager = checks.NewNpmChecksManager(projectCfg.TrustedScopes, allowlistWithDeps, httpClient, m)

		// Save in the map.
		supportedProjectsToInfo[projectName] = &projectInfo
	}

	srv := &Server{
		supportedProjectsToInfo: supportedProjectsToInfo,
		httpClient:              httpClient,
	}

	return srv, nil
}

// Server is the state of the server.
type Server struct {
	dbClient                *db.FirestoreDB
	supportedProjectsToInfo map[string]*ProjectInfo
	httpClient              *http.Client
}

// ProjectInfo details the artifacts used by a supported project.
type ProjectInfo struct {
	verdacciPort          int
	rejectionsLogFilePath string
	rejectionsLogMutex    sync.RWMutex

	checksManager types.ChecksManager
}

// user returns the currently logged in user, or a placeholder if running locally.
func (srv *Server) user(r *http.Request) string {
	user := "barney@example.org"
	if !*baseapp.Local {
		user = login.LoggedInAs(r)
	}
	return user
}

// rejectionsLogHandler displays the rejection logs for the specified project.
func (srv *Server) rejectionsLogHandler(h http.Handler, projectInfo *ProjectInfo) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		projectInfo.rejectionsLogMutex.RLock()
		http.ServeFile(w, r, projectInfo.rejectionsLogFilePath)
		defer projectInfo.rejectionsLogMutex.RUnlock()
		return
	})
}

// verdaccioReverseProxyHandler is the endpoint that can serve as a NPM registry.
func (srv *Server) verdaccioReverseProxyHandler(h http.Handler, projectName string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		projectInfo := srv.supportedProjectsToInfo[projectName]
		targetURL := fmt.Sprintf("http://localhost:%d", projectInfo.verdacciPort)
		target, err := url.Parse(targetURL)
		if err != nil {
			httputils.ReportError(w, err, fmt.Sprintf("Unable to parse target URL %s", targetURL), http.StatusInternalServerError)
			return
		}

		director := func(req *http.Request) {
			// Set the schedule and host to the proxy.
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
		}
		proxy := &httputil.ReverseProxy{Director: director}

		// Now perform pre-download security checks.
		checksResult, rejectionReason, err := projectInfo.checksManager.PerformChecks(r.URL.String())
		if err != nil {
			httputils.ReportError(w, err, fmt.Sprintf("Error running security checks on %s", r.URL.String()), http.StatusInternalServerError)
			return
		}
		if !checksResult {
			// Record in to the log endpoint why the package was rejected.
			projectInfo.rejectionsLogMutex.Lock()
			f, err := os.OpenFile(projectInfo.rejectionsLogFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				projectInfo.rejectionsLogMutex.Unlock()
				httputils.ReportError(w, err, fmt.Sprintf("Failed to open %s", projectInfo.rejectionsLogFilePath), http.StatusInternalServerError)
				return
			}
			rejectionsLogger := log.New(f, "", log.LstdFlags|log.Lshortfile)
			rejectionsLogger.Println(rejectionReason)
			f.Close()
			projectInfo.rejectionsLogMutex.Unlock()

			// Information about 451 status code is here: https://www.rfc-editor.org/rfc/rfc7725.html
			httputils.ReportError(w, err, rejectionReason, http.StatusUnavailableForLegalReasons)
			return
		}

		// At this point all security checks passed.
		proxy.ServeHTTP(w, r)
	})
}

// See baseapp.App.
func (srv *Server) AddHandlers(r *mux.Router) {
	// For login/logout.
	r.HandleFunc(login.DEFAULT_OAUTH2_CALLBACK, login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)

	// All endpoints that require authentication should be added to this router.
	appRouter := mux.NewRouter()

	for project := range srv.supportedProjectsToInfo {
		projectInfo := srv.supportedProjectsToInfo[project]
		appRouter.Handle(fmt.Sprintf("/rejection-logs-%s", project), srv.rejectionsLogHandler(appRouter, projectInfo)).Methods("GET")

		projectEndpoint := fmt.Sprintf("/%s", project)
		// This path supports both GETs and POSTs because:
		// All registry API calls use GETS- https://github.com/npm/registry/blob/master/docs/REGISTRY-API.md
		// But the audit call at "/-/npm/v1/security/audits" uses a POST.
		r.PathPrefix(projectEndpoint + "/").Handler(http.StripPrefix(projectEndpoint, srv.verdaccioReverseProxyHandler(r, project)))
	}

	// Use the appRouter as a handler and wrap it into middleware that enforces authentication.
	appHandler := http.Handler(appRouter)
	if !*baseapp.Local {
		appHandler = login.ForceAuth(appRouter, login.DEFAULT_REDIRECT_URL)
	}

	r.PathPrefix("/").Handler(appHandler)
}

// See baseapp.App.
func (srv *Server) AddMiddleware() []mux.MiddlewareFunc {
	return []mux.MiddlewareFunc{}
}

func main() {
	// Parse flags.
	flag.Parse()

	baseapp.Serve(New,
		[]string{*host},
		// Do not GZip the response.
		// See https://verdaccio.org/docs/reverse-proxy/#invalid-checksum
		baseapp.DisableResponseGZip{},
	)
}
