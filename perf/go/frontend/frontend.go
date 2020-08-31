// Package frontend contains the Go code that servers the Perf web UI.
package frontend

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/gorilla/mux"
	"github.com/jcgregorio/logger"
	"github.com/spf13/pflag"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/auditlog"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/calc"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sklog/glog_and_cloud"
	"go.skia.org/infra/go/sklog/sklog_impl"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/alertfilter"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/bug"
	"go.skia.org/infra/perf/go/builders"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/dfbuilder"
	"go.skia.org/infra/perf/go/dist"
	"go.skia.org/infra/perf/go/dryrun"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/notify"
	"go.skia.org/infra/perf/go/psrefresh"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/shortcut"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/types"
	"google.golang.org/api/option"
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
)

// Frontend is the server for the Perf web UI.
type Frontend struct {
	perfGit *perfgit.Git

	cidl *cid.CommitIDLookup

	templates *template.Template

	frameRequests *dataframe.RunningFrameRequests

	clusterRequests *regression.RunningRegressionDetectionRequests

	regStore regression.Store

	continuous []*regression.Continuous

	storageClient *storage.Client

	alertStore alerts.Store

	shortcutStore shortcut.Store

	configProvider regression.ConfigProvider

	notifier *notify.Notifier

	traceStore tracestore.TraceStore

	emailAuth *email.GMail

	dryrunRequests *dryrun.Requests

	paramsetRefresher *psrefresh.ParamSetRefresher

	dfBuilder dataframe.DataFrameBuilder

	// distFileSystem is the ./dist directory of files produced by webpack.
	distFileSystem http.FileSystem

	flags *config.FrontendFlags
}

// New returns a new Frontend instance.
//
// We pass in the FlagSet so that we can emit the flag values into the logs.
func New(flags *config.FrontendFlags, fs *pflag.FlagSet) (*Frontend, error) {
	f := &Frontend{
		flags: flags,
	}
	f.initialize(fs)

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
	"dryRunAlert.html",
}

func (f *Frontend) loadTemplates() {
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
}

func (f *Frontend) templateHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if f.flags.Local {
			f.loadTemplates()
		}
		context := SkPerfConfig{
			Radius:         f.flags.Radius,
			KeyOrder:       strings.Split(f.flags.KeyOrder, ","),
			NumShift:       f.flags.NumShift,
			Interesting:    float32(f.flags.Interesting),
			StepUpOnly:     f.flags.StepUpOnly,
			CommitRangeURL: f.flags.CommitRangeURL,
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

// scriptHandler serves up a template as a script.
func (f *Frontend) scriptHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		if f.flags.Local {
			f.loadTemplates()
		}
		if err := f.templates.ExecuteTemplate(w, name, nil); err != nil {
			sklog.Error("Failed to expand template:", err)
		}
	}
}

// newParamsetProvider returns a regression.ParamsetProvider which produces a paramset
// for the current tiles.
//
func newParamsetProvider(pf *psrefresh.ParamSetRefresher) regression.ParamsetProvider {
	return func() paramtools.ParamSet {
		return pf.Get()
	}
}

// newAlertsConfigProvider returns a regression.ConfigProvider which produces a slice
// of alerts.Config to run continuous clustering against.
func (f *Frontend) newAlertsConfigProvider() regression.ConfigProvider {
	return func() ([]*alerts.Alert, error) {
		return f.alertStore.List(context.Background(), false)
	}
}

