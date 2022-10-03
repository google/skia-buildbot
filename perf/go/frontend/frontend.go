// Package frontend contains the Go code that servers the Perf web UI.
package frontend

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"io/ioutil"
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

	"github.com/gorilla/mux"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/alogin/proxylogin"
	"go.skia.org/infra/go/auditlog"
	"go.skia.org/infra/go/calc"
	"go.skia.org/infra/go/email"
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
	"go.skia.org/infra/perf/go/bug"
	"go.skia.org/infra/perf/go/builders"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/dfbuilder"
	"go.skia.org/infra/perf/go/dryrun"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/notify"
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
)

// Frontend is the server for the Perf web UI.
type Frontend struct {
	perfGit *perfgit.Git

	templates *template.Template

	loadTemplatesOnce sync.Once

	regStore regression.Store

	continuous []*continuous.Continuous

	// provides access to the ingested files.
	ingestedFS fs.FS

	alertStore alerts.Store

	shortcutStore shortcut.Store

	configProvider continuous.ConfigProvider

	notifier *notify.Notifier

	traceStore tracestore.TraceStore

	emailAuth *email.GMail

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
	b, err := ioutil.ReadAll(f)
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
	"clusters2.html",
	"triage.html",
	"alerts.html",
	"help.html",
	"dryrunalert.html",
	"trybot.html",
}

func (f *Frontend) loadTemplatesImpl() {
	f.templates = template.New("")
	for _, filename := range templateFilenames {
		contents, err := fileContentsFromFileSystem(f.distFileSystem, filename)
		if err != nil {
			sklog.Fatal(err)
		}
		f.templates = f.templates.New(filename)
		_, err = f.templates.Parse(contents)
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
// in Javascript under the sk.perf variable.
type SkPerfConfig struct {
	Radius         int      `json:"radius"`           // The number of commits when doing clustering.
	KeyOrder       []string `json:"key_order"`        // The order of the keys to appear first in query-sk elements.
	NumShift       int      `json:"num_shift"`        // The number of commits the shift navigation buttons should jump.
	Interesting    float32  `json:"interesting"`      // The threshold for a cluster to be interesting.
	StepUpOnly     bool     `json:"step_up_only"`     // If true then only regressions that are a step up are displayed.
	CommitRangeURL string   `json:"commit_range_url"` // A URI Template to be used for expanding details on a range of commits. See cluster-summary2-sk.
	Demo           bool     `json:"demo"`             // True if this is a demo page, as opposed to being in production. Used to make puppeteer tests deterministic.
	DisplayGroupBy bool     `json:"display_group_by"` // True if the Group By section of Alert config should be displayed.
}

func (f *Frontend) templateHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		f.loadTemplates()
		context := SkPerfConfig{
			Radius:         f.flags.Radius,
			KeyOrder:       strings.Split(f.flags.KeyOrder, ","),
			NumShift:       f.flags.NumShift,
			Interesting:    float32(f.flags.Interesting),
			StepUpOnly:     f.flags.StepUpOnly,
			CommitRangeURL: f.flags.CommitRangeURL,
			DisplayGroupBy: f.flags.DisplayGroupBy,
		}
		b, err := json.MarshalIndent(context, "", "  ")
		if err != nil {
			sklog.Errorf("Failed to JSON encode sk.perf context: %s", err)
		}
		if err := f.templates.ExecuteTemplate(w, name, map[string]template.JS{"context": template.JS(string(b))}); err != nil {
			sklog.Error("Failed to expand template:", err)
		}
	}
}

// newParamsetProvider returns a regression.ParamsetProvider which produces a paramset
// for the current tiles.
//
func newParamsetProvider(pf *psrefresh.ParamSetRefresher) regression.ParamsetProvider {
	return func() paramtools.ReadOnlyParamSet {
		return pf.Get()
	}
}

// newAlertsConfigProvider returns a regression.ConfigProvider which produces a slice
// of alerts.Config to run continuous clustering against.
func (f *Frontend) newAlertsConfigProvider() continuous.ConfigProvider {
	return func() ([]*alerts.Alert, error) {
		return f.alertStore.List(context.Background(), false)
	}
}

