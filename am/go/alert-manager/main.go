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
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/iap"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
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

// AuditLog is used to create structured logs for auditable actions.
//
// TODO(jcgregorio) Break out as its own library or maybe fold into sklog?
type AuditLog struct {
	Action string      `json:"action"`
	App    string      `json:"app"`
	Body   interface{} `json:"body"`
	Type   string      `json:"type"`
	User   string      `json:"user"`
}

func auditLog(r *http.Request, action string, body Incident) {
	a := AuditLog{
		Type:   "audit",
		App:    "alert-manager",
		Action: action,
		User:   r.Header.Get(iap.EMAIL_HEADER),
		Body:   body,
	}
	b, err := json.Marshal(a)
	if err != nil {
		sklog.Errorf("Failed to marshall audit log entry: %s", err)
	}
	fmt.Println(string(b))
}

// Server is the state of the server.
type Server struct {
	// TODO add Cloud Datastore
	templates *template.Template
	salt      []byte // Salt for csrf cookies.
}

func New() (*Server, error) {
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../../dist")
	}

	salt := []byte("32-byte-long-auth-key")
	if !*local {
		var err error
		salt, err = ioutil.ReadFile("/var/skia/salt.txt")
		if err != nil {
			return nil, err
		}
	}

	srv := &Server{
		salt: salt,
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

type Silence struct {
	Active         bool                `json:"active"`
	User           string              `json:"user"`
	ParamSet       paramtools.ParamSet `json:"param_set" datastore:"-"`
	SerialParamSet string              `json:"serial_param_set"`
	Created        uint64              `json:"created"`
	Duration       string              `json:"duration"` // A string in a format human.ParseDuration can parse, e.g. "2h".
}

// Note is one note attached to an Incident.
type Note struct {
	Text   string `json:"text"`
	Author string `json:"author"`
	TS     uint64 `json:"ts"` // Time in seconds since the epoch.
}

// Well known keys for Incident.Params.
const (
	ALERT_NAME  = "alertname"
	CATEGORY    = "category"
	SEVERITY    = "severity"
	ID          = "id"
	ASSIGNED_TO = "assigned_to"
)

// Incident
//
// Will appear in either the list of active or silenced incidents,
// so we don't keep that as part of the state since it is derived info.
type Incident struct {
	// The ID is an md5 hash of all the Params. Stored at key ID+Start under a parent entity keyed at ID.
	//	ID         string            `json:"id"` - ID is stored in Params.
	// AssignedTo string            `json:"assigned_to"` // Email address. - AssignedTo is store in Params.
	Active       bool              `json:"active"` // Or archived.
	Start        uint64            `json:"start"`  // Time in seconds since the epoch.
	Finish       uint64            `json:"finish"` // Time in seconds since the epoch.
	Params       paramtools.Params `json:"params" datastore:"-"`
	SerialParams string            `json:"serial_params"`
	Notes        []Note            `json:"notes"`
}

func (srv *Server) incidentHandler(w http.ResponseWriter, r *http.Request) {
	// TODO a slice of incidents.
	w.Header().Set("Content-Type", "application/json")
	/*
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
	*/
}

func (srv *Server) applySecurityWrappers(h http.Handler) http.Handler {
	// Configure Content Security Policy (CSP).
	cspOpts := csp.Opts{
		DefaultSrc: []string{csp.SourceNone},
		ConnectSrc: []string{csp.SourceSelf},
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
		AllowedHosts:          []string{"alert-manager.skia.org"},
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
		"alert-manager",
		common.PrometheusOpt(promPort),
	)

	srv, err := New()
	if err != nil {
		sklog.Fatalf("Failed to create Server: %s", err)
	}

	// Start a Go routine that listens for PubSub events and converts them into
	// Incidents, where HEALTHZ values are stripped out and ID and location
	// are populated in the Params.

	// Callers can get all Incidents, or ones filtered through the Silences.

	r := mux.NewRouter()
	r.HandleFunc("/", srv.mainHandler)
	r.HandleFunc("/_/incidents", srv.incidentHandler)
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
		h = iap.New(h, *aud, allow)
	}
	h = httputils.LoggingGzipRequestResponse(h)
	http.Handle("/", h)
	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
