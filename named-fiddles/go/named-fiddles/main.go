package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"runtime"

	"github.com/gorilla/mux"
	"go.skia.org/infra/fiddlek/go/store"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/iap"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// flags
var (
	aud                = flag.String("aud", "", "The aud value, from the Identity-Aware Proxy JWT Audience for the given backend.")
	authGroup          = flag.String("auth_group", "google/skia-staff@google.com", "The chrome infra auth group to use for restricting access.")
	local              = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port               = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort           = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir       = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	chromeInfraAuthJWT = flag.String("chrome_infra_auth_jwt", "/var/secrets/skia-public-auth/key.json", "The JWT key for the service account that has access to chrome infra auth.")
)

type AuditLog struct {
	Type   string `json:"type"`
	User   string `json:"user"`
	Action string `json:"action"`
	Body   Named  `json:"body"`
}

func auditLog(r *http.Request, action string, body Named) {
	a := AuditLog{
		Type:   "audit",
		Action: action,
		Body:   body,
		User:   r.Header.Get(iap.EMAIL_HEADER),
	}
	b, err := json.Marshal(a)
	if err != nil {
		sklog.Errorf("Failed to marshall audit log entry: %s", err)
	}
	sklog.Error(string(b))
}

// Server is the state of the server.
type Server struct {
	store     *store.Store
	templates *template.Template
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

	srv := &Server{
		store: st,
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
	if err := srv.templates.ExecuteTemplate(w, "index.html", nil); err != nil {
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
	auditLog(r, "update", req)
	sklog.Infof("Got %v", req)

	if req.Hash == "" {
		httputils.ReportError(w, r, nil, "Invalid request, Hash must be non-empty.")
		return
	}
	// Name == ""     => Create
	// NewName != ""  => Rename
	// NewName == ""  => Update
	if err := srv.store.WriteName(req.NewName, req.Hash, req.User); err != nil {
		httputils.ReportError(w, r, err, "Failed update.")
		return
	}
	if req.Name != "" {
		// Rename, so delete old Name.
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
	auditLog(r, "delete", req)
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

func main() {
	common.InitWithMust(
		"named-fiddles",
		common.PrometheusOpt(promPort),
	)

	srv, err := New()
	if err != nil {
		sklog.Fatalf("Failed to start: %s", err)
	}

	r := mux.NewRouter()
	r.HandleFunc("/", srv.mainHandler)
	r.HandleFunc("/_/update", srv.updateHandler)
	r.HandleFunc("/_/delete", srv.deleteHandler)
	r.HandleFunc("/_/named", srv.namedHandler)
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))))

	// TODO(jcgregorio) Implement CSRF.
	h := httputils.LoggingGzipRequestResponse(r)

	if !*local {
		client, err := auth.NewJWTServiceAccountClient("", *chromeInfraAuthJWT, nil, auth.SCOPE_USERINFO_EMAIL)
		if err != nil {
			sklog.Fatal(err)
		}
		allowed, err := iap.NewAllowedFromChromeInfraAuth(client, *authGroup)
		if err != nil {
			sklog.Fatal(err)
		}
		h = iap.New(h, *aud, allowed)
	}
	http.Handle("/", h)
	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
