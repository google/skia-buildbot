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
	"sort"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/flynn/json5"
	"github.com/gorilla/mux"
	"go.skia.org/infra/autoroll/go/manual"
	"go.skia.org/infra/autoroll/go/modes"
	"go.skia.org/infra/autoroll/go/roller"
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

	manualRollDB manual.DB              = nil
	throttleDB   unthrottle.Throttle    = nil
	rollerNames  []string               = nil
	rollers      map[string]*autoroller = nil
)

// Struct used for organizing information about a roller.
type autoroller struct {
	Cfg *roller.AutoRollerConfig

	// Interactions with the roller through the DB.
	Mode     modes.ModeHistory
	Status   *status.Cache
	Strategy strategy.StrategyHistory
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

func Init() {
	reloadTemplates()
}

func getRoller(w http.ResponseWriter, r *http.Request) *autoroller {
	name, ok := mux.Vars(r)["roller"]
	if !ok {
		http.Error(w, "Unable to find roller name in request path.", http.StatusBadRequest)
		return nil
	}
	roller, ok := rollers[name]
	if !ok {
		http.Error(w, "No such roller", http.StatusNotFound)
		return nil
	}
	return roller
}

func modeJsonHandler(w http.ResponseWriter, r *http.Request) {
	roller := getRoller(w, r)
	if roller == nil {
		// Errors are handled by getRoller().
		return
	}

	var mode struct {
		Message string `json:"message"`
		Mode    string `json:"mode"`
	}
	defer util.Close(r.Body)
	if err := json.NewDecoder(r.Body).Decode(&mode); err != nil {
		httputils.ReportError(w, err, "Failed to decode request body.", http.StatusInternalServerError)
		return
	}

	if err := roller.Mode.Add(context.Background(), mode.Mode, login.LoggedInAs(r), mode.Message); err != nil {
		httputils.ReportError(w, err, "Failed to set AutoRoll mode.", http.StatusInternalServerError)
		return
	}

	// Return the ARB status.
	statusJsonHandler(w, r)
}

func strategyJsonHandler(w http.ResponseWriter, r *http.Request) {
	roller := getRoller(w, r)
	if roller == nil {
		// Errors are handled by getRoller().
		return
	}

	var strategy struct {
		Message  string `json:"message"`
		Strategy string `json:"strategy"`
	}
	defer util.Close(r.Body)
	if err := json.NewDecoder(r.Body).Decode(&strategy); err != nil {
		httputils.ReportError(w, err, "Failed to decode request body.", http.StatusInternalServerError)
		return
	}

	if err := roller.Strategy.Add(context.Background(), strategy.Strategy, login.LoggedInAs(r), strategy.Message); err != nil {
		httputils.ReportError(w, err, "Failed to set AutoRoll strategy.", http.StatusInternalServerError)
		return
	}

	// Return the ARB status.
	statusJsonHandler(w, r)
}

func statusJsonHandler(w http.ResponseWriter, r *http.Request) {
	roller := getRoller(w, r)
	if roller == nil {
		// Errors are handled by getRoller().
		return
	}

	w.Header().Set("Content-Type", "application/json")
	// Obtain the status info. Only display potentially sensitive info if the user is a logged-in
	// Googler.
	status := roller.Status.Get()
	if !login.IsAdmin(r) {
		status.Error = ""
	}
	mode := roller.Mode.CurrentMode()
	strategy := roller.Strategy.CurrentStrategy()

	// Obtain manual roll requests, if supported by the roller.
	var manualRequests []*manual.ManualRollRequest
	if roller.Cfg.SupportsManualRolls {
		var err error
		manualRequests, err = manualRollDB.GetRecent(roller.Cfg.RollerName, len(status.NotRolledRevisions))
		if err != nil {
			httputils.ReportError(w, err, "Failed to obtain manual roll requests.", http.StatusInternalServerError)
			return
		}
	} else {
		// NotRolledRevisions can take up a lot of space, and they aren't needed
		// if the roller doesn't support manual rolls.
		status.NotRolledRevisions = nil
	}

	// Encode response.
	if err := json.NewEncoder(w).Encode(&autoRollStatus{
		AutoRollStatus: status,
		Config:         roller.Cfg,
		ManualRequests: manualRequests,
		Mode:           mode,
		Strategy:       strategy,
	}); err != nil {
		httputils.ReportError(w, err, "Failed to encode response.", http.StatusInternalServerError)
		return
	}
}

func miniStatusJsonHandler(w http.ResponseWriter, r *http.Request) {
	roller := getRoller(w, r)
	if roller == nil {
		// Errors are handled by getRoller().
		return
	}

	w.Header().Set("Content-Type", "application/json")
	status := roller.Status.GetMini()
	mode := roller.Mode.CurrentMode()
	if err := json.NewEncoder(w).Encode(&autoRollMiniStatus{
		AutoRollMiniStatus: status,
		Mode:               mode.Mode,
	}); err != nil {
		httputils.ReportError(w, err, "Failed to obtain status.", http.StatusInternalServerError)
		return
	}
}

func unthrottleHandler(w http.ResponseWriter, r *http.Request) {
	roller := getRoller(w, r)
	if roller == nil {
		// Errors are handled by getRoller().
		return
	}

	if err := throttleDB.Unthrottle(context.Background(), roller.Cfg.RollerName); err != nil {
		httputils.ReportError(w, err, "Failed to unthrottle.", http.StatusInternalServerError)
		return
	}
}

func rollerHandler(w http.ResponseWriter, r *http.Request) {
	roller := getRoller(w, r)
	if roller == nil {
		// Errors are handled by getRoller().
		return
	}

	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *local {
		reloadTemplates()
	}
	page := struct {
		ChildName  string
		ParentName string
	}{
		ChildName:  roller.Cfg.ChildDisplayName,
		ParentName: roller.Cfg.ParentDisplayName,
	}
	if err := rollerTemplate.Execute(w, page); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
	}
}

