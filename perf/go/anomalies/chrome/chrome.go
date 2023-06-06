package chrome

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/anomalies"
	"go.skia.org/infra/perf/go/types"
	"golang.org/x/oauth2/google"
)

const (
	chromePerfURL = "https://skia-bridge-dot-chromeperf.appspot.com/anomalies/find"
	contentType   = "application/json"
)

type chromePerfRequest struct {
	Tests       []string `json:"tests"`
	MaxRevision string   `json:"max_revision"`
	MinRevision string   `json:"min_revision"`
}

type chromePerfResponse struct {
	Anomalies map[string][]anomalies.Anomaly `json:"anomalies"`
}

// ChromePerfClient implements anomalies.Store.
type ChromePerfClient struct {
	httpClient         *http.Client
	getAnomaliesCalled metrics2.Counter
	getAnomaliesFailed metrics2.Counter
}

// New returns a new ChromePerf instance.
func New(ctx context.Context) (*ChromePerfClient, error) {
	tokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeAllCloudAPIs)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create chrome perf client.")
	}

	client := httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client()
	return &ChromePerfClient{
		httpClient:         client,
		getAnomaliesCalled: metrics2.GetCounter("chrome_perf_get_anomalies_called"),
		getAnomaliesFailed: metrics2.GetCounter("chrome_perf_get_anomalies_failed"),
	}, nil
}

// GetAnomalies implements ChromePerf, it calls chrome perf API to fetch anomlies.
func (cp *ChromePerfClient) GetAnomalies(ctx context.Context, traceNames []string, startCommitPosition int, endCommitPosition int) (anomalies.AnomalyMap, error) {
	cp.getAnomaliesCalled.Inc(1)
	testPathes := make([]string, 0)
	testPathTraceNameMap := make(map[string]string)
	for _, traceName := range traceNames {
		// Build chrome perf test_path from skia perf traceName
		testPath, err := TraceNameToTestPath(traceName)
		if err != nil {
			sklog.Errorf("Failed to build chrome perf test path from trace name %q: %s", traceName, err)
		} else {
			testPathes = append(testPathes, testPath)
			testPathTraceNameMap[testPath] = traceName
		}
	}

	// Call Chrome Perf API to fetch anomalies.
	chromePerfResp, err := cp.callChromePerf(ctx, testPathes, startCommitPosition, endCommitPosition)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to call chrome perf endpoint.")
	}

	return GetAnomalyMapFromChromePerfResult(chromePerfResp, testPathTraceNameMap), nil
}

func GetAnomalyMapFromChromePerfResult(chromePerfResponse *chromePerfResponse, testPathTraceNameMap map[string]string) anomalies.AnomalyMap {
	result := anomalies.AnomalyMap{}
	for testPath, anomalyArr := range chromePerfResponse.Anomalies {
		if traceName, ok := testPathTraceNameMap[testPath]; !ok {
			sklog.Errorf("Got unknown test path %s from chrome perf for testPathTraceNameMap: %s", testPath, testPathTraceNameMap)
		} else {
			commitNumberAnomalyMap := anomalies.CommitNumberAnomalyMap{}
			for _, anomaly := range anomalyArr {
				commitNumberAnomalyMap[types.CommitNumber(anomaly.EndRevision)] = anomaly
			}
			result[traceName] = commitNumberAnomalyMap
		}
	}

	return result
}

func (cp *ChromePerfClient) callChromePerf(ctx context.Context, testPathes []string, startCommitPosition int, endCommitPosition int) (*chromePerfResponse, error) {
	request := &chromePerfRequest{
		Tests:       testPathes,
		MaxRevision: strconv.Itoa(endCommitPosition),
		MinRevision: strconv.Itoa(startCommitPosition),
	}
	requestBodyJSONStr, err := json.Marshal(request)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create chrome perf request.")
	}

	httpResponse, err := httputils.PostWithContext(ctx, cp.httpClient, chromePerfURL, contentType, strings.NewReader(string(requestBodyJSONStr)))
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get chrome perf response.")
	}

	respBody, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to read body from chrome perf response.")
	}

	resp := chromePerfResponse{}
	err = json.Unmarshal([]byte(respBody), &resp)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse chrome perf response body.")
	}

	return &resp, nil
}

// traceNameToTestPath converts trace name to Chrome Perf test path.
// For example, for the trace name, ",benchmark=Blazor,bot=MacM1,
// master=ChromiumPerf,test=timeToFirstContentfulPaint_avg,subtest_1=111,
// subtest_2=222,...subtest_7=777,unit=microsecond,improvement_direction=up,"
// test path will be: "ChromiumPerf/MacM1/Blazor/timeToFirstContentfulPaint_avg
// /111/222/.../777"
func TraceNameToTestPath(traceName string) (testPath string, err error) {
	keyValueEquations := strings.Split(traceName, ",")
	if len(keyValueEquations) == 0 {
		return "", fmt.Errorf("Cannot build test path from trace name: %q.", traceName)
	}

	paramKeyValueMap := map[string]string{}
	for _, keyValueEquation := range keyValueEquations {
		keyValueArray := strings.Split(keyValueEquation, "=")
		if len(keyValueArray) == 2 {
			paramKeyValueMap[keyValueArray[0]] = keyValueArray[1]
		}
	}

	testPath = ""
	if val, ok := paramKeyValueMap["master"]; ok {
		testPath += val
	} else {
		return "", fmt.Errorf("Cannot get master from trace name: %q.", traceName)
	}

	if val, ok := paramKeyValueMap["bot"]; ok {
		testPath += "/" + val
	} else {
		return "", fmt.Errorf("Cannot get bot from trace name: %q.", traceName)
	}

	if val, ok := paramKeyValueMap["benchmark"]; ok {
		testPath += "/" + val
	} else {
		return "", fmt.Errorf("Cannot get benchmark from trace name: %q.", traceName)
	}

	if val, ok := paramKeyValueMap["test"]; ok {
		testPath += "/" + val
	} else {
		return "", fmt.Errorf("Cannot get test from trace name: %q.", traceName)
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
