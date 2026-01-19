package chromeperf

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.opencensus.io/trace"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/types"
)

const (
	AnomalyAPIName   = "anomalies"
	AddFuncName      = "add"
	FindFuncName     = "find"
	FindTimeFuncName = "find_time"
	GetFuncName      = "get"
	// IgnoreBugIDFlag is the bug ID used to indicate that an anomaly should be ignored.
	IgnoreBugIDFlag = -2
)

var statToSuffixMap = map[string]string{
	"error":   "std",
	"max":     "max",
	"min":     "min",
	"sum":     "sum",
	"count":   "count",
	"average": "avg",
}

var invalidChars = []string{"?"}

// CommitNumberAnomalyMap is a map of Anomaly, keyed by commit number.
type CommitNumberAnomalyMap map[types.CommitNumber]Anomaly

// AnomalyMap is a map of CommitNumberAnomalyMap, keyed by traceId.
type AnomalyMap map[string]CommitNumberAnomalyMap

// Anomaly represents a performance anomaly (regression or improvement) detected
// in a perf trace.
//
// TODO(ansid): now the anomaly object can also be obtained from the database
// instead of Chrome Perf API, so chromeperf.Anomaly should be renamed to
// something more generic.
type Anomaly struct {
	Id            string `json:"id"`
	TestPath      string `json:"test_path"`
	BugId         int    `json:"bug_id"`
	StartRevision int    `json:"start_revision"`
	EndRevision   int    `json:"end_revision"`

	// The hashes below are needed for cases where the commit numbers are
	// different in chromeperf and in the perf instance. We can use these
	// hashes to look up the correct commit number from the database.
	StartRevisionHash string `json:"start_revision_hash,omitempty"`
	EndRevisionHash   string `json:"end_revision_hash,omitempty"`

	IsImprovement       bool     `json:"is_improvement"`
	Recovered           bool     `json:"recovered"`
	State               string   `json:"state"`
	Statistics          string   `json:"statistic"`
	Unit                string   `json:"units"`
	DegreeOfFreedom     float64  `json:"degrees_of_freedom"`
	MedianBeforeAnomaly float64  `json:"median_before_anomaly"`
	MedianAfterAnomaly  float64  `json:"median_after_anomaly"`
	PValue              float64  `json:"p_value"`
	SegmentSizeAfter    int      `json:"segment_size_after"`
	SegmentSizeBefore   int      `json:"segment_size_before"`
	StdDevBeforeAnomaly float64  `json:"std_dev_before_anomaly"`
	TStatistics         float64  `json:"t_statistic"`
	SubscriptionName    string   `json:"subscription_name"`
	BugComponent        string   `json:"bug_component"`
	BugLabels           []string `json:"bug_labels"`
	BugCcEmails         []string `json:"bug_cc_emails"`
	BisectIDs           []string `json:"bisect_ids"`
	Timestamp           string   `json:"timestamp,omitempty"`
	Multiplicity        int      `json:"multiplicity,omitempty"`
}

