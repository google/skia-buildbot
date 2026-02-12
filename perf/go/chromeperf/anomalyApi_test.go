package chromeperf

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/perf/go/config"
)

var CFG = &config.InstanceConfig{
	Experiments: config.Experiments{
		EnableSkiaBridgeAggregation: false,
	},
}

// newChromePerfClientWithoutTokenSource creates a new instance of ChromePerfClient without a tokenSource.
func newChromePerfClientWithoutTokenSource(urlOverride string, directCall bool) (ChromePerfClient, error) {
	return &chromePerfClientImpl{
		httpClient:       httputils.DefaultClientConfig().Client(),
		urlOverride:      urlOverride,
		directCallLegacy: directCall,
	}, nil
}

func TestSendRegression_RequestIsValid_Success(t *testing.T) {
	anomalyResponse := &ReportRegressionResponse{
		AnomalyId:    "1234",
		AlertGroupId: "5678",
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewEncoder(w).Encode(anomalyResponse)
		require.NoError(t, err)
	}))

	ctx := context.Background()
	cpClient, err := newChromePerfClientWithoutTokenSource(ts.URL, false)
	assert.Nil(t, err, "No error expected when creating a new client.")
	anomalyClient := newAnomalyApiClient(cpClient, nil, CFG)
	response, err := anomalyClient.ReportRegression(ctx, "/some/path", 1, 10, "proj", false, "bot", false, 5, 10)
	assert.NotNil(t, response)
	assert.Nil(t, err, "No error expected in the SendRegression call.")
	assert.Equal(t, anomalyResponse, response)
}

func TestSendRegression_ServerReturnsError_ReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))

	defer ts.Close()
	ctx := context.Background()
	cpClient, err := newChromePerfClientWithoutTokenSource(ts.URL, false)
	assert.Nil(t, err, "No error expected when creating a new client.")
	anomalyClient := newAnomalyApiClient(cpClient, nil, CFG)
	response, err := anomalyClient.ReportRegression(ctx, "/some/path", 1, 10, "proj", false, "bot", false, 5, 10)
	assert.Nil(t, response, "Nil response expected for server error.")
	assert.NotNil(t, err, "Non nil error expected.")
}

func TestTraceNameToTestPath(t *testing.T) {
	for name, subTest := range subTests {
		t.Run(name, func(t *testing.T) {
			subTest.subTestFunction(t, subTest.traceName)
		})
	}
}

// subTestFunction is a func we will call to test one aspect of *SQLTraceStore.
type subTestFunction func(t *testing.T, traceName string)

var subTests = map[string]struct {
	subTestFunction subTestFunction
	traceName       string
}{
	"testTraceNameToTestPath_Success":                          {testTraceNameToTestPath_Success, ",stat=value,benchmark=Blazor,bot=MacM1,master=ChromiumPerf,test=timeToFirstContentfulPaint_avg,subtest_1=subtest111,subtest_2=subtest222,subtest_3=subtest333,subtest_4=subtest444,subtest_5=subtest555,subtest_6=subtest666,subtest_7=subtest777,unit=microsecond,improvement_direction=up,"},
	"testTraceNameToTestPath_StatNotValue_NoTracePathReturned": {testTraceNameToTestPath_StatNotValue_NoTracePathReturned, ",stat=error,benchmark=Blazor,bot=MacM1,master=ChromiumPerf,test=timeToFirstContentfulPaint_avg,subtest_1=subtest111,subtest_2=subtest222,subtest_3=subtest333,subtest_4=subtest444,subtest_5=subtest555,subtest_6=subtest666,subtest_7=subtest777,unit=microsecond,improvement_direction=up,"},
	"testTraceNameToTestPath_NoMaster_Error":                   {testTraceNameToTestPath_NoMaster_Error, ",stat=value,benchmark=Blazor,bot=MacM1,test=timeToFirstContentfulPaint_avg,subtest_1=subtest111,subtest_2=subtest222,subtest_3=subtest333,subtest_4=subtest444,subtest_5=subtest555,subtest_6=subtest666,subtest_7=subtest777,unit=microsecond,improvement_direction=up,"},
	"testTraceNameToTestPath_NoBot_Error":                      {testTraceNameToTestPath_NoBot_Error, ",stat=value,benchmark=Blazor,master=ChromiumPerf,test=timeToFirstContentfulPaint_avg,subtest_1=subtest111,subtest_2=subtest222,subtest_3=subtest333,subtest_4=subtest444,subtest_5=subtest555,subtest_6=subtest666,subtest_7=subtest777,unit=microsecond,improvement_direction=up,"},
	"testTraceNameToTestPath_NoTest_Error":                     {testTraceNameToTestPath_NoTest_Error, ",stat=value,benchmark=Blazor,bot=MacM1,master=ChromiumPerf,subtest_1=subtest111,subtest_2=subtest222,subtest_3=subtest333,subtest_4=subtest444,subtest_5=subtest555,subtest_6=subtest666,subtest_7=subtest777,unit=microsecond,improvement_direction=up,"},
	"testTraceNameToTestPath_InvalidTraceName_Error":           {testTraceNameToTestPath_InvalidTraceName_Error, "stat=value,benchmark=Blazor.bot=MacM1.master=ChromiumPerf.test=timeToFirstContentfulPaint_avg.subtest_1=subtest111.subtest_2=subtest222.subtest_3=subtest333.subtest_4=subtest444.subtest_5=subtest555.subtest_6=subtest666.subtest_7=subtest777.unit=microsecond.improvement_direction=up."},
	"testTraceNameToTestPath_WithComma_Success":                {testTraceNameToTestPath_WithComma_Success, ",stat=value,benchmark=Blazor,bot=MacM1,master=ChromiumPerf,test=timeToFirstContentfulPaint_avg,subtest_1=Array.slice(200,700),unit=microsecond,improvement_direction=up,"},
}

