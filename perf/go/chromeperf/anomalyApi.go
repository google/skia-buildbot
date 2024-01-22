package chromeperf

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/types"
)

const (
	AnomalyAPIName   = "anomalies"
	AddFuncName      = "add"
	FindFuncName     = "find"
	FindTimeFuncName = "find_time"
)

// CommitNumberAnomalyMap is a map of Anomaly, keyed by commit number.
type CommitNumberAnomalyMap map[types.CommitNumber]Anomaly

// AnomalyMap is a map of CommitNumberAnomalyMap, keyed by traceId.
type AnomalyMap map[string]CommitNumberAnomalyMap

// Anomaly defines the object return from Chrome Perf API.
type Anomaly struct {
	Id                  int     `json:"id"`
	TestPath            string  `json:"test_path"`
	BugId               int     `json:"bug_id"`
	StartRevision       int     `json:"start_revision"`
	EndRevision         int     `json:"end_revision"`
	IsImprovement       bool    `json:"is_improvement"`
	Recovered           bool    `json:"recovered"`
	State               string  `json:"state"`
	Statistics          string  `json:"statistic"`
	Unit                string  `json:"units"`
	DegreeOfFreedom     float64 `json:"degrees_of_freedom"`
	MedianBeforeAnomaly float64 `json:"median_before_anomaly"`
	MedianAfterAnomaly  float64 `json:"median_after_anomaly"`
	PValue              float64 `json:"p_value"`
	SegmentSizeAfter    int     `json:"segment_size_after"`
	SegmentSizeBefore   int     `json:"segment_size_before"`
	StdDevBeforeAnomaly float64 `json:"std_dev_before_anomaly"`
	TStatistics         float64 `json:"t_statistic"`
}

// AnomalyForRevision defines struct to contain anomaly data for a specific revision
type AnomalyForRevision struct {
	StartRevision int                 `json:"start_revision"`
	EndRevision   int                 `json:"end_revision"`
	Anomaly       Anomaly             `json:"anomaly"`
	Params        map[string][]string `json:"params"`
	TestPath      string              `json:"test_path"`
}

// RevisionInfo defines struct to contain revision information
type RevisionInfo struct {
	Master        string `json:"master"`
	Bot           string `json:"bot"`
	Benchmark     string `json:"benchmark"`
	StartRevision int    `json:"start_revision"`
	EndRevision   int    `json:"end_revision"`
	Test          string `json:"test"`
	IsImprovement bool   `json:"is_improvement"`
	BugId         string `json:"bug_id"`
	ExploreUrl    string `json:"explore_url"`
}

// GetAnomaliesRequest struct to request anomalies from the chromeperf api.
// The parameters can be one of below described.
// 1. Revision: Retrieves anomalies around that revision number.
// 2. Tests-MinRevision-MaxRevision: Retrieves anomalies for the given set of tests between the min and max revisions
type GetAnomaliesRequest struct {
	Tests       []string `json:"tests,omitempty"`
	MaxRevision string   `json:"max_revision,omitempty"`
	MinRevision string   `json:"min_revision,omitempty"`
	Revision    int      `json:"revision,omitempty"`
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

	// GetAnomalies retrieves anomalies for a given set of traces within the supplied commit positions.
	GetAnomalies(ctx context.Context, traceNames []string, startCommitPosition int, endCommitPosition int) (AnomalyMap, error)

	// GetAnomaliesTimeBased retrieves anomalies for a given set of traces within the supplied commit positions.
	GetAnomaliesTimeBased(ctx context.Context, traceNames []string, startTime time.Time, endTime time.Time) (AnomalyMap, error)

	// GetAnomaliesAroundRevision retrieves traces with anomalies that were generated around a specific commit
	GetAnomaliesAroundRevision(ctx context.Context, revision int) ([]AnomalyForRevision, error)
}

// anomalyApiClientImpl implements AnomalyApiClient
type anomalyApiClientImpl struct {
	chromeperfClient   chromePerfClient
	sendAnomalyCalled  metrics2.Counter
	sendAnomalyFailed  metrics2.Counter
	getAnomaliesCalled metrics2.Counter
	getAnomaliesFailed metrics2.Counter
}

