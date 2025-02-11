// Package frontend contains the Go code that servers the Perf web UI.
package frontend

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/unrolled/secure"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/alogin/proxylogin"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/calc"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sklog/sklogimpl"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/anomalies"
	"go.skia.org/infra/perf/go/anomalies/cache"
	"go.skia.org/infra/perf/go/builders"
	"go.skia.org/infra/perf/go/chromeperf"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/config/validate"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/dfbuilder"
	"go.skia.org/infra/perf/go/dryrun"
	"go.skia.org/infra/perf/go/favorites"
	"go.skia.org/infra/perf/go/frontend/api"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/graphsshortcut"
	"go.skia.org/infra/perf/go/notify"
	"go.skia.org/infra/perf/go/notifytypes"
	"go.skia.org/infra/perf/go/pinpoint"
	"go.skia.org/infra/perf/go/progress"
	"go.skia.org/infra/perf/go/psrefresh"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/regression/continuous"
	"go.skia.org/infra/perf/go/shortcut"
	"go.skia.org/infra/perf/go/subscription"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/tracing"
	"go.skia.org/infra/perf/go/trybot/results"
	"go.skia.org/infra/perf/go/trybot/results/dfloader"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/urlprovider"
	"go.skia.org/infra/perf/go/userissue"
	pp_service "go.skia.org/infra/pinpoint/go/service"
)

const (
	// regressionCountDuration is how far back we look for regression in the /_/reg/count endpoint.
	regressionCountDuration = -14 * 24 * time.Hour

	// paramsetRefresherPeriod is how often we refresh our canonical paramset from the OPS's
	// stored in the last two tiles.
	paramsetRefresherPeriod = 1 * time.Hour

	// startClusterDelay is the time we wait between starting each clusterer, to avoid hammering
	// the trace store all at once.
	startClusterDelay = 2 * time.Second

	// longRunningRequestTimeout is a limit on long running processes.
	longRunningRequestTimeout = 20 * time.Minute

	// How often to update the git repo from origin.
	gitRepoUpdatePeriod = time.Minute

	// defaultDatabaseTimeout is the context timeout used when the frontend is
	// making a request that involves the database. For more complex requests
	// use config.QueryMaxRuntime.
	defaultDatabaseTimeout = time.Minute

	// livenessTimeout is the context timeout used when checking the health
	// status of the frontend to cockroachDB. If the health check fails,
	// then the pod will restart. Queries to the CDB regressions table takes
	// < 1 second.
	livenessTimeout = 10 * time.Second
)

var (
	// googleAnalyticsSnippet is rendered into page html templates for configs
	// that specfy a value for [config.Config.GoogleAnalyticsMeasurementID], aka
	// 'ga_measurement_id' in the config's json file.
	//go:embed googleanalytics.html
	googleAnalyticsSnippet string

	// cookieConsentSnippet adds a cookie consent banner that gets rendered into
	// the perf-scaffold-sk element's footer if it is present, or at the bottom
	// of the body element otherwise.
	//go:embed cookieconsent.html
	cookieConsentSnippet string
)

// Frontend is the server for the Perf web UI.
type Frontend struct {
	perfGit perfgit.Git

	templates *template.Template

	loadTemplatesOnce sync.Once

	regStore regression.Store

	subStore subscription.Store

	favStore favorites.Store

	continuous []*continuous.Continuous

	// provides access to the ingested files.
	ingestedFS fs.FS

	alertStore alerts.Store

	shortcutStore shortcut.Store

	configProvider alerts.ConfigProvider

	graphsShortcutStore graphsshortcut.Store

	notifier notify.Notifier

	traceStore tracestore.TraceStore

	userIssueStore userissue.Store

	dryrunRequests *dryrun.Requests

	paramsetRefresher psrefresh.ParamSetRefresher

	dfBuilder dataframe.DataFrameBuilder

	trybotResultsLoader results.Loader

	// distFileSystem is the ./dist directory of files produced by Bazel.
	distFileSystem http.FileSystem

	flags *config.FrontendFlags

	// progressTracker tracks long running web requests.
	progressTracker progress.Tracker

	loginProvider alogin.Login

	// The HOST parsed out of Config.URL.
	host string

	anomalyStore anomalies.Store

	pinpoint *pinpoint.Client

	alertGroupClient chromeperf.AlertGroupApiClient

	anomalyApiClient chromeperf.AnomalyApiClient

	chromeperfClient chromeperf.ChromePerfClient

	urlProvider *urlprovider.URLProvider
}

// New returns a new Frontend instance.
func New(flags *config.FrontendFlags) (*Frontend, error) {
	f := &Frontend{
		flags: flags,
	}
	f.initialize()

	return f, nil
}