func testTraceNameToTestPath_Success(t *testing.T, traceName string) {
	testPath, err := TraceNameToTestPath(traceName, false)
	require.NoError(t, err)
	assert.Equal(t, "ChromiumPerf/MacM1/Blazor/timeToFirstContentfulPaint_avg/subtest111/subtest222/subtest333/subtest444/subtest555/subtest666/subtest777", testPath)
}

func testTraceNameToTestPath_StatNotValue_NoTracePathReturned(t *testing.T, traceName string) {
	testPath, err := TraceNameToTestPath(traceName, false)
	require.NoError(t, err)
	assert.Equal(t, "ChromiumPerf/MacM1/Blazor/timeToFirstContentfulPaint_avg/subtest111/subtest222/subtest333/subtest444/subtest555/subtest666/subtest777", testPath)
}

func testTraceNameToTestPath_NoMaster_Error(t *testing.T, traceName string) {
	testPath, err := TraceNameToTestPath(traceName, false)
	require.Error(t, err)
	assert.Equal(t, "", testPath)
}

func testTraceNameToTestPath_NoBot_Error(t *testing.T, traceName string) {
	testPath, err := TraceNameToTestPath(traceName, false)
	require.Error(t, err)
	assert.Equal(t, "", testPath)
}

func testTraceNameToTestPath_NoTest_Error(t *testing.T, traceName string) {
	testPath, err := TraceNameToTestPath(traceName, false)
	require.Error(t, err)
	assert.Equal(t, "", testPath)
}

func testTraceNameToTestPath_InvalidTraceName_Error(t *testing.T, traceName string) {
	testPath, err := TraceNameToTestPath(traceName, false)
	require.Error(t, err)
	assert.Equal(t, "", testPath)
}

func testTraceNameToTestPath_WithComma_Success(t *testing.T, traceName string) {
	testPath, err := TraceNameToTestPath(traceName, false)
	require.NoError(t, err)
	assert.Equal(t, "ChromiumPerf/MacM1/Blazor/timeToFirstContentfulPaint_avg/Array.slice(200,700)", testPath)
}

func TestTraceNameToTestPath_WithStat(t *testing.T) {
	var subTests = map[string]struct {
		oldParams      string
		expectedParams string
	}{
		// If 'test' has a suffix, the 'stat' will be ignored.
		"TestWithSuffixWithGeneralStat": {
			",benchmark=Blazor,bot=MacM1,master=ChromiumPerf,test=timeToFirstContentfulPaint_max,stat=value,subtest_1=subtest111,",
			"ChromiumPerf/MacM1/Blazor/timeToFirstContentfulPaint_max/subtest111",
		},
		"TestWithSuffixWithMatchingStat": {
			",benchmark=Blazor,bot=MacM1,master=ChromiumPerf,test=timeToFirstContentfulPaint_max,stat=max,subtest_1=subtest111,",
			"ChromiumPerf/MacM1/Blazor/timeToFirstContentfulPaint_max/subtest111",
		},
		// If 'test' has no suffix, we should find a suffix based on 'stat'.
		"TestWithNoSuffixWithMatchingStat": {
			",benchmark=Blazor,bot=MacM1,master=ChromiumPerf,test=timeToFirstContentfulPaint,stat=max,subtest_1=subtest111,",
			"ChromiumPerf/MacM1/Blazor/timeToFirstContentfulPaint_max/subtest111",
		},
		"TestWithNoSuffixWithStatValue": {
			",benchmark=Blazor,bot=MacM1,master=ChromiumPerf,test=timeToFirstContentfulPaint,stat=value,subtest_1=subtest111,",
			"ChromiumPerf/MacM1/Blazor/timeToFirstContentfulPaint_avg/subtest111",
		},
		"TestWithNoSuffixWithNoStat": {
			",benchmark=Blazor,bot=MacM1,master=ChromiumPerf,test=timeToFirstContentfulPaint,subtest_1=subtest111,",
			"ChromiumPerf/MacM1/Blazor/timeToFirstContentfulPaint/subtest111",
		},
	}
	for name, subTest := range subTests {
		t.Run(name, func(t *testing.T) {
			newTestPath, err := TraceNameToTestPath(subTest.oldParams, true)
			assert.NoError(t, err)
			assert.Equal(t, subTest.expectedParams, newTestPath)
		})
	}
}

