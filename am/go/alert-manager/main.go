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
	"os"
	"path/filepath"
	"runtime"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/99designs/goodies/http/secure_headers/csp"
	"github.com/gorilla/csrf"
	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
	"go.skia.org/infra/am/go/incident"
	"go.skia.org/infra/go/alerts"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/iap"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"
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
	namespace          = flag.String("namespace", "", "The Cloud Datastore namespace, such as 'perf'.")
	project            = flag.String("project", "skia-public", "The Google Cloud project name.")
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

func auditLog(r *http.Request, action string, body interface{}) {
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
	incidentStore *incident.Store
	templates     *template.Template
	salt          []byte // Salt for csrf cookies.
	allow         allowed.Allow
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

	var allow allowed.Allow
	if !*local {
		client, err := auth.NewJWTServiceAccountClient("", *chromeInfraAuthJWT, nil, auth.SCOPE_USERINFO_EMAIL)
		if err != nil {
			return nil, err
		}
		allow, err = allowed.NewAllowedFromChromeInfraAuth(client, *authGroup)
		if err != nil {
			return nil, err
		}
	} else {
		allow = allowed.NewAllowedFromList([]string{"fred@example.org", "barney@example.org", "wilma@example.org"})
	}

	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(*local, pubsub.ScopePubSub, "https://www.googleapis.com/auth/datastore")
	if err != nil {
		return nil, err
	}

	if *namespace == "" {
		return nil, fmt.Errorf("The --namespace flag is required. See infra/DATASTORE.md for format details.\n")
	}
	if !*local && !util.In(*namespace, []string{ds.ALERT_MANAGER_NS}) {
		return nil, fmt.Errorf("When running in prod the datastore namespace must be a known value.")
	}
	if err := ds.InitWithOpt(*project, *namespace, option.WithTokenSource(ts)); err != nil {
		return nil, fmt.Errorf("Failed to init Cloud Datastore: %s", err)
	}

	client, err := pubsub.NewClient(ctx, *project, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	topic := client.Topic(alerts.TOPIC)
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	subName := fmt.Sprintf("%s-%s", alerts.TOPIC, hostname)
	sub := client.Subscription(subName)
	ok, err := sub.Exists(ctx)
	if err != nil {
		return nil, fmt.Errorf("Failed checking subscription existence: %s", err)
	}
	if !ok {
		sub, err = client.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
			Topic: topic,
		})
		if err != nil {
			return nil, fmt.Errorf("Failed creating subscription: %s", err)
		}
	}

	srv := &Server{
		salt:          salt,
		incidentStore: incident.NewStore(ds.DS, []string{"kubernetes_pod_name", "instance", "pod_template_hash"}),
		allow:         allow,
	}
	srv.loadTemplates()

	go func() {
		for {
			err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
				msg.Ack()
				var m map[string]string
				if err := json.Unmarshal(msg.Data, &m); err != nil {
					sklog.Error(err)
					return
				}
				if m[alerts.TYPE] == alerts.TYPE_HEALTHZ {
					sklog.Infof("healthz received")
				} else {
					_, err := srv.incidentStore.AlertArrival(m)
					if err != nil {
						sklog.Errorf("Error processing alert: %s", err)
					}
				}
			})
			if err != nil {
				sklog.Errorf("Failed receiving pubsub message: %s", err)
			}
		}
	}()

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

type AddNoteRequest struct {
	Text string `json:"text"`
	Key  string `json:"key"`
}

func (srv *Server) addNoteHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req AddNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, r, err, "Failed to decode add note request.")
		return
	}
	auditLog(r, "add-note", req)
	note := incident.Note{
		Text:   req.Text,
		TS:     time.Now().Unix(),
		Author: "fred@example.org",
	}
	in, err := srv.incidentStore.AddNote(req.Key, note)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to add note.")
		return
	}
	if err := json.NewEncoder(w).Encode(in); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

type DelNoteRequest struct {
	Index int    `json:"index"`
	Key   string `json:"key"`
}

func (srv *Server) delNoteHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req DelNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, r, err, "Failed to decode add note request.")
		return
	}
	auditLog(r, "del-note", req)
	in, err := srv.incidentStore.DeleteNote(req.Key, req.Index)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to add note.")
		return
	}
	if err := json.NewEncoder(w).Encode(in); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) incidentHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ins, err := srv.incidentStore.GetAll()
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to load incidents.")
		return
	}
	if err := json.NewEncoder(w).Encode(ins); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
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
	r.HandleFunc("/_/add_note", srv.addNoteHandler).Methods("POST")
	r.HandleFunc("/_/del_note", srv.delNoteHandler).Methods("POST")
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))))

	h := srv.applySecurityWrappers(r)

	if !*local {
		h = iap.New(h, *aud, srv.allow)
	}
	h = httputils.LoggingGzipRequestResponse(h)
	http.Handle("/", h)
	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