func fileContentsFromFileSystem(fileSystem http.FileSystem, filename string) (string, error) {
	f, err := fileSystem.Open(filename)
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to open %q", filename)
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to read %q", filename)
	}
	if err := f.Close(); err != nil {
		return "", skerr.Wrapf(err, "Failed to close %q", filename)
	}
	return string(b), nil
}

var templateFilenames = []string{
	"newindex.html",
	"multiexplore.html",
	"clusters2.html",
	"triage.html",
	"alerts.html",
	"help.html",
	"dryrunalert.html",
	"trybot.html",
	"favorites.html",
	"revisions.html",
	"regressions.html",
	"report.html",
}

func (f *Frontend) loadTemplatesImpl() {
	f.templates = template.New("")
	for _, filename := range templateFilenames {
		contents, err := fileContentsFromFileSystem(f.distFileSystem, filename)
		if err != nil {
			sklog.Fatal(err)
		}
		f.templates = f.templates.New(filename).Delims("{%", "%}").Option("missingkey=error")
		_, err = f.templates.Parse(contents)
		if err != nil {
			sklog.Fatal(err)
		}
	}
	for name, snippet := range map[string]string{"googleanalytics": googleAnalyticsSnippet, "cookieconsent": cookieConsentSnippet} {
		f.templates = f.templates.New(name).Delims("{%", "%}").Option("missingkey=error")
		_, err := f.templates.Parse(snippet)
		if err != nil {
			sklog.Fatal(err)
		}
	}
}

func (f *Frontend) loadTemplates() {
	if f.flags.Local {
		f.loadTemplatesImpl()
		return
	}
	f.loadTemplatesOnce.Do(f.loadTemplatesImpl)
}

// SkPerfConfig is the configuration data that will appear
// in Javascript under the window.perf variable.
type SkPerfConfig struct {
	Radius                     int                `json:"radius"`                          // The number of commits when doing clustering.
	KeyOrder                   []string           `json:"key_order"`                       // The order of the keys to appear first in query-sk elements.
	NumShift                   int                `json:"num_shift"`                       // The number of commits the shift navigation buttons should jump.
	Interesting                float32            `json:"interesting"`                     // The threshold for a cluster to be interesting.
	StepUpOnly                 bool               `json:"step_up_only"`                    // If true then only regressions that are a step up are displayed.
	CommitRangeURL             string             `json:"commit_range_url"`                // A URI Template to be used for expanding details on a range of commits. See cluster-summary2-sk.
	Demo                       bool               `json:"demo"`                            // True if this is a demo page, as opposed to being in production. Used to make puppeteer tests deterministic.
	DisplayGroupBy             bool               `json:"display_group_by"`                // True if the Group By section of Alert config should be displayed.
	HideListOfCommitsOnExplore bool               `json:"hide_list_of_commits_on_explore"` // True if the commit-detail-panel-sk element on the Explore details tab should be hidden.
	Notifications              notifytypes.Type   `json:"notifications"`                   // The type of notifications that can be sent.
	FetchChromePerfAnomalies   bool               `json:"fetch_chrome_perf_anomalies"`     // If true explore-sk will show the bisect button
	FeedbackURL                string             `json:"feedback_url"`                    // The URL for the Provide Feedback link
	ChatURL                    string             `json:"chat_url"`                        // The URL for the Ask the Team link
	HelpURLOverride            string             `json:"help_url_override"`               // If specified, this URL will override the help link
	TraceFormat                config.TraceFormat `json:"trace_format"`                    // Trace formatter to use
	NeedAlertAction            bool               `json:"need_alert_action"`               // Action to take for the alert.
	BugHostURL                 string             `json:"bug_host_url"`                    // The URL for the bug host for the instance.
	GitRepoUrl                 string             `json:"git_repo_url"`                    // The URL for the associated git repo.
	KeysForCommitRange         []string           `json:"keys_for_commit_range"`           // The link keys for commit range url display of individual points.
	SkipCommitDetailDisplay    bool               `json:"skip_commit_detail_display"`      // Do not display commit detail
	ImageTag                   string             `json:"image_tag"`                       // The image tag that the running instance is built from, typically a git commit hash.
}