// initialize the application.
func (f *Frontend) initialize(fs *pflag.FlagSet) {
	rand.Seed(time.Now().UnixNano())

	// Log to stdout.
	glog_and_cloud.SetLogger(
		glog_and_cloud.NewSLogCloudLogger(logger.NewFromOptions(&logger.Options{
			SyncWriter: os.Stdout,
		})),
	)

	// Note that we don't use common.* here, instead doing the setup manually
	// because we are using a pflag.FlagSet instead of the global
	// flag.CommandLine.
	//
	// If this works out then maybe we can fold flag.FlagSet support into
	// common.
	//
	// TODO(jcgregorio) Remove the call to fs.Parse once skiaperf/main.go is
	// removed and everything is run via perfserver.
	if err := fs.Parse(os.Args[1:]); err != nil {
		sklog.Fatal(err)
	}
	fs.VisitAll(func(f *pflag.Flag) {
		sklog.Infof("Flags: --%s=%v", f.Name, f.Value)
	})

	runtime.GOMAXPROCS(runtime.NumCPU())

	// Record UID and GID.
	sklog.Infof("Running as %d:%d", os.Getuid(), os.Getgid())

	// Init metrics.
	metrics2.InitPrometheus(f.flags.PromPort)
	_ = metrics2.NewLiveness("uptime", nil)

	// Init auth.
	redirectURL := fmt.Sprintf("http://localhost%s/oauth2callback/", f.flags.Port)
	if !f.flags.Local {
		redirectURL = login.DEFAULT_REDIRECT_URL
	}
	if f.flags.AuthBypassList == "" {
		f.flags.AuthBypassList = login.DEFAULT_ALLOWED_DOMAINS
	}
	if err := login.Init(redirectURL, f.flags.AuthBypassList, ""); err != nil {
		sklog.Fatalf("Failed to initialize the login system: %s", err)
	}

	// Keep HTTP request metrics.
	severities := sklog_impl.AllSeverities()
	metricLookup := make([]metrics2.Counter, len(severities))
	for _, sev := range severities {
		metricLookup[sev] = metrics2.GetCounter("num_log_lines", map[string]string{"level": sev.String()})
	}
	metricsCallback := func(severity sklog_impl.Severity) {
		metricLookup[severity].Inc(1)
	}
	sklog_impl.SetMetricsCallback(metricsCallback)

	// Load the config file.
	if err := config.Init(f.flags.ConfigFilename); err != nil {
		sklog.Fatal(err)
	}
	if f.flags.ConnectionString != "" {
		config.Config.DataStoreConfig.ConnectionString = f.flags.ConnectionString
	}
	cfg := config.Config

	var err error
	ctx := context.Background()
	f.distFileSystem, err = dist.New()
	if err != nil {
		sklog.Fatal(err)
	}

	scopes := []string{storage.ScopeReadOnly, auth.SCOPE_GERRIT}

	sklog.Info("About to create token source.")
	ts, err := auth.NewDefaultTokenSource(f.flags.Local, scopes...)
	if err != nil {
		sklog.Fatalf("Failed to get TokenSource: %s", err)
	}

	if !f.flags.Local {
		if _, err := gitauth.New(ts, "/tmp/git-cookie", true, ""); err != nil {
			sklog.Fatal(err)
		}
	}

	sklog.Info("About to init GCS.")
	f.storageClient, err = storage.NewClient(ctx, option.WithTokenSource(ts))
	if err != nil {
		sklog.Fatalf("Failed to authenicate to cloud storage: %s", err)
	}

	sklog.Info("About to parse templates.")
	f.loadTemplates()

	sklog.Info("About to build trace store.")

	f.traceStore, err = builders.NewTraceStoreFromConfig(ctx, f.flags.Local, config.Config)
	if err != nil {
		sklog.Fatalf("Failed to build TraceStore: %s", err)
	}

	sklog.Info("About to build paramset refresher.")

	f.paramsetRefresher = psrefresh.NewParamSetRefresher(f.traceStore)
	if err := f.paramsetRefresher.Start(paramsetRefresherPeriod); err != nil {
		sklog.Fatalf("Failed to build paramsetRefresher: %s", err)
	}

	sklog.Info("About to build perfgit.")

	f.perfGit, err = builders.NewPerfGitFromConfig(ctx, f.flags.Local, config.Config)
	if err != nil {
		sklog.Fatalf("Failed to build perfgit.Git: %s", err)
	}

	sklog.Info("About to build dfbuilder.")
	f.dfBuilder = dfbuilder.NewDataFrameBuilderFromTraceStore(f.perfGit, f.traceStore)

	sklog.Info("About to build cidl.")
	f.cidl = cid.New(ctx, f.perfGit, config.Config)

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

	if !f.flags.NoEmail {
		f.emailAuth, err = email.NewFromFiles(f.flags.EmailTokenCacheFile, f.flags.EmailClientSecretFile)
		if err != nil {
			sklog.Fatalf("Failed to create email auth: %v", err)
		}
		f.notifier = notify.New(f.emailAuth, config.Config.URL)
	} else {
		f.notifier = notify.New(notify.NoEmail{}, config.Config.URL)
	}

	f.frameRequests = dataframe.NewRunningFrameRequests(f.perfGit, f.dfBuilder, f.shortcutStore)
	f.clusterRequests = regression.NewRunningRegressionDetectionRequests(f.perfGit, float32(f.flags.Interesting), f.dfBuilder, f.shortcutStore)
	f.regStore, err = builders.NewRegressionStoreFromConfig(ctx, f.flags.Local, cfg)
	if err != nil {
		sklog.Fatalf("Failed to build regression.Store: %s", err)
	}
	f.configProvider = f.newAlertsConfigProvider()
	paramsProvider := newParamsetProvider(f.paramsetRefresher)

	f.dryrunRequests = dryrun.New(f.cidl, f.dfBuilder, f.shortcutStore, paramsProvider, f.perfGit)

	if f.flags.DoClustering {
		go func() {
			for i := 0; i < f.flags.NumContinuousParallel; i++ {
				// Start running continuous clustering looking for regressions.
				time.Sleep(startClusterDelay)
				c := regression.NewContinuous(f.perfGit, f.cidl, f.configProvider, f.regStore, f.shortcutStore, f.flags.NumContinuous, f.flags.Radius, f.notifier, paramsProvider, f.dfBuilder,
					f.flags.Local, config.Config.IngestionConfig.SourceConfig.Project, config.Config.IngestionConfig.FileIngestionTopicName, f.flags.EventDrivenRegressionDetection)
				f.continuous = append(f.continuous, c)
				go c.Run(context.Background())
			}
		}()
	}
}

