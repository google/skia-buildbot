package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"runtime"
	"time"

	"github.com/99designs/goodies/http/secure_headers/csp"
	"github.com/gorilla/csrf"
	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
	"go.skia.org/infra/fiddlek/go/client"
	"go.skia.org/infra/fiddlek/go/store"
	"go.skia.org/infra/fiddlek/go/types"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auditlog"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// flags
var (
	aud                = flag.String("aud", "", "The aud value, from the Identity-Aware Proxy JWT Audience for the given backend.")
	authGroup          = flag.String("auth_group", "google/skia-staff@google.com", "The chrome infra auth group to use for restricting access.")
	chromeInfraAuthJWT = flag.String("chrome_infra_auth_jwt", "/var/secrets/skia-public-auth/key.json", "The JWT key for the service account that has access to chrome infra auth.")
	local              = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	period             = flag.Duration("period", time.Hour, "How often to check if the named fiddles are valid.")
	port               = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort           = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	repoURL            = flag.String("repo_url", "https://skia.googlesource.com/skia", "Repo url")
	repoDir            = flag.String("repo_dir", "/tmp/skia_named_fiddles", "Directory the repo is checked out into.")
	resourcesDir       = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
)

// Server is the state of the server.
type Server struct {
	store     *store.Store
	templates *template.Template
	salt      []byte // Salt for csrf cookies.
	repo      *gitinfo.GitInfo

	liveness    metrics2.Liveness    // liveness of the continuous validation process.
	errorsInRun metrics2.Counter     // errorsInRun is the number of errors in a single validation run.
	numInvalid  metrics2.Int64Metric // numInvalid is the number of fiddles that are currently invalid.
}

func New() (*Server, error) {
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../../dist")
	}

	st, err := store.New(*local)
	if err != nil {
		return nil, fmt.Errorf("Failed to create client for GCS: %s", err)
	}
	salt := []byte("32-byte-long-auth-key")
	if !*local {
		salt, err = ioutil.ReadFile("/var/skia/salt.txt")
		if err != nil {
			return nil, err
		}
	}

	if !*local {
		ts, err := auth.NewDefaultTokenSource(false, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT)
		if err != nil {
			sklog.Fatalf("Failed authentication: %s", err)
		}
		// Use the gitcookie created by the gitauth package.
		if _, err := gitauth.New(ts, "/tmp/gitcookies", true, ""); err != nil {
			sklog.Fatalf("Failed to create git cookie updater: %s", err)
		}
		sklog.Infof("Git authentication set up successfully.")
	}

	repo, err := gitinfo.CloneOrUpdate(context.Background(), *repoURL, *repoDir, false)
	if err != nil {
		return nil, fmt.Errorf("Failed to create git repo: %s", err)
	}

	srv := &Server{
		store: st,
		salt:  salt,
		repo:  repo,

		liveness:    metrics2.NewLiveness("named_fiddles_check"),
		errorsInRun: metrics2.GetCounter("named_fiddles_errors_in_run", nil),
		numInvalid:  metrics2.GetInt64Metric("named_fiddles_total_invalid"),
	}
	srv.loadTemplates()
	go srv.checkValid()
	return srv, nil
}

