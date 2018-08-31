package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"runtime"

	"github.com/99designs/goodies/http/secure_headers/csp"
	"github.com/gorilla/csrf"
	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
	"go.skia.org/infra/fiddlek/go/store"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auditlog"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// flags
var (
	aud                = flag.String("aud", "", "The aud value, from the Identity-Aware Proxy JWT Audience for the given backend.")
	authGroup          = flag.String("auth_group", "google/skia-staff@google.com", "The chrome infra auth group to use for restricting access.")
	chromeInfraAuthJWT = flag.String("chrome_infra_auth_jwt", "/var/secrets/skia-public-auth/key.json", "The JWT key for the service account that has access to chrome infra auth.")
	local              = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port               = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort           = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir       = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
)

// Server is the state of the server.
type Server struct {
	store     *store.Store
	templates *template.Template
	salt      []byte // Salt for csrf cookies.
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

	srv := &Server{
		store: st,
		salt:  salt,
	}
	srv.loadTemplates()
	return srv, nil
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

func (srv *Server) updateHandler(w http.ResponseWriter, r *http.Request) {
	defer util.Close(r.Body)
	var req Named
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, r, err, "Error decoding JSON.")
		return
	}
	req.User = login.LoggedInAs(r)
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
	if err := srv.store.WriteName(name, req.Hash, req.User); err != nil {
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
			Status: "",
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
		StyleSrc:   []string{csp.SourceSelf},
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
		client, err := auth.NewJWTServiceAccountClient("", *chromeInfraAuthJWT, nil, auth.SCOPE_USERINFO_EMAIL)
		if err != nil {
			sklog.Fatal(err)
		}
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
