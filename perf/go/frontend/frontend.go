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
	"net/http/pprof"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/unrolled/secure"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/alogin/proxylogin"
	"go.skia.org/infra/go/auditlog"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/calc"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sklog/sklogimpl"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/alertfilter"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/anomalies"
	"go.skia.org/infra/perf/go/anomalies/cache"
	"go.skia.org/infra/perf/go/bug"
	"go.skia.org/infra/perf/go/builders"
	"go.skia.org/infra/perf/go/chromeperf"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/config/validate"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/dfbuilder"
	"go.skia.org/infra/perf/go/dryrun"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/graphsshortcut"
	"go.skia.org/infra/perf/go/ingest/format"
	"go.skia.org/infra/perf/go/notify"
	"go.skia.org/infra/perf/go/notifytypes"
	"go.skia.org/infra/perf/go/pinpoint"
	"go.skia.org/infra/perf/go/progress"
	"go.skia.org/infra/perf/go/psrefresh"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/regression/continuous"
	"go.skia.org/infra/perf/go/shortcut"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/tracing"
	"go.skia.org/infra/perf/go/trybot/results"
	"go.skia.org/infra/perf/go/trybot/results/dfloader"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
	"go.skia.org/infra/perf/go/urlprovider"
)

const (
	// regressionCountDuration is how far back we look for regression in the /_/reg/count endpoint.
	regressionCountDuration = -14 * 24 * time.Hour

	// defaultAlertCategory is the category that will be used by the /_/alerts/ endpoint.
	defaultAlertCategory = "Prod"

	// paramsetRefresherPeriod is how often we refresh our canonical paramset from the OPS's
	// stored in the last two tiles.
	paramsetRefresherPeriod = 5 * time.Minute

	// startClusterDelay is the time we wait between starting each clusterer, to avoid hammering
	// the trace store all at once.
	startClusterDelay = 2 * time.Second

	// defaultBugURLTemplate is the URL template to use if the user
	// doesn't supply one.
	defaultBugURLTemplate = "https://bugs.chromium.org/p/skia/issues/entry?comment=This+bug+was+found+via+SkiaPerf.%0A%0AVisit+this+URL+to+see+the+details+of+the+suspicious+cluster%3A%0A%0A++{cluster_url}%0A%0AThe+suspect+commit+is%3A%0A%0A++{commit_url}%0A%0A++{message}&labels=FromSkiaPerf%2CType-Defect%2CPriority-Medium"

	// longRunningRequestTimeout is a limit on long running processes.
	longRunningRequestTimeout = 20 * time.Minute

	// How often to update the git repo from origin.
	gitRepoUpdatePeriod = time.Minute

	// defaultDatabaseTimeout is the context timeout used when the frontend is
	// making a request that involves the database. For more complex requests
	// use config.QueryMaxRuntime.
	defaultDatabaseTimeout = time.Minute
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

	continuous []*continuous.Continuous

	// provides access to the ingested files.
	ingestedFS fs.FS

	alertStore alerts.Store

	shortcutStore shortcut.Store

	configProvider alerts.ConfigProvider

	graphsShortcutStore graphsshortcut.Store

	notifier notify.Notifier

	traceStore tracestore.TraceStore

	dryrunRequests *dryrun.Requests

	paramsetRefresher *psrefresh.ParamSetRefresher

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
func newParamsetProvider(pf *psrefresh.ParamSetRefresher) regression.ParamsetProvider {
	return func() paramtools.ReadOnlyParamSet {
		return pf.Get()
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

	sklog.Info("About to build paramset refresher.")

	f.paramsetRefresher = psrefresh.NewParamSetRefresher(f.traceStore, f.flags.NumParamSetsForQueries)
	if err := f.paramsetRefresher.Start(paramsetRefresherPeriod); err != nil {
		sklog.Fatalf("Failed to build paramsetRefresher: %s", err)
	}

	sklog.Info("About to build perfgit.")

	f.perfGit, err = builders.NewPerfGitFromConfig(ctx, f.flags.Local, config.Config)
	if err != nil {
		sklog.Fatalf("Failed to build perfgit.Git: %s", err)
	}

	// TODO(jcgregorio) Remove one `perfserver maintenance` is running for all instances.
	if !f.flags.DisableGitUpdate {
		// Update the git repo periodically since perfGit.LogEntry does interrogate
		// the git repo itself instead of using the SQL backend.
		//
		// TODO(jcgregorio) Remove once perfgit stores full commit messages.
		go func() {
			for range time.Tick(gitRepoUpdatePeriod) {
				timeoutContext, cancel := context.WithTimeout(ctx, defaultDatabaseTimeout)
				if err := f.perfGit.Update(timeoutContext); err != nil {
					sklog.Errorf("Failed to update git repo: %s", err)
				}
				cancel()
			}
		}()
	}

	sklog.Info("About to build dfbuilder.")

	sklog.Info("Filter parent traces: %s", config.Config.FilterParentTraces)
	f.dfBuilder = dfbuilder.NewDataFrameBuilderFromTraceStore(
		f.perfGit,
		f.traceStore,
		f.flags.NumParamSetsForQueries,
		dfbuilder.Filtering(config.Config.FilterParentTraces))

	var urlParamsProvider urlprovider.ParamsProvider = nil
	if config.Config.FetchChromePerfAnomalies {
		anomalyApiClient, err := chromeperf.NewAnomalyApiClient(ctx)
		if err != nil {
			sklog.Fatal("Failed to build chrome anomaly api client: %s", err)
		}
		f.anomalyStore, err = cache.New(anomalyApiClient)
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

		ignoreParams := config.Config.QueryConfig.ChromePerfIgnoreParams
		paramsMap := config.Config.QueryConfig.ChromePerfParamsMap
		urlParamsProvider = &urlprovider.ChromeParamsProvider{
			IgnoreParams: ignoreParams,
			ParamsMap:    paramsMap,
		}
	} else {
		urlParamsProvider = &urlprovider.DefaultParamsProvider{}
	}

	f.urlProvider = urlprovider.New(f.perfGit, urlParamsProvider)

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
	f.notifier, err = notify.New(ctx, &config.Config.NotifyConfig, config.Config.URL, f.flags.CommitRangeURL)
	if err != nil {
		sklog.Fatal(err)
	}

	f.regStore, err = builders.NewRegressionStoreFromConfig(ctx, f.flags.Local, cfg)
	if err != nil {
		sklog.Fatalf("Failed to build regression.Store: %s", err)
	}
	f.configProvider, err = alerts.NewConfigProvider(ctx, f.alertStore, 600)
	if err != nil {
		sklog.Fatalf("Failed to create alerts configprovider: %s", err)
	}
	paramsProvider := newParamsetProvider(f.paramsetRefresher)

	f.dryrunRequests = dryrun.New(f.perfGit, f.progressTracker, f.shortcutStore, f.dfBuilder, paramsProvider)

	if f.flags.DoClustering {
		go func() {
			for i := 0; i < f.flags.NumContinuousParallel; i++ {
				// Start running continuous clustering looking for regressions.
				time.Sleep(startClusterDelay)
				c := continuous.New(f.perfGit, f.shortcutStore, f.configProvider, f.regStore, f.notifier, paramsProvider, f.dfBuilder,
					cfg, f.flags)
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
			Nonce   string
			Funcs   map[string]calc.Func
			Context template.JS
		}{
			Nonce:   secure.CSPNonce(r.Context()),
			Funcs:   calcContext.Funcs,
			Context: context,
		}
		if err := f.templates.ExecuteTemplate(w, "help.html", templateContext); err != nil {
			sklog.Error("Failed to expand template:", err)
		}
	}
}

func (f *Frontend) alertsHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	count, err := f.regressionCount(ctx, defaultAlertCategory)
	if err != nil {
		httputils.ReportError(w, err, "Failed to load untriaged count.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Add("Access-Control-Allow-Origin", "*")
	resp := alerts.AlertsStatus{
		Alerts: count,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
}

func (f *Frontend) initpageHandler(w http.ResponseWriter, _ *http.Request) {
	resp := &frame.FrameResponse{
		DataFrame: &dataframe.DataFrame{
			ParamSet: f.getParamSet(),
		},
		Skps: []int{},
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
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

// RangeRequest is used in cidRangeHandler and is used to query for a range of
// cid.CommitIDs that include the range between [begin, end) and include the
// explicit CommitID of "Source, Offset".
type RangeRequest struct {
	Offset types.CommitNumber `json:"offset"`
	Begin  int64              `json:"begin"`
	End    int64              `json:"end"`
}

// cidRangeHandler accepts a POST'd JSON serialized RangeRequest
// and returns a serialized JSON slice of cid.CommitDetails.
func (f *Frontend) cidRangeHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	var rr RangeRequest
	if err := json.NewDecoder(r.Body).Decode(&rr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}

	resp, err := f.perfGit.CommitSliceFromTimeRange(ctx, time.Unix(rr.Begin, 0), time.Unix(rr.End, 0))
	if err != nil {
		httputils.ReportError(w, err, "Failed to look up commits", http.StatusInternalServerError)
		return
	}

	if rr.Offset != types.BadCommitNumber {
		details, err := f.perfGit.CommitFromCommitNumber(ctx, rr.Offset)
		if err != nil {
			httputils.ReportError(w, err, "Failed to look up commit", http.StatusInternalServerError)
			return
		}
		resp = append(resp, details)
	}

	// Filter if we have a restricted set of branches.
	ret := []provider.Commit{}
	if len(config.Config.IngestionConfig.Branches) != 0 {
		for _, details := range resp {
			for _, branch := range config.Config.IngestionConfig.Branches {
				if strings.HasSuffix(details.Subject, branch) {
					ret = append(ret, details)
					continue
				}
			}
		}
	} else {
		ret = resp
	}

	if err := json.NewEncoder(w).Encode(ret); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
}

// frameStartHandler starts a FrameRequest running and returns the ID
// of the Go routine doing the work.
//
// Building a DataFrame can take a long time to complete, so we run the request
// in a Go routine and break the building of DataFrames into three separate
// requests:
//   - Start building the DataFrame (_/frame/start), which returns an identifier of the long
//     running request, {id}.
//   - Query the status of the running request (_/frame/status/{id}).
//   - Finally return the constructed DataFrame (_/frame/results/{id}).
func (f *Frontend) frameStartHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fr := frame.NewFrameRequest()
	if err := json.NewDecoder(r.Body).Decode(fr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}
	auditlog.LogWithUser(r, f.loginProvider.LoggedInAs(r).String(), "query", fr)
	// Remove all empty queries.
	q := []string{}
	for _, s := range fr.Queries {
		if strings.TrimSpace(s) != "" {
			q = append(q, s)
		}
	}
	fr.Queries = q

	if len(fr.Formulas) == 0 && len(fr.Queries) == 0 && fr.Keys == "" {
		httputils.ReportError(w, fmt.Errorf("Invalid query."), "Empty queries are not allowed.", http.StatusInternalServerError)
		return
	}

	f.progressTracker.Add(fr.Progress)
	go func() {
		// Intentionally using a background context here because the calculation will go on in the background after
		// the request finishes
		ctx, span := trace.StartSpan(context.Background(), "frameStartRequest")
		timeoutCtx, cancel := context.WithTimeout(ctx, config.QueryMaxRunTime)
		defer cancel()
		defer span.End()
		err := frame.ProcessFrameRequest(timeoutCtx, fr, f.perfGit, f.dfBuilder, f.shortcutStore, f.anomalyStore, config.Config.GitRepoConfig.CommitNumberRegex == "")
		if err != nil {
			fr.Progress.Error(err.Error())
		} else {
			fr.Progress.Finished()
		}
	}()

	if err := fr.Progress.JSON(w); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
}

func (f *Frontend) alertGroupQueryHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()

	sklog.Info("Received alert group request")
	if f.alertGroupClient == nil {
		sklog.Info("Alert Grouping is not enabled")
		httputils.ReportError(w, nil, "Alert Grouping is not enabled", http.StatusNotFound)
		return
	}
	groupId := r.URL.Query().Get("group_id")
	sklog.Infof("Group id is %s", groupId)
	ctx, span := trace.StartSpan(ctx, "alertGroupQueryRequest")
	defer span.End()
	alertGroupDetails, err := f.alertGroupClient.GetAlertGroupDetails(ctx, groupId)
	if err != nil {
		sklog.Errorf("Error in retrieving alert group details: %s", err)
	}

	if alertGroupDetails != nil {
		sklog.Infof("Retrieved %d anomalies for alert group id %s", len(alertGroupDetails.Anomalies), groupId)

		explore := r.URL.Query().Get("e")
		var redirectUrl string
		if explore == "" {
			queryParamsPerTrace := alertGroupDetails.GetQueryParamsPerTrace(ctx)
			graphs := []graphsshortcut.GraphConfig{}
			for _, queryParams := range queryParamsPerTrace {
				queryString := f.urlProvider.GetQueryStringFromParameters(queryParams)
				graphs = append(graphs, graphsshortcut.GraphConfig{
					Queries:  []string{queryString},
					Formulas: []string{},
				})
			}

			shortcutObj := graphsshortcut.GraphsShortcut{
				Graphs: graphs,
			}

			shortcutId, err := f.graphsShortcutStore.InsertShortcut(ctx, &shortcutObj)
			if err != nil {
				// Something went wrong while inserting shortcut.
				sklog.Errorf("Error inserting shortcut %s", err)
				// Let's redirect the user to the explore page instead.
				queryParams := alertGroupDetails.GetQueryParams(ctx)
				redirectUrl = f.urlProvider.Explore(ctx, int(alertGroupDetails.StartCommitNumber), int(alertGroupDetails.EndCommitNumber), queryParams)
			} else {
				redirectUrl = f.urlProvider.MultiGraph(ctx, int(alertGroupDetails.StartCommitNumber), int(alertGroupDetails.EndCommitNumber), shortcutId)
			}

		} else {
			queryParams := alertGroupDetails.GetQueryParams(ctx)
			redirectUrl = f.urlProvider.Explore(ctx, int(alertGroupDetails.StartCommitNumber), int(alertGroupDetails.EndCommitNumber), queryParams)
		}
		sklog.Infof("Generated url: %s", redirectUrl)
		http.Redirect(w, r, redirectUrl, http.StatusSeeOther)
		return
	}
}

// CountHandlerRequest is the JSON format for the countHandler request.
type CountHandlerRequest struct {
	Q     string `json:"q"`
	Begin int    `json:"begin"`
	End   int    `json:"end"`
}

// CountHandlerResponse is the JSON format if the countHandler response.
type CountHandlerResponse struct {
	Count    int                         `json:"count"`
	Paramset paramtools.ReadOnlyParamSet `json:"paramset"`
}

// countHandler takes the POST'd query and runs that against the current
// dataframe and returns how many traces match the query.
func (f *Frontend) countHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	var cr CountHandlerRequest
	if err := json.NewDecoder(r.Body).Decode(&cr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}

	u, err := url.ParseQuery(cr.Q)
	if err != nil {
		httputils.ReportError(w, err, "Invalid URL query.", http.StatusInternalServerError)
		return
	}
	q, err := query.New(u)
	if err != nil {
		httputils.ReportError(w, err, "Invalid query.", http.StatusInternalServerError)
		return
	}
	resp := CountHandlerResponse{}
	fullPS := f.getParamSet()
	if cr.Q == "" {
		resp.Count = 0
		resp.Paramset = fullPS
	} else {
		count, ps, err := f.dfBuilder.PreflightQuery(ctx, q, fullPS)
		if err != nil {
			httputils.ReportError(w, err, "Failed to Preflight the query, too many key-value pairs selected. Limit is 200.", http.StatusBadRequest)
			return
		}

		resp.Count = int(count)
		resp.Paramset = filterParamSetIfNeeded(ps.Freeze())
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
}

// CIDHandlerResponse is the form of the response from the /_/cid/ endpoint.
type CIDHandlerResponse struct {
	// CommitSlice describes all the commits requested.
	CommitSlice []provider.Commit `json:"commitSlice"`

	// LogEntry is the full git log entry for the first commit in the
	// CommitSlice.
	LogEntry string `json:"logEntry"`
}

// cidHandler takes the POST'd list of dataframe.ColumnHeaders, and returns a
// serialized slice of cid.CommitDetails.
func (f *Frontend) cidHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	cids := []types.CommitNumber{}
	if err := json.NewDecoder(r.Body).Decode(&cids); err != nil {
		httputils.ReportError(w, err, "Could not decode POST body.", http.StatusInternalServerError)
		return
	}

	commits, err := f.perfGit.CommitSliceFromCommitNumberSlice(ctx, cids)
	if err != nil {
		httputils.ReportError(w, err, "Failed to lookup all commit ids", http.StatusInternalServerError)
		return
	}
	logEntry, err := f.perfGit.LogEntry(ctx, cids[0])
	if err != nil {
		logEntry = "<<< Failed to load >>>"
		sklog.Errorf("Failed to get log entry: %s", err)
	}

	resp := CIDHandlerResponse{
		CommitSlice: commits,
		LogEntry:    logEntry,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
}

// ClusterStartResponse is serialized as JSON for the response in
// clusterStartHandler.
type ClusterStartResponse struct {
	ID string `json:"id"`
}

// clusterStartHandler takes a POST'd RegressionDetectionRequest and starts a
// long running Go routine to do the actual regression detection.
//
// The results of the long running process are stored in the
// RegressionDetectionProcess.Progress.Results.
func (f *Frontend) clusterStartHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	req := regression.NewRegressionDetectionRequest()
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		httputils.ReportError(w, err, "Could not decode POST body.", http.StatusInternalServerError)
		return
	}
	auditlog.LogWithUser(r, f.loginProvider.LoggedInAs(r).String(), "cluster", req)

	cb := func(ctx context.Context, _ *regression.RegressionDetectionRequest, clusterResponse []*regression.RegressionDetectionResponse, _ string) {
		// We don't do GroupBy clustering, so there will only be one clusterResponse.
		req.Progress.Results(clusterResponse[0])
	}
	f.progressTracker.Add(req.Progress)

	go func() {
		// This intentionally does not use r.Context() because we want it to outlive this request.
		err := regression.ProcessRegressions(context.Background(), req, cb, f.perfGit, f.shortcutStore, f.dfBuilder, f.paramsetRefresher.Get(), regression.ExpandBaseAlertByGroupBy, regression.ReturnOnError, config.Config.AnomalyConfig)
		if err != nil {
			sklog.Errorf("ProcessRegressions returned: %s", err)
			req.Progress.Error("Failed to load data.")
		} else {
			req.Progress.Finished()
		}
	}()

	if err := req.Progress.JSON(w); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
}

// keysHandler handles the POST requests of a list of keys.
//
//	{
//	   "keys": [
//	        ",arch=x86,...",
//	        ",arch=x86,...",
//	   ]
//	}
//
// And returns the ID of the new shortcut to that list of keys:
//
//	{
//	  "id": 123456,
//	}
func (f *Frontend) keysHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	id, err := f.shortcutStore.Insert(ctx, r.Body)
	if err != nil {
		httputils.ReportError(w, err, "Error inserting shortcut.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(map[string]string{"id": id}); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
	}
}

type GetGraphsShortcutRequest struct {
	ID string `json:"id"`
}

func (f *Frontend) getGraphsShortcutHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	var ggsr GetGraphsShortcutRequest
	if err := json.NewDecoder(r.Body).Decode(&ggsr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}

	sc, err := f.graphsShortcutStore.GetShortcut(ctx, ggsr.ID)

	if err != nil {
		httputils.ReportError(w, err, "Failed to get keys shortcut.", http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(sc); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
	}
}

func (f *Frontend) createGraphsShortcutHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	shortcut := &graphsshortcut.GraphsShortcut{}
	if err := json.NewDecoder(r.Body).Decode(shortcut); err != nil {
		httputils.ReportError(w, err, "Unable to read shortcut body.", http.StatusInternalServerError)
		return
	}

	id, err := f.graphsShortcutStore.InsertShortcut(ctx, shortcut)
	if err != nil {
		httputils.ReportError(w, err, "Error inserting graphs shortcut.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(map[string]string{"id": id}); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
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

func (f *Frontend) isEditor(w http.ResponseWriter, r *http.Request, action string, body interface{}) bool {
	user := f.loginProvider.LoggedInAs(r)
	if !f.loginProvider.HasRole(r, roles.Editor) {
		httputils.ReportError(w, fmt.Errorf("Not logged in."), "You must be logged in to complete this action.", http.StatusUnauthorized)
		return false
	}
	auditlog.LogWithUser(r, user.String(), action, body)
	return true
}

// TriageRequest is used in triageHandler.
type TriageRequest struct {
	Cid         types.CommitNumber      `json:"cid"`
	Alert       alerts.Alert            `json:"alert"`
	Triage      regression.TriageStatus `json:"triage"`
	ClusterType string                  `json:"cluster_type"`
}

// TriageResponse is used in triageHandler.
type TriageResponse struct {
	Bug string `json:"bug"` // URL to bug reporting page.
}

// triageHandler takes a POST'd TriageRequest serialized as JSON
// and performs the triage.
//
// If successful it returns a 200, or an HTTP status code of 500 otherwise.
func (f *Frontend) triageHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	tr := &TriageRequest{}
	if err := json.NewDecoder(r.Body).Decode(tr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}
	if !f.isEditor(w, r, "triage", tr) {
		return
	}
	detail, err := f.perfGit.CommitFromCommitNumber(ctx, tr.Cid)
	if err != nil {
		httputils.ReportError(w, err, "Failed to find CommitID.", http.StatusInternalServerError)
		return
	}

	key := tr.Alert.IDAsString
	if tr.ClusterType == "low" {
		err = f.regStore.TriageLow(ctx, detail.CommitNumber, key, tr.Triage)
	} else {
		err = f.regStore.TriageHigh(ctx, detail.CommitNumber, key, tr.Triage)
	}

	if err != nil {
		httputils.ReportError(w, err, "Failed to triage.", http.StatusInternalServerError)
		return
	}
	link := fmt.Sprintf("%s/t/?begin=%d&end=%d&subset=all", r.Header.Get("Origin"), detail.Timestamp, detail.Timestamp+1)

	resp := &TriageResponse{}

	if tr.Triage.Status == regression.Negative && config.Config.NotifyConfig.Notifications != notifytypes.MarkdownIssueTracker {
		cfgs, err := f.configProvider.GetAllAlertConfigs(ctx, false)
		if err != nil {
			sklog.Errorf("Failed to load configs looking for BugURITemplate: %s", err)
		}
		uritemplate := defaultBugURLTemplate
		for _, c := range cfgs {
			if c.IDAsString == tr.Alert.IDAsString {
				if c.BugURITemplate != "" {
					uritemplate = c.BugURITemplate
				}
				break
			}
		}
		resp.Bug = bug.Expand(uritemplate, link, detail, tr.Triage.Message)
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
	}
}

// unixTimestampRangeToCommitNumberRange converts a range of commits given in
// Unit timestamps into a range of types.CommitNumbers.
//
// Note this could return two equal commitNumbers.
func (f *Frontend) unixTimestampRangeToCommitNumberRange(ctx context.Context, begin, end int64) (types.CommitNumber, types.CommitNumber, error) {
	beginCommitNumber, err := f.perfGit.CommitNumberFromTime(ctx, time.Unix(begin, 0))
	if err != nil {
		return types.BadCommitNumber, types.BadCommitNumber, skerr.Fmt("Didn't find any commit for begin: %d", begin)
	}
	endCommitNumber, err := f.perfGit.CommitNumberFromTime(ctx, time.Unix(end, 0))
	if err != nil {
		return types.BadCommitNumber, types.BadCommitNumber, skerr.Fmt("Didn't find any commit for end: %d", end)
	}
	return beginCommitNumber, endCommitNumber, nil
}

// regressionCount returns the number of commits that have regressions for alerts
// in the given category. The time range of commits is REGRESSION_COUNT_DURATION.
func (f *Frontend) regressionCount(ctx context.Context, category string) (int, error) {
	configs, err := f.configProvider.GetAllAlertConfigs(ctx, false)
	if err != nil {
		return 0, err
	}

	// Query for Regressions in the range.
	end := time.Now()

	begin := end.Add(regressionCountDuration)
	commitNumberBegin, commitNumberEnd, err := f.unixTimestampRangeToCommitNumberRange(ctx, begin.Unix(), end.Unix())
	if err != nil {
		return 0, err
	}
	regMap, err := f.regStore.Range(ctx, commitNumberBegin, commitNumberEnd)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, regs := range regMap {
		for _, cfg := range configs {
			if reg, ok := regs.ByAlertID[cfg.IDAsString]; ok {
				if cfg.Category == category && !reg.Triaged() {
					// If any alert for the commit is in the category and is untriaged then we count that row only once.
					count += 1
					break
				}
			}
		}
	}
	return count, nil
}

// regressionCountHandler returns a JSON object with the number of untriaged
// alerts that appear in the REGRESSION_COUNT_DURATION. The category
// can be supplied by the 'cat' query parameter and defaults to "".
func (f *Frontend) regressionCountHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	category := r.FormValue("cat")
	count, err := f.regressionCount(ctx, category)
	if err != nil {
		httputils.ReportError(w, err, "Failed to count regressions.", http.StatusInternalServerError)
	}

	if err := json.NewEncoder(w).Encode(struct{ Count int }{Count: count}); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
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
		key := fmt.Sprintf(
			"%s-%s-%s-%s",
			anomalyData.Params["master"],
			anomalyData.Params["bot"],
			anomalyData.Params["benchmark"],
			anomalyData.Params["test"])
		if _, ok := revisionInfoMap[key]; !ok {
			exploreUrl := f.urlProvider.Explore(
				ctx,
				anomalyData.StartRevision,
				anomalyData.EndRevision,
				anomalyData.Params)
			bugId := ""
			if anomalyData.Anomaly.BugId > 0 {
				bugId = strconv.Itoa(anomalyData.Anomaly.BugId)
			}
			revisionInfoMap[key] = chromeperf.RevisionInfo{
				StartRevision: anomalyData.StartRevision,
				EndRevision:   anomalyData.EndRevision,
				Master:        anomalyData.Params["master"][0],
				Bot:           anomalyData.Params["bot"][0],
				Benchmark:     anomalyData.Params["benchmark"][0],
				Test:          anomalyData.Params["test"][0],
				BugId:         bugId,
				ExploreUrl:    exploreUrl,
			}
		} else {
			revInfo := revisionInfoMap[key]
			if anomalyData.StartRevision < revInfo.StartRevision {
				revInfo.StartRevision = anomalyData.StartRevision
			}

			if anomalyData.EndRevision > revInfo.EndRevision {
				revInfo.EndRevision = anomalyData.EndRevision
			}

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

// Subset is the Subset of regressions we are querying for.
type Subset string

const (
	SubsetAll         Subset = "all"         // Include all regressions in a range.
	SubsetRegressions Subset = "regressions" // Only include regressions in a range that are alerting.
	SubsetUntriaged   Subset = "untriaged"   // All untriaged alerting regressions regardless of range.
)

var AllRegressionSubset = []Subset{SubsetAll, SubsetRegressions, SubsetUntriaged}

// RegressionRangeRequest is used in regressionRangeHandler and is used to query for a range of
// of Regressions.
//
// Begin and End are Unix timestamps in seconds.
type RegressionRangeRequest struct {
	Begin       int64  `json:"begin"`
	End         int64  `json:"end"`
	Subset      Subset `json:"subset"`
	AlertFilter string `json:"alert_filter"` // Can be an alertfilter constant, or a category prefixed with "cat:".
}

// RegressionRow are all the Regression's for a specific commit. It is used in
// RegressionRangeResponse.
//
// The Columns have the same order as RegressionRangeResponse.Header.
type RegressionRow struct {
	Commit  provider.Commit          `json:"cid"`
	Columns []*regression.Regression `json:"columns"`
}

// RegressionRangeResponse is the response from regressionRangeHandler.
type RegressionRangeResponse struct {
	Header     []*alerts.Alert  `json:"header"`
	Table      []*RegressionRow `json:"table"`
	Categories []string         `json:"categories"`
}

// regressionRangeHandler accepts a POST'd JSON serialized RegressionRangeRequest
// and returns a serialized JSON RegressionRangeResponse:
//
//	{
//	  header: [ "query1", "query2", "query3", ...],
//	  table: [
//	    { cid: cid1, columns: [ Regression, Regression, Regression, ...], },
//	    { cid: cid2, columns: [ Regression, null,       Regression, ...], },
//	    { cid: cid3, columns: [ Regression, Regression, Regression, ...], },
//	  ]
//	}
//
// Note that there will be nulls in the columns slice where no Regression have been found.
func (f *Frontend) regressionRangeHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	rr := &RegressionRangeRequest{}
	if err := json.NewDecoder(r.Body).Decode(rr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}
	commitNumberBegin, commitNumberEnd, err := f.unixTimestampRangeToCommitNumberRange(ctx, rr.Begin, rr.End)
	if err != nil {
		httputils.ReportError(w, err, "Invalid time range.", http.StatusInternalServerError)
		return
	}

	// Query for Regressions in the range.
	regMap, err := f.regStore.Range(ctx, commitNumberBegin, commitNumberEnd)
	if err != nil {
		httputils.ReportError(w, err, "Failed to retrieve clusters.", http.StatusInternalServerError)
		return
	}

	headers, err := f.configProvider.GetAllAlertConfigs(ctx, false)
	if err != nil {
		httputils.ReportError(w, err, "Failed to retrieve alert configs.", http.StatusInternalServerError)
		return
	}

	// Build the full list of categories.
	categorySet := util.StringSet{}
	for _, header := range headers {
		categorySet[header.Category] = true
	}

	// Filter down the alerts according to rr.AlertFilter.
	if rr.AlertFilter == alertfilter.OWNER {
		user := f.loginProvider.LoggedInAs(r)
		filteredHeaders := []*alerts.Alert{}
		for _, a := range headers {
			if a.Owner == string(user) {
				filteredHeaders = append(filteredHeaders, a)
			}
		}
		if len(filteredHeaders) > 0 {
			headers = filteredHeaders
		} else {
			sklog.Infof("User doesn't own any alerts.")
		}
	} else if strings.HasPrefix(rr.AlertFilter, "cat:") {
		selectedCategory := rr.AlertFilter[4:]
		filteredHeaders := []*alerts.Alert{}
		for _, a := range headers {
			if a.Category == selectedCategory {
				filteredHeaders = append(filteredHeaders, a)
			}
		}
		if len(filteredHeaders) > 0 {
			headers = filteredHeaders
		} else {
			sklog.Infof("No alert in that category: %q", selectedCategory)
		}
	}

	// Get a list of commits for the range.
	var commits []provider.Commit
	if rr.Subset == SubsetAll {
		commits, err = f.perfGit.CommitSliceFromTimeRange(ctx, time.Unix(rr.Begin, 0), time.Unix(rr.End, 0))
		if err != nil {
			httputils.ReportError(w, err, "Failed to load git info.", http.StatusInternalServerError)
			return
		}
	} else {
		// If rr.Subset == UNTRIAGED_QS or FLAGGED_QS then only get the commits that
		// exactly line up with the regressions in regMap.
		keys := []types.CommitNumber{}
		for k := range regMap {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			return keys[i] < keys[j]
		})
		commits, err = f.perfGit.CommitSliceFromCommitNumberSlice(ctx, keys)
		if err != nil {
			httputils.ReportError(w, err, "Failed to load git info.", http.StatusInternalServerError)
			return
		}

	}

	// Reverse the order of the cids, so the latest
	// commit shows up first in the UI display.
	revCids := make([]provider.Commit, len(commits), len(commits))
	for i, c := range commits {
		revCids[len(commits)-1-i] = c
	}

	categories := categorySet.Keys()
	sort.Strings(categories)

	// Build the RegressionRangeResponse.
	ret := RegressionRangeResponse{
		Header:     headers,
		Table:      []*RegressionRow{},
		Categories: categories,
	}

	for _, cid := range revCids {
		row := &RegressionRow{
			Commit:  cid,
			Columns: make([]*regression.Regression, len(headers), len(headers)),
		}
		count := 0
		if r, ok := regMap[cid.CommitNumber]; ok {
			for i, h := range headers {
				key := h.IDAsString
				if reg, ok := r.ByAlertID[key]; ok {
					if rr.Subset == SubsetUntriaged && reg.Triaged() {
						continue
					}
					row.Columns[i] = reg
					count += 1
				}
			}
		}
		if count == 0 && rr.Subset != SubsetAll {
			continue
		}
		ret.Table = append(ret.Table, row)
	}
	if err := json.NewEncoder(w).Encode(ret); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
	}
}

// CommitDetailsRequest is for deserializing incoming POST requests
// in detailsHandler.
type CommitDetailsRequest struct {
	CommitNumber types.CommitNumber `json:"cid"`
	TraceID      string             `json:"traceid"`
}

func (f *Frontend) detailsHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	includeResults := r.FormValue("results") != "false"
	dr := &CommitDetailsRequest{}
	if err := json.NewDecoder(r.Body).Decode(dr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}

	// If the trace is really a calculation then don't provide any details, but
	// also don't generate an error.
	if !query.IsValid(dr.TraceID) {
		ret := format.Format{
			Version: 0, // Specifying an unacceptable version of the format causes the control to be hidden.
		}
		if err := json.NewEncoder(w).Encode(ret); err != nil {
			sklog.Errorf("writing detailsHandler error response: %s", err)
		}
		return
	}

	name, err := f.traceStore.GetSource(ctx, dr.CommitNumber, dr.TraceID)
	if err != nil {
		httputils.ReportError(w, err, "Failed to load details", http.StatusInternalServerError)
		return
	}

	reader, err := f.ingestedFS.Open(name)
	if err != nil {
		httputils.ReportError(w, err, "Failed to get reader for source file location", http.StatusInternalServerError)
		return
	}
	defer util.Close(reader)
	res := map[string]interface{}{}
	if err := json.NewDecoder(reader).Decode(&res); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON source file", http.StatusInternalServerError)
		return
	}
	if !includeResults {
		delete(res, "results")
	}
	b, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		httputils.ReportError(w, err, "Failed to re-encode JSON source file", http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(b); err != nil {
		sklog.Errorf("Failed to write JSON source file: %s", err)
	}
}

