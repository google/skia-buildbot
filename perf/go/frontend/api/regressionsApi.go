package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/auditlog"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/alertfilter"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/bug"
	"go.skia.org/infra/perf/go/chromeperf"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/graphsshortcut"
	"go.skia.org/infra/perf/go/notifytypes"
	"go.skia.org/infra/perf/go/progress"
	"go.skia.org/infra/perf/go/psrefresh"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/shortcut"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/urlprovider"
)

const (
	// regressionCountDuration is how far back we look for regression in the /_/reg/count endpoint.
	regressionCountDuration = -14 * 24 * time.Hour

	// defaultAlertCategory is the category that will be used by the /_/alerts/ endpoint.
	defaultAlertCategory = "Prod"

	// defaultBugURLTemplate is the URL template to use if the user
	// doesn't supply one.
	defaultBugURLTemplate = "https://bugs.chromium.org/p/skia/issues/entry?comment=This+bug+was+found+via+SkiaPerf.%0A%0AVisit+this+URL+to+see+the+details+of+the+suspicious+cluster%3A%0A%0A++{cluster_url}%0A%0AThe+suspect+commit+is%3A%0A%0A++{commit_url}%0A%0A++{message}&labels=FromSkiaPerf%2CType-Defect%2CPriority-Medium"
)

// NewRegressionsApi returns a new instance of regressionsApi.
func NewRegressionsApi(loginProvider alogin.Login, configProvider alerts.ConfigProvider, alertStore alerts.Store, regStore regression.Store, perfGit perfgit.Git, anomalyApiClient chromeperf.AnomalyApiClient, urlProvider *urlprovider.URLProvider, graphsShortcutStore graphsshortcut.Store, alertGroupClient chromeperf.AlertGroupApiClient, progressTracker progress.Tracker, shortcutStore shortcut.Store, dfBuilder dataframe.DataFrameBuilder, paramsetRefresher psrefresh.ParamSetRefresher) regressionsApi {
	return regressionsApi{
		loginProvider:       loginProvider,
		configProvider:      configProvider,
		alertStore:          alertStore,
		regStore:            regStore,
		perfGit:             perfGit,
		alertGroupClient:    alertGroupClient,
		anomalyApiClient:    anomalyApiClient,
		urlProvider:         urlProvider,
		graphsShortcutStore: graphsShortcutStore,
		progressTracker:     progressTracker,
		shortcutStore:       shortcutStore,
		dfBuilder:           dfBuilder,
		paramsetRefresher:   paramsetRefresher,
	}
}

// regressionsApi provides a struct to handle regressions related api requests.
type regressionsApi struct {
	loginProvider       alogin.Login
	configProvider      alerts.ConfigProvider
	alertStore          alerts.Store
	regStore            regression.Store
	perfGit             perfgit.Git
	anomalyApiClient    chromeperf.AnomalyApiClient
	urlProvider         *urlprovider.URLProvider
	graphsShortcutStore graphsshortcut.Store
	alertGroupClient    chromeperf.AlertGroupApiClient
	progressTracker     progress.Tracker
	shortcutStore       shortcut.Store
	dfBuilder           dataframe.DataFrameBuilder
	paramsetRefresher   psrefresh.ParamSetRefresher
}

// RegisterHandlers registers the api handlers for their respective routes.
func (r regressionsApi) RegisterHandlers(router *chi.Mux) {
	router.HandleFunc("/_/alerts", r.alertsHandler)
	router.Post("/_/reg", r.regressionRangeHandler)
	router.Get("/_/reg/count", r.regressionCountHandler)
	router.Get("/_/regressions", r.regressionsHandler)
	router.Get("/_/alertgroup", r.alertGroupQueryHandler)
	router.Get("/_/anomaly", r.anomalyHandler)
	router.Post("/_/triage", r.triageHandler)
	router.Post("/_/cluster/start", r.clusterStartHandler)
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
func (rApi regressionsApi) regressionRangeHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	rr := &RegressionRangeRequest{}
	if err := json.NewDecoder(r.Body).Decode(rr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}
	commitNumberBegin, commitNumberEnd, err := rApi.unixTimestampRangeToCommitNumberRange(ctx, rr.Begin, rr.End)
	if err != nil {
		httputils.ReportError(w, err, "Invalid time range.", http.StatusInternalServerError)
		return
	}

	// Query for Regressions in the range.
	regMap, err := rApi.regStore.Range(ctx, commitNumberBegin, commitNumberEnd)
	if err != nil {
		httputils.ReportError(w, err, "Failed to retrieve clusters.", http.StatusInternalServerError)
		return
	}

	headers, err := rApi.configProvider.GetAllAlertConfigs(ctx, false)
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
		user := rApi.loginProvider.LoggedInAs(r)
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
		commits, err = rApi.perfGit.CommitSliceFromTimeRange(ctx, time.Unix(rr.Begin, 0), time.Unix(rr.End, 0))
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
		commits, err = rApi.perfGit.CommitSliceFromCommitNumberSlice(ctx, keys)
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

// regressionCountHandler returns a JSON object with the number of untriaged
// alerts that appear in the REGRESSION_COUNT_DURATION. The category
// can be supplied by the 'cat' query parameter and defaults to "".
func (rApi regressionsApi) regressionCountHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	category := r.FormValue("cat")
	count, err := rApi.regressionCount(ctx, category)
	if err != nil {
		httputils.ReportError(w, err, "Failed to count regressions.", http.StatusInternalServerError)
	}

	if err := json.NewEncoder(w).Encode(struct{ Count int }{Count: count}); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
	}
}