// NewAnomalyApiClient returns a new AnomalyApiClient instance.
func NewAnomalyApiClient(ctx context.Context) (AnomalyApiClient, error) {
	cpClient, err := newChromePerfClient(ctx, "")
	if err != nil {
		return nil, err
	}

	return newAnomalyApiClient(cpClient), nil
}

func newAnomalyApiClient(cpClient chromePerfClient) AnomalyApiClient {
	return &anomalyApiClientImpl{
		chromeperfClient:   cpClient,
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
	err := cp.chromeperfClient.sendPostRequest(ctx, AnomalyAPIName, AddFuncName, request, &response, acceptedStatusCodes)
	if err != nil {
		cp.sendAnomalyFailed.Inc(1)
		return nil, skerr.Wrapf(err, "Failed to get chrome perf response when sending anomalies.")
	}

	cp.sendAnomalyCalled.Inc(1)
	return &response, nil
}

// GetAnomalies implements AnomalyApiClient, it calls chrome perf API to fetch anomlies.
func (cp *anomalyApiClientImpl) GetAnomalies(ctx context.Context, traceNames []string, startCommitPosition int, endCommitPosition int) (AnomalyMap, error) {
	testPathes := make([]string, 0)
	testPathTraceNameMap := make(map[string]string)
	for _, traceName := range traceNames {
		// Build chrome perf test_path from skia perf traceName
		testPath, stat, err := traceNameToTestPath(traceName)
		if err != nil {
			sklog.Errorf("Failed to build chrome perf test path from trace name %q: %s", traceName, err)
		} else if stat == "value" { // We will only show anomalies for the traces of the 'value' stat.
			testPathes = append(testPathes, testPath)
			testPathTraceNameMap[testPath] = traceName
		}
	}

	if len(testPathes) > 0 {
		cp.getAnomaliesCalled.Inc(1)
		// Call Chrome Perf API to fetch anomalies.
		request := &GetAnomaliesRequest{
			Tests:       testPathes,
			MaxRevision: strconv.Itoa(endCommitPosition),
			MinRevision: strconv.Itoa(startCommitPosition),
		}
		getAnomaliesResp := &GetAnomaliesResponse{}
		err := cp.chromeperfClient.sendPostRequest(ctx, AnomalyAPIName, FindFuncName, *request, getAnomaliesResp, []int{200})
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to call chrome perf endpoint.")
		}
		return getAnomalyMapFromChromePerfResult(getAnomaliesResp, testPathTraceNameMap), nil
	}

	return AnomalyMap{}, nil
}

func (cp *anomalyApiClientImpl) GetAnomaliesTimeBased(ctx context.Context, traceNames []string, startTime time.Time, endTime time.Time) (AnomalyMap, error) {
	testPaths := make([]string, 0)
	testPathTraceNameMap := make(map[string]string)
	for _, traceName := range traceNames {
		// Build chrome perf test_path from skia perf traceName
		testPath, stat, err := traceNameToTestPath(traceName)
		if err != nil {
			sklog.Errorf("Failed to build chrome perf test path from trace name %q: %s", traceName, err)
		} else if stat == "value" { // We will only show anomalies for the traces of the 'value' stat.
			testPaths = append(testPaths, testPath)
			testPathTraceNameMap[testPath] = traceName
		}
	}

	if len(testPaths) > 0 {
		cp.getAnomaliesCalled.Inc(1)
		// Call Chrome Perf API to fetch anomalies.
		request := &GetAnomaliesTimeBasedRequest{
			Tests:     testPaths,
			StartTime: startTime,
			EndTime:   endTime,
		}
		getAnomaliesResp := &GetAnomaliesResponse{}
		err := cp.chromeperfClient.sendPostRequest(ctx, AnomalyAPIName, FindTimeFuncName, *request, getAnomaliesResp, []int{200})
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to call chrome perf endpoint.")
		}
		return getAnomalyMapFromChromePerfResult(getAnomaliesResp, testPathTraceNameMap), nil
	}

	return AnomalyMap{}, nil
}