// validate a given named fiddle.
//
// Returns false if the named fiddle fails when running or compiling.
func (srv *Server) validate(n store.Named) bool {
	c := httputils.NewTimeoutClient()
	sklog.Infof("Validating: %s", n.Name)
	// Load the fiddle.
	getResp, err := c.Get(fmt.Sprintf("https://fiddle.skia.org/e/%s", n.Hash))
	if err != nil {
		sklog.Warningf("Failed to fetch %q = %q: %s", n.Name, n.Hash, err)
		srv.errorsInRun.Inc(1)
		return true
	}

	// Then re-run it.
	b, err := ioutil.ReadAll(getResp.Body)
	if err != nil {
		sklog.Warningf("Failed to read fiddle: %s", err)
		return true
	}
	runResults, success := client.Do(b, false, "https://fiddle.skia.org", func(*types.RunResults) bool {
		return true
	})
	if !success {
		srv.errorsInRun.Inc(1)
		return true
	}
	// Update the status.
	status := ""
	if runResults == nil {
		status = "Failed to run."
	} else if len(runResults.CompileErrors) > 0 || runResults.RunTimeError != "" {
		// update validity
		status = fmt.Sprintf("%v %s", runResults.CompileErrors, runResults.RunTimeError)
		if len(status) > 100 {
			status = status[:100]
		}
	}
	if status != "" {
		sklog.Infof("%q is invalid.", n.Name)
	}
	if err := srv.store.SetStatus(n.Name, status); err != nil {
		sklog.Errorf("Failed to write updated status for %s: %s", n.Name, err)
		srv.errorsInRun.Inc(1)
	}
	return status == ""
}

// step is a single run of the fiddle verifier.
func (srv *Server) step() {
	named, err := srv.store.ListAllNames()
	if err != nil {
		sklog.Errorf("Failed to retrive the named fiddles to check: %s ", err)
	}
	defer srv.liveness.Reset()
	srv.errorsInRun.Reset()
	sklog.Infof("Starting validation run.")
	var numInvalid int64
	for _, n := range named {
		if !srv.validate(n) {
			numInvalid += 1
		}
	}
	srv.numInvalid.Update(numInvalid)
}

// checkValid periodically checks if all the named fiddles are valid.
func (srv *Server) checkValid() {
	srv.step()
	for _ = range time.Tick(time.Minute) {
		srv.step()
	}
}

func (srv *Server) loadTemplates() {
	srv.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*resourcesDir, "index.html"),
	))
}

func (srv *Server) mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *local {
		srv.loadTemplates()
	}
	if err := srv.templates.ExecuteTemplate(w, "index.html", map[string]string{
		// base64 encode the csrf to avoid golang templating escaping.
		"csrf": base64.StdEncoding.EncodeToString([]byte(csrf.Token(r))),
	}); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

type Named struct {
	Name    string `json:"name"`
	User    string `json:"user"`
	Hash    string `json:"hash"`
	NewName string `json:"new_name,omitempty"`
	Status  string `json:"status"`
}

// user returns the currently logged in user, or a placeholder if running locally.
func (srv *Server) user(r *http.Request) string {
	user := "barney@example.org"
	if !*local {
		user = login.LoggedInAs(r)
	}
	return user
}

func (srv *Server) updateHandler(w http.ResponseWriter, r *http.Request) {
	defer util.Close(r.Body)
	var req Named
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, r, err, "Error decoding JSON.")
		return
	}
	req.User = srv.user(r)
	auditlog.Log(r, "update", req)

	if req.Hash == "" {
		httputils.ReportError(w, r, nil, "Invalid request, Hash must be non-empty.")
		return
	}
	if err := srv.store.Exists(req.Hash); err != nil {
		httputils.ReportError(w, r, err, "Hash is not a valid fiddle.")
		return
	}
	// Name == ""     => Create
	// NewName != ""  => Rename
	// NewName == ""  => Update
	name := req.NewName
	if req.NewName == "" {
		// This is an update.
		name = req.Name
	}
	if name == "" {
		httputils.ReportError(w, r, nil, "Name must not be empty.")
		return
	}
	if !srv.store.ValidName(name) {
		httputils.ReportError(w, r, nil, "Invalid characaters found in name.")
		return
	}
	if err := srv.store.WriteName(name, req.Hash, req.User, req.Status); err != nil {
		httputils.ReportError(w, r, err, "Failed update.")
		return
	}
	if req.Name != "" && req.NewName != "" {
		// This is a rename so delete old Name.
		if err := srv.store.DeleteName(req.Name); err != nil {
			httputils.ReportError(w, r, err, "Failed delete on rename.")
			return
		}
	}
	// Re-check the validity of the named fiddle in the background.
	go func() {
		n := store.Named{
			Name: req.Name,
			User: req.User,
			Hash: req.Hash,
		}
		_ = srv.validate(n)
	}()
	srv.namedHandler(w, r)
}