// alertsHandler returns the regression count for the default alert category.
func (rApi regressionsApi) alertsHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	count, err := rApi.regressionCount(ctx, defaultAlertCategory)
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

// regressionCount returns the number of commits that have regressions for alerts
// in the given category. The time range of commits is REGRESSION_COUNT_DURATION.
func (rApi regressionsApi) regressionCount(ctx context.Context, category string) (int, error) {
	configs, err := rApi.configProvider.GetAllAlertConfigs(ctx, false)
	if err != nil {
		return 0, err
	}

	// Query for Regressions in the range.
	end := time.Now()

	begin := end.Add(regressionCountDuration)
	commitNumberBegin, commitNumberEnd, err := rApi.unixTimestampRangeToCommitNumberRange(ctx, begin.Unix(), end.Unix())
	if err != nil {
		return 0, err
	}
	regMap, err := rApi.regStore.Range(ctx, commitNumberBegin, commitNumberEnd)
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

// unixTimestampRangeToCommitNumberRange converts a range of commits given in
// Unit timestamps into a range of types.CommitNumbers.
//
// Note this could return two equal commitNumbers.
func (rApi regressionsApi) unixTimestampRangeToCommitNumberRange(ctx context.Context, begin, end int64) (types.CommitNumber, types.CommitNumber, error) {
	beginCommitNumber, err := rApi.perfGit.CommitNumberFromTime(ctx, time.Unix(begin, 0))
	if err != nil {
		return types.BadCommitNumber, types.BadCommitNumber, skerr.Fmt("Didn't find any commit for begin: %d", begin)
	}
	endCommitNumber, err := rApi.perfGit.CommitNumberFromTime(ctx, time.Unix(end, 0))
	if err != nil {
		return types.BadCommitNumber, types.BadCommitNumber, skerr.Fmt("Didn't find any commit for end: %d", end)
	}
	return beginCommitNumber, endCommitNumber, nil
}

// regressionsHandler returns a list of regressions for a given subscription.
// TODO(b/477238168) Unused outside regressions demo. This is doubled by anomaly api's GetAnomalyList.
func (rApi regressionsApi) regressionsHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	ctx, span := trace.StartSpan(ctx, "regressionsQueryRequest")
	defer span.End()

	limit, req, err := parseQueryValues(r, w)
	if err != nil {
		httputils.ReportError(w, err, "malformed regressions request", http.StatusBadRequest)
	}
	// GetAnomalyList request does not contain a "limit" field, so we provide it separately.
	regressionsList, err := rApi.regStore.GetRegressionsBySubName(ctx, req, limit)
	if err != nil {
		httputils.ReportError(w, err, "Unable to fetch regressions", http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(regressionsList); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
	}
}

// anomalyHandler handles the request for the anomaly api.
func (rApi regressionsApi) anomalyHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()

	if rApi.anomalyApiClient == nil {
		sklog.Info("Anomaly service is not enabled")
		httputils.ReportError(w, nil, "Anomaly service is not enabled", http.StatusNotFound)
		return
	}
	key := r.URL.Query().Get("key")
	ctx, span := trace.StartSpan(ctx, "anomalyGetRequest")
	defer span.End()
	queryParams, anomaly, err := rApi.anomalyApiClient.GetAnomalyFromUrlSafeKey(ctx, key)
	if err != nil {
		httputils.ReportError(w, err, "Error retrieving anomaly data", http.StatusBadRequest)
		return
	}

	// Generate the explore page url for the given params.
	queryParams["stat"] = []string{"value"}
	graphs := []graphsshortcut.GraphConfig{}
	queryString := rApi.urlProvider.GetQueryStringFromParameters(queryParams)

	// Let's generate the graph config that represents the graph for the queryString.
	// This is then inserted as a shortcut and we generate the multigraph url with
	// the created shortcut.
	graphs = append(graphs, graphsshortcut.GraphConfig{
		Queries:  []string{queryString},
		Formulas: []string{},
	})
	shortcutObj := graphsshortcut.GraphsShortcut{
		Graphs: graphs,
	}

	graphQueryParams := getGraphQueryParamsForAnomalyId([]string{anomaly.Id})
	var redirectUrl string
	shortcutId, err := rApi.graphsShortcutStore.InsertShortcut(ctx, &shortcutObj)
	if err != nil {
		// Something went wrong while inserting shortcut. Let's fall back to the explore page.
		sklog.Errorf("Error inserting shortcut %s", err)
		redirectUrl = rApi.urlProvider.Explore(ctx, anomaly.StartRevision, anomaly.EndRevision, queryParams, true, graphQueryParams)
	} else {
		redirectUrl = rApi.urlProvider.MultiGraph(ctx, anomaly.StartRevision, anomaly.EndRevision, shortcutId, true, graphQueryParams)
	}

	sklog.Infof("Generated url: %s", redirectUrl)
	http.Redirect(w, r, redirectUrl, http.StatusSeeOther)
}