// getPageContext returns the value of `window.perf` serialized as JSON.
//
// These are values that the JS running in the browser needs to operate and
// should be present on every page. Returned as template.JS so that the template
// expansion correctly renders this as executable JS.
func (f *Frontend) getPageContext() (template.JS, error) {
	pc := SkPerfConfig{
		Radius:                     f.flags.Radius,
		KeyOrder:                   strings.Split(f.flags.KeyOrder, ","),
		NumShift:                   f.flags.NumShift,
		Interesting:                float32(f.flags.Interesting),
		StepUpOnly:                 f.flags.StepUpOnly,
		CommitRangeURL:             f.flags.CommitRangeURL,
		Demo:                       false,
		DisplayGroupBy:             f.flags.DisplayGroupBy,
		HideListOfCommitsOnExplore: f.flags.HideListOfCommitsOnExplore,
		Notifications:              config.Config.NotifyConfig.Notifications,
		FetchChromePerfAnomalies:   config.Config.FetchChromePerfAnomalies,
		FeedbackURL:                config.Config.FeedbackURL,
		ChatURL:                    config.Config.ChatURL,
		HelpURLOverride:            config.Config.HelpURLOverride,
		TraceFormat:                config.Config.TraceFormat,
		NeedAlertAction:            config.Config.NeedAlertAction,
		BugHostURL:                 config.Config.BugHostUrl,
		GitRepoUrl:                 config.Config.GitRepoConfig.URL,
		KeysForCommitRange:         config.Config.DataPointConfig.KeysForCommitRange,
		SkipCommitDetailDisplay:    config.Config.DataPointConfig.SkipCommitDetailDisplay,
		ImageTag:                   os.Getenv("IMAGE_TAG"),
	}
	b, err := json.MarshalIndent(pc, "", "  ")
	if err != nil {
		sklog.Errorf("Failed to JSON encode window.perf context: %s", err)
	}

	return template.JS(string(b)), nil
}

func (f *Frontend) templateHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		f.loadTemplates()
		context, err := f.getPageContext()
		if err != nil {
			sklog.Errorf("Failed to JSON encode window.perf context: %s", err)
		}
		if err := f.templates.ExecuteTemplate(w, name, map[string]interface{}{
			"context":                      context,
			"GoogleAnalyticsMeasurementID": config.Config.GoogleAnalyticsMeasurementID,
			// Look in //machine/pages/BUILD.bazel for where the nonce templates are injected.
			"Nonce": secure.CSPNonce(r.Context()),
		}); err != nil {
			sklog.Error("Failed to expand template:", err)
		}
	}
}

// newParamsetProvider returns a regression.ParamsetProvider which produces a paramset
// for the current tiles.
func newParamsetProvider(pf psrefresh.ParamSetRefresher) regression.ParamsetProvider {
	return func() paramtools.ReadOnlyParamSet {
		return pf.GetAll()
	}
}

