package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/anomalygroup"
	"go.skia.org/infra/perf/go/chromeperf"
	"go.skia.org/infra/perf/go/chromeperf/compat"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/culprit"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/subscription"
	pb "go.skia.org/infra/perf/go/subscription/proto/v1"
	"go.skia.org/infra/perf/go/types"
)

const (
	defaultAnomaliesRequestTimeout = time.Second * 30
	regressionsPageSize            = 50
)

type anomaliesApi struct {
	chromeperfClient  chromeperf.ChromePerfClient
	loginProvider     alogin.Login
	perfGit           perfgit.Git
	subStore          subscription.Store
	alertStore        alerts.Store
	culpritStore      culprit.Store
	regStore          regression.Store
	anomalygroupStore anomalygroup.Store
	preferLegacy      bool
}

// Response object for the request from sheriff list UI.
type GetSheriffListResponse struct {
	SheriffList []string `json:"sheriff_list"`
	Error       string   `json:"error"`
}

// Response object for the request from the anomaly table UI.
type GetAnomaliesResponse struct {
	Subscription *pb.Subscription `json:"subscription"`
	// List of alerts to display.
	Alerts []alerts.Alert `json:"alerts"`
	// The list of anomalies.
	Anomalies []chromeperf.Anomaly `json:"anomaly_list"`
	// The cursor of the current query. It will be used to 'Load More' for the next query.
	QueryCursor string `json:"anomaly_cursor"`
	// Error message if any.
	Error string `json:"error"`
}

// Request object from report page to load the anomalies from Chromeperf
type GetGroupReportRequest struct {
	// A revision number.
	Revison string `json:"rev"`
	// Comma-separated list of urlsafe Anomaly keys.
	AnomalyIDs string `json:"anomalyIDs"`
	// A Buganizer bug number ID.
	BugID string `json:"bugID"`
	// An Anomaly Group ID
	AnomalyGroupID string `json:"anomalyGroupID"`
	// A hash of a group of anomaly keys.
	Sid string `json:"sid"`
}

type GetGroupReportByKeysRequest struct {
	// comma separated anomaly keys
	Keys string `json:"keys"`
	// host value to filter anomalies
	Host string `json:"host"`
}

type Timerange struct {
	Begin int64 `json:"begin"`
	End   int64 `json:"end"`
}

type GetGroupReportResponse struct {
	// The list of anomalies.
	Anomalies []chromeperf.Anomaly `json:"anomaly_list"`
	// The state id (hash of a list of anomaly keys)
	// It is used in a share-able link for a report with multiple keys.
	// This is generated on Chromeperf side and returned on POST call to /alerts_skia_by_keys
	StateId string `json:"sid"`
	// The list of anomalies which should be checked in report page.
	SelectedKeys []string `json:"selected_keys"`
	// Error message if any.
	Error string `json:"error"`
	// List of timeranges that will let report page know in what range to render
	// each graph.
	TimerangeMap map[string]Timerange `json:"timerange_map"`

	// Indicates if the instance uses standard integer commit numbers.
	// True if config.Config.GitRepoConfig.CommitNumberRegex is empty.
	IsCommitNumberBased bool `json:"is_commit_number_based"`
}

func (api anomaliesApi) RegisterHandlers(router *chi.Mux) {
	// Endpoints for using Chromeperf data.
	router.Get("/_/anomalies/sheriff_list", api.GetSheriffListDefault)
	router.Get("/_/anomalies/anomaly_list", api.GetAnomalyListDefault)
	router.Post("/_/anomalies/group_report", api.GetGroupReportDefault)

	// Endpoints for using data from the instance database.
	router.Get("/_/anomalies/sheriff_list_skia", api.GetSheriffList)
	router.Get("/_/anomalies/anomaly_list_skia", api.GetAnomalyList)
}