// initialize the application.
func (f *Frontend) initialize() {
	rand.Seed(time.Now().UnixNano())

	runtime.GOMAXPROCS(runtime.NumCPU())

	if err := tracing.Init(f.flags.Local); err != nil {
		sklog.Fatalf("Failed to start tracing: %s", err)
	}

	// Record UID and GID.
	sklog.Infof("Running as %d:%d", os.Getuid(), os.Getgid())

	// Init metrics.
	metrics2.InitPrometheus(f.flags.PromPort)
	_ = metrics2.NewLiveness("uptime", nil)

	// Add tracker for long running requests.
	var err error
	f.progressTracker, err = progress.NewTracker("/_/status/")
	if err != nil {
		sklog.Fatalf("Failed to initialize Tracker: %s", err)
	}
	f.progressTracker.Start(context.Background())

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
	if err := config.Init(f.flags.ConfigFilename); err != nil {
		sklog.Fatal(err)
	}
	if f.flags.ConnectionString != "" {
		config.Config.DataStoreConfig.ConnectionString = f.flags.ConnectionString
	}
	cfg := config.Config

	// Configure login.
	f.loginProvider, err = proxylogin.New(
		cfg.AuthConfig.HeaderName,
		cfg.AuthConfig.EmailRegex,
		cfg.AuthConfig.LoginURL,
		cfg.AuthConfig.LogoutURL)
	if err != nil {
		sklog.Fatalf("Failed to initialize login: %s", err)
	}

	ctx := context.Background()

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

	// Update the git repo periodically since perfGit.LogEntry does interrogate
	// the git repo itself instead of using the SQL backend.
	//
	// TODO(jcgregorio) Remove once perfgit stores full commit messages.
	go func() {
		for range time.Tick(gitRepoUpdatePeriod) {
			if err := f.perfGit.Update(ctx); err != nil {
				sklog.Errorf("Failed to update git repo: %s", err)
			}
		}
	}()

	sklog.Info("About to build dfbuilder.")
	f.dfBuilder = dfbuilder.NewDataFrameBuilderFromTraceStore(f.perfGit, f.traceStore, f.flags.NumParamSetsForQueries)

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

	if f.flags.NoEmail {
		f.notifier = notify.New(notify.NoEmail{}, config.Config.URL)
	} else {
		f.notifier = notify.New(notify.NewEmailService(), config.Config.URL)
	}

	f.regStore, err = builders.NewRegressionStoreFromConfig(ctx, f.flags.Local, cfg)
	if err != nil {
		sklog.Fatalf("Failed to build regression.Store: %s", err)
	}
	f.configProvider = f.newAlertsConfigProvider()
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
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "text/html")
		ctx := calc.NewContext(nil, nil)
		if err := f.templates.ExecuteTemplate(w, "help.html", ctx); err != nil {
			sklog.Error("Failed to expand template:", err)
		}
	}
}

