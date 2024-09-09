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
)

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
	cpClient, err := newChromePerfClient(context.Background(), ts.URL)
	assert.Nil(t, err, "No error expected when creating a new client.")
	anomalyClient := newAnomalyApiClient(cpClient)
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
	cpClient, err := newChromePerfClient(context.Background(), ts.URL)
	assert.Nil(t, err, "No error expected when creating a new client.")
	anomalyClient := newAnomalyApiClient(cpClient)
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
}

func testTraceNameToTestPath_Success(t *testing.T, traceName string) {
	testPath, stat, err := traceNameToTestPath(traceName)
	require.NoError(t, err)
	assert.Equal(t, "ChromiumPerf/MacM1/Blazor/timeToFirstContentfulPaint_avg/subtest111/subtest222/subtest333/subtest444/subtest555/subtest666/subtest777", testPath)
	assert.Equal(t, "value", stat)
}

func testTraceNameToTestPath_StatNotValue_NoTracePathReturned(t *testing.T, traceName string) {
	testPath, stat, err := traceNameToTestPath(traceName)
	require.NoError(t, err)
	assert.Equal(t, "ChromiumPerf/MacM1/Blazor/timeToFirstContentfulPaint_avg/subtest111/subtest222/subtest333/subtest444/subtest555/subtest666/subtest777", testPath)
	assert.Equal(t, "error", stat)
}

func testTraceNameToTestPath_NoMaster_Error(t *testing.T, traceName string) {
	testPath, stat, err := traceNameToTestPath(traceName)
	require.Error(t, err)
	assert.Equal(t, "", testPath)
	assert.Equal(t, "", stat)
}

func testTraceNameToTestPath_NoBot_Error(t *testing.T, traceName string) {
	testPath, stat, err := traceNameToTestPath(traceName)
	require.Error(t, err)
	assert.Equal(t, "", testPath)
	assert.Equal(t, "", stat)
}

func testTraceNameToTestPath_NoTest_Error(t *testing.T, traceName string) {
	testPath, stat, err := traceNameToTestPath(traceName)
	require.Error(t, err)
	assert.Equal(t, "", testPath)
	assert.Equal(t, "", stat)
}

func testTraceNameToTestPath_InvalidTraceName_Error(t *testing.T, traceName string) {
	testPath, stat, err := traceNameToTestPath(traceName)
	require.Error(t, err)
	assert.Equal(t, "", testPath)
	assert.Equal(t, "", stat)
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
	cpClient, err := newChromePerfClient(context.Background(), ts.URL)
	assert.Nil(t, err, "No error expected when creating a new client.")
	anomalyClient := newAnomalyApiClient(cpClient)
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
	cpClient, err := newChromePerfClient(context.Background(), ts.URL)
	assert.Nil(t, err, "No error expected when creating a new client.")
	anomalyClient := newAnomalyApiClient(cpClient)

	resp, err := anomalyClient.GetAnomaliesAroundRevision(ctx, 1234)
	require.NoError(t, err)
	assert.Equal(t, 1, len(resp))
	assert.Equal(t, "mysubtest_withinvalidchar", resp[0].Params["subtest_1"][0])
}