func NewAnomaliesApi(loginProvider alogin.Login, chromeperfClient chromeperf.ChromePerfClient, perfGit perfgit.Git, subStore subscription.Store, alertStore alerts.Store, culpritStore culprit.Store, regStore regression.Store, anomalygroupStore anomalygroup.Store, preferLegacy bool) anomaliesApi {
	return anomaliesApi{
		loginProvider:     loginProvider,
		chromeperfClient:  chromeperfClient,
		perfGit:           perfGit,
		subStore:          subStore,
		alertStore:        alertStore,
		culpritStore:      culpritStore,
		regStore:          regStore,
		anomalygroupStore: anomalygroupStore,
		preferLegacy:      preferLegacy,
	}
}

func (api anomaliesApi) GetSheriffListDefault(w http.ResponseWriter, r *http.Request) {
	if api.preferLegacy {
		api.GetSheriffListLegacy(w, r)
	} else {
		api.GetSheriffList(w, r)
	}
}

func (api anomaliesApi) GetAnomalyListDefault(w http.ResponseWriter, r *http.Request) {
	if api.preferLegacy {
		api.GetAnomalyListLegacy(w, r)
	} else {
		api.GetAnomalyList(w, r)
	}
}

func (api anomaliesApi) GetGroupReportDefault(w http.ResponseWriter, r *http.Request) {
	if api.preferLegacy {
		api.GetGroupReportLegacy(w, r)
	} else {
		api.GetGroupReport(w, r)
	}
}

func (api anomaliesApi) GetSheriffListLegacy(w http.ResponseWriter, r *http.Request) {
	if api.loginProvider.LoggedInAs(r) == "" {
		httputils.ReportError(w, errors.New("Not logged in"), "You must be logged in to complete this action.", http.StatusUnauthorized)
		return
	}

	sklog.Debug("[SkiaTriage] Get sheriff config list request received from frontend.")

	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), defaultAnomaliesRequestTimeout)
	defer cancel()

	getSheriffListResponse := &GetSheriffListResponse{}
	err := api.chromeperfClient.SendGetRequest(ctx, "sheriff_configs_skia", "", url.Values{}, getSheriffListResponse)
	if err != nil {
		httputils.ReportError(w, err, "Failed to finish get sheriff list request.", http.StatusInternalServerError)
		return
	}

	if getSheriffListResponse.Error != "" {
		httputils.ReportError(w, errors.New(getSheriffListResponse.Error), "Load sheriff list request returned error.", http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(getSheriffListResponse); err != nil {
		httputils.ReportError(w, err, "Failed to write sheriff list to UI response.", http.StatusInternalServerError)
		return
	}
	sklog.Debugf("[SkiaTriage] sheriff config list is loaded: %v", getSheriffListResponse.SheriffList)
}

func (api anomaliesApi) GetAnomalyListLegacy(w http.ResponseWriter, r *http.Request) {
	if api.loginProvider.LoggedInAs(r) == "" {
		httputils.ReportError(w, errors.New("Not logged in"), "You must be logged in to complete this action.", http.StatusUnauthorized)
		return
	}

	query_values := r.URL.Query()
	sklog.Debugf("[SkiaTriage] Get anomalies request received from frontend: %v", query_values)
	if query_values.Get("host") == "" {
		query_values["host"] = []string{config.Config.URL}
	}

	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), defaultAnomaliesRequestTimeout)
	defer cancel()
	getAnoamliesResponse := &GetAnomaliesResponse{}

	err := api.chromeperfClient.SendGetRequest(ctx, "alerts_skia", "", query_values, getAnoamliesResponse)
	if err != nil {
		httputils.ReportError(w, err, "Get anomalies request failed due to an internal server error. Please try again.", http.StatusInternalServerError)
		return
	}

	if getAnoamliesResponse.Error != "" {
		httputils.ReportError(w, errors.New(getAnoamliesResponse.Error), fmt.Sprintf("Error when getting the anomaly list. Please double check each request parameter, and try again: %v", getAnoamliesResponse.Error), http.StatusBadRequest)
		return
	}

	if err := json.NewEncoder(w).Encode(getAnoamliesResponse); err != nil {
		httputils.ReportError(w, err, "Failed to write get anoamlies response.", http.StatusInternalServerError)
		return
	}
	sklog.Debugf("[SkiaTriage] %d anomalies are received.", len(getAnoamliesResponse.Anomalies))
}

