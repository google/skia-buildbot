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
	"time"

	"cloud.google.com/go/datastore"
	"github.com/flynn/json5"
	"github.com/gorilla/mux"
	"go.skia.org/infra/autoroll/go/manual"
	"go.skia.org/infra/autoroll/go/modes"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/autoroll/go/rpc"
	"go.skia.org/infra/autoroll/go/status"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/autoroll/go/unthrottle"
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

	rollerConfigs map[string]*roller.AutoRollerConfig
)

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

func getRoller(w http.ResponseWriter, r *http.Request) *roller.AutoRollerConfig {
	name, ok := mux.Vars(r)["roller"]
	if !ok {
		http.Error(w, "Unable to find roller name in request path.", http.StatusBadRequest)
		return nil
	}
	roller, ok := rollerConfigs[name]
	if !ok {
		http.Error(w, "No such roller", http.StatusNotFound)
		return nil
	}
	return roller
}

func rollerHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	cfg := getRoller(w, r)
	if cfg == nil {
		return // Errors are handled by getRoller.
	}
	page := struct {
		ChildName  string
		ParentName string
		Roller     string
	}{
		ChildName:  cfg.ChildDisplayName,
		ParentName: cfg.ParentDisplayName,
		Roller:     cfg.RollerName,
	}
	if err := rollerTemplate.Execute(w, page); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
	}
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if err := mainTemplate.Execute(w, nil); err != nil {
		httputils.ReportError(w, errors.New("Failed to expand template."), fmt.Sprintf("Failed to expand template: %s", err), http.StatusInternalServerError)
	}
}

func runServer(ctx context.Context, serverURL string, srv http.Handler) {
	r := mux.NewRouter()
	r.HandleFunc("/", mainHandler)
	r.PathPrefix("/dist/").Handler(http.StripPrefix("/dist/", http.HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))))
	r.HandleFunc(login.DEFAULT_OAUTH2_CALLBACK, login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	rollerRouter := r.PathPrefix("/r/{roller}").Subrouter()
	rollerRouter.HandleFunc("", rollerHandler)
	r.PathPrefix(rpc.AutoRollServicePathPrefix).Handler(srv)
	h := httputils.LoggingRequestResponse(r)
	if !*local {
		if *internal {
			h = login.RestrictViewer(h)
			h = login.ForceAuth(h, login.DEFAULT_OAUTH2_CALLBACK)
		}
		h = httputils.HealthzAndHTTPS(h)
	}
	http.Handle("/", h)
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
	throttleDB := unthrottle.NewDatastore(ctx)

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
	rollerConfigs = make(map[string]*roller.AutoRollerConfig, len(cfgs))
	rollers := make(map[string]*rpc.AutoRoller, len(cfgs))
	for _, cfg := range cfgs {
		if err := cfg.Validate(); err != nil {
			sklog.Fatalf("Invalid roller config %q: %s", cfg.RollerName, err)
		}
		// Public frontend only displays public rollers, private-private.
		if *internal != cfg.IsInternal {
			sklog.Fatalf("Internal/external mismatch for %s", cfg.RollerName)
		}

		// Set up DBs for the roller.
		arbMode, err := modes.NewDatastoreModeHistory(ctx, cfg.RollerName)
		if err != nil {
			sklog.Fatal(err)
		}
		go util.RepeatCtx(ctx, 10*time.Second, func(ctx context.Context) {
			if err := arbMode.Update(ctx); err != nil {
				sklog.Error(err)
			}
		})
		arbStatusDB := status.NewDatastoreDB()
		arbStatus, err := status.NewCache(ctx, arbStatusDB, cfg.RollerName)
		if err != nil {
			sklog.Fatal(err)
		}
		go util.RepeatCtx(ctx, 10*time.Second, func(ctx context.Context) {
			if err := arbStatus.Update(ctx); err != nil {
				sklog.Error(err)
			}
		})
		arbStrategy, err := strategy.NewDatastoreStrategyHistory(ctx, cfg.RollerName, cfg.ValidStrategies())
		if err != nil {
			sklog.Fatal(err)
		}
		go util.RepeatCtx(ctx, 10*time.Second, func(ctx context.Context) {
			if err := arbStrategy.Update(ctx); err != nil {
				sklog.Error(err)
			}
		})
		rollers[cfg.RollerName] = &rpc.AutoRoller{
			Cfg:      cfg,
			Mode:     arbMode,
			Status:   arbStatus,
			Strategy: arbStrategy,
		}
		rollerConfigs[cfg.RollerName] = cfg
	}

	// TODO(borenet): Use CRIA groups instead of @google.com, ie. admins are
	// "google/skia-root@google.com", editors are specified in each roller's
	// config file, and viewers are either public or @google.com.
	var viewAllow allowed.Allow
	if *internal {
		viewAllow = allowed.UnionOf(allowed.NewAllowedFromList(allowedViewers), allowed.Googlers())
	}
	editAllow := allowed.Googlers()
	adminAllow := allowed.Googlers()
	srv := rpc.NewAutoRollServer(ctx, rollers, manualRollDB, throttleDB, viewAllow, editAllow, adminAllow)
	if err != nil {
		sklog.Fatal(err)
	}

	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}
	login.InitWithAllow(serverURL+login.DEFAULT_OAUTH2_CALLBACK, adminAllow, editAllow, viewAllow)

	// Create the server.
	runServer(ctx, serverURL, srv)
}