func TestHasSuffixInTestValue(t *testing.T) {
	var fake_mapping = map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	var subTests = map[string]struct {
		stringValue string
		expected    bool
	}{
		"TestValueInMap": {
			"abc_value2", true,
		},
		"TestValueNotInMap": {
			"abc_value3", false,
		},
		"TestValueWithNoUC": {
			"abc", false,
		},
	}
	for name, subTest := range subTests {
		t.Run(name, func(t *testing.T) {
			result := hasSuffixInTestValue(subTest.stringValue, fake_mapping)
			assert.Equal(t, subTest.expected, result)
		})
	}
}

func TestGetAnomaly_Success(t *testing.T) {
	master := "m"
	bot := "testBot"
	benchmark := "bench"
	test := "myTest"
	subtest := "mysubtest"
	testPath := fmt.Sprintf("%s/%s/%s/%s/%s", master, bot, benchmark, test, subtest)
	subscription_name := "sub_name"
	bug_component := "bug_component"
	bug_labels := []string{"label1", "label2", "label3"}
	anomaly := Anomaly{
		StartRevision:    1111,
		EndRevision:      2222,
		TestPath:         testPath,
		SubscriptionName: subscription_name,
		BugComponent:     bug_component,
		BugLabels:        bug_labels,
	}
	anomalyResponse := &GetAnomaliesResponse{
		Anomalies: map[string][]Anomaly{
			testPath: {anomaly},
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewEncoder(w).Encode(anomalyResponse)
		require.NoError(t, err)
	}))

	ctx := context.Background()
	cpClient, err := newChromePerfClientWithoutTokenSource(ts.URL, false)
	assert.Nil(t, err, "No error expected when creating a new client.")
	anomalyClient := newAnomalyApiClient(cpClient, nil, CFG)
	params, anomalyResp, err := anomalyClient.GetAnomalyFromUrlSafeKey(ctx, "someKey")
	assert.Nil(t, err)
	assert.Equal(t, anomaly.StartRevision, anomalyResp.StartRevision)
	assert.Equal(t, anomaly.EndRevision, anomalyResp.EndRevision)
	assert.Equal(t, master, params["master"][0])
	assert.Equal(t, bot, params["bot"][0])
	assert.Equal(t, benchmark, params["benchmark"][0])
	assert.Equal(t, test, params["test"][0])
	assert.Equal(t, subtest, params["subtest_1"][0])
	assert.Equal(t, subscription_name, anomalyResp.SubscriptionName)
	assert.Equal(t, bug_component, anomalyResp.BugComponent)
	assert.Equal(t, 3, len(anomalyResp.BugLabels))
	assert.Equal(t, 0, len(anomalyResp.BugCcEmails))
	assert.Equal(t, 0, len(anomalyResp.BisectIDs))
}

func TestGetAnomaly_InvalidChar_Success(t *testing.T) {
	master := "m"
	bot := "testBot"
	benchmark := "bench"
	test := "myTest"
	subtest := "mysubtest?withinvalidchar"
	testPath := fmt.Sprintf("%s/%s/%s/%s/%s", master, bot, benchmark, test, subtest)
	anomaly := Anomaly{
		StartRevision: 1111,
		EndRevision:   2222,
		TestPath:      testPath,
	}
	anomalyResponse := &GetAnomaliesResponse{
		Anomalies: map[string][]Anomaly{
			testPath: {anomaly},
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewEncoder(w).Encode(anomalyResponse)
		require.NoError(t, err)
	}))

	ctx := context.Background()
	cpClient, err := newChromePerfClientWithoutTokenSource(ts.URL, false)
	assert.Nil(t, err, "No error expected when creating a new client.")
	anomalyClient := newAnomalyApiClient(cpClient, nil, CFG)

	resp, err := anomalyClient.GetAnomaliesAroundRevision(ctx, 1234)
	require.NoError(t, err)
	assert.Equal(t, 1, len(resp))
	assert.Equal(t, "mysubtest_withinvalidchar", resp[0].Params["subtest_1"][0])
}

