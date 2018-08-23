/*
	Frontend server for interacting with the AutoRoller.
*/

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"google.golang.org/api/option"

	"go.skia.org/infra/autoroll/go/google3"
	"go.skia.org/infra/autoroll/go/modes"
	"go.skia.org/infra/autoroll/go/status"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/autoroll/go/unthrottle"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/webhook"
)

var (
	// Interactions with the roller through the DB.
	arbMode     *modes.ModeHistory
	arbStatus   *status.AutoRollStatusCache
	arbStrategy *strategy.StrategyHistory

	mainTemplate *template.Template = nil

	// Name of the roller.
	rollerName string
)

// flags
var (
	host         = flag.String("host", "localhost", "HTTP service host")
	internal     = flag.Bool("internal", false, "If true, display the internal rollers.")
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port         = flag.String("port", ":8000", "HTTP service port (e.g., ':8000')")
	promPort     = flag.String("prom_port", ":20001", "Metrics service address (e.g., ':10110')")
	resourcesDir = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	workdir      = flag.String("workdir", ".", "Directory to use for scratch work.")
)

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
	))
}

func isGoogle3Roller() bool {
	return strings.Contains(rollerName, "google3")
}

func Init() {
	reloadTemplates()
}

func modeJsonHandler(w http.ResponseWriter, r *http.Request) {
	if !login.IsGoogler(r) {
		httputils.ReportError(w, r, fmt.Errorf("User does not have edit rights: %q", login.LoggedInAs(r)), "You must be logged in with an @google.com account to do that.")
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

	if err := arbMode.Add(context.Background(), mode.Mode, login.LoggedInAs(r), mode.Message); err != nil {
		httputils.ReportError(w, r, err, "Failed to set AutoRoll mode.")
		return
	}

	// Return the ARB status.
	statusJsonHandler(w, r)
}

func strategyJsonHandler(w http.ResponseWriter, r *http.Request) {
	if !login.IsGoogler(r) {
		httputils.ReportError(w, r, fmt.Errorf("User does not have edit rights: %q", login.LoggedInAs(r)), "You must be logged in with an @google.com account to do that.")
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

	if err := arbStrategy.Add(context.Background(), strategy.Strategy, login.LoggedInAs(r), strategy.Message); err != nil {
		httputils.ReportError(w, r, err, "Failed to set AutoRoll strategy.")
		return
	}

	// Return the ARB status.
	statusJsonHandler(w, r)
}

func statusJsonHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// Obtain the status info. Only display potentially sensitive info if the user is a logged-in
	// Googler.
	status := arbStatus.Get()
	if !login.IsGoogler(r) {
		status.Error = ""
		if isGoogle3Roller() {
			cleanIssue := func(issue *autoroll.AutoRollIssue) {
				// Clearing Issue and Subject out of an abundance of caution.
				issue.Issue = 0
				issue.Subject = ""
				issue.TryResults = nil
			}
			for _, issue := range status.Recent {
				cleanIssue(issue)
			}
			if status.CurrentRoll != nil {
				cleanIssue(status.CurrentRoll)
			}
			if status.LastRoll != nil {
				cleanIssue(status.LastRoll)
			}
		}
	}
	// Overwrite the mode and strategy in the status object in case they
	// have been updated on the front end but the roller has not cycled to
	// reflect the change in the status object.
	// TODO(borenet): We should really just remove modes and strategies from
	// the status package and just merge them in here.
	mode := arbMode.CurrentMode()
	status.Mode = mode
	strategy := arbStrategy.CurrentStrategy()
	status.Strategy = strategy

	// Encode response.
	if err := json.NewEncoder(w).Encode(status); err != nil {
		httputils.ReportError(w, r, err, "Failed to obtain status.")
		return
	}
}

func miniStatusJsonHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	status := arbStatus.GetMini()
	if err := json.NewEncoder(w).Encode(&status); err != nil {
		httputils.ReportError(w, r, err, "Failed to obtain status.")
		return
	}
}

func unthrottleHandler(w http.ResponseWriter, r *http.Request) {
	if !login.IsGoogler(r) {
		httputils.ReportError(w, r, fmt.Errorf("User does not have edit rights: %q", login.LoggedInAs(r)), "You must be logged in with an @google.com account to do that.")
		return
	}
	if err := unthrottle.Unthrottle(context.Background(), rollerName); err != nil {
		httputils.ReportError(w, r, err, "Failed to unthrottle.")
		return
	}
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *local {
		reloadTemplates()
	}
	mainPage := struct {
		ProjectName string
	}{
		ProjectName: arbStatus.Get().ChildName,
	}
	if err := mainTemplate.Execute(w, mainPage); err != nil {
		sklog.Errorln("Failed to expand template:", err)
	}
}

func runServer(ctx context.Context, serverURL string) {
	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))
	r.HandleFunc("/", mainHandler)
	r.HandleFunc("/json/mode", modeJsonHandler).Methods("POST")
	r.HandleFunc("/json/ministatus", httputils.CorsHandler(miniStatusJsonHandler))
	r.HandleFunc("/json/status", httputils.CorsHandler(statusJsonHandler))
	r.HandleFunc("/json/strategy", strategyJsonHandler).Methods("POST")
	r.HandleFunc("/json/unthrottle", unthrottleHandler).Methods("POST")
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	sklog.AddLogsRedirect(r)

	// Add handlers for Google3 roller.
	if isGoogle3Roller() {
		if err := webhook.InitRequestSaltFromMetadata(metadata.WEBHOOK_REQUEST_SALT); err != nil {
			sklog.Fatal(err)
		}
		arb, err := google3.NewAutoRoller(ctx, *workdir, common.REPO_SKIA, "master", rollerName)
		if err != nil {
			sklog.Fatal(err)
		}
		arb.AddHandlers(r)
		arb.Start(ctx, time.Minute, time.Minute)
	}

	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

func main() {
	common.InitWithMust(
		"autoroll-fe",
		common.PrometheusOpt(promPort),
		common.CloudLoggingOpt(),
	)
	defer common.Defer()

	Init()
	skiaversion.MustLogVersion()

	ts, err := auth.NewDefaultTokenSource(*local)
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

	hostname, err := os.Hostname()
	if err != nil {
		sklog.Fatalf("Could not get hostname: %s", err)
	}
	rollerName = hostname
	if *local {
		rollerName = fmt.Sprintf("autoroll_%s", hostname)
	}

	ctx := context.Background()
	arbMode, err = modes.NewModeHistory(ctx, rollerName)
	if err != nil {
		sklog.Fatal(err)
	}
	go util.RepeatCtx(10*time.Second, ctx, func() {
		if err := arbMode.Update(ctx); err != nil {
			sklog.Error(err)
		}
	})
	arbStatus, err = status.NewCache(ctx, rollerName)
	if err != nil {
		sklog.Fatal(err)
	}
	go util.RepeatCtx(10*time.Second, ctx, func() {
		if err := arbStatus.Update(ctx); err != nil {
			sklog.Error(err)
		}
	})
	arbStrategy, err = strategy.NewStrategyHistory(ctx, rollerName, "", nil)
	if err != nil {
		sklog.Fatal(err)
	}
	go util.RepeatCtx(10*time.Second, ctx, func() {
		if err := arbStrategy.Update(ctx); err != nil {
			sklog.Error(err)
		}
	})

	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}
	login.SimpleInitMust(*port, *local)

	runServer(ctx, serverURL)
}
