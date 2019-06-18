/*
	Frontend server for interacting with the AutoRoller.
*/

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"text/template"
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
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"
)

var (
	// flags.
	configDir         = flag.String("config_dir", "", "Directory containing only configuration files for all rollers.")
	firestoreInstance = flag.String("firestore_instance", "", "Firestore instance to use, eg. \"production\"")
	host              = flag.String("host", "localhost", "HTTP service host")
	internal          = flag.Bool("internal", false, "If true, display the internal rollers.")
	local             = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port              = flag.String("port", ":8000", "HTTP service port (e.g., ':8000')")
	promPort          = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir      = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")

	WHITELISTED_VIEWERS = []string{
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
	rollerNames  []string               = nil
	rollers      map[string]*autoroller = nil
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
	ManualRequests      []*manual.ManualRollRequest `json:"manualRequests"`
	Mode                *modes.ModeChange           `json:"mode"`
	ParentWaterfall     string                      `json:"parentWaterfall"`
	Strategy            *strategy.StrategyChange    `json:"strategy"`
	SupportsManualRolls bool                        `json:"supportsManualRolls"`
}

type autoRollMiniStatus struct {
	*status.AutoRollMiniStatus
	ChildName  string `json:"childName,omitempty"`
	Mode       string `json:"mode"`
	ParentName string `json:"parentName,omitempty"`
}

func reloadTemplates() {
	// Change the current working directory to two directories up from this source file so that we
	// can read templates and serve static (res/) files.

	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}
	mainTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/main.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/navbar.html"),
	))
	rollerTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/roller.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/navbar.html"),
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
		httputils.ReportError(w, r, err, "Failed to decode request body.")
		return
	}

	if err := roller.Mode.Add(context.Background(), mode.Mode, login.LoggedInAs(r), mode.Message); err != nil {
		httputils.ReportError(w, r, err, "Failed to set AutoRoll mode.")
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
		httputils.ReportError(w, r, err, "Failed to decode request body.")
		return
	}

	if err := roller.Strategy.Add(context.Background(), strategy.Strategy, login.LoggedInAs(r), strategy.Message); err != nil {
		httputils.ReportError(w, r, err, "Failed to set AutoRoll strategy.")
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
			httputils.ReportError(w, r, err, "Failed to obtain manual roll requests.")
			return
		}
	} else {
		// NotRolledRevisions can take up a lot of space, and they aren't needed
		// if the roller doesn't support manual rolls.
		status.NotRolledRevisions = nil
	}

	// Encode response.
	if err := json.NewEncoder(w).Encode(&autoRollStatus{
		AutoRollStatus:      status,
		ManualRequests:      manualRequests,
		Mode:                mode,
		ParentWaterfall:     roller.Cfg.ParentWaterfall,
		Strategy:            strategy,
		SupportsManualRolls: roller.Cfg.SupportsManualRolls,
	}); err != nil {
		httputils.ReportError(w, r, err, "Failed to encode response.")
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
		httputils.ReportError(w, r, err, "Failed to obtain status.")
		return
	}
}

func unthrottleHandler(w http.ResponseWriter, r *http.Request) {
	roller := getRoller(w, r)
	if roller == nil {
		// Errors are handled by getRoller().
		return
	}

	if err := unthrottle.Unthrottle(context.Background(), roller.Cfg.RollerName); err != nil {
		httputils.ReportError(w, r, err, "Failed to unthrottle.")
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
		ChildName:  roller.Cfg.ChildName,
		ParentName: roller.Cfg.ParentName,
	}
	if err := rollerTemplate.Execute(w, page); err != nil {
		httputils.ReportError(w, r, err, "Failed to expand template.")
	}
}

func jsonAllHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	statuses := make(map[string]*autoRollMiniStatus, len(rollers))
	for name, roller := range rollers {
		status := roller.Status.GetMini()
		mode := roller.Mode.CurrentMode()
		statuses[name] = &autoRollMiniStatus{
			AutoRollMiniStatus: status,
			ChildName:          roller.Cfg.ChildName,
			Mode:               mode.Mode,
			ParentName:         roller.Cfg.ParentName,
		}
	}
	if err := json.NewEncoder(w).Encode(statuses); err != nil {
		httputils.ReportError(w, r, err, "Failed to obtain status.")
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
		httputils.ReportError(w, r, err, "Failed to decode request body.")
		return
	}
	req.Requester = login.LoggedInAs(r)
	req.RollerName = roller.Cfg.RollerName
	req.Status = manual.STATUS_PENDING
	req.Timestamp = firestore.FixTimestamp(time.Now())
	if err := manualRollDB.Put(&req); err != nil {
		httputils.ReportError(w, r, err, "Failed to insert manual roll request.")
		return
	}
	if err := json.NewEncoder(w).Encode(&req); err != nil {
		httputils.ReportError(w, r, err, "Failed to encode response.")
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
		Rollers []string
	}{
		Rollers: rollerNames,
	}
	if err := mainTemplate.Execute(w, page); err != nil {
		httputils.ReportError(w, r, errors.New("Failed to expand template."), fmt.Sprintf("Failed to expand template: %s", err))
	}
}