// This function is to redirect the report page request to the group_report
// endpoint in Chromeperf.
func (api anomaliesApi) GetGroupReportLegacy(w http.ResponseWriter, r *http.Request) {
	if api.loginProvider.LoggedInAs(r) == "" {
		httputils.ReportError(w, errors.New("Not logged in"), "You must be logged in to complete this action.", http.StatusUnauthorized)
		return
	}

	var err error
	var groupReportRequest GetGroupReportRequest
	if err = json.NewDecoder(r.Body).Decode(&groupReportRequest); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON on anomaly group report request.", http.StatusInternalServerError)
		return
	}
	sklog.Debugf("[SkiaTriage] Anomaly group report request received from frontend: %v", groupReportRequest)

	if !IsGroupReportRequestValid(groupReportRequest) {
		httputils.ReportError(w, err, "Group report request is invalid.", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), defaultAnomaliesRequestTimeout)
	defer cancel()
	groupReportResponse := &GetGroupReportResponse{}

	host := config.Config.URL
	if groupReportRequest.AnomalyIDs != "" {
		if len(strings.Split(groupReportRequest.AnomalyIDs, ",")) == 1 {
			err = api.chromeperfClient.SendGetRequest(
				ctx, "alerts_skia_by_key", "", url.Values{"key": {groupReportRequest.AnomalyIDs}, "host": []string{host}}, groupReportResponse)
		} else {
			groupReportByKeysRequest := &GetGroupReportByKeysRequest{
				Keys: groupReportRequest.AnomalyIDs,
				Host: host,
			}
			err = api.chromeperfClient.SendPostRequest(ctx, "alerts_skia_by_keys", "", groupReportByKeysRequest, groupReportResponse, []int{200, 400, 500})
		}
	} else if groupReportRequest.BugID != "" {
		err = api.chromeperfClient.SendGetRequest(
			ctx, "alerts_skia_by_bug_id", "", url.Values{"bug_id": {groupReportRequest.BugID}, "host": []string{host}}, groupReportResponse)
	} else if groupReportRequest.Sid != "" {
		err = api.chromeperfClient.SendGetRequest(
			ctx, "alerts_skia_by_sid", "", url.Values{"sid": {groupReportRequest.Sid}, "host": []string{host}}, groupReportResponse)
	} else if groupReportRequest.Revison != "" {
		err = api.chromeperfClient.SendGetRequest(
			ctx, fmt.Sprintf("alerts/skia/rev/%s", groupReportRequest.Revison), "", url.Values{"host": []string{host}}, groupReportResponse)
	} else if groupReportRequest.AnomalyGroupID != "" {
		err = api.chromeperfClient.SendGetRequest(
			ctx, fmt.Sprintf("alerts/skia/group_id/%s", groupReportRequest.AnomalyGroupID), "", url.Values{"host": []string{host}}, groupReportResponse)
	} else {
		httputils.ReportError(w, errors.New("Invalid Request"), fmt.Sprintf("Group report request does not have valid parameters: %v", groupReportRequest), http.StatusBadRequest)
		sklog.Debug("[SkiaTriage] Group report request does not have valid parameters")
		return
	}

	if err != nil {
		httputils.ReportError(w, err, "Anomaly group report request failed due to an internal server error. Please try again.", http.StatusInternalServerError)
		sklog.Debugf("[SkiaTriage] Anomaly group report request failed due to an internal server error: %v", err)
		return
	}

	if groupReportResponse.Error != "" {
		httputils.ReportError(w, errors.New(groupReportResponse.Error), fmt.Sprintf("Error when getting the anomaly report group. Please double check each request parameter, and try again: %v", groupReportResponse.Error), http.StatusBadRequest)
		sklog.Debugf("[SkiaTriage] Error when getting the anomaly report group: %v", groupReportResponse.Error)
		return
	}

	// b/383913153: mitigation on the anomaly rendering scenario.
	for i := range groupReportResponse.Anomalies {
		groupReportResponse.Anomalies[i].TestPath, err = cleanTestName(groupReportResponse.Anomalies[i].TestPath)
		if err != nil {
			httputils.ReportError(w, err, "Failed to clean up test name by regex.", http.StatusInternalServerError)
			sklog.Debugf("[SkiaTriage] Failed to clean up test name by regex: %v", err)
			return
		}
	}

	groupReportResponse.TimerangeMap, err = api.getTimerangeMap(ctx, groupReportResponse.Anomalies)
	if err != nil {
		httputils.ReportError(w, err, "Failed to get timerange map.", http.StatusInternalServerError)
		sklog.Debugf("[SkiaTriage] Failed to get timerange map: %v", err)
		return
	}

	groupReportResponse.IsCommitNumberBased = config.Config.GitRepoConfig.CommitNumberRegex != ""

	if err := json.NewEncoder(w).Encode(groupReportResponse); err != nil {
		httputils.ReportError(w, err, "Failed to write anomaly report response.", http.StatusInternalServerError)
		sklog.Debugf("[SkiaTriage] Failed to write anomaly report response: %v", err)
		return
	}
	sklog.Debugf("[SkiaTriage] %d anomalies are received from anomaly report group.", len(groupReportResponse.Anomalies))
}