// ShiftRequest is a request to find the timestamps of a range of commits.
type ShiftRequest struct {
	// Begin is the commit number at the beginning of the range.
	Begin types.CommitNumber `json:"begin"`

	// End is the commit number at the end of the range.
	End types.CommitNumber `json:"end"`
}

// ShiftResponse are the timestamps from a ShiftRequest.
type ShiftResponse struct {
	Begin int64 `json:"begin"` // In seconds from the epoch.
	End   int64 `json:"end"`   // In seconds from the epoch.
}

// shiftHandler computes a new begin and end timestamp for a dataframe given
// the current begin and end offsets.
func (f *Frontend) shiftHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	var sr ShiftRequest
	if err := json.NewDecoder(r.Body).Decode(&sr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}
	sklog.Infof("ShiftRequest: %#v", &sr)

	var begin time.Time
	var end time.Time
	var err error

	commit, err := f.perfGit.CommitFromCommitNumber(ctx, sr.Begin)
	if err != nil {
		httputils.ReportError(w, err, "Failed to look up begin commit.", http.StatusBadRequest)
		return
	}
	begin = time.Unix(commit.Timestamp, 0)

	commit, err = f.perfGit.CommitFromCommitNumber(ctx, sr.End)
	if err != nil {
		// If sr.End isn't a valid offset then just use the most recent commit.
		lastCommitNumber, err := f.perfGit.CommitNumberFromTime(ctx, time.Time{})
		if err != nil {
			httputils.ReportError(w, err, "Failed to look up last commit.", http.StatusBadRequest)
			return
		}
		commit, err = f.perfGit.CommitFromCommitNumber(ctx, lastCommitNumber)
		if err != nil {
			httputils.ReportError(w, err, "Failed to look up end commit.", http.StatusBadRequest)
			return
		}
	}
	end = time.Unix(commit.Timestamp, 0)

	resp := ShiftResponse{
		Begin: begin.Unix(),
		End:   end.Unix(),
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to write JSON response: %s", err)
	}
}