func getAllMiniStatuses() map[string]*autoRollMiniStatus {
	statuses := make(map[string]*autoRollMiniStatus, len(rollers))
	for name, roller := range rollers {
		status := roller.Status.GetMini()
		mode := roller.Mode.CurrentMode()
		modeStr := ""
		if mode != nil {
			modeStr = mode.Mode
		}
		statuses[name] = &autoRollMiniStatus{
			AutoRollMiniStatus: status,
			ChildName:          roller.Cfg.ChildDisplayName,
			Mode:               modeStr,
			ParentName:         roller.Cfg.ParentDisplayName,
		}
	}
	return statuses
}

func jsonAllHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	statuses := getAllMiniStatuses()
	if err := json.NewEncoder(w).Encode(statuses); err != nil {
		httputils.ReportError(w, err, "Failed to obtain status.", http.StatusInternalServerError)
		return
	}
}

func newManualRollHandler(w http.ResponseWriter, r *http.Request) {
	roller := getRoller(w, r)
	if roller == nil {
		// Errors are handled by getRoller().
		return
	}
	var req manual.ManualRollRequest
	defer util.Close(r.Body)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode request body.", http.StatusInternalServerError)
		return
	}
	req.Requester = login.LoggedInAs(r)
	req.RollerName = roller.Cfg.RollerName
	req.Status = manual.STATUS_PENDING
	req.Timestamp = firestore.FixTimestamp(time.Now())
	if err := manualRollDB.Put(&req); err != nil {
		httputils.ReportError(w, err, "Failed to insert manual roll request.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to encode response.", http.StatusInternalServerError)
		return
	}
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *local {
		reloadTemplates()
	}
	page := struct {
		Rollers map[string]*autoRollMiniStatus
	}{
		Rollers: getAllMiniStatuses(),
	}
	if err := mainTemplate.Execute(w, page); err != nil {
		httputils.ReportError(w, errors.New("Failed to expand template."), fmt.Sprintf("Failed to expand template: %s", err), http.StatusInternalServerError)
	}
}

func runServer(ctx context.Context, serverURL string) {
	// TODO(borenet): Use CRIA groups instead of @google.com, ie. admins are
	// "google/skia-root@google.com", editors are specified in each roller's
	// config file, and viewers are either public or @google.com.
	var viewAllow allowed.Allow
	if *internal {
		viewAllow = allowed.UnionOf(allowed.NewAllowedFromList(allowedViewers), allowed.Googlers())
	}
	login.InitWithAllow(serverURL+login.DEFAULT_OAUTH2_CALLBACK, allowed.Googlers(), allowed.Googlers(), viewAllow)

	r := mux.NewRouter()
	r.HandleFunc("/", httputils.OriginTrial(mainHandler, *local))
	r.PathPrefix("/dist/").Handler(http.StripPrefix("/dist/", http.HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))))
	r.HandleFunc("/json/all", jsonAllHandler)
	r.HandleFunc(login.DEFAULT_OAUTH2_CALLBACK, login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)

	rollerRouter := r.PathPrefix("/r/{roller}").Subrouter()
	rollerRouter.HandleFunc("", httputils.OriginTrial(rollerHandler, *local))
	rollerRouter.HandleFunc("/json/ministatus", httputils.CorsHandler(miniStatusJsonHandler))
	rollerRouter.HandleFunc("/json/status", httputils.CorsHandler(statusJsonHandler))
	rollerRouter.Handle("/json/mode", login.RestrictEditorFn(modeJsonHandler)).Methods("POST")
	rollerRouter.Handle("/json/manual", login.RestrictEditorFn(newManualRollHandler)).Methods("POST")
	rollerRouter.Handle("/json/strategy", login.RestrictEditorFn(strategyJsonHandler)).Methods("POST")
	rollerRouter.Handle("/json/unthrottle", login.RestrictEditorFn(unthrottleHandler)).Methods("POST")
	h := httputils.LoggingGzipRequestResponse(r)
	if !*local {
		if viewAllow != nil {
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

	Init()

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

	manualRollDB, err = manual.NewDBWithParams(ctx, firestore.FIRESTORE_PROJECT, *firestoreInstance, ts)
	if err != nil {
		sklog.Fatal(err)
	}
	throttleDB = unthrottle.NewDatastore(ctx)

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

	// Process the configs.
	rollerNames = []string{}
	rollers = map[string]*autoroller{}
	for _, cfg := range cfgs {
		if err := cfg.Validate(); err != nil {
			sklog.Fatalf("Invalid roller config %q: %s", cfg.RollerName, err)
		}

		// Public frontend only displays public rollers, private-private.
		if *internal != cfg.IsInternal {
			continue
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
		rollerNames = append(rollerNames, cfg.RollerName)
		rollers[cfg.RollerName] = &autoroller{
			Cfg:      cfg,
			Mode:     arbMode,
			Status:   arbStatus,
			Strategy: arbStrategy,
		}
	}
	sort.Strings(rollerNames)

	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	runServer(ctx, serverURL)
}