// GetSheriffListSkia handles requests to retrieve the list of sheriffs from the Skia internal store.
func (api anomaliesApi) GetSheriffList(w http.ResponseWriter, r *http.Request) {
	if api.loginProvider.LoggedInAs(r) == "" {
		httputils.ReportError(w, errors.New("Not logged in"), "You must be logged in to complete this action.", http.StatusUnauthorized)
		return
	}

	sklog.Debug("[SkiaTriage] Get sheriff config list from Skia request received from frontend.")

	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), defaultAnomaliesRequestTimeout)
	defer cancel()

	getSheriffListResponse := &GetSheriffListResponse{}

	subscriptions, err := api.subStore.GetAllActiveSubscriptions(ctx)
	if err != nil {
		httputils.ReportError(w, err, "Failed to get all active subscriptions.", http.StatusInternalServerError)
		return
	}

	for _, sub := range subscriptions {
		getSheriffListResponse.SheriffList = append(getSheriffListResponse.SheriffList, sub.Name)
	}

	if err := json.NewEncoder(w).Encode(getSheriffListResponse); err != nil {
		httputils.ReportError(w, err, "Failed to write sheriff list to UI response.", http.StatusInternalServerError)
		return
	}

	sklog.Debugf("[SkiaTriage] sheriff config list is loaded: %v", getSheriffListResponse.SheriffList)
}

