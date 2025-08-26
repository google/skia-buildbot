package chromeperf

/*

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
*/