// helpHandler handles the GET of the main page.
func (f *Frontend) helpHandler(w http.ResponseWriter, r *http.Request) {
	sklog.Infof("Help Handler: %q\n", r.URL.Path)
	if f.flags.Local {
		f.loadTemplates()
	}
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "text/html")
		ctx := calc.NewContext(nil, nil)
		if err := f.templates.ExecuteTemplate(w, "help.html", ctx); err != nil {
			sklog.Error("Failed to expand template:", err)
		}
	}
}

type alertsStatus struct {
	Alerts int `json:"alerts"`
}

func (f *Frontend) alertsHandler(w http.ResponseWriter, r *http.Request) {
	count, err := f.regressionCount(r.Context(), defaultAlertCategory)
	if err != nil {
		httputils.ReportError(w, err, "Failed to load untriaged count.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Add("Access-Control-Allow-Origin", "*")
	resp := alertsStatus{
		Alerts: count,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
}

func (f *Frontend) initpageHandler(w http.ResponseWriter, r *http.Request) {
	resp := &dataframe.FrameResponse{
		DataFrame: &dataframe.DataFrame{
			ParamSet: f.paramsetRefresher.Get(),
		},
		Skps: []int{},
		Msg:  "",
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
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

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
}

// frameStartResponse is serialized as JSON for the response in
// frameStartHandler.
type frameStartResponse struct {
	ID string `json:"id"`
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
	fr := &dataframe.FrameRequest{}
	if err := json.NewDecoder(r.Body).Decode(fr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}
	auditlog.Log(r, "query", fr)
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
	resp := frameStartResponse{
		ID: f.frameRequests.Add(ctx, fr),
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
}

// frameStatus is used to serialize a JSON response in frameStatusHandler.
type frameStatus struct {
	State   dataframe.ProcessState `json:"state"`
	Message string                 `json:"message"`
	Percent float32                `json:"percent"`
}

// frameStatusHandler returns the status of a pending FrameRequest.
//
// See frameStartHandler for more details.
func (f *Frontend) frameStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]
	state, message, percent, err := f.frameRequests.Status(id)
	if err != nil {
		httputils.ReportError(w, err, message, http.StatusInternalServerError)
		return
	}

	resp := frameStatus{
		State:   state,
		Message: message,
		Percent: percent,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode response: %s %#v", err, resp)
	}
}

// frameResultsHandler returns the results of a pending FrameRequest.
//
// See frameStatusHandler for more details.
func (f *Frontend) frameResultsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]
	df, err := f.frameRequests.Response(id)
	if err != nil {
		httputils.ReportError(w, err, "Async processing of frame failed.", http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(df); err != nil {
		sklog.Errorf("Failed to encode response: %s", err)
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
	Count    int                 `json:"count"`
	Paramset paramtools.ParamSet `json:"paramset"`
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
	if cr.Q == "" {
		ps := f.paramsetRefresher.Get()
		resp.Count = 0
		resp.Paramset = ps
	} else {
		count, ps, err := f.dfBuilder.PreflightQuery(r.Context(), time.Now(), q)
		if err != nil {
			httputils.ReportError(w, err, "Failed to Preflight the query, too many key-value pairs selected. Limit is 200.", http.StatusBadRequest)
			return
		}

		resp.Count = int(count)
		resp.Paramset = ps
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
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
	resp := make([]perfgit.Commit, len(cids))
	resp, err := f.perfGit.CommitSliceFromCommitNumberSlice(ctx, cids)
	if err != nil {
		httputils.ReportError(w, err, "Failed to lookup all commit ids", http.StatusInternalServerError)
		return
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
// long running Go routine to do the actual regression detection. The ID of the
// long running routine is returned to be used in subsequent calls to
// clusterStatusHandler to check on the status of the work.
func (f *Frontend) clusterStartHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	req := &regression.RegressionDetectionRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Could not decode POST body.", http.StatusInternalServerError)
		return
	}
	auditlog.Log(r, "cluster", req)
	id, err := f.clusterRequests.Add(context.Background(), req)
	sklog.Infof("Added to clusterRequests")
	if err != nil {
		httputils.ReportError(w, err, "Cluster request was invalid", http.StatusInternalServerError)
		return
	}
	resp := ClusterStartResponse{
		ID: id,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
}

// ClusterStatus is used to serialize a JSON response in clusterStatusHandler.
type ClusterStatus struct {
	State   regression.ProcessState                 `json:"state"`
	Message string                                  `json:"message"`
	Value   *regression.RegressionDetectionResponse `json:"value"`
}

// clusterStatusHandler is used to check on the status of a long running cluster
// request. The ID of the routine is passed in via the URL path. A JSON
// serialized ClusterStatus is returned, with ClusterStatus.Value being
// populated only when the clustering process has finished.
func (f *Frontend) clusterStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]

	status := &ClusterStatus{}
	state, msg, err := f.clusterRequests.Status(id)
	if err != nil {
		httputils.ReportError(w, err, msg, http.StatusInternalServerError)
		return
	}
	status.State = state
	status.Message = msg
	if state == regression.ProcessSuccess {
		value, err := f.clusterRequests.Response(id)
		if err != nil {
			httputils.ReportError(w, err, "Failed to retrieve results.", http.StatusInternalServerError)
			return
		}
		status.Value = value
	}

	if err := json.NewEncoder(w).Encode(status); err != nil {
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
	details, err := f.cidl.Lookup(ctx, []*cid.CommitID{
		{
			Offset: types.CommitNumber(begin),
		},
		{
			Offset: types.CommitNumber(end),
		},
	})
	if err != nil {
		httputils.ReportError(w, err, "Could not convert indices to hashes.", http.StatusInternalServerError)
		return
	}
	beginTime := details[0].Timestamp
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

// TriageRequest is used in triageHandler.
type TriageRequest struct {
	Cid         *cid.CommitID           `json:"cid"`
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
	w.Header().Set("Content-Type", "application/json")
	if login.LoggedInAs(r) == "" {
		httputils.ReportError(w, fmt.Errorf("Not logged in."), "You must be logged in to triage.", http.StatusInternalServerError)
		return
	}
	tr := &TriageRequest{}
	if err := json.NewDecoder(r.Body).Decode(tr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}
	auditlog.Log(r, "triage", tr)
	detail, err := f.cidl.Lookup(context.Background(), []*cid.CommitID{tr.Cid})
	if err != nil {
		httputils.ReportError(w, err, "Failed to find CommitID.", http.StatusInternalServerError)
		return
	}

	key := tr.Alert.IDAsString
	if tr.ClusterType == "low" {
		err = f.regStore.TriageLow(r.Context(), detail[0].Offset, key, tr.Triage)
	} else {
		err = f.regStore.TriageHigh(r.Context(), detail[0].Offset, key, tr.Triage)
	}

	if err != nil {
		httputils.ReportError(w, err, "Failed to triage.", http.StatusInternalServerError)
		return
	}
	link := fmt.Sprintf("%s/t/?begin=%d&end=%d&subset=all", r.Header.Get("Origin"), detail[0].Timestamp, detail[0].Timestamp+1)

	resp := &TriageResponse{}

	if tr.Triage.Status == regression.Negative {
		cfgs, err := f.configProvider()
		if err != nil {
			sklog.Errorf("Failed to load configs looking for BugURITemplate: %s", err)
		}
		uritemplate := defaultBugURLTemplate
		for _, c := range cfgs {
			if c.ID == tr.Alert.ID {
				uritemplate = c.BugURITemplate
				break
			}
		}
		resp.Bug = bug.Expand(uritemplate, link, detail[0], tr.Triage.Message)
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
	Id      *cid.CommitDetail        `json:"cid"`
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
		user := login.LoggedInAs(r)
		filteredHeaders := []*alerts.Alert{}
		for _, a := range headers {
			if a.Owner == user {
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
	var ids []*cid.CommitID
	if rr.Subset == SubsetAll {
		commits, err := f.perfGit.CommitSliceFromTimeRange(r.Context(), time.Unix(rr.Begin, 0), time.Unix(rr.End, 0))
		if err != nil {
			httputils.ReportError(w, err, "Failed to load git info.", http.StatusInternalServerError)
			return
		}
		ids = make([]*cid.CommitID, len(commits), len(commits))
		for i, c := range commits {
			ids[i] = &cid.CommitID{
				Offset: c.CommitNumber,
			}
		}
	} else {
		// If rr.Subset == UNTRIAGED_QS or FLAGGED_QS then only get the commits that
		// exactly line up with the regressions in regMap.
		ids = make([]*cid.CommitID, 0, len(regMap))
		keys := []types.CommitNumber{}
		for k := range regMap {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			return keys[i] < keys[j]
		})
		for _, key := range keys {
			c := &cid.CommitID{
				Offset: key,
			}
			if err != nil {
				httputils.ReportError(w, err, "Got an invalid commit id.", http.StatusInternalServerError)
				return
			}
			ids = append(ids, c)
		}
	}

	// Convert the CommitIDs to CommitDetails.
	cids, err := f.cidl.Lookup(ctx, ids)
	if err != nil {
		httputils.ReportError(w, err, "Failed to look up commit details", http.StatusInternalServerError)
		return
	}

	// Reverse the order of the cids, so the latest
	// commit shows up first in the UI display.
	revCids := make([]*cid.CommitDetail, len(cids), len(cids))
	for i, c := range cids {
		revCids[len(cids)-1-i] = c
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
			Id:      cid,
			Columns: make([]*regression.Regression, len(headers), len(headers)),
		}
		count := 0
		if r, ok := regMap[types.CommitNumber(cid.Offset)]; ok {
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
	status := []regression.Current{}
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
	u, err := url.Parse(name)
	if err != nil {
		httputils.ReportError(w, err, "Failed to parse source file location.", http.StatusInternalServerError)
		return
	}
	if u.Host == "" || u.Path == "" {
		httputils.ReportError(w, fmt.Errorf("Invalid source location: %q", name), "Invalid source location.", http.StatusInternalServerError)
		return
	}
	sklog.Infof("Host: %q Path: %q", u.Host, u.Path)
	reader, err := f.storageClient.Bucket(u.Host).Object(u.Path[1:]).NewReader(context.Background())
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
	if login.LoggedInAs(r) == "" {
		httputils.ReportError(w, fmt.Errorf("Not logged in."), "You must be logged in to edit alerts.", http.StatusInternalServerError)
		return
	}

	cfg := &alerts.Alert{}
	if err := json.NewDecoder(r.Body).Decode(cfg); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}
	auditlog.Log(r, "alert-update", cfg)
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
	if login.LoggedInAs(r) == "" {
		httputils.ReportError(w, fmt.Errorf("Not logged in."), "You must be logged in to delete alerts.", http.StatusInternalServerError)
		return
	}

	sid := mux.Vars(r)["id"]
	id, err := strconv.ParseInt(sid, 10, 64)
	if err != nil {
		httputils.ReportError(w, err, "Failed to parse alert id.", http.StatusInternalServerError)
	}
	auditlog.Log(r, "alert-delete", sid)
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

func alertBugTryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if login.LoggedInAs(r) == "" {
		httputils.ReportError(w, fmt.Errorf("Not logged in."), "You must be logged in to test alerts.", http.StatusInternalServerError)
		return
	}

	req := &TryBugRequest{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}
	auditlog.Log(r, "alert-bug-try", req)
	resp := &TryBugResponse{
		URL: bug.ExampleExpand(req.BugURITemplate),
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode response: %s", err)
	}
}

func (f *Frontend) alertNotifyTryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if login.LoggedInAs(r) == "" {
		httputils.ReportError(w, fmt.Errorf("Not logged in."), "You must be logged in to try alerts.", http.StatusInternalServerError)
		return
	}

	req := &alerts.Alert{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}
	auditlog.Log(r, "alert-notify-try", req)
	if err := f.notifier.ExampleSend(req); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to send email: %s", err), http.StatusInternalServerError)
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

var internalOnlyExceptions = []string{
	"/oauth2callback/",
	"/_/reg/count",
}

// internalOnlyHandler wraps the handler with a handler that only allows
// authenticated access, with the exception of the endpoints listed in
// internalOnlyExceptions.
func internalOnlyHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if util.In(r.URL.Path, internalOnlyExceptions) || login.LoggedInAs(r) != "" {
			h.ServeHTTP(w, r)
		} else {
			http.Redirect(w, r, login.LoginURL(w, r), http.StatusTemporaryRedirect)
		}
	})
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
	router.HandleFunc("/d/", f.templateHandler("dryRunAlert.html"))
	router.HandleFunc("/g/{dest:[ect]}/{hash:[a-zA-Z0-9]+}", f.gotoHandler)
	router.HandleFunc("/help/", f.helpHandler)
	router.HandleFunc("/logout/", login.LogoutHandler)
	router.HandleFunc("/loginstatus/", login.StatusHandler)
	router.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)

	// JSON handlers.
	router.HandleFunc("/_/initpage/", f.initpageHandler)
	router.HandleFunc("/_/cidRange/", f.cidRangeHandler).Methods("POST")
	router.HandleFunc("/_/count/", f.countHandler).Methods("POST")
	router.HandleFunc("/_/cid/", f.cidHandler).Methods("POST")
	router.HandleFunc("/_/keys/", f.keysHandler).Methods("POST")
	router.HandleFunc("/_/frame/start", f.frameStartHandler).Methods("POST")
	router.HandleFunc("/_/frame/status/{id:[a-zA-Z0-9]+}", f.frameStatusHandler).Methods("GET")
	router.HandleFunc("/_/frame/results/{id:[a-zA-Z0-9]+}", f.frameResultsHandler).Methods("GET")
	router.HandleFunc("/_/cluster/start", f.clusterStartHandler).Methods("POST")
	router.HandleFunc("/_/cluster/status/{id:[a-zA-Z0-9]+}", f.clusterStatusHandler).Methods("GET")

	router.HandleFunc("/_/dryrun/start", f.dryrunRequests.StartHandler).Methods("POST")
	router.HandleFunc("/_/dryrun/status/{id:[a-zA-Z0-9]+}", f.dryrunRequests.StatusHandler).Methods("GET")

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
	router.HandleFunc("/_/alert/bug/try", alertBugTryHandler).Methods("POST")
	router.HandleFunc("/_/alert/notify/try", f.alertNotifyTryHandler).Methods("POST")

	var h http.Handler = router
	if f.flags.InternalOnly {
		h = internalOnlyHandler(h)
	}
	h = httputils.LoggingGzipRequestResponse(h)
	if !f.flags.Local {
		h = httputils.HealthzAndHTTPS(h)
	}
	http.Handle("/", h)

	sklog.Info("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(f.flags.Port, nil))
}