// initialize the application.
func (f *Frontend) initialize() {
	rand.Seed(time.Now().UnixNano())

	runtime.GOMAXPROCS(runtime.NumCPU())

	// Record UID and GID.
	sklog.Infof("Running as %d:%d", os.Getuid(), os.Getgid())

	ctx := context.Background()
	// Init metrics.
	metrics2.InitPrometheus(f.flags.PromPort)
	_ = metrics2.NewLiveness("uptime", nil)

	// Add tracker for long running requests.
	var err error
	f.progressTracker, err = progress.NewTracker("/_/status/")
	if err != nil {
		sklog.Fatalf("Failed to initialize Tracker: %s", err)
	}
	f.progressTracker.Start(ctx)

	// Keep HTTP request metrics.
	severities := sklogimpl.AllSeverities()
	metricLookup := make([]metrics2.Counter, len(severities))
	for _, sev := range severities {
		metricLookup[sev] = metrics2.GetCounter("num_log_lines", map[string]string{"level": sev.String()})
	}
	metricsCallback := func(severity sklogimpl.Severity) {
		metricLookup[severity].Inc(1)
	}
	sklogimpl.SetMetricsCallback(metricsCallback)

	// Load the config file.
	if err := validate.LoadAndValidate(f.flags.ConfigFilename); err != nil {
		sklog.Fatal(err)
	}
	if f.flags.ConnectionString != "" {
		config.Config.DataStoreConfig.ConnectionString = f.flags.ConnectionString
	}
	if f.flags.FeedbackURL != "" {
		config.Config.FeedbackURL = f.flags.FeedbackURL
	}
	cfg := config.Config

	if err := tracing.Init(f.flags.Local, cfg); err != nil {
		sklog.Fatalf("Failed to start tracing: %s", err)
	}

	u, err := url.Parse(cfg.URL)
	if err != nil {
		sklog.Fatal(err)
	}
	f.host = u.Host

	// Configure login.
	f.loginProvider, err = proxylogin.New(
		cfg.AuthConfig.HeaderName,
		cfg.AuthConfig.EmailRegex)
	if err != nil {
		sklog.Fatalf("Failed to initialize login: %s", err)
	}

	// Fix up resources dir values.
	if f.flags.ResourcesDir == "" {
		_, filename, _, _ := runtime.Caller(1)
		f.flags.ResourcesDir = filepath.Join(filepath.Dir(filename), "../../dist")
	}

	f.distFileSystem = http.Dir(f.flags.ResourcesDir)

	sklog.Info("About to init GCS.")
	f.ingestedFS, err = builders.NewIngestedFSFromConfig(ctx, config.Config, f.flags.Local)
	if err != nil {
		sklog.Fatalf("Failed to authenicate to storage provider: %s", err)
	}

	sklog.Info("About to parse templates.")
	f.loadTemplates()

	sklog.Info("About to build trace store.")

	f.traceStore, err = builders.NewTraceStoreFromConfig(ctx, f.flags.Local, config.Config)
	if !f.flags.DisableMetricsUpdate {
		go f.traceStore.StartBackgroundMetricsGathering()
	}

	if err != nil {
		sklog.Fatalf("Failed to build TraceStore: %s", err)
	}

	sklog.Info("About to build perfgit.")

	f.perfGit, err = builders.NewPerfGitFromConfig(ctx, f.flags.Local, config.Config)
	if err != nil {
		sklog.Fatalf("Failed to build perfgit.Git: %s", err)
	}

	sklog.Info("About to build dfbuilder.")

	sklog.Info("Filter parent traces: %s", config.Config.FilterParentTraces)
	f.dfBuilder = dfbuilder.NewDataFrameBuilderFromTraceStore(
		f.perfGit,
		f.traceStore,
		f.flags.NumParamSetsForQueries,
		dfbuilder.Filtering(config.Config.FilterParentTraces))

	sklog.Info("About to build paramset refresher.")

	paramsetRefresher := psrefresh.NewDefaultParamSetRefresher(f.traceStore, f.flags.NumParamSetsForQueries, f.dfBuilder, config.Config.QueryConfig)
	if config.Config.QueryConfig.CacheConfig.Enabled {
		cache, err := builders.GetCacheFromConfig(ctx, *config.Config)
		if err != nil {
			sklog.Fatalf("Error creating cache from the config : %v", err)
		}
		f.paramsetRefresher = psrefresh.NewCachedParamSetRefresher(paramsetRefresher, cache)
	} else {
		f.paramsetRefresher = paramsetRefresher
	}

	if err := f.paramsetRefresher.Start(paramsetRefresherPeriod); err != nil {
		sklog.Fatalf("Failed to build paramsetRefresher: %s", err)
	}

	if config.Config.FetchChromePerfAnomalies {
		f.anomalyApiClient, err = chromeperf.NewAnomalyApiClient(ctx, f.perfGit)
		if err != nil {
			sklog.Fatal("Failed to build chrome anomaly api client: %s", err)
		}
		f.anomalyStore, err = cache.New(f.anomalyApiClient)
		if err != nil {
			sklog.Fatal("Failed to build anomalies.Store: %s", err)
		}

		f.pinpoint, err = pinpoint.New(ctx)
		if err != nil {
			sklog.Fatal("Failed to build pinpoint.Client: %s", err)
		}

		f.alertGroupClient, err = chromeperf.NewAlertGroupApiClient(ctx)
		if err != nil {
			sklog.Fatal("Failed to build alert group client: %s", err)
		}

		f.chromeperfClient, err = chromeperf.NewChromePerfClient(ctx, "", true)
		if err != nil {
			sklog.Fatal("Failed to build chromeperf client: %s", err)
		}
	}

	f.urlProvider = urlprovider.New(f.perfGit)

	// TODO(jcgregorio) Implement store.TryBotStore and add a reference to it here.
	f.trybotResultsLoader = dfloader.New(f.dfBuilder, nil, f.perfGit)

	alerts.DefaultSparse = f.flags.DefaultSparse

	sklog.Info("About to build alertStore.")
	f.alertStore, err = builders.NewAlertStoreFromConfig(ctx, f.flags.Local, config.Config)
	if err != nil {
		sklog.Fatal(err)
	}
	f.shortcutStore, err = builders.NewShortcutStoreFromConfig(ctx, f.flags.Local, config.Config)
	if err != nil {
		sklog.Fatal(err)
	}
	f.graphsShortcutStore, err = builders.NewGraphsShortcutStoreFromConfig(ctx, f.flags.Local, config.Config)
	if err != nil {
		sklog.Fatal(err)
	}

	if f.flags.NoEmail {
		config.Config.NotifyConfig.Notifications = notifytypes.None
	}
	f.notifier, err = notify.New(ctx, &config.Config.NotifyConfig, config.Config.URL, f.flags.CommitRangeURL, f.traceStore, f.ingestedFS)
	if err != nil {
		sklog.Fatal(err)
	}

	f.configProvider, err = alerts.NewConfigProvider(ctx, f.alertStore, 600)
	if err != nil {
		sklog.Fatalf("Failed to create alerts configprovider: %s", err)
	}

	f.regStore, err = builders.NewRegressionStoreFromConfig(ctx, f.flags.Local, cfg, f.configProvider)
	if err != nil {
		sklog.Fatalf("Failed to build regression.Store: %s", err)
	}

	f.subStore, err = builders.NewSubscriptionStoreFromConfig(ctx, cfg)
	if err != nil {
		sklog.Fatalf("Failed to build subscription.Store: %s", err)
	}

	f.favStore, err = builders.NewFavoriteStoreFromConfig(ctx, cfg)
	if err != nil {
		sklog.Fatalf("Failed to build favorite.Store: %s", err)
	}

	f.userIssueStore, err = builders.NewUserIssueStoreFromConfig(ctx, cfg)
	if err != nil {
		sklog.Fatalf("Failed to build userissue.Store: %s", err)
	}

	paramsProvider := newParamsetProvider(f.paramsetRefresher)

	f.dryrunRequests = dryrun.New(f.perfGit, f.progressTracker, f.shortcutStore, f.dfBuilder, paramsProvider)

	if f.flags.DoClustering {
		go func() {
			for i := 0; i < f.flags.NumContinuousParallel; i++ {
				// Start running continuous clustering looking for regressions.
				time.Sleep(startClusterDelay)
				c := continuous.New(f.perfGit, f.shortcutStore, f.configProvider, f.regStore, f.notifier, paramsProvider, *f.urlProvider,
					f.dfBuilder, cfg, f.flags)
				f.continuous = append(f.continuous, c)
				go c.Run(context.Background())
			}
		}()
	}
}