func (srv *Server) deleteHandler(w http.ResponseWriter, r *http.Request) {
	defer util.Close(r.Body)
	var req Named
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, r, err, "Error decoding JSON.")
		return
	}
	auditlog.Log(r, "delete", req)
	if err := srv.store.DeleteName(req.Name); err != nil {
		httputils.ReportError(w, r, err, "Failed to delete.")

	}
	srv.namedHandler(w, r)
}

func (srv *Server) namedHandler(w http.ResponseWriter, r *http.Request) {
	named, err := srv.store.ListAllNames()
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to read named fiddles.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	resp := make([]*Named, 0, len(named))
	for _, n := range named {
		resp = append(resp, &Named{
			Name:   n.Name,
			Hash:   n.Hash,
			User:   n.User,
			Status: n.Status,
		})
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) applySecurityWrappers(h http.Handler) http.Handler {
	// Configure Content Security Policy (CSP).
	cspOpts := csp.Opts{
		DefaultSrc: []string{csp.SourceNone},
		ConnectSrc: []string{"https://skia.org", csp.SourceSelf},
		ImgSrc:     []string{csp.SourceSelf},
		StyleSrc:   []string{csp.SourceSelf, csp.SourceUnsafeInline},
		ScriptSrc:  []string{csp.SourceSelf},
	}

	if *local {
		// webpack uses eval() in development mode, so allow unsafe-eval when local.
		cspOpts.ScriptSrc = append(cspOpts.ScriptSrc, "'unsafe-eval'")
	}

	// Apply CSP and other security minded headers.
	secureMiddleware := secure.New(secure.Options{
		AllowedHosts:          []string{"named-fiddles.skia.org", "skia.org"},
		HostsProxyHeaders:     []string{"X-Forwarded-Host"},
		SSLRedirect:           true,
		SSLProxyHeaders:       map[string]string{"X-Forwarded-Proto": "https"},
		STSSeconds:            60 * 60 * 24 * 365,
		STSIncludeSubdomains:  true,
		ContentSecurityPolicy: cspOpts.Header(),
		IsDevelopment:         *local,
	})

	h = secureMiddleware.Handler(h)

	// Protect against CSRF.
	h = csrf.Protect(srv.salt, csrf.Secure(!*local))(h)
	return h
}

func main() {
	common.InitWithMust(
		"named-fiddles",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)

	srv, err := New()
	if err != nil {
		sklog.Fatalf("Failed to create Server: %s", err)
	}

	r := mux.NewRouter()
	r.HandleFunc("/", srv.mainHandler)
	r.HandleFunc("/_/update", srv.updateHandler).Methods("POST")
	r.HandleFunc("/_/delete", srv.deleteHandler).Methods("POST")
	r.HandleFunc("/_/named", srv.namedHandler)
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))))

	h := srv.applySecurityWrappers(r)
	if !*local {
		ts, err := auth.NewJWTServiceAccountTokenSource("", *chromeInfraAuthJWT, auth.SCOPE_USERINFO_EMAIL)
		if err != nil {
			sklog.Fatal(err)
		}
		client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
		allow, err := allowed.NewAllowedFromChromeInfraAuth(client, *authGroup)
		if err != nil {
			sklog.Fatal(err)
		}
		login.InitWithAllow(*port, *local, nil, nil, allow)
	}

	if !*local {
		h = httputils.LoggingGzipRequestResponse(h)
		h = login.RestrictViewer(h)
		h = login.ForceAuth(h, login.DEFAULT_REDIRECT_URL)
		h = httputils.HealthzAndHTTPS(h)
	}
	http.Handle("/", h)

	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
