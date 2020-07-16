/*
	Frontend server for interacting with the AutoRoller.
*/

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"cloud.google.com/go/datastore"
	"github.com/flynn/json5"
	"github.com/gorilla/mux"
	"go.skia.org/infra/autoroll/go/manual"
	"go.skia.org/infra/autoroll/go/modes"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/autoroll/go/rpc"
	"go.skia.org/infra/autoroll/go/status"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"
)

var (
	// flags.
	configs           = common.NewMultiStringFlag("config", nil, "Base 64 encoded config in JSON format. Supply this flag once for each roller. Mutually exclusive with --config_file.")
	configFiles       = common.NewMultiStringFlag("config_file", nil, "Path to autoroller config file. Supply this flag once for each roller. Mutually exclusive with --config.")
	firestoreInstance = flag.String("firestore_instance", "", "Firestore instance to use, eg. \"production\"")
	host              = flag.String("host", "localhost", "HTTP service host")
	internal          = flag.Bool("internal", false, "If true, display the internal rollers.")
	local             = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port              = flag.String("port", ":8000", "HTTP service port (e.g., ':8000')")
	promPort          = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir      = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	hang              = flag.Bool("hang", false, "If true, don't spin up the server, just hang without doing anything.")

	allowedViewers = []string{
		"prober@skia-public.iam.gserviceaccount.com",
		"skia-status@skia-public.iam.gserviceaccount.com",
		"skia-status-internal@skia-corp.google.com.iam.gserviceaccount.com",
		"status@skia-buildbots.google.com.iam.gserviceaccount.com",
		"status-internal@skia-buildbots.google.com.iam.gserviceaccount.com",
		"showy-dashboards@prod.google.com",
	}

	mainTemplate   *template.Template = nil
	rollerTemplate *template.Template = nil
)

// Struct used for organizing information about a roller.
type autoroller struct {
	Cfg *roller.AutoRollerConfig

	// Interactions with the roller through the DB.
	Mode     *modes.ModeHistory
	Status   *status.AutoRollStatusCache
	Strategy *strategy.StrategyHistory
}

// Union types for combining roller status with modes and strategies.
type autoRollStatus struct {
	*status.AutoRollStatus
	Config         *roller.AutoRollerConfig    `json:"config"`
	ManualRequests []*manual.ManualRollRequest `json:"manualRequests"`
	Mode           *modes.ModeChange           `json:"mode"`
	Strategy       *strategy.StrategyChange    `json:"strategy"`
}

type autoRollMiniStatus struct {
	*status.AutoRollMiniStatus
	ChildName  string `json:"childName,omitempty"`
	Mode       string `json:"mode"`
	ParentName string `json:"parentName,omitempty"`
}

func reloadTemplates() {
	if *resourcesDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			sklog.Fatal(err)
		}
		*resourcesDir = filepath.Join(wd, "dist")
	}
	sklog.Infof("Reading resources from %s", *resourcesDir)
	mainTemplate = template.Must(template.New("index.html").Funcs(map[string]interface{}{
		"marshal": func(data interface{}) template.JS {
			b, _ := json.Marshal(data)
			return template.JS(b)
		},
	}).ParseFiles(
		filepath.Join(*resourcesDir, "index.html"),
	))
	rollerTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "roller.html"),
	))
}

func rollerHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *local {
		reloadTemplates()
	}
	page := struct{}{}
	if err := rollerTemplate.Execute(w, page); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
	}
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *local {
		reloadTemplates()
	}
	page := struct{}{}
	if err := mainTemplate.Execute(w, page); err != nil {
		httputils.ReportError(w, errors.New("Failed to expand template."), fmt.Sprintf("Failed to expand template: %s", err), http.StatusInternalServerError)
	}
}