func runServer(ctx context.Context, serverURL string) {
	// TODO(borenet): Use CRIA groups instead of @google.com, ie. admins are
	// "google/skia-root@google.com", editors are specified in each roller's
	// config file, and viewers are either public or @google.com.
	var viewAllow allowed.Allow
	if *internal {
		viewAllow = allowed.UnionOf(allowed.NewAllowedFromList(WHITELISTED_VIEWERS), allowed.Googlers())
	}
	login.InitWithAllow(serverURL+login.DEFAULT_OAUTH2_CALLBACK, allowed.Googlers(), allowed.Googlers(), viewAllow)

	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))
	r.HandleFunc("/", mainHandler)
	r.HandleFunc("/json/all", jsonAllHandler)
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc(login.DEFAULT_OAUTH2_CALLBACK, login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)

	rollerRouter := r.PathPrefix("/r/{roller}").Subrouter()
	rollerRouter.HandleFunc("", rollerHandler)
	rollerRouter.HandleFunc("/json/ministatus", httputils.CorsHandler(miniStatusJsonHandler))
	rollerRouter.HandleFunc("/json/status", httputils.CorsHandler(statusJsonHandler))
	rollerRouter.Handle("/json/mode", login.RestrictEditorFn(modeJsonHandler)).Methods("POST")
	rollerRouter.Handle("/json/manual", login.RestrictEditorFn(newManualRollHandler)).Methods("POST")
	rollerRouter.Handle("/json/strategy", login.RestrictEditorFn(strategyJsonHandler)).Methods("POST")
	rollerRouter.Handle("/json/unthrottle", login.RestrictEditorFn(unthrottleHandler)).Methods("POST")
	sklog.AddLogsRedirect(r)
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
	skiaversion.MustLogVersion()

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

	// Read the config files for the rollers.
	if *configDir == "" {
		sklog.Fatal("--config_dir is required.")
	}
	dirEntries, err := ioutil.ReadDir(*configDir)
	if err != nil {
		sklog.Fatal(err)
	}
	rollerNames = make([]string, 0, len(dirEntries))
	rollers = make(map[string]*autoroller, len(dirEntries))
	for _, entry := range dirEntries {
		if !entry.IsDir() {
			// Load the config.
			var cfg roller.AutoRollerConfig
			if err := util.WithReadFile(path.Join(*configDir, entry.Name()), func(f io.Reader) error {
				return json5.NewDecoder(f).Decode(&cfg)
			}); err != nil {
				sklog.Fatal(err)
			}
			if err := cfg.Validate(); err != nil {
				sklog.Fatalf("Invalid roller config: %s %s", entry.Name(), err)
			}

			// Public frontend only displays public rollers, private-private.
			if *internal != cfg.IsInternal {
				continue
			}

			// Set up DBs for the roller.
			arbMode, err := modes.NewModeHistory(ctx, cfg.RollerName)
			if err != nil {
				sklog.Fatal(err)
			}
			go util.RepeatCtx(10*time.Second, ctx, func() {
				if err := arbMode.Update(ctx); err != nil {
					sklog.Error(err)
				}
			})
			arbStatus, err := status.NewCache(ctx, cfg.RollerName)
			if err != nil {
				sklog.Fatal(err)
			}
			go util.RepeatCtx(10*time.Second, ctx, func() {
				if err := arbStatus.Update(ctx); err != nil {
					sklog.Error(err)
				}
			})
			arbStrategy, err := strategy.NewStrategyHistory(ctx, cfg.RollerName, "", arbStatus.Get().ValidStrategies)
			if err != nil {
				sklog.Fatal(err)
			}
			go util.RepeatCtx(10*time.Second, ctx, func() {
				if err := arbStrategy.Update(ctx); err != nil {
					sklog.Error(err)
				}
			})
			rollerNames = append(rollerNames, cfg.RollerName)
			rollers[cfg.RollerName] = &autoroller{
				Cfg:      &cfg,
				Mode:     arbMode,
				Status:   arbStatus,
				Strategy: arbStrategy,
			}
		}
	}
	sort.Strings(rollerNames)

	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	runServer(ctx, serverURL)
}