func (f *Frontend) alertsHandler(w http.ResponseWriter, r *http.Request) {
	count, err := f.regressionCount(r.Context(), defaultAlertCategory)
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

func (f *Frontend) initpageHandler(w http.ResponseWriter, r *http.Request) {
	resp := &frame.FrameResponse{
		DataFrame: &dataframe.DataFrame{
			ParamSet: f.paramsetRefresher.Get(),
		},
		Skps: []int{},
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
}

func (f *Frontend) trybotLoadHandler(w http.ResponseWriter, r *http.Request) {
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
	w.Header().Set("Content-Type", "application/json")
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
	ctx := r.Context()
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
	ret := []perfgit.Commit{}
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
//  * Start building the DataFrame (_/frame/start), which returns an identifier of the long
//    running request, {id}.
//  * Query the status of the running request (_/frame/status/{id}).
//  * Finally return the constructed DataFrame (_/frame/results/{id}).
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

	ctx, span := trace.StartSpan(context.Background(), "frameStartRequest")
	defer span.End()
	f.progressTracker.Add(fr.Progress)

	go func() {
		err := frame.ProcessFrameRequest(ctx, fr, f.perfGit, f.dfBuilder, f.shortcutStore)
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
	fullPS := f.paramsetRefresher.Get()
	if cr.Q == "" {
		resp.Count = 0
		resp.Paramset = fullPS
	} else {
		count, ps, err := f.dfBuilder.PreflightQuery(r.Context(), q, fullPS)
		if err != nil {
			httputils.ReportError(w, err, "Failed to Preflight the query, too many key-value pairs selected. Limit is 200.", http.StatusBadRequest)
			return
		}

		resp.Count = int(count)
		resp.Paramset = ps.Freeze()
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
}

// CIDHandlerResponse is the form of the response from the /_/cid/ endpoint.
type CIDHandlerResponse struct {
	// CommitSlice describes all the commits requested.
	CommitSlice []perfgit.Commit `json:"commitSlice"`

	// LogEntry is the full git log entry for the first commit in the
	// CommitSlice.
	LogEntry string `json:"logEntry"`
}

// cidHandler takes the POST'd list of dataframe.ColumnHeaders, and returns a
// serialized slice of cid.CommitDetails.
func (f *Frontend) cidHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "application/json")
	cids := []types.CommitNumber{}
	if err := json.NewDecoder(r.Body).Decode(&cids); err != nil {
		httputils.ReportError(w, err, "Could not decode POST body.", http.StatusInternalServerError)
		return
	}

	// If alerts.DefaultSparse is true then we only respond with information
	// about the very first commit.
	if alerts.DefaultSparse {
		cids = cids[:1]
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

	cb := func(_ *regression.RegressionDetectionRequest, clusterResponse []*regression.RegressionDetectionResponse, _ string) {
		// We don't do GroupBy clustering, so there will only be one clusterResponse.
		req.Progress.Results(clusterResponse[0])
	}
	f.progressTracker.Add(req.Progress)

	go func() {
		err := regression.ProcessRegressions(context.Background(), req, cb, f.perfGit, f.shortcutStore, f.dfBuilder, f.paramsetRefresher.Get(), regression.ExpandBaseAlertByGroupBy, regression.ReturnOnError)
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
//    {
//       "keys": [
//            ",arch=x86,...",
//            ",arch=x86,...",
//       ]
//    }
//
// And returns the ID of the new shortcut to that list of keys:
//
//   {
//     "id": 123456,
//   }
func (f *Frontend) keysHandler(w http.ResponseWriter, r *http.Request) {
	id, err := f.shortcutStore.Insert(r.Context(), r.Body)
	if err != nil {
		httputils.ReportError(w, err, "Error inserting shortcut.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
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
	if r.Method != "GET" {
		http.Error(w, "Method not allowed.", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		httputils.ReportError(w, err, "Could not parse query parameters.", http.StatusInternalServerError)
		return
	}
	ctx := context.Background()
	query := r.Form
	hash := mux.Vars(r)["hash"]
	dest := mux.Vars(r)["dest"]
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
	// Always back up on second since we had an issue with duplicate times for
	// commits: skbug.com/10698.
	beginTime := details[0].Timestamp - 1
	endTime := details[1].Timestamp + 1
	query.Set("begin", fmt.Sprintf("%d", beginTime))
	query.Set("end", fmt.Sprintf("%d", endTime))

	if dest == "e" {
		http.Redirect(w, r, fmt.Sprintf("/e/?%s", query.Encode()), http.StatusFound)
	} else if dest == "c" {
		query.Set("offset", fmt.Sprintf("%d", index))
		http.Redirect(w, r, fmt.Sprintf("/c/?%s", query.Encode()), http.StatusFound)
	} else if dest == "t" {
		query.Set("subset", "all")
		http.Redirect(w, r, fmt.Sprintf("/t/?%s", query.Encode()), http.StatusFound)
	}
}

func (f *Frontend) isEditor(w http.ResponseWriter, r *http.Request, action string, body interface{}) bool {
	user := f.loginProvider.LoggedInAs(r)
	if f.loginProvider.HasRole(r, roles.Editor) {
		httputils.ReportError(w, fmt.Errorf("Not logged in."), "You must be logged in to complete this action.", http.StatusInternalServerError)
		return false
	}
	auditlog.LogWithUser(r, user.String(), "triage", body)
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
	ctx := r.Context()
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
		err = f.regStore.TriageLow(r.Context(), detail.CommitNumber, key, tr.Triage)
	} else {
		err = f.regStore.TriageHigh(r.Context(), detail.CommitNumber, key, tr.Triage)
	}

	if err != nil {
		httputils.ReportError(w, err, "Failed to triage.", http.StatusInternalServerError)
		return
	}
	link := fmt.Sprintf("%s/t/?begin=%d&end=%d&subset=all", r.Header.Get("Origin"), detail.Timestamp, detail.Timestamp+1)

	resp := &TriageResponse{}

	if tr.Triage.Status == regression.Negative {
		cfgs, err := f.configProvider()
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
	configs, err := f.configProvider()
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
	regMap, err := f.regStore.Range(context.Background(), commitNumberBegin, commitNumberEnd)
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
	w.Header().Set("Content-Type", "application/json")
	category := r.FormValue("cat")

	count, err := f.regressionCount(r.Context(), category)
	if err != nil {
		httputils.ReportError(w, err, "Failed to count regressions.", http.StatusInternalServerError)
	}

	if err := json.NewEncoder(w).Encode(struct{ Count int }{Count: count}); err != nil {
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
	Commit  perfgit.Commit           `json:"cid"`
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
//    {
//      header: [ "query1", "query2", "query3", ...],
//      table: [
//        { cid: cid1, columns: [ Regression, Regression, Regression, ...], },
//        { cid: cid2, columns: [ Regression, null,       Regression, ...], },
//        { cid: cid3, columns: [ Regression, Regression, Regression, ...], },
//      ]
//    }
//
// Note that there will be nulls in the columns slice where no Regression have been found.
func (f *Frontend) regressionRangeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := context.Background()
	rr := &RegressionRangeRequest{}
	if err := json.NewDecoder(r.Body).Decode(rr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}
	commitNumberBegin, commitNumberEnd, err := f.unixTimestampRangeToCommitNumberRange(r.Context(), rr.Begin, rr.End)
	if err != nil {
		httputils.ReportError(w, err, "Invalid time range.", http.StatusInternalServerError)
		return
	}

	// Query for Regressions in the range.
	regMap, err := f.regStore.Range(r.Context(), commitNumberBegin, commitNumberEnd)
	if err != nil {
		httputils.ReportError(w, err, "Failed to retrieve clusters.", http.StatusInternalServerError)
		return
	}

	headers, err := f.configProvider()
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
	var commits []perfgit.Commit
	if rr.Subset == SubsetAll {
		commits, err = f.perfGit.CommitSliceFromTimeRange(r.Context(), time.Unix(rr.Begin, 0), time.Unix(rr.End, 0))
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
	revCids := make([]perfgit.Commit, len(commits), len(commits))
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

func (f *Frontend) regressionCurrentHandler(w http.ResponseWriter, r *http.Request) {
	status := []continuous.Current{}
	for _, c := range f.continuous {
		status = append(status, c.CurrentStatus())
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		sklog.Errorf("Failed to encode status: %s", err)
	}
}

// CommitDetailsRequest is for deserializing incoming POST requests
// in detailsHandler.
type CommitDetailsRequest struct {
	CommitNumber types.CommitNumber `json:"cid"`
	TraceID      string             `json:"traceid"`
}

func (f *Frontend) detailsHandler(w http.ResponseWriter, r *http.Request) {
	includeResults := r.FormValue("results") != "false"
	w.Header().Set("Content-Type", "application/json")
	dr := &CommitDetailsRequest{}
	if err := json.NewDecoder(r.Body).Decode(dr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}

	var err error
	name := ""
	name, err = f.traceStore.GetSource(r.Context(), dr.CommitNumber, dr.TraceID)
	if err != nil {
		httputils.ReportError(w, err, "Failed to load details", http.StatusInternalServerError)
		return
	}

	sklog.Infof("Full URL to source: %q", name)
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
	w.Header().Set("Content-Type", "application/json")
	var sr ShiftRequest
	if err := json.NewDecoder(r.Body).Decode(&sr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}
	sklog.Infof("ShiftRequest: %#v", &sr)

	ctx := r.Context()
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
	w.Header().Set("Content-Type", "application/json")
	show := mux.Vars(r)["show"]
	resp, err := f.alertStore.List(r.Context(), show == "true")
	if err != nil {
		httputils.ReportError(w, err, "Failed to retrieve alert configs.", http.StatusInternalServerError)
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to write JSON response: %s", err)
	}
}

func alertNewHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(alerts.NewConfig()); err != nil {
		sklog.Errorf("Failed to write JSON response: %s", err)
	}
}

// AlertUpdateResponse is the JSON response when an Alert is created or udpated.
type AlertUpdateResponse struct {
	IDAsString string
}

func (f *Frontend) alertUpdateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	cfg := &alerts.Alert{}
	if err := json.NewDecoder(r.Body).Decode(cfg); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}

	if !f.isEditor(w, r, "alert-update", cfg) {
		return
	}

	if err := f.alertStore.Save(r.Context(), cfg); err != nil {
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
	w.Header().Set("Content-Type", "application/json")

	sid := mux.Vars(r)["id"]
	id, err := strconv.ParseInt(sid, 10, 64)
	if err != nil {
		httputils.ReportError(w, err, "Failed to parse alert id.", http.StatusInternalServerError)
	}

	if !f.isEditor(w, r, "alert-delete", sid) {
		return
	}

	if err := f.alertStore.Delete(r.Context(), int(id)); err != nil {
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
	w.Header().Set("Content-Type", "application/json")

	req := &alerts.Alert{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}

	if !f.isEditor(w, r, "alert-notify-try", req) {
		return
	}

	if err := f.notifier.ExampleSend(r.Context(), req); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to send email: %s", err), http.StatusInternalServerError)
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

// Serve content on the configured endpoints.Serve.
//
// This method does not return.
func (f *Frontend) Serve() {
	// Start the internal server on the internal port if requested.
	if f.flags.InternalPort != "" {
		// Add the profiling endpoints to the internal router.
		internalRouter := mux.NewRouter()

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
	router := mux.NewRouter()

	router.PathPrefix("/dist/").HandlerFunc(f.makeDistHandler())

	// Redirects for the old Perf URLs.
	router.HandleFunc("/", oldMainHandler)
	router.HandleFunc("/clusters/", oldClustersHandler)
	router.HandleFunc("/alerts/", oldAlertsHandler)

	// New endpoints that use ptracestore will go here.
	router.HandleFunc("/e/", f.templateHandler("newindex.html"))
	router.HandleFunc("/c/", f.templateHandler("clusters2.html"))
	router.HandleFunc("/t/", f.templateHandler("triage.html"))
	router.HandleFunc("/a/", f.templateHandler("alerts.html"))
	router.HandleFunc("/d/", f.templateHandler("dryrunalert.html"))
	router.HandleFunc("/r/", f.templateHandler("trybot.html"))
	router.HandleFunc("/g/{dest:[ect]}/{hash:[a-zA-Z0-9]+}", f.gotoHandler)
	router.HandleFunc("/help/", f.helpHandler)

	// JSON handlers.

	// Common endpoint for all long-running requests.
	router.HandleFunc("/_/status/{id:[a-zA-Z0-9-]+}", f.progressTracker.Handler).Methods("GET")

	router.HandleFunc("/_/initpage/", f.initpageHandler)
	router.HandleFunc("/_/cidRange/", f.cidRangeHandler).Methods("POST")
	router.HandleFunc("/_/count/", f.countHandler).Methods("POST")
	router.HandleFunc("/_/cid/", f.cidHandler).Methods("POST")
	router.HandleFunc("/_/keys/", f.keysHandler).Methods("POST")

	router.HandleFunc("/_/frame/start", f.frameStartHandler).Methods("POST")
	router.HandleFunc("/_/cluster/start", f.clusterStartHandler).Methods("POST")
	router.HandleFunc("/_/trybot/load/", f.trybotLoadHandler).Methods("POST")
	router.HandleFunc("/_/dryrun/start", f.dryrunRequests.StartHandler).Methods("POST")

	router.HandleFunc("/_/reg/", f.regressionRangeHandler).Methods("POST")
	router.HandleFunc("/_/reg/count", f.regressionCountHandler).Methods("GET")
	router.HandleFunc("/_/reg/current", f.regressionCurrentHandler).Methods("GET")
	router.HandleFunc("/_/triage/", f.triageHandler).Methods("POST")
	router.HandleFunc("/_/alerts/", f.alertsHandler)
	router.HandleFunc("/_/details/", f.detailsHandler).Methods("POST")
	router.HandleFunc("/_/shift/", f.shiftHandler).Methods("POST")
	router.HandleFunc("/_/alert/list/{show}", f.alertListHandler).Methods("GET")
	router.HandleFunc("/_/alert/new", alertNewHandler).Methods("GET")
	router.HandleFunc("/_/alert/update", f.alertUpdateHandler).Methods("POST")
	router.HandleFunc("/_/alert/delete/{id:[0-9]+}", f.alertDeleteHandler).Methods("POST")
	router.HandleFunc("/_/alert/bug/try", f.alertBugTryHandler).Methods("POST")
	router.HandleFunc("/_/alert/notify/try", f.alertNotifyTryHandler).Methods("POST")

	router.HandleFunc("/_/login/status", f.loginStatus).Methods("GET")

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