// helpHandler handles the GET of the main page.
func (f *Frontend) helpHandler(w http.ResponseWriter, r *http.Request) {
	sklog.Infof("Help Handler: %q\n", r.URL.Path)
	f.loadTemplates()
	context, err := f.getPageContext()
	if err != nil {
		sklog.Errorf("Failed to JSON encode window.perf context: %s", err)
	}

	if r.Method == "GET" {
		w.Header().Set("Content-Type", "text/html")
		calcContext := calc.NewContext(nil, nil)
		templateContext := struct {
			Nonce                        string
			Funcs                        map[string]calc.Func
			GoogleAnalyticsMeasurementID string
			Context                      template.JS
		}{
			Nonce:                        secure.CSPNonce(r.Context()),
			Funcs:                        calcContext.Funcs,
			GoogleAnalyticsMeasurementID: config.Config.GoogleAnalyticsMeasurementID,
			Context:                      context,
		}
		if err := f.templates.ExecuteTemplate(w, "help.html", templateContext); err != nil {
			sklog.Error("Failed to expand template:", err)
		}
	}
}

// liveness is used by the front end service to verify that cockroachDB
// connections are still working. /liveness handler is polled by
// kubernetes probes. If the connection is down, the pod will restart
// and connection to CDB should re-establish.
func (f *Frontend) liveness(h http.Handler) http.Handler {
	s := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/liveness" {
			ctx, cancel := context.WithTimeout(r.Context(), livenessTimeout)
			defer cancel()

			if err := f.favStore.Liveness(ctx); err != nil {
				httputils.ReportError(w, err, "Health check - failed to connect to CockroachDB.", http.StatusInternalServerError)
			} else {
				w.WriteHeader(http.StatusOK)
			}
			return
		}
		h.ServeHTTP(w, r)
	}
	return http.HandlerFunc(s)
}

func (f *Frontend) trybotLoadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req results.TryBotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}
	prog := progress.New()
	f.progressTracker.Add(prog)
	go func() {
		ctx, span := trace.StartSpan(context.Background(), "trybotLoadHandler")
		defer span.End()

		ctx, cancel := context.WithTimeout(ctx, longRunningRequestTimeout)
		defer cancel()

		resp, err := f.trybotResultsLoader.Load(ctx, req, nil)
		if err != nil {
			prog.Error("Failed to load results.")
			sklog.Errorf("trybot failed to load results: %s", err)
			return
		}
		prog.FinishedWithResults(resp)
	}()
	if err := prog.JSON(w); err != nil {
		sklog.Errorf("Failed to encode trybot results: %s", err)
	}
}