func (f *Frontend) alertListHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	show := chi.URLParam(r, "show")
	resp, err := f.configProvider.GetAllAlertConfigs(ctx, show == "true")
	if err != nil {
		httputils.ReportError(w, err, "Failed to retrieve alert configs.", http.StatusInternalServerError)
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to write JSON response: %s", err)
	}
}

func (f *Frontend) alertNewHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(alerts.NewConfig()); err != nil {
		sklog.Errorf("Failed to write JSON response: %s", err)
	}
}

// AlertUpdateResponse is the JSON response when an Alert is created or udpated.
type AlertUpdateResponse struct {
	IDAsString string
}

func refreshConfigProvider(ctx context.Context, configProvider alerts.ConfigProvider) {
	err := configProvider.Refresh(ctx)
	if err != nil {
		sklog.Errorf("Error refreshing alert configs: %s", err)
	}
}

func (f *Frontend) alertUpdateHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	defer refreshConfigProvider(ctx, f.configProvider)
	w.Header().Set("Content-Type", "application/json")

	cfg := &alerts.Alert{}
	if err := json.NewDecoder(r.Body).Decode(cfg); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}

	if !f.isEditor(w, r, "alert-update", cfg) {
		return
	}

	if err := cfg.Validate(); err != nil {
		httputils.ReportError(w, err, "Invalid Alert", http.StatusInternalServerError)
	}

	if err := f.alertStore.Save(ctx, cfg); err != nil {
		httputils.ReportError(w, err, "Failed to save alerts.Config.", http.StatusInternalServerError)
	}
	err := json.NewEncoder(w).Encode(AlertUpdateResponse{
		IDAsString: cfg.IDAsString,
	})
	if err != nil {
		sklog.Errorf("Failed to write JSON response: %s", err)
	}
}