// GetAnomaliesAroundRevision implements AnomalyApiClient, returns anomalies around the given revision.
func (cp *anomalyApiClientImpl) GetAnomaliesAroundRevision(ctx context.Context, revision int) ([]AnomalyForRevision, error) {
	request := &GetAnomaliesRequest{
		Revision: revision,
	}

	getAnomaliesResp := &GetAnomaliesResponse{}
	err := cp.chromeperfClient.sendPostRequest(ctx, AnomalyAPIName, FindFuncName, *request, getAnomaliesResp, []int{200})
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to call chrome perf endpoint.")
	}

	response := []AnomalyForRevision{}
	for testPath, cpAnomalies := range getAnomaliesResp.Anomalies {
		for _, anomaly := range cpAnomalies {
			response = append(response, AnomalyForRevision{
				TestPath:      testPath,
				StartRevision: anomaly.StartRevision,
				EndRevision:   anomaly.EndRevision,
				Anomaly:       anomaly,
				Params:        getParams(testPath),
			})
		}
	}

	return response, nil
}

// traceNameToTestPath converts trace name to Chrome Perf test path.
// For example, for the trace name, ",benchmark=Blazor,bot=MacM1,
// master=ChromiumPerf,test=timeToFirstContentfulPaint_avg,subtest_1=111,
// subtest_2=222,...subtest_7=777,unit=microsecond,improvement_direction=up,"
// test path will be: "ChromiumPerf/MacM1/Blazor/timeToFirstContentfulPaint_avg
// /111/222/.../777"
// It also returns the trace statistics like 'value', 'sum', 'max', 'min' or 'error'
func traceNameToTestPath(traceName string) (string, string, error) {
	keyValueEquations := strings.Split(traceName, ",")
	if len(keyValueEquations) == 0 {
		return "", "", fmt.Errorf("Cannot build test path from trace name: %q.", traceName)
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
		return "", "", fmt.Errorf("Cannot get master from trace name: %q.", traceName)
	}

	if val, ok := paramKeyValueMap["bot"]; ok {
		testPath += "/" + val
	} else {
		return "", "", fmt.Errorf("Cannot get bot from trace name: %q.", traceName)
	}

	if val, ok := paramKeyValueMap["benchmark"]; ok {
		testPath += "/" + val
	} else {
		return "", "", fmt.Errorf("Cannot get benchmark from trace name: %q.", traceName)
	}

	if val, ok := paramKeyValueMap["test"]; ok {
		testPath += "/" + val
	} else {
		return "", "", fmt.Errorf("Cannot get test from trace name: %q.", traceName)
	}

	for i := 1; i <= 7; i++ {
		key := "subtest_" + strconv.Itoa(i)

		if val, ok := paramKeyValueMap[key]; ok {
			testPath += "/" + val
		} else {
			break
		}
	}

	return testPath, statistics, nil
}

func getAnomalyMapFromChromePerfResult(getAnomaliesResp *GetAnomaliesResponse, testPathTraceNameMap map[string]string) AnomalyMap {
	result := AnomalyMap{}
	for testPath, anomalyArr := range getAnomaliesResp.Anomalies {
		if traceName, ok := testPathTraceNameMap[testPath]; !ok {
			sklog.Errorf("Got unknown test path %s from chrome perf for testPathTraceNameMap: %s", testPath, testPathTraceNameMap)
		} else {
			commitNumberAnomalyMap := CommitNumberAnomalyMap{}
			for _, anomaly := range anomalyArr {
				commitNumberAnomalyMap[types.CommitNumber(anomaly.EndRevision)] = anomaly
			}
			result[traceName] = commitNumberAnomalyMap
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
	paramKeys := []string{"master", "bot", "benchmark", "test", "subtest_1", "subtest_2", "subtest_3"}
	if len(testPathParts) > len(paramKeys) {
		return params
	}

	i := 0
	for _, testPart := range testPathParts {
		if _, ok := params[paramKeys[i]]; !ok {
			params[paramKeys[i]] = []string{testPart}
		} else {
			params[paramKeys[i]] = append(params[paramKeys[i]], testPart)
		}
		i++
	}

	return params
}