// gotoHandler handles redirecting from a git hash to either the explore,
// clustering, or triage page.
//
// Sets begin and end to a range of commits on either side of the selected
// commit.
//
// Preserves query parameters that are passed into /g/ and passes them onto the
// target URL.
func (f *Frontend) gotoHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()

	if r.Method != "GET" {
		http.Error(w, "Method not allowed.", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		httputils.ReportError(w, err, "Could not parse query parameters.", http.StatusInternalServerError)
		return
	}
	gotoQuery := r.Form
	hash := chi.URLParam(r, "hash")
	dest := chi.URLParam(r, "dest")
	index, err := f.perfGit.CommitNumberFromGitHash(ctx, hash)
	if err != nil {
		httputils.ReportError(w, err, "Could not look up git hash.", http.StatusInternalServerError)
		return
	}
	lastIndex, err := f.perfGit.CommitNumberFromTime(ctx, time.Time{})
	if err != nil {
		httputils.ReportError(w, fmt.Errorf("Failed to find last commit"), "Failed to find last commit.", http.StatusInternalServerError)
		return
	}

	delta := config.GotoRange
	// If redirecting to the Triage page then always show just a single commit.
	if dest == "t" {
		delta = 0
	}
	begin := int(index) - delta
	if begin < 0 {
		begin = 0
	}
	end := int(index) + delta
	if end > int(lastIndex) {
		end = int(lastIndex)
	}
	details, err := f.perfGit.CommitSliceFromCommitNumberSlice(ctx, []types.CommitNumber{
		types.CommitNumber(begin),
		types.CommitNumber(end)})
	if err != nil {
		httputils.ReportError(w, err, "Could not convert indices to hashes.", http.StatusInternalServerError)
		return
	}
	// Always back up one second since we had an issue with duplicate times for
	// commits: skbug.com/10698.
	beginTime := details[0].Timestamp - 1
	endTime := details[1].Timestamp + 1
	gotoQuery.Set("begin", fmt.Sprintf("%d", beginTime))
	gotoQuery.Set("end", fmt.Sprintf("%d", endTime))

	if dest == "e" {
		http.Redirect(w, r, fmt.Sprintf("/e/?%s", gotoQuery.Encode()), http.StatusFound)
	} else if dest == "c" {
		gotoQuery.Set("offset", fmt.Sprintf("%d", index))
		http.Redirect(w, r, fmt.Sprintf("/c/?%s", gotoQuery.Encode()), http.StatusFound)
	} else if dest == "t" {
		gotoQuery.Set("subset", "all")
		http.Redirect(w, r, fmt.Sprintf("/t/?%s", gotoQuery.Encode()), http.StatusFound)
	}
}

func (f *Frontend) revisionHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	ctx, span := trace.StartSpan(ctx, "revisionQueryRequest")
	defer span.End()
	revisionIdStr := r.URL.Query().Get("rev")
	revisionId, err := strconv.Atoi(revisionIdStr)
	if err != nil {
		httputils.ReportError(w, err, "Revision value is not an integer", http.StatusBadRequest)
		return
	}

	anomaliesForRevision, err := f.anomalyStore.GetAnomaliesAroundRevision(ctx, revisionId)
	if err != nil {
		httputils.ReportError(w, err, "Failed to get tests with anomalies for revision", http.StatusInternalServerError)
		return
	}
	// Create url for the test paths
	_, err = f.perfGit.CommitFromCommitNumber(ctx, types.CommitNumber(revisionId))
	if err != nil {
		sklog.Error("Error getting commit info")
	}

	revisionInfoMap := map[string]chromeperf.RevisionInfo{}
	for _, anomalyData := range anomaliesForRevision {
		key := anomalyData.GetKey()
		queryParams := url.Values{
			"highlight_anomalies": []string{strconv.Itoa(anomalyData.Anomaly.Id)},
		}
		if _, ok := revisionInfoMap[key]; !ok {
			exploreUrl := f.urlProvider.Explore(
				ctx,
				anomalyData.StartRevision,
				anomalyData.EndRevision,
				anomalyData.Params,
				true,
				queryParams)
			bugId := ""
			if anomalyData.Anomaly.BugId > 0 {
				bugId = strconv.Itoa(anomalyData.Anomaly.BugId)
			}

			startCommit, _ := f.perfGit.CommitFromCommitNumber(ctx, types.CommitNumber(anomalyData.StartRevision))
			startTime := startCommit.Timestamp

			endCommit, _ := f.perfGit.CommitFromCommitNumber(ctx, types.CommitNumber(anomalyData.EndRevision))
			endTime := time.Unix(endCommit.Timestamp, 0).AddDate(0, 0, 1).Unix()

			revisionInfoMap[key] = chromeperf.RevisionInfo{
				StartRevision: anomalyData.StartRevision,
				EndRevision:   anomalyData.EndRevision,
				StartTime:     startTime,
				EndTime:       endTime,
				Master:        anomalyData.GetParamValue("master"),
				Bot:           anomalyData.GetParamValue("bot"),
				Benchmark:     anomalyData.GetParamValue("benchmark"),
				TestPath:      anomalyData.GetTestPath(),
				BugId:         bugId,
				ExploreUrl:    exploreUrl,
				Query:         f.urlProvider.GetQueryStringFromParameters(anomalyData.Params),
				AnomalyIds:    []string{strconv.Itoa(anomalyData.Anomaly.Id)},
			}
		} else {
			revInfo := revisionInfoMap[key]
			if anomalyData.StartRevision < revInfo.StartRevision {
				revInfo.StartRevision = anomalyData.StartRevision
			}

			if anomalyData.EndRevision > revInfo.EndRevision {
				revInfo.EndRevision = anomalyData.EndRevision
			}

			revInfo.AnomalyIds = append(revInfo.AnomalyIds, strconv.Itoa(anomalyData.Anomaly.Id))
			revisionInfoMap[key] = revInfo
		}
	}

	revisionInfos := []chromeperf.RevisionInfo{}
	for _, info := range revisionInfoMap {
		revisionInfos = append(revisionInfos, info)
	}
	sklog.Infof("Returning %d anomaly groups", len(revisionInfoMap))
	if err := json.NewEncoder(w).Encode(revisionInfos); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
	}
}

