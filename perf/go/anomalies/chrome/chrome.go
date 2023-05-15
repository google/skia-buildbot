package chrome

import (
	"fmt"
	"strconv"
	"strings"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/anomalies"
)

// ChromePerfClient implements ChromePerf.
type ChromePerfClient struct {
	getAnomaliesCalled metrics2.Counter
	getAnomaliesFailed metrics2.Counter
}

// New returns a new ChromePerf instance.
func New() *ChromePerfClient {
	return &ChromePerfClient{
		getAnomaliesCalled: metrics2.GetCounter("chrome_perf_get_anomalies_called"),
		getAnomaliesFailed: metrics2.GetCounter("chrome_perf_get_anomalies_failed"),
	}
}

// GetAnomalies implements ChromePerf, it calls chrome perf API to fetch anomlies.
func (cp *ChromePerfClient) GetAnomalies(traceNames []string, startCommitPosition int, endCommitPosition int) (anomalies.AnomalyMap, error) {
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

	// TODO(sunpeng) implement this method: call Chrome Perf API to fetch anomalies.

	return nil, nil
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