// GetAnomalyListSkia handles requests to retrieve the list of anomalies from the Skia internal store as
// well as Subscription and Alert information.
func (api anomaliesApi) GetAnomalyList(w http.ResponseWriter, r *http.Request) {
	if api.loginProvider.LoggedInAs(r) == "" {
		httputils.ReportError(w, errors.New("Not logged in"), "You must be logged in to complete this action.", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	queryValues := parseGetAnomalyListRequest(r)

	ctx, cancel := context.WithTimeout(r.Context(), defaultAnomaliesRequestTimeout)
	defer cancel()

	getAnomaliesResponse := &GetAnomaliesResponse{}
	sub, err := api.subStore.GetActiveSubscription(ctx, queryValues.SubName)
	if sub == nil {
		httputils.ReportError(w, err, "No matching subscription found", http.StatusNotFound)
		return
	}
	if err != nil {
		httputils.ReportError(w, err, "Failed to get subscription", http.StatusInternalServerError)
		return
	}

	getAnomaliesResponse.Subscription = sub

	alertsFromStore, err := api.alertStore.ListForSubscription(ctx, queryValues.SubName)
	if err != nil {
		httputils.ReportError(w, err, "Failed to get list of alerts", http.StatusInternalServerError)
		return
	}

	alertsForResponse := make([]alerts.Alert, len(alertsFromStore))
	for i, alertPtr := range alertsFromStore {
		if alertPtr != nil {
			alertsForResponse[i] = *alertPtr
		}
	}
	getAnomaliesResponse.Alerts = alertsForResponse

	regressions, err := api.regStore.GetRegressionsBySubName(ctx, queryValues, regressionsPageSize)
	if err != nil {
		httputils.ReportError(w, err, "Failed to get regressions", http.StatusInternalServerError)
		return
	}
	anomalies := make([]chromeperf.Anomaly, 0)
	for _, reg := range regressions {
		convertedAnomalies, err := compat.ConvertRegressionToAnomalies(reg)
		if err != nil {
			sklog.Warningf("Could not convert regression with id %s to anomalies: %s", reg.Id, err)
			continue
		}
		for _, commitNumberMap := range convertedAnomalies {
			for _, anomaly := range commitNumberMap {
				anomalies = append(anomalies, anomaly)
			}
		}
	}

	getAnomaliesResponse.Anomalies = anomalies
	if len(anomalies) > 0 {
		getAnomaliesResponse.QueryCursor = "maybe_more_anomalies"
	}

	if err := json.NewEncoder(w).Encode(getAnomaliesResponse); err != nil {
		httputils.ReportError(w, err, "Failed to write get anoamlies response.", http.StatusInternalServerError)
		return
	}
	sklog.Debugf("[SkiaTriage] %d anomalies are received.", len(getAnomaliesResponse.Anomalies))
}

func (api anomaliesApi) GetGroupReport(w http.ResponseWriter, r *http.Request) {
	if api.loginProvider.LoggedInAs(r) == "" {
		httputils.ReportError(w, errors.New("Not logged in"), "You must be logged in to complete this action.", http.StatusUnauthorized)
		return
	}

	var err error
	var groupReportRequest GetGroupReportRequest
	if err = json.NewDecoder(r.Body).Decode(&groupReportRequest); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON on anomaly group report request.", http.StatusInternalServerError)
		return
	}
	sklog.Debugf("[SkiaTriage] Anomaly group report request received from frontend: Revision: %s, AnomalyIDs: %s, BugID: %s, AnomalyGroupID: %s, Sid: %s", groupReportRequest.Revison, groupReportRequest.AnomalyIDs, groupReportRequest.BugID, groupReportRequest.AnomalyGroupID, groupReportRequest.Sid)

	if !IsGroupReportRequestValid(groupReportRequest) {
		httputils.ReportError(w, err, "Group report request is invalid.", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), defaultAnomaliesRequestTimeout)
	defer cancel()
	var groupReportResponse *GetGroupReportResponse

	if groupReportRequest.AnomalyIDs != "" {
		groupReportResponse, err = api.getGroupReportByAnomalyId(ctx, groupReportRequest)
	} else if groupReportRequest.BugID != "" {
		groupReportResponse, err = api.getGroupReportByBugId(ctx, groupReportRequest)
	} else if groupReportRequest.Sid != "" {
		httputils.ReportError(w, errors.New("not implemented"), "This API is not implemented for this parameter.", http.StatusInternalServerError)
		sklog.Debugf("Unsupported parameters for group report: %v", groupReportRequest)
		return
	} else if groupReportRequest.Revison != "" {
		groupReportResponse, err = api.getGroupReportByRevision(ctx, groupReportRequest)
	} else if groupReportRequest.AnomalyGroupID != "" {
		groupReportResponse, err = api.getGroupReportByAnomalyGroupId(ctx, groupReportRequest)
	} else {
		httputils.ReportError(w, errors.New("invalid Request"), fmt.Sprintf("Group report request does not have valid parameters: %v", groupReportRequest), http.StatusBadRequest)
		sklog.Debug("[SkiaTriage] Group report request does not have valid parameters")
		return
	}
	if err != nil {
		httputils.ReportError(w, err, "Failed to get group report: ", http.StatusInternalServerError)
		sklog.Error(err)
		return
	}

	if groupReportResponse.Error != "" {
		// TODO(b/454277955) dead code? Since groupReportResponse is created here, not by chromeperf, we don't need to check this field here
		// Quite possibly, this field is no longer needed in general.
		httputils.ReportError(w, errors.New(groupReportResponse.Error), fmt.Sprintf("Error when getting the anomaly report group. Please double check each request parameter, and try again: %v", groupReportResponse.Error), http.StatusBadRequest)
		sklog.Debugf("[SkiaTriage] Error when getting the anomaly report group: %v", groupReportResponse.Error)
		return
	}

	// b/383913153: mitigation on the anomaly rendering scenario.
	for i := range groupReportResponse.Anomalies {
		groupReportResponse.Anomalies[i].TestPath, err = cleanTestName(groupReportResponse.Anomalies[i].TestPath)
		if err != nil {
			httputils.ReportError(w, err, "Failed to clean up test name by regex.", http.StatusInternalServerError)
			sklog.Debugf("[SkiaTriage] Failed to clean up test name by regex: %v", err)
			return
		}
	}

	if groupReportRequest.Sid != "" || groupReportRequest.AnomalyIDs != "" {
		groupReportResponse.SelectedKeys = api.getSelectedKeys(groupReportResponse.Anomalies)
	} else {
		groupReportResponse.SelectedKeys = []string{}
	}

	groupReportResponse.TimerangeMap, err = api.getTimerangeMap(ctx, groupReportResponse.Anomalies)
	if err != nil {
		httputils.ReportError(w, err, "Failed to get timerange map.", http.StatusInternalServerError)
		sklog.Errorf("[SkiaTriage] Failed to get timerange map: %v", err)
		return
	}

	// TODO(b/454277955) Populate remaining fields of GetGroupReportResponse:
	// StateId, Error
	api.performNeedForSidCheck(groupReportResponse.SelectedKeys)

	groupReportResponse.IsCommitNumberBased = config.Config.GitRepoConfig.CommitNumberRegex != ""

	if err := json.NewEncoder(w).Encode(groupReportResponse); err != nil {
		httputils.ReportError(w, err, "Failed to write anomaly report response.", http.StatusInternalServerError)
		sklog.Debugf("[SkiaTriage] Failed to write anomaly report response: %v", err)
		return
	}
	sklog.Debugf("[SkiaTriage] %d anomalies are received from anomaly report group.", len(groupReportResponse.Anomalies))
}

// The group report page should only regard one input parameters.
// If the request has more than one parameters, we consider it invalid.
func IsGroupReportRequestValid(req GetGroupReportRequest) bool {
	valid_param_count := 0
	if req.AnomalyIDs != "" {
		valid_param_count += 1
	}
	if req.BugID != "" {
		valid_param_count += 1
	}
	if req.Sid != "" {
		valid_param_count += 1
	}
	if req.Revison != "" {
		valid_param_count += 1
	}
	if req.AnomalyGroupID != "" {
		valid_param_count += 1
	}
	return valid_param_count == 1
}

func (api anomaliesApi) getSelectedKeys(anomalies []chromeperf.Anomaly) []string {
	selectedKeys := make([]string, len(anomalies))
	for i, anomaly := range anomalies {
		selectedKeys[i] = anomaly.Id
	}
	return selectedKeys
}

func (api anomaliesApi) getTimerangeMap(ctx context.Context, anomalies []chromeperf.Anomaly) (map[string]Timerange, error) {
	timerangeMap := make(map[string]Timerange)
	for i := range anomalies {
		anomaly := &anomalies[i]
		var startTime int64
		var endTime int64

		if strings.Contains(config.Config.InstanceName, "fuchsia") && api.preferLegacy {
			timestampStr := anomaly.Timestamp
			const layout = "2006-01-02T15:04:05.999999" // Layout for "ISO Format"

			timestamp, err := time.Parse(layout, timestampStr)
			if err != nil {
				return nil, skerr.Wrap(err)
			}

			// Since we don't have a start and end revision to determine range, we use
			// one day before and one day after to capture this range, although less accurately.
			startTime = int64(timestamp.AddDate(0, 0, -1).Unix())
			endTime = int64(timestamp.AddDate(0, 0, 1).Unix())
		} else {
			startCommit, err := api.perfGit.CommitFromCommitNumber(ctx, types.CommitNumber(anomaly.StartRevision))
			if err != nil {
				sklog.Debugf("[SkiaTriage] CommitFromCommitNumber returns err: %v", err)
				return nil, err
			}
			startTime = int64(startCommit.Timestamp)

			endCommit, err := api.perfGit.CommitFromCommitNumber(ctx, types.CommitNumber(anomaly.EndRevision))
			if err != nil {
				sklog.Debugf("[SkiaTriage] CommitFromCommitNumber returns err: %v", err)
				return nil, err
			}

			// We will shift the end time by a day so the graph doesn't render the anomalies right at the end
			endTime = int64(time.Unix(endCommit.Timestamp, 0).AddDate(0, 0, 1).Unix())
		}

		timerangeMap[anomaly.Id] = Timerange{Begin: startTime, End: endTime}
	}
	return timerangeMap, nil
}

// Experimental, remove if we see the need to implement SID.
func (api anomaliesApi) performNeedForSidCheck(selectedKeys []string) {
	totalLen := len(url.QueryEscape(strings.Join(selectedKeys, ",")))
	if totalLen > needForSidLimit {
		sklog.Warningf("need-for-sid: length of anomaly ids reached %d, consider implementing sid", totalLen)
	}
}

// cleanTestName cleans the given test name using the query.ForceValid function.
func cleanTestName(testName string) (string, error) {
	var invalidParamCharRegex *regexp.Regexp
	var err error
	invalidParamCharRegex = query.InvalidChar
	if config.Config.InvalidParamCharRegex != "" {
		invalidParamCharRegex, err = regexp.Compile(config.Config.InvalidParamCharRegex)
		if err != nil {
			return testName, skerr.Wrap(err)
		}
	}

	// Split the test name into parts.
	parts := strings.Split(testName, "/")
	// Clean each part individually.
	for i := range parts {
		parts[i] = query.ForceValidWithRegex(map[string]string{"a": parts[i]}, invalidParamCharRegex)["a"]
	}
	// Join the cleaned parts back together.
	return strings.Join(parts, "/"), nil
}

func parseGetAnomalyListRequest(r *http.Request) regression.GetAnomalyListRequest {
	query := r.URL.Query()
	includeTriaged, err := strconv.ParseBool(query.Get("triaged"))
	if err != nil {
		includeTriaged = false
	}
	includeImprovements, err := strconv.ParseBool(query.Get("improvements"))
	if err != nil {
		includeImprovements = false
	}
	paginationOffset, err := strconv.Atoi(query.Get("pagination_offset"))
	if err != nil {
		paginationOffset = 0
	}
	queryValues := regression.GetAnomalyListRequest{
		SubName:             query.Get("sheriff"),
		IncludeTriaged:      includeTriaged,
		IncludeImprovements: includeImprovements,
		QueryCursor:         query.Get("anomaly_cursor"),
		Host:                query.Get("host"),
		PaginationOffset:    paginationOffset,
	}
	return queryValues
}