func (f *Frontend) loginStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	sklog.Infof("X-WEBAUTH-USER header value: %s", r.Header.Get("X-WEBAUTH-USER"))
	if err := json.NewEncoder(w).Encode(f.loginProvider.Status(r)); err != nil {
		httputils.ReportError(w, err, "Failed to encode login status", http.StatusInternalServerError)
	}
}

func (f *Frontend) makeDistHandler() func(http.ResponseWriter, *http.Request) {
	fileServer := http.StripPrefix("/dist", http.FileServer(f.distFileSystem))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "max-age=300")
		fileServer.ServeHTTP(w, r)
	}
}

func oldMainHandler(w http.ResponseWriter, r *http.Request) {
	instanceConf := config.Config
	landingPath := instanceConf.LandingPageRelPath
	if landingPath == "" {
		landingPath = "/e/"
	}
	http.Redirect(w, r, landingPath, http.StatusMovedPermanently)
}

func oldClustersHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/c/", http.StatusMovedPermanently)
}

func oldAlertsHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/t/", http.StatusMovedPermanently)
}

func (f *Frontend) RoleEnforcedHandler(role roles.Role, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if f.loginProvider.Status(r).EMail.String() == "" {
			http.Error(w, "User is not logged in or is not authorized.", http.StatusUnauthorized)
			return
		}

		if !f.loginProvider.HasRole(r, role) {
			http.Error(w, "User is not authenticated.", http.StatusForbidden)
			return
		}
		handler.ServeHTTP(w, r)
	})
}

// defaultsHandler returns the default settings
func (f *Frontend) defaultsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(config.Config.QueryConfig); err != nil {
		sklog.Errorf("Error writing the default query config json to response: %s", err)
	}
}