// UnmarshalJSON handles the fact that anomaly IDs can be sent as either a
// string (preferred) or a number (legacy chromeperf format). It transparently
// converts numeric IDs to strings.
func (a *Anomaly) UnmarshalJSON(data []byte) error {
	// Create a type alias to avoid recursive calls to UnmarshalJSON
	type Alias Anomaly
	aux := &struct {
		Id interface{} `json:"id"`
		*Alias
	}{
		Alias: (*Alias)(a),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Handle the "id" field, which can be a string or a number in the JSON.
	switch v := aux.Id.(type) {
	case string:
		a.Id = v
	case float64:
		// Convert the number to a string without decimals.
		a.Id = fmt.Sprintf("%.0f", v)
	}

	return nil
}

// AnomalyForRevision defines struct to contain anomaly data for a specific revision
type AnomalyForRevision struct {
	StartRevision int                 `json:"start_revision"`
	EndRevision   int                 `json:"end_revision"`
	Anomaly       Anomaly             `json:"anomaly"`
	Params        map[string][]string `json:"params"`
	TestPath      string              `json:"test_path"`
}

// GetKey returns a string representing a key based on the params for the anomaly.
func (anomaly *AnomalyForRevision) GetKey() string {
	paramKeys := []string{"master", "bot", "benchmark", "test", "subtest_1", "subtest_2", "subtest_3", "subtest_4", "subtest_5"}
	var sb strings.Builder
	for _, key := range paramKeys {
		paramValue, ok := anomaly.Params[key]
		if ok {
			sb.WriteString(paramValue[0])
			sb.WriteString("-")
		}
	}

	return sb.String()
}

// GetParamValue returns the value for the given param name in the anomaly.
// Returns empty string if the value is not present.
func (anomaly *AnomalyForRevision) GetParamValue(paramName string) string {
	if paramValue, ok := anomaly.Params[paramName]; ok {
		return paramValue[0]
	}

	return ""
}

// GetTestPath returns the test path representation for the anomaly.
// To maintain parity with the legacy dashboard, this testpath does not
// include the master/bot/benchmark portion of the path.
func (anomaly *AnomalyForRevision) GetTestPath() string {
	var sb strings.Builder
	sb.WriteString(anomaly.GetParamValue("test"))

	subtestKeys := []string{"subtest_1", "subtest_2", "subtest_3", "subtest_4", "subtest_5"}
	for _, key := range subtestKeys {
		value := anomaly.GetParamValue(key)
		if value == "" {
			break
		}
		sb.WriteString("/")
		sb.WriteString(value)
	}

	return sb.String()
}

// RevisionInfo defines struct to contain revision information
type RevisionInfo struct {
	Master        string   `json:"master"`
	Bot           string   `json:"bot"`
	Benchmark     string   `json:"benchmark"`
	StartRevision int      `json:"start_revision"`
	EndRevision   int      `json:"end_revision"`
	StartTime     int64    `json:"start_time"`
	EndTime       int64    `json:"end_time"`
	TestPath      string   `json:"test"`
	IsImprovement bool     `json:"is_improvement"`
	BugId         string   `json:"bug_id"`
	ExploreUrl    string   `json:"explore_url"`
	Query         string   `json:"query"`
	AnomalyIds    []string `json:"anomaly_ids"`
}

// GetAnomaliesRequest struct to request anomalies from the chromeperf api.
// The parameters can be one of below described.
// 1. Revision: Retrieves anomalies around that revision number.
// 2. Tests-MinRevision-MaxRevision: Retrieves anomalies for the given set of tests between the min and max revisions
type GetAnomaliesRequest struct {
	Tests           []string `json:"tests,omitempty"`
	MaxRevision     string   `json:"max_revision,omitempty"`
	MinRevision     string   `json:"min_revision,omitempty"`
	Revision        int      `json:"revision,omitempty"`
	NeedAggregation bool     `json:"need_aggregation,omitempty"`
}

type GetAnomaliesTimeBasedRequest struct {
	Tests     []string  `json:"tests,omitempty"`
	StartTime time.Time `json:"start_time,omitempty"`
	EndTime   time.Time `json:"end_time,omitempty"`
}

type GetAnomaliesResponse struct {
	Anomalies map[string][]Anomaly `json:"anomalies"`
}

// ReportRegressionRequest provides a struct for the data that is sent over
// to chromeperf when a regression is detected.
type ReportRegressionRequest struct {
	StartRevision       int32   `json:"start_revision"`
	EndRevision         int32   `json:"end_revision"`
	ProjectID           string  `json:"project_id"`
	TestPath            string  `json:"test_path"`
	IsImprovement       bool    `json:"is_improvement"`
	BotName             string  `json:"bot_name"`
	Internal            bool    `json:"internal_only"`
	MedianBeforeAnomaly float32 `json:"median_before_anomaly"`
	MedianAfterAnomaly  float32 `json:"median_after_anomaly"`
}

// ReportRegressionResponse provides a struct to hold the response data
// returned by the add anomalies api.
type ReportRegressionResponse struct {
	AnomalyId    string `json:"anomaly_id"`
	AlertGroupId string `json:"alert_group_id"`
}

// AnomalyApiClient provides interface to interact with chromeperf "anomalies" api
type AnomalyApiClient interface {
	// ReportRegression sends regression information to chromeperf.
	ReportRegression(ctx context.Context, testPath string, startCommitPosition int32, endCommitPosition int32, projectId string, isImprovement bool, botName string, internal bool, medianBefore float32, medianAfter float32) (*ReportRegressionResponse, error)

	// GetAnomalyFromUrlSafeKey returns the anomaly details based on the urlsafe key.
	GetAnomalyFromUrlSafeKey(ctx context.Context, key string) (map[string][]string, Anomaly, error)

	// GetAnomalies retrieves anomalies for a given set of traces within the supplied commit positions.
	GetAnomalies(ctx context.Context, traceNames []string, startCommitPosition int, endCommitPosition int) (AnomalyMap, error)

	// GetAnomaliesTimeBased retrieves anomalies for a given set of traces within the supplied commit positions.
	GetAnomaliesTimeBased(ctx context.Context, traceNames []string, startTime time.Time, endTime time.Time) (AnomalyMap, error)

	// GetAnomaliesAroundRevision retrieves traces with anomalies that were generated around a specific commit
	GetAnomaliesAroundRevision(ctx context.Context, revision int) ([]AnomalyForRevision, error)
}

// anomalyApiClientImpl implements AnomalyApiClient
type anomalyApiClientImpl struct {
	chromeperfClient   ChromePerfClient
	perfGit            perfgit.Git
	config             *config.InstanceConfig
	sendAnomalyCalled  metrics2.Counter
	sendAnomalyFailed  metrics2.Counter
	getAnomaliesCalled metrics2.Counter
	getAnomaliesFailed metrics2.Counter
}

// NewAnomalyApiClient returns a new AnomalyApiClient instance.
func NewAnomalyApiClient(ctx context.Context, git perfgit.Git, config *config.InstanceConfig) (AnomalyApiClient, error) {
	cpClient, err := NewChromePerfClient(ctx, "", false)
	if err != nil {
		return nil, err
	}

	return newAnomalyApiClient(cpClient, git, config), nil
}

func newAnomalyApiClient(cpClient ChromePerfClient, git perfgit.Git, config *config.InstanceConfig) AnomalyApiClient {
	return &anomalyApiClientImpl{
		chromeperfClient:   cpClient,
		perfGit:            git,
		config:             config,
		sendAnomalyCalled:  metrics2.GetCounter("chrome_perf_send_anomaly_called"),
		sendAnomalyFailed:  metrics2.GetCounter("chrome_perf_send_anomaly_failed"),
		getAnomaliesCalled: metrics2.GetCounter("chrome_perf_get_anomalies_called"),
		getAnomaliesFailed: metrics2.GetCounter("chrome_perf_get_anomalies_failed"),
	}
}

// ReportRegression implements AnomalyApiClient, sends regression information to chromeperf.
func (cp *anomalyApiClientImpl) ReportRegression(
	ctx context.Context,
	testPath string,
	startCommitPosition int32,
	endCommitPosition int32,
	projectId string,
	isImprovement bool,
	botName string,
	internal bool,
	medianBefore float32,
	medianAfter float32) (*ReportRegressionResponse, error) {
	request := &ReportRegressionRequest{
		TestPath:            testPath,
		StartRevision:       startCommitPosition,
		EndRevision:         endCommitPosition,
		ProjectID:           projectId,
		IsImprovement:       isImprovement,
		BotName:             botName,
		Internal:            internal,
		MedianBeforeAnomaly: medianBefore,
		MedianAfterAnomaly:  medianAfter,
	}

	acceptedStatusCodes := []int{
		200, // Success
		404, // NotFound - This is returned if the param value names are different.
	}
	response := ReportRegressionResponse{}
	err := cp.chromeperfClient.SendPostRequest(ctx, AnomalyAPIName, AddFuncName, request, &response, acceptedStatusCodes)
	if err != nil {
		cp.sendAnomalyFailed.Inc(1)
		return nil, skerr.Wrapf(err, "Failed to get chrome perf response when sending anomalies.")
	}

	cp.sendAnomalyCalled.Inc(1)
	return &response, nil
}

// GetAnomalyFromUrlSafeKey returns the anomaly details based on the urlsafe key.
func (cp *anomalyApiClientImpl) GetAnomalyFromUrlSafeKey(ctx context.Context, key string) (map[string][]string, Anomaly, error) {
	getAnomaliesResp := &GetAnomaliesResponse{}
	var anomaly Anomaly
	err := cp.chromeperfClient.SendGetRequest(ctx, AnomalyAPIName, GetFuncName, url.Values{"key": {key}}, getAnomaliesResp)
	if err != nil {
		return nil, anomaly, skerr.Wrapf(err, "Failed to get anomaly data based on url safe key: %s", key)
	}

	var queryParams map[string][]string

	if len(getAnomaliesResp.Anomalies) > 0 {
		for testPath, respAnomaly := range getAnomaliesResp.Anomalies {
			queryParams = getParams(testPath)
			anomaly = respAnomaly[0]
			break
		}
	}

	return queryParams, anomaly, nil
}

// Returns a de-duplicated slice from a slice input.
func DedupStringSlice(s []string) []string {
	if len(s) == 0 {
		return []string{}
	}

	seen := make(map[string]interface{}) // Map to store seen strings
	result := []string{}                 // Slice to store unique strings

	for _, item := range s {
		if _, exists := seen[item]; !exists {
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}

// GetAnomalies implements AnomalyApiClient, it calls chrome perf API to fetch anomlies.
func (cp *anomalyApiClientImpl) GetAnomalies(ctx context.Context, traceNames []string, startCommitPosition int, endCommitPosition int) (AnomalyMap, error) {
	testPathes := make([]string, 0)
	testPathTraceNameMap := make(map[string]string)
	for _, traceName := range traceNames {
		// Build chrome perf test_path from skia perf traceName
		testPath, err := TraceNameToTestPath(traceName, cp.config.Experiments.EnableSkiaBridgeAggregation)
		if err != nil {
			sklog.Errorf("Failed to build chrome perf test path from trace name %q: %s", traceName, err)
		} else {
			testPathes = append(testPathes, testPath)
			testPathTraceNameMap[testPath] = traceName
		}
	}
	// Dedup testPaths to remove duplicate efforts
	testPathes = DedupStringSlice(testPathes)

	if len(testPathes) > 0 {
		cp.getAnomaliesCalled.Inc(1)
		// Call Chrome Perf API to fetch anomalies.
		request := &GetAnomaliesRequest{
			Tests:       testPathes,
			MaxRevision: strconv.Itoa(endCommitPosition),
			MinRevision: strconv.Itoa(startCommitPosition),
		}
		if cp.config.Experiments.EnableSkiaBridgeAggregation {
			// This triggers aggregation on testname_avg traces to also include query on testname without suffix.
			request.NeedAggregation = true
		}
		getAnomaliesResp := &GetAnomaliesResponse{}
		err := cp.chromeperfClient.SendPostRequest(ctx, AnomalyAPIName, FindFuncName, *request, getAnomaliesResp, []int{200})
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to call chrome perf endpoint.")
		}
		return cp.getAnomalyMapFromChromePerfResult(ctx, getAnomaliesResp, testPathTraceNameMap), nil
	}

	return AnomalyMap{}, nil
}

// TODO(b/383913153): add reverse mapping before loading anomalies
func (cp *anomalyApiClientImpl) GetAnomaliesTimeBased(ctx context.Context, traceNames []string, startTime time.Time, endTime time.Time) (AnomalyMap, error) {
	ctx, span := trace.StartSpan(ctx, "chromeperf.anomalyApiClientImpl.GetAnomaliesTimeBased")
	defer span.End()
	testPathes := make([]string, 0)
	testPathTraceNameMap := make(map[string]string)
	for _, traceName := range traceNames {
		// Build chrome perf test_path from skia perf traceName
		testPath, err := TraceNameToTestPath(traceName, cp.config.EnableSheriffConfig)
		if err != nil {
			sklog.Errorf("Failed to build chrome perf test path from trace name %q: %s", traceName, err)
		} else {
			testPathes = append(testPathes, testPath)
			testPathTraceNameMap[testPath] = traceName
		}

	}
	// Dedup testPaths to remove duplicate efforts
	testPathes = DedupStringSlice(testPathes)

	if len(testPathes) > 0 {
		cp.getAnomaliesCalled.Inc(1)
		// Call Chrome Perf API to fetch anomalies.
		request := &GetAnomaliesTimeBasedRequest{
			Tests:     testPathes,
			StartTime: startTime,
			EndTime:   endTime,
		}
		getAnomaliesResp := &GetAnomaliesResponse{}
		err := cp.chromeperfClient.SendPostRequest(ctx, AnomalyAPIName, FindTimeFuncName, *request, getAnomaliesResp, []int{200})
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to call chrome perf endpoint.")
		}
		return cp.getAnomalyMapFromChromePerfResult(ctx, getAnomaliesResp, testPathTraceNameMap), nil
	}

	return AnomalyMap{}, nil
}

// GetAnomaliesAroundRevision implements AnomalyApiClient, returns anomalies around the given revision.
func (cp *anomalyApiClientImpl) GetAnomaliesAroundRevision(ctx context.Context, revision int) ([]AnomalyForRevision, error) {
	ctx, span := trace.StartSpan(ctx, "chromeperf.anomalyApiClientImpl.GetAnomaliesAroundRevision")
	defer span.End()

	request := &GetAnomaliesRequest{
		Revision: revision,
	}

	getAnomaliesResp := &GetAnomaliesResponse{}
	err := cp.chromeperfClient.SendPostRequest(ctx, AnomalyAPIName, FindFuncName, *request, getAnomaliesResp, []int{200})
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to call chrome perf endpoint.")
	}

	response := []AnomalyForRevision{}
	for testPath, cpAnomalies := range getAnomaliesResp.Anomalies {
		for _, anomaly := range cpAnomalies {
			params := getParams(testPath)
			if len(testPath) > 0 && len(params) == 0 {
				return nil, skerr.Fmt("Test path likely has more params than expected. Test path: %s", testPath)
			}

			response = append(response, AnomalyForRevision{
				TestPath:      testPath,
				StartRevision: anomaly.StartRevision,
				EndRevision:   anomaly.EndRevision,
				Anomaly:       anomaly,
				Params:        params,
			})
		}
	}

	return response, nil
}