func TestDedupStringSlice(t *testing.T) {
	var subTests = map[string]struct {
		input    []string
		expected []string
	}{
		"TestDedupStringSlice_empty":      {[]string{}, []string{}},
		"TestDedupStringSlice_no_dup":     {[]string{"abc", "def"}, []string{"abc", "def"}},
		"TestDedupStringSlice_remove_dup": {[]string{"abc", "abc", "def", "abc"}, []string{"abc", "def"}},
	}
	for name, subTest := range subTests {
		t.Run(name, func(t *testing.T) {
			got := DedupStringSlice(subTest.input)
			assert.Equal(t, got, subTest.expected)
		})
	}
}

func TestAnomaly_UnmarshalJSON_IdAsString(t *testing.T) {
	jsonData := `{"id": "12345"}`
	var anomaly Anomaly
	err := json.Unmarshal([]byte(jsonData), &anomaly)
	assert.NoError(t, err)
	assert.Equal(t, "12345", anomaly.Id)
}

func TestAnomaly_UnmarshalJSON_IdAsNumber(t *testing.T) {
	jsonData := `{"id": 12345}`
	var anomaly Anomaly
	err := json.Unmarshal([]byte(jsonData), &anomaly)
	assert.NoError(t, err)
	assert.Equal(t, "12345", anomaly.Id)
}

func TestAnomaly_UnmarshalJSON_FullObject(t *testing.T) {
	jsonData := `{
		"id": 12345,
		"test_path": "some/test/path",
		"bug_id": 54321,
		"start_revision": 1,
		"end_revision": 2,
		"is_improvement": true,
		"recovered": false,
		"state": "some_state",
		"statistic": "avg",
		"units": "ms",
		"degrees_of_freedom": 10,
		"median_before_anomaly": 1.1,
		"median_after_anomaly": 2.2,
		"p_value": 0.01,
		"segment_size_after": 100,
		"segment_size_before": 100,
		"std_dev_before_anomaly": 0.5,
		"t_statistic": 2.5,
		"subscription_name": "some_subscription",
		"bug_component": "some_component",
		"bug_labels": ["label1", "label2"],
		"bug_cc_emails": ["a@a.com", "b@b.com"],
		"bisect_ids": ["bisect1", "bisect2"],
		"timestamp": "2025-08-01T12:00:00Z"
	}`
	var anomaly Anomaly
	err := json.Unmarshal([]byte(jsonData), &anomaly)
	assert.NoError(t, err)
	assert.Equal(t, "12345", anomaly.Id)
	assert.Equal(t, "some/test/path", anomaly.TestPath)
	assert.Equal(t, 54321, anomaly.BugId)
	assert.Equal(t, 1, anomaly.StartRevision)
	assert.Equal(t, 2, anomaly.EndRevision)
	assert.True(t, anomaly.IsImprovement)
	assert.False(t, anomaly.Recovered)
	assert.Equal(t, "some_state", anomaly.State)
	assert.Equal(t, "avg", anomaly.Statistics)
	assert.Equal(t, "ms", anomaly.Unit)
	assert.Equal(t, 10.0, anomaly.DegreeOfFreedom)
	assert.Equal(t, 1.1, anomaly.MedianBeforeAnomaly)
	assert.Equal(t, 2.2, anomaly.MedianAfterAnomaly)
	assert.Equal(t, 0.01, anomaly.PValue)
	assert.Equal(t, 100, anomaly.SegmentSizeAfter)
	assert.Equal(t, 100, anomaly.SegmentSizeBefore)
	assert.Equal(t, 0.5, anomaly.StdDevBeforeAnomaly)
	assert.Equal(t, 2.5, anomaly.TStatistics)
	assert.Equal(t, "some_subscription", anomaly.SubscriptionName)
	assert.Equal(t, "some_component", anomaly.BugComponent)
	assert.Equal(t, []string{"label1", "label2"}, anomaly.BugLabels)
	assert.Equal(t, []string{"a@a.com", "b@b.com"}, anomaly.BugCcEmails)
	assert.Equal(t, []string{"bisect1", "bisect2"}, anomaly.BisectIDs)
	assert.Equal(t, "2025-08-01T12:00:00Z", anomaly.Timestamp)
}