// GetHandler creates the http.Handler for all supported endpoints.
func (f *Frontend) GetHandler(allowedHosts []string) http.Handler {
	// Resources are served directly.
	router := chi.NewRouter()

	ah := []string{f.host}
	if len(allowedHosts) > 0 {
		ah = append(ah, allowedHosts...)
	}

	local := true
	if f.flags != nil {
		local = f.flags.Local
	}
	router.Use(baseapp.SecurityMiddleware(ah, local, nil))

	router.HandleFunc("/dist/*", f.makeDistHandler())

	// Redirects for the old Perf URLs.
	router.HandleFunc("/", oldMainHandler)
	router.HandleFunc("/clusters/", oldClustersHandler)
	router.HandleFunc("/alerts/", oldAlertsHandler)

	// New endpoints that use ptracestore will go here.
	router.HandleFunc("/e/", f.templateHandler("newindex.html"))
	router.HandleFunc("/m/", f.templateHandler("multiexplore.html"))
	router.HandleFunc("/c/", f.templateHandler("clusters2.html"))
	router.HandleFunc("/t/", f.templateHandler("triage.html"))
	router.HandleFunc("/d/", f.templateHandler("dryrunalert.html"))
	router.HandleFunc("/r/", f.templateHandler("trybot.html"))
	router.HandleFunc("/f/", f.templateHandler("favorites.html"))
	router.HandleFunc("/v/", f.templateHandler("revisions.html"))
	router.HandleFunc("/u/", f.templateHandler("report.html"))
	router.HandleFunc("/g/{dest:[ect]}/{hash:[a-zA-Z0-9]+}", f.gotoHandler)
	router.HandleFunc("/help/", f.helpHandler)

	// The legacy page for /a/ is alerts.html.
	// Sheriff Config based alerts will route to regressions.html.
	if config.Config.NewAlertsPage {
		router.HandleFunc("/a/", f.templateHandler("regressions.html"))
		// (b/391716594) Need an entry to set up test alerts for migration purposes.
		router.HandleFunc("/admin/alerts/", f.templateHandler("alerts.html"))
	} else {
		router.HandleFunc("/a/", f.templateHandler("alerts.html"))
		router.Get("/r2/", f.templateHandler("regressions.html"))
	}

	// TODO(ashwinpv): This should move to using the backend service.
	// JSON handlers.
	// Pinpoint JSON API handlers - /pinpoint/v1/...
	if ph, err := pp_service.NewJSONHandler(context.Background(), pp_service.New(nil, nil)); err != nil {
		// Only log the error, the service should continue to run.
		sklog.Error("Fail to initalize pinpoint service %s.", err)
	} else {
		router.Mount("/pinpoint", f.RoleEnforcedHandler(roles.Bisecter, ph))
	}

	// Common endpoint for all long-running requests.
	if f.progressTracker != nil {
		router.Get("/_/status/{id:[a-zA-Z0-9-]+}", f.progressTracker.Handler)
	}

	// TODO(ashwinpv): The trybot page looks to be unused. Confirm and delete if that's the case.
	router.Post("/_/trybot/load/", f.trybotLoadHandler)

	apis := f.getFrontendApis()

	for _, frontEndApi := range apis {
		frontEndApi.RegisterHandlers(router)
	}
	router.Get("/_/login/status", f.loginStatus)

	router.Get("/_/defaults/", f.defaultsHandler)
	router.Get("/_/revision/", f.revisionHandler)

	return router
}

// getFrontendApis returns a list of apis supported by the Frontend service.
func (f *Frontend) getFrontendApis() []api.FrontendApi {
	return []api.FrontendApi{
		api.NewFavoritesApi(f.loginProvider, f.favStore),
		api.NewAlertsApi(f.loginProvider, f.configProvider, f.alertStore, f.notifier, f.subStore, f.dryrunRequests),
		api.NewAnomaliesApi(f.loginProvider, f.chromeperfClient, f.perfGit, f.subStore, f.alertStore),
		api.NewRegressionsApi(f.loginProvider, f.configProvider, f.alertStore, f.regStore, f.perfGit, f.anomalyApiClient, f.urlProvider, f.graphsShortcutStore, f.alertGroupClient, f.progressTracker, f.shortcutStore, f.dfBuilder, f.paramsetRefresher),
		api.NewQueryApi(f.paramsetRefresher),
		api.NewShortCutsApi(f.shortcutStore, f.graphsShortcutStore),
		api.NewGraphApi(f.flags.NumParamSetsForQueries, f.loginProvider, f.dfBuilder, f.perfGit, f.traceStore, f.shortcutStore, f.anomalyStore, f.progressTracker, f.ingestedFS),
		api.NewPinpointApi(f.loginProvider, f.pinpoint),
		api.NewSheriffConfigApi(f.loginProvider),
		api.NewTriageApi(f.loginProvider, f.chromeperfClient, f.anomalyStore),
		api.NewUserIssueApi(f.loginProvider, f.userIssueStore),
	}
}

// Serve content on the configured endpoints.Serve.
//
// This method does not return.
func (f *Frontend) Serve() {
	// Start the internal server on the internal port if requested.
	if f.flags.InternalPort != "" {
		go func() {
			sklog.Infof("Internal server on %q", f.flags.InternalPort)
			httputils.ServePprof(f.flags.InternalPort)
		}()
	}

	var h http.Handler = f.GetHandler(config.Config.AllowedHosts)
	h = httputils.LoggingGzipRequestResponse(h)
	if !f.flags.Local {
		h = httputils.HealthzAndHTTPS(h)
		// add liveness handler after https routing since these are applied in
		// reverse order to ensure k8 pod can access the endpoint without
		// 301 moved permanently status
		h = f.liveness(h)
	}
	http.Handle("/", h)

	sklog.Info("Ready to serve.")

	// We create our own server here instead of using http.ListenAndServe, so
	// that we don't expose the /debug/pprof endpoints to the open web.
	server := &http.Server{
		Addr:    f.flags.Port,
		Handler: h,
	}
	sklog.Fatal(server.ListenAndServe())
}