func (f *Frontend) alertDeleteHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	defer refreshConfigProvider(ctx, f.configProvider)
	w.Header().Set("Content-Type", "application/json")

	sid := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(sid, 10, 64)
	if err != nil {
		httputils.ReportError(w, err, "Failed to parse alert id.", http.StatusInternalServerError)
	}

	if !f.isEditor(w, r, "alert-delete", sid) {
		return
	}

	if err := f.alertStore.Delete(ctx, int(id)); err != nil {
		httputils.ReportError(w, err, "Failed to delete the alerts.Config.", http.StatusInternalServerError)
		return
	}
}

// TryBugRequest is a request to try a bug template URI.
type TryBugRequest struct {
	BugURITemplate string `json:"bug_uri_template"`
}

// TryBugResponse is response to a TryBugRequest.
type TryBugResponse struct {
	URL string `json:"url"`
}

func (f *Frontend) alertBugTryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	req := &TryBugRequest{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}

	if !f.isEditor(w, r, "alert-bug-try", req) {
		return
	}

	resp := &TryBugResponse{
		URL: bug.ExampleExpand(req.BugURITemplate),
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode response: %s", err)
	}
}

func (f *Frontend) alertNotifyTryHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	req := &alerts.Alert{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}

	if !f.isEditor(w, r, "alert-notify-try", req) {
		return
	}

	if err := f.notifier.ExampleSend(ctx, req); err != nil {
		httputils.ReportError(w, err, "Failed to send notification: Have you given the service account for this instance Issue Editor permissions on the component?", http.StatusInternalServerError)
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
	http.Redirect(w, r, "/e/", http.StatusMovedPermanently)
}

func oldClustersHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/c/", http.StatusMovedPermanently)
}

func oldAlertsHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/t/", http.StatusMovedPermanently)
}

// createBisectHandler takes the POST'd create bisect request
// then it calls Pinpoint Service API to create bisect job and returns the job id and job url.
func (f *Frontend) createBisectHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	if !f.loginProvider.HasRole(r, roles.Bisecter) {
		http.Error(w, "User is not logged in or is not authorized to start bisect.", http.StatusForbidden)
		return
	}

	if f.pinpoint == nil {
		err := skerr.Fmt("Pinpoint client has not been initialized.")
		httputils.ReportError(w, err, "Create bisect is not enabled for this instance, please check configuration file.", http.StatusInternalServerError)
		return
	}

	var cbr pinpoint.CreateBisectRequest
	if err := json.NewDecoder(r.Body).Decode(&cbr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}
	sklog.Debugf("Got request of creating bisect job: %+v", cbr)

	resp, err := f.pinpoint.CreateBisect(ctx, cbr)
	if err != nil {
		httputils.ReportError(w, err, "Failed to create bisect job.", http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to parse the response of creating bisect job: %s", err)
	}
}

// favoritesHandler returns the favorites config for the instance
func (f *Frontend) favoritesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	fav := config.Favorites{
		Sections: []config.FavoritesSectionConfig{},
	}
	if config.Config.Favorites.Sections != nil {
		fav = config.Config.Favorites
	}
	if err := json.NewEncoder(w).Encode(fav); err != nil {
		sklog.Errorf("Error writing the Favorites json to response: %s", err)
	}
}