// TraceNameToTestPath converts trace name to Chrome Perf test path.
// For example, for the trace name, ",benchmark=Blazor,bot=MacM1,
// master=ChromiumPerf,test=timeToFirstContentfulPaint_avg,subtest_1=111,
// subtest_2=222,...subtest_7=777,unit=microsecond,improvement_direction=up,"
// test path will be: "ChromiumPerf/MacM1/Blazor/timeToFirstContentfulPaint_avg
// /111/222/.../777"
func TraceNameToTestPath(traceName string, statToSuffixRequired bool) (string, error) {
	keyValueEquations := strings.Split(traceName, ",")
	if len(keyValueEquations) == 0 {
		return "", fmt.Errorf("cannot build test path from trace name: %q", traceName)
	}

	paramKeyValueMap := map[string]string{}
	for _, keyValueEquation := range keyValueEquations {
		keyValueArray := strings.Split(keyValueEquation, "=")
		if len(keyValueArray) == 2 {
			paramKeyValueMap[keyValueArray[0]] = keyValueArray[1]
		}
	}

	statistics := ""
	if val, ok := paramKeyValueMap["stat"]; ok {
		statistics = val
	}

	testPath := ""
	if val, ok := paramKeyValueMap["master"]; ok {
		testPath += val
	} else {
		return "", fmt.Errorf("cannot get master from trace name: %q", traceName)
	}

	if val, ok := paramKeyValueMap["bot"]; ok {
		testPath += "/" + val
	} else {
		return "", fmt.Errorf("cannot get bot from trace name: %q", traceName)
	}

	if val, ok := paramKeyValueMap["benchmark"]; ok {
		testPath += "/" + val
	} else {
		return "", fmt.Errorf("cannot get benchmark from trace name: %q", traceName)
	}

	if val, ok := paramKeyValueMap["test"]; ok {
		if statToSuffixRequired {
			// Due to historical reasons, for a Chromeperf test path like .../testName_max/...,
			// we may have three possible data in a paramset in Skia perf:
			//   1. test=testName_max,stat=value
			//   2. test=testName_max,stat=max
			//   3. test=testName,stat=max
			// For either of the above, we want to have the same test path.
			// Except that:
			//   We don't have a good way to handle when stat value is 'value'. The original test_path
			// can be .../testName/... or .../testName_avg/...
			// In favor of the existing customers, we will use _avg to load anomalies.
			// The value of "average" is put here as a place holder that we can tell the difference if we decide
			// to use different stat value for testName and testName_avg.
			// As a result, anomalies detected with the 'test' as 'testName' will NOT be able to be loaded.
			// If the test value has stat suffix already (case #1, #2), we don't need to do anything.
			// Otherwise, attach the correct stat suffix to the test value.
			if !hasSuffixInTestValue(val, statToSuffixMap) {
				suffix, ok := statToSuffixMap[statistics]
				if ok {
					val += "_" + suffix
				} else if statistics == "value" {
					val += "_" + "avg"
				} else {
					sklog.Warningf("Unexpected value for statistics in paramset: %s", statistics)
				}
			}
		}
		testPath += "/" + val
	} else {
		return "", fmt.Errorf("cannot get test from trace name: %q", traceName)
	}

	for i := 1; i <= 7; i++ {
		key := "subtest_" + strconv.Itoa(i)

		if val, ok := paramKeyValueMap[key]; ok {
			testPath += "/" + val
		} else {
			break
		}
	}

	return testPath, nil
}