func runServer(ctx context.Context, serverURL string, rpcs http.Handler) {
	//login.InitWithAllow(serverURL+login.DEFAULT_OAUTH2_CALLBACK, allowed.Googlers(), allowed.Googlers(), viewAllow)

	r := mux.NewRouter()
	r.HandleFunc("/", mainHandler)
	r.PathPrefix("/dist/").Handler(http.StripPrefix("/dist/", http.HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))))
	r.HandleFunc(login.DEFAULT_OAUTH2_CALLBACK, login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	rollerRouter := r.PathPrefix("/r/{roller}").Subrouter()
	rollerRouter.HandleFunc("", rollerHandler)
	r.Handle(rpc.AutoRollRPCsPathPrefix, rpcs)
	/*h := httputils.LoggingGzipRequestResponse(r)
	if !*local {
		if viewAllow != nil {
			h = login.RestrictViewer(h)
			h = login.ForceAuth(h, login.DEFAULT_OAUTH2_CALLBACK)
		}
		h = httputils.HealthzAndHTTPS(h)
	}*/
	http.Handle("/", r)
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

func main() {
	common.InitWithMust(
		"autoroll-fe",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	defer common.Defer()

	reloadTemplates()

	if *hang {
		select {}
	}

	ts, err := auth.NewDefaultTokenSource(*local, auth.SCOPE_USERINFO_EMAIL, datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatal(err)
	}
	namespace := ds.AUTOROLL_NS
	if *internal {
		namespace = ds.AUTOROLL_INTERNAL_NS
	}
	if err := ds.InitWithOpt(common.PROJECT_ID, namespace, option.WithTokenSource(ts)); err != nil {
		sklog.Fatal(err)
	}

	ctx := context.Background()

	manualRollDB, err := manual.NewDBWithParams(ctx, firestore.FIRESTORE_PROJECT, *firestoreInstance, ts)
	if err != nil {
		sklog.Fatal(err)
	}

	// Read the configs for the rollers.
	if len(*configs) > 0 && len(*configFiles) > 0 {
		sklog.Fatal("--config and --config_file are mutually exclusive.")
	} else if len(*configs) == 0 && len(*configFiles) == 0 {
		sklog.Fatal("At least one instance of --config or --config_file is required.")
	}
	cfgs := make([]*roller.AutoRollerConfig, 0, len(*configs)+len(*configFiles))
	for _, cfgStr := range *configs {
		b, err := base64.StdEncoding.DecodeString(cfgStr)
		if err != nil {
			sklog.Fatal(err)
		}
		var cfg roller.AutoRollerConfig
		if err := json5.NewDecoder(bytes.NewReader(b)).Decode(&cfg); err != nil {
			sklog.Fatal(err)
		}
		cfgs = append(cfgs, &cfg)
	}
	for _, path := range *configFiles {
		var cfg roller.AutoRollerConfig
		if err := util.WithReadFile(path, func(f io.Reader) error {
			return json5.NewDecoder(f).Decode(&cfg)
		}); err != nil {
			sklog.Fatal(err)
		}
		cfgs = append(cfgs, &cfg)
	}

	// Validate the configs.
	for _, cfg := range cfgs {
		if err := cfg.Validate(); err != nil {
			sklog.Fatalf("Invalid roller config %q: %s", cfg.RollerName, err)
		}

		// Public frontend only displays public rollers, private-private.
		if *internal != cfg.IsInternal {
			sklog.Fatalf("Internal/external mismatch for %s", cfg.RollerName)
		}
	}

	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	// Create the server.
	// TODO(borenet): Use CRIA groups instead of @google.com, ie. admins are
	// "google/skia-root@google.com", editors are specified in each roller's
	// config file, and viewers are either public or @google.com.
	viewAllow := allowed.Allow(allowed.Anyone())
	if *internal {
		viewAllow = allowed.UnionOf(allowed.NewAllowedFromList(allowedViewers), allowed.Googlers())
	}
	editAllow := allowed.Googlers()
	adminAllow := allowed.Googlers()
	rpcs, err := rpc.NewAutoRollServer(ctx, cfgs, manualRollDB, viewAllow, editAllow, adminAllow)
	if err != nil {
		sklog.Fatal(err)
	}

	runServer(ctx, serverURL, rpcs)
}