// defaultsHandler returns the default settings
func (f *Frontend) defaultsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(config.Config.QueryConfig); err != nil {
		sklog.Errorf("Error writing the default query config json to response: %s", err)
	}
}

// Serve content on the configured endpoints.Serve.
//
// This method does not return.
func (f *Frontend) Serve() {
	// Start the internal server on the internal port if requested.
	if f.flags.InternalPort != "" {
		// Add the profiling endpoints to the internal router.
		internalRouter := chi.NewRouter()

		// Register pprof handlers
		internalRouter.HandleFunc("/debug/pprof/", pprof.Index)
		internalRouter.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		internalRouter.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		internalRouter.HandleFunc("/debug/pprof/profile", pprof.Profile)
		internalRouter.HandleFunc("/debug/pprof/trace", pprof.Trace)
		internalRouter.HandleFunc("/debug/pprof/{profile}", pprof.Index)

		go func() {
			sklog.Infof("Internal server on %q", f.flags.InternalPort)
			sklog.Info(http.ListenAndServe(f.flags.InternalPort, internalRouter))
		}()
	}

	// Resources are served directly.
	router := chi.NewRouter()

	allowedHosts := []string{f.host}
	if len(config.Config.AllowedHosts) > 0 {
		allowedHosts = append(allowedHosts, config.Config.AllowedHosts...)
	}

	router.Use(baseapp.SecurityMiddleware(allowedHosts, f.flags.Local, nil))

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
	router.HandleFunc("/a/", f.templateHandler("alerts.html"))
	router.HandleFunc("/d/", f.templateHandler("dryrunalert.html"))
	router.HandleFunc("/r/", f.templateHandler("trybot.html"))
	router.HandleFunc("/f/", f.templateHandler("favorites.html"))
	router.HandleFunc("/v/", f.templateHandler("revisions.html"))
	router.HandleFunc("/g/{dest:[ect]}/{hash:[a-zA-Z0-9]+}", f.gotoHandler)
	router.HandleFunc("/help/", f.helpHandler)

	// JSON handlers.

	// Common endpoint for all long-running requests.
	router.Get("/_/status/{id:[a-zA-Z0-9-]+}", f.progressTracker.Handler)

	router.Get("/_/alertgroup", f.alertGroupQueryHandler)
	router.HandleFunc("/_/initpage/", f.initpageHandler)
	router.Post("/_/cidRange/", f.cidRangeHandler)
	router.Post("/_/count/", f.countHandler)
	router.Post("/_/cid/", f.cidHandler)
	router.Post("/_/keys/", f.keysHandler)

	router.Post("/_/frame/start", f.frameStartHandler)
	router.Post("/_/cluster/start", f.clusterStartHandler)
	router.Post("/_/trybot/load/", f.trybotLoadHandler)
	router.Post("/_/dryrun/start", f.dryrunRequests.StartHandler)

	router.Post("/_/reg/", f.regressionRangeHandler)
	router.Get("/_/reg/count", f.regressionCountHandler)
	router.Post("/_/triage/", f.triageHandler)
	router.HandleFunc("/_/alerts/", f.alertsHandler)
	router.Post("/_/details/", f.detailsHandler)
	router.Post("/_/shift/", f.shiftHandler)
	router.Get("/_/alert/list/{show}", f.alertListHandler)
	router.Get("/_/alert/new", f.alertNewHandler)
	router.Post("/_/alert/update", f.alertUpdateHandler)
	router.Post("/_/alert/delete/{id:[0-9]+}", f.alertDeleteHandler)
	router.Post("/_/alert/bug/try", f.alertBugTryHandler)
	router.Post("/_/alert/notify/try", f.alertNotifyTryHandler)

	router.Get("/_/login/status", f.loginStatus)

	router.Post("/_/shortcut/get", f.getGraphsShortcutHandler)
	router.Post("/_/shortcut/update", f.createGraphsShortcutHandler)

	router.Post("/_/bisect/create", f.createBisectHandler)

	router.Get("/_/favorites/", f.favoritesHandler)
	router.Get("/_/defaults/", f.defaultsHandler)
	router.Get("/_/revision/", f.revisionHandler)
	var h http.Handler = router
	h = httputils.LoggingGzipRequestResponse(h)
	if !f.flags.Local {
		h = httputils.HealthzAndHTTPS(h)
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

// getParamSet returns a fresh paramtools.ParamSet that represents all the
// traces stored in the two most recent tiles in the trace store. It is filtered
// if such filtering is turned on in the config.
func (f *Frontend) getParamSet() paramtools.ReadOnlyParamSet {
	paramSet := f.paramsetRefresher.Get()

	return filterParamSetIfNeeded(paramSet)
}

// filterParamSetIfNeeded filters the paramset if any filters have been specified in
// the query config.
func filterParamSetIfNeeded(paramSet paramtools.ReadOnlyParamSet) paramtools.ReadOnlyParamSet {
	if config.Config.QueryConfig.IncludedParams != nil {
		filteredParamSet := paramtools.NewParamSet()
		for _, key := range config.Config.QueryConfig.IncludedParams {
			if val, ok := paramSet[key]; ok {
				existing, exists := filteredParamSet[key]
				if exists {
					existing = append(existing, val...)
				} else {
					existing = val
				}
				filteredParamSet[key] = existing
			}
		}

		paramSet = paramtools.ReadOnlyParamSet(filteredParamSet)
	}

	return paramSet
}