func hasSuffixInTestValue(testValue string, statToSuffixMap map[string]string) bool {
	testParts := strings.Split(testValue, "_")
	suffix := testParts[len(testParts)-1]
	for _, value := range statToSuffixMap {
		if suffix == value {
			return true
		}
	}
	return false
}

func (cp *anomalyApiClientImpl) getAnomalyMapFromChromePerfResult(ctx context.Context, getAnomaliesResp *GetAnomaliesResponse, testPathTraceNameMap map[string]string) AnomalyMap {
	result := AnomalyMap{}
	for testPath, anomalyArr := range getAnomaliesResp.Anomalies {
		sklog.Debugf("Received Anomalies on testPath: %v, with anomalyArr: %v", testPath, anomalyArr)
		if traceName, ok := testPathTraceNameMap[testPath]; !ok {
			sklog.Errorf("Got unknown test path %s from chrome perf for testPathTraceNameMap: %s", testPath, testPathTraceNameMap)
		} else {
			commitNumberAnomalyMap := CommitNumberAnomalyMap{}
			for _, anomaly := range anomalyArr {
				if anomaly.EndRevisionHash == "" {
					commitNumberAnomalyMap[types.CommitNumber(anomaly.EndRevision)] = anomaly
				} else {
					commitNum, err := cp.perfGit.CommitNumberFromGitHash(ctx, anomaly.EndRevisionHash)
					if err != nil {
						sklog.Errorf("Unable to find commit number for git hash %s", anomaly.EndRevisionHash)
						continue
					}

					commitNumberAnomalyMap[commitNum] = anomaly
				}

			}
			result[traceName] = commitNumberAnomalyMap
			sklog.Debugf("Logged traceName: %v, with commitNumberAnomalyMap: %v", traceName, commitNumberAnomalyMap)
		}
	}

	return result
}

func getParams(testPath string) map[string][]string {
	params := map[string][]string{}
	testPathParts := strings.Split(testPath, "/")
	if len(testPathParts) == 0 {
		return params
	}
	paramKeys := []string{"master", "bot", "benchmark", "test", "subtest_1", "subtest_2", "subtest_3", "subtest_4", "subtest_5"}
	if len(testPathParts) > len(paramKeys) {
		return params
	}

	i := 0
	for _, testPart := range testPathParts {
		for _, invalidChar := range invalidChars {
			testPart = strings.ReplaceAll(testPart, invalidChar, "_")
		}
		if _, ok := params[paramKeys[i]]; !ok {
			params[paramKeys[i]] = []string{testPart}
		} else {
			params[paramKeys[i]] = append(params[paramKeys[i]], testPart)
		}
		i++
	}

	return params
}