// alertGroupQueryHandler redirects the user to the relevant plot for the given alert group id.
func (rApi regressionsApi) alertGroupQueryHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()

	if rApi.alertGroupClient == nil {
		sklog.Info("Alert Grouping is not enabled")
		httputils.ReportError(w, nil, "Alert Grouping is not enabled", http.StatusNotFound)
		return
	}
	groupId := r.URL.Query().Get("group_id")
	sklog.Infof("Group id is %s", groupId)
	ctx, span := trace.StartSpan(ctx, "alertGroupQueryRequest")
	defer span.End()
	alertGroupDetails, err := rApi.alertGroupClient.GetAlertGroupDetails(ctx, groupId)
	if err != nil {
		sklog.Errorf("Error in retrieving alert group details: %s", err)
	}

	if alertGroupDetails != nil {
		sklog.Infof("Retrieved %d anomalies for alert group id %s", len(alertGroupDetails.Anomalies), groupId)

		anomalyIds := []string{}
		for anomalyId := range alertGroupDetails.Anomalies {
			anomalyIds = append(anomalyIds, anomalyId)
		}

		highlightAnomalyParams := getGraphQueryParamsForAnomalyId(anomalyIds)
		explore := r.URL.Query().Get("e")
		var redirectUrl string
		if explore == "" {
			queryParamsPerTrace := alertGroupDetails.GetQueryParamsPerTrace(ctx)
			graphs := []graphsshortcut.GraphConfig{}
			for _, queryParams := range queryParamsPerTrace {
				queryString := rApi.urlProvider.GetQueryStringFromParameters(queryParams)
				graphs = append(graphs, graphsshortcut.GraphConfig{
					Queries:  []string{queryString},
					Formulas: []string{},
				})
			}

			shortcutObj := graphsshortcut.GraphsShortcut{
				Graphs: graphs,
			}

			shortcutId, err := rApi.graphsShortcutStore.InsertShortcut(ctx, &shortcutObj)
			if err != nil {
				// Something went wrong while inserting shortcut.
				sklog.Errorf("Error inserting shortcut %s", err)
				// Let's redirect the user to the explore page instead.
				queryParams := alertGroupDetails.GetQueryParams(ctx)
				redirectUrl = rApi.urlProvider.Explore(ctx, int(alertGroupDetails.StartCommitNumber), int(alertGroupDetails.EndCommitNumber), queryParams, false, highlightAnomalyParams)
			} else {
				startCommit := alertGroupDetails.StartCommitNumber
				endCommit := alertGroupDetails.EndCommitNumber
				if alertGroupDetails.StartCommitHash != "" && alertGroupDetails.EndCommitHash != "" {
					commitNum, err := rApi.perfGit.CommitNumberFromGitHash(ctx, alertGroupDetails.StartCommitHash)
					if err != nil {
						httputils.ReportError(w, err, fmt.Sprintf("Invalid git hash %s received for commit number %d from chromeperf", alertGroupDetails.StartCommitHash, startCommit), http.StatusInternalServerError)
						return
					} else {
						startCommit = int32(commitNum)
					}

					commitNum, err = rApi.perfGit.CommitNumberFromGitHash(ctx, alertGroupDetails.EndCommitHash)
					if err != nil {
						httputils.ReportError(w, err, fmt.Sprintf("Invalid git hash %s received for commit number %d from chromeperf", alertGroupDetails.EndCommitHash, endCommit), http.StatusInternalServerError)
						return
					} else {
						endCommit = int32(commitNum)
					}
				}
				redirectUrl = rApi.urlProvider.MultiGraph(ctx, int(startCommit), int(endCommit), shortcutId, false, highlightAnomalyParams)
			}

		} else {
			queryParams := alertGroupDetails.GetQueryParams(ctx)
			redirectUrl = rApi.urlProvider.Explore(ctx, int(alertGroupDetails.StartCommitNumber), int(alertGroupDetails.EndCommitNumber), queryParams, false, highlightAnomalyParams)
		}
		sklog.Infof("Generated url: %s", redirectUrl)
		http.Redirect(w, r, redirectUrl, http.StatusSeeOther)
		return
	}
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
func (rApi regressionsApi) triageHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	tr := &TriageRequest{}
	if err := json.NewDecoder(r.Body).Decode(tr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}
	if !rApi.isEditor(w, r, "triage", tr) {
		return
	}
	detail, err := rApi.perfGit.CommitFromCommitNumber(ctx, tr.Cid)
	if err != nil {
		httputils.ReportError(w, err, "Failed to find CommitID.", http.StatusInternalServerError)
		return
	}

	key := tr.Alert.IDAsString
	if tr.ClusterType == "low" {
		err = rApi.regStore.TriageLow(ctx, detail.CommitNumber, key, tr.Triage)
	} else {
		err = rApi.regStore.TriageHigh(ctx, detail.CommitNumber, key, tr.Triage)
	}

	if err != nil {
		httputils.ReportError(w, err, "Failed to triage.", http.StatusInternalServerError)
		return
	}
	link := fmt.Sprintf("%s/t/?begin=%d&end=%d&subset=all", r.Header.Get("Origin"), detail.Timestamp, detail.Timestamp+1)

	resp := &TriageResponse{}

	if tr.Triage.Status == regression.Negative && config.Config.NotifyConfig.Notifications != notifytypes.MarkdownIssueTracker {
		cfgs, err := rApi.configProvider.GetAllAlertConfigs(ctx, false)
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

func (rApi regressionsApi) isEditor(w http.ResponseWriter, r *http.Request, action string, body interface{}) bool {
	user := rApi.loginProvider.LoggedInAs(r)
	if !rApi.loginProvider.HasRole(r, roles.Editor) {
		httputils.ReportError(w, fmt.Errorf("Not logged in."), "You must be logged in to complete this action.", http.StatusUnauthorized)
		return false
	}
	auditlog.LogWithUser(r, user.String(), action, body)
	return true
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
func (rApi regressionsApi) clusterStartHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	req := regression.NewRegressionDetectionRequest()
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		httputils.ReportError(w, err, "Could not decode POST body.", http.StatusInternalServerError)
		return
	}
	auditlog.LogWithUser(r, rApi.loginProvider.LoggedInAs(r).String(), "cluster", req)

	cb := func(ctx context.Context, _ *regression.RegressionDetectionRequest, clusterResponse []*regression.RegressionDetectionResponse, _ string) {
		// We don't do GroupBy clustering, so there will only be one clusterResponse.
		req.Progress.Results(clusterResponse[0])
	}
	rApi.progressTracker.Add(req.Progress)

	go func() {
		// This intentionally does not use r.Context() because we want it to outlive this request.
		err := regression.ProcessRegressions(context.Background(), req, cb, rApi.perfGit, rApi.shortcutStore, rApi.dfBuilder, rApi.paramsetRefresher.GetAll(), regression.ExpandBaseAlertByGroupBy, regression.ReturnOnError, config.Config.AnomalyConfig, nil)
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

func getGraphQueryParamsForAnomalyId(anomalyIds []string) url.Values {
	return url.Values{
		"highlight_anomalies": anomalyIds,
	}
}

func parseQueryValues(r *http.Request, w http.ResponseWriter) (int, regression.GetAnomalyListRequest, error) {
	query := r.URL.Query()
	subName := query.Get("sub_name")
	limit, err := strconv.Atoi(query.Get("limit"))
	if err != nil {
		return 0, regression.GetAnomalyListRequest{}, skerr.Wrapf(err, "limit value is not an integer")
	}
	offset, err := strconv.Atoi(query.Get("offset"))
	if err != nil {
		return 0, regression.GetAnomalyListRequest{}, skerr.Wrapf(err, "offset value is not an integer")
	}
	triagedStr := query.Get("triaged")
	if triagedStr == "" {
		triagedStr = "false"
	}
	triaged, err := strconv.ParseBool(triagedStr)
	if err != nil {
		return 0, regression.GetAnomalyListRequest{}, skerr.Wrapf(err, "include triaged value is not boolean")
	}
	improvementsStr := query.Get("improvements")
	if improvementsStr == "" {
		improvementsStr = "false"
	}
	improvements, err := strconv.ParseBool(improvementsStr)
	if err != nil {
		return 0, regression.GetAnomalyListRequest{}, skerr.Wrapf(err, "include improvements value is not boolean")
	}

	req := regression.GetAnomalyListRequest{
		SubName:             subName,
		PaginationOffset:    offset,
		IncludeTriaged:      triaged,
		IncludeImprovements: improvements,
	}
	return limit, req, nil
}
