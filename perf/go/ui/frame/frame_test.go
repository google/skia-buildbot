package frame

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/anomalies/cache"
	"go.skia.org/infra/perf/go/chromeperf"
	chromeperfMock "go.skia.org/infra/perf/go/chromeperf/mock"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/dataframe/mocks"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/git/gittest"
	"go.skia.org/infra/perf/go/pivot"
	"go.skia.org/infra/perf/go/progress"
	"go.skia.org/infra/perf/go/shortcut"
	shortcutStoreMock "go.skia.org/infra/perf/go/shortcut/mocks"
	traceStoreMocks "go.skia.org/infra/perf/go/tracestore/mocks"
	"go.skia.org/infra/perf/go/types"
)

const (
	testShortcutKey = "some-key-value"
)

var (
	testTimeBegin = time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC)
	testTimeEnd   = time.Date(2020, 1, 1, 2, 0, 0, 0, time.UTC)
	errTestError  = errors.New("my test error")
)

const (
	traceName1 = ",benchmark=Blazor,bot=MacM1,master=ChromiumPerf,test=test1,"
	traceName2 = ",benchmark=Blazor,bot=MacM1,master=ChromiumPerf,test=test2,"

	testPath1 = "ChromiumPerf/MacM1/Blazor/test1"
	testPath2 = "ChromiumPerf/MacM1/Blazor/test2"

	startCommitPosition = 11
	endCommitPosition   = 21
)

var anomaly1 = chromeperf.Anomaly{
	Id:            "111",
	TestPath:      testPath1,
	StartRevision: startCommitPosition,
	EndRevision:   endCommitPosition,
	IsImprovement: false,
	Recovered:     false,
	State:         "unknown",
	Statistics:    "avg",
	Unit:          "ms",
	PValue:        1.1,
}
var anomaly2 = chromeperf.Anomaly{
	Id:            "222",
	TestPath:      testPath2,
	StartRevision: startCommitPosition,
	EndRevision:   endCommitPosition,
	IsImprovement: false,
	Recovered:     false,
	State:         "unknown",
	Statistics:    "avg",
	Unit:          "ms",
	PValue:        2.2,
}

var chromePerfAnomalyMap = chromeperf.AnomalyMap{
	traceName1: map[types.CommitNumber]chromeperf.Anomaly{12: anomaly1},
	traceName2: map[types.CommitNumber]chromeperf.Anomaly{15: anomaly2},
}

var traceSet = types.TraceSet{
	traceName1: types.Trace([]float32{1.2, 2.1}),
	traceName2: types.Trace([]float32{1.3, 3.1}),
}

var traceNames = []string{traceName1, traceName2}

var testPathes = []string{testPath1, testPath2}

var errMock = errors.New("this is my mock test error")

var ctx = context.Background()

func TestGetSkps_Success(t *testing.T) {
	ctx, db, _, _, _, instanceConfig := gittest.NewForTest(t)
	g, err := perfgit.New(ctx, false, db, instanceConfig)
	require.NoError(t, err)

	instanceConfig.GitRepoConfig.FileChangeMarker = "bar.txt"
	config.Config = instanceConfig

	skps, err := getSkps(ctx, []*dataframe.ColumnHeader{
		{
			Offset: 0,
		},
		{
			Offset: 7,
		},
	}, g)
	require.NoError(t, err)
	assert.Equal(t, []int{3, 6}, skps)
}

func TestGetSkps_SuccessIfFileChangeMarkerNotSet(t *testing.T) {
	ctx, db, _, _, _, instanceConfig := gittest.NewForTest(t)
	g, err := perfgit.New(ctx, false, db, instanceConfig)
	require.NoError(t, err)

	instanceConfig.GitRepoConfig.FileChangeMarker = ""
	config.Config = instanceConfig

	skps, err := getSkps(ctx, []*dataframe.ColumnHeader{
		{
			Offset: 0,
		},
		{
			Offset: 7,
		},
	}, g)
	require.NoError(t, err)
	assert.Empty(t, skps)
}

func TestGetSkps_ErrOnBadCommitNumber(t *testing.T) {
	ctx, db, _, _, _, instanceConfig := gittest.NewForTest(t)
	g, err := perfgit.New(ctx, false, db, instanceConfig)
	require.NoError(t, err)

	instanceConfig.GitRepoConfig.FileChangeMarker = "bar.txt"
	config.Config = instanceConfig

	_, err = getSkps(ctx, []*dataframe.ColumnHeader{
		{
			Offset: -3,
		},
		{
			Offset: -1,
		},
	}, g)
	require.Error(t, err)
}

func TestGetMetadataForTraces_Success(t *testing.T) {
	// The dataframe contains 3 commits 1,2,3 and 2 traces
	//
	//  [",arch=x86,config=8888,"] = {1, 2, 3}
	//	[",arch=x86,config=565,"]  = {2, 4, 6}
	_, df, _ := frameRequestForTest(t)
	mockMetadataStore := traceStoreMocks.NewMetadataStore(t)

	config.Config = &config.InstanceConfig{
		DataPointConfig: config.DataPointConfig{
			KeysForUsefulLinks: []string{"link1", "link2", "link3", "link4", "link5", "link6"},
		},
	}

	traceids := []string{",arch=x86,config=8888,", ",arch=x86,config=565,"}
	sourceFileIds := []int64{
		12345,
		23456,
		34567,
		45678,
		56789,
		67890,
	}
	sInfo1 := types.NewTraceSourceInfo()
	sInfo1.Add(1, sourceFileIds[0])
	sInfo2 := types.NewTraceSourceInfo()
	sInfo2.Add(2, sourceFileIds[1])
	df.SourceInfo = map[string]*types.TraceSourceInfo{
		traceids[0]: sInfo1,
		traceids[1]: sInfo1,
	}

	sourceFileLinks := map[int64]map[string]string{
		sourceFileIds[0]: {
			"link1": "val1",
		},
		sourceFileIds[1]: {
			"link2": "val2",
		},
		sourceFileIds[2]: {
			"link3": "val3",
		},
		sourceFileIds[3]: {
			"link4": "val4",
		},
		sourceFileIds[4]: {
			"link5": "val5",
		},
		sourceFileIds[5]: {
			"link6": "val6",
		},
	}
	mockMetadataStore.On("GetMetadataForSourceFileIDs", ctx, mock.Anything).Return(sourceFileLinks, nil)
	ctx := context.Background()
	traceMetadata, err := getMetadataForTraces(ctx, df, mockMetadataStore)
	assert.NoError(t, err)
	assert.NotNil(t, traceMetadata)
	assert.Equal(t, len(traceids), len(traceMetadata))
}

func TestProcessFrameRequest_InvalidQuery_ReturnsError(t *testing.T) {
	config.Config = &config.InstanceConfig{}
	fr := &FrameRequest{
		Queries:  []string{"http://[::1]a"}, // A known query that will fail to parse.
		Progress: progress.New(),
	}
	err := ProcessFrameRequest(context.Background(), fr, nil, nil, nil, nil, nil, nil, false)
	require.Error(t, err)
	var b bytes.Buffer
	err = fr.Progress.JSON(&b)
	require.NoError(t, err)
	assert.Equal(t, `{"status":"Running","messages":[{"key":"Loading","value":"Queries"}],"url":""}`, strings.TrimSpace(b.String()))
}

// frameRequestForTest returns a mock DataFrameBuilder, a frameRequestProcess,
// and a populated DateFrame for testing.
//
// The DataFrame returned has the following Traces:
//
//	[",arch=x86,config=8888,"] = {1, 2, 3}
//	[",arch=x86,config=565,"]  = {2, 4, 6}
func frameRequestForTest(t *testing.T) (*mocks.DataFrameBuilder, *dataframe.DataFrame, *frameRequestProcess) {
	t.Helper()
	if config.Config == nil {
		config.Config = &config.InstanceConfig{}
	}
	dfbMock := &mocks.DataFrameBuilder{}
	ssMock := &shortcutStoreMock.Store{}

	fr := &frameRequestProcess{
		request: &FrameRequest{
			Queries:     []string{"arch=x86"},
			RequestType: REQUEST_COMPACT,
			Progress:    progress.New(),
			NumCommits:  10,
		},
		dfBuilder:     dfbMock,
		shortcutStore: ssMock,
	}

	df := dataframe.NewEmpty()
	df.TraceSet[",arch=x86,config=8888,"] = types.Trace{1, 2, 3}
	df.TraceSet[",arch=x86,config=565,"] = types.Trace{2, 4, 6}
	const numHeaders = 3
	df.Header = make([]*dataframe.ColumnHeader, numHeaders)
	for i := 0; i < numHeaders; i++ {
		df.Header[i] = &dataframe.ColumnHeader{
			Offset:    types.CommitNumber(i + 1),
			Timestamp: dataframe.TimestampSeconds(testTimeBegin.Unix() + int64(i)),
		}
	}
	df.BuildParamSet()

	t.Cleanup(func() {
		dfbMock.AssertExpectations(t)
	})

	return dfbMock, df, fr
}

func TestDoSearch_InvalidQuery_ReturnsError(t *testing.T) {

	_, _, fr := frameRequestForTest(t)
	_, err := fr.doSearch(context.Background(), "http://[::1]a", testTimeBegin, testTimeEnd)

	require.Error(t, err)
}

func TestDoSearch_ValidQueryCompact_ReturnsDataFrameWithQueryResults(t *testing.T) {

	dfbMock, df, fr := frameRequestForTest(t)
	dfbMock.On("NewNFromQuery", testutils.AnyContext, testTimeEnd, mock.Anything, fr.request.NumCommits, fr.request.Progress).Return(df, nil)

	actualDf, err := fr.doSearch(context.Background(), "config=8888", testTimeBegin, testTimeEnd)
	require.NoError(t, err)
	require.Equal(t, df, actualDf)
}

func TestDoSearch_ValidQueryTimeRange_ReturnsDataFrameWithQueryResults(t *testing.T) {

	dfbMock, df, fr := frameRequestForTest(t)
	fr.request.RequestType = REQUEST_TIME_RANGE

	dfbMock.On("NewFromQueryAndRange", testutils.AnyContext, testTimeBegin, testTimeEnd, mock.Anything, fr.request.Progress).Return(df, nil)

	actualDf, err := fr.doSearch(context.Background(), "config=8888", testTimeBegin, testTimeEnd)
	require.NoError(t, err)
	require.Equal(t, df, actualDf)
}

func TestDoKeys_ShortcutStoreReturnsError_ReturnsError(t *testing.T) {

	_, _, fr := frameRequestForTest(t)
	ssMock := fr.shortcutStore.(*shortcutStoreMock.Store)
	testShortcutKey := "some-key-value"
	ssMock.On("Get", testutils.AnyContext, testShortcutKey).Return(nil, errTestError)
	_, err := fr.doKeys(context.Background(), testShortcutKey, testTimeBegin, testTimeEnd)

	require.Error(t, err)
}

func TestDoKeys_ValidKeyID_ReturnsDataFrameWithTracesFromShortcut(t *testing.T) {

	dfbMock, df, fr := frameRequestForTest(t)
	ssMock := fr.shortcutStore.(*shortcutStoreMock.Store)

	// Create valid shortCut.Shortcut for "Get" to return.
	shortCutKeys := []string{}
	copy(shortCutKeys, df.ParamSet.Keys())
	sort.Strings(shortCutKeys)
	shortCut := &shortcut.Shortcut{
		Keys: shortCutKeys,
	}

	ssMock.On("Get", testutils.AnyContext, testShortcutKey).Return(shortCut, nil)
	dfbMock.On("NewNFromKeys", testutils.AnyContext, testTimeEnd, shortCut.Keys, fr.request.NumCommits, fr.request.Progress).Return(df, nil)
	actualDf, err := fr.doKeys(context.Background(), testShortcutKey, testTimeBegin, testTimeEnd)

	require.NoError(t, err)
	require.Equal(t, df, actualDf)
}

func TestDoKeys_ValidKeyIDTimeRange_ReturnsDataFrameWithTracesFromShortcut(t *testing.T) {

	dfbMock, df, fr := frameRequestForTest(t)
	ssMock := fr.shortcutStore.(*shortcutStoreMock.Store)

	fr.request.RequestType = REQUEST_TIME_RANGE

	// Create valid shortCut.Shortcut for "Get" to return.
	shortCutKeys := []string{}
	copy(shortCutKeys, df.ParamSet.Keys())
	sort.Strings(shortCutKeys)
	shortCut := &shortcut.Shortcut{
		Keys: shortCutKeys,
	}

	ssMock.On("Get", testutils.AnyContext, testShortcutKey).Return(shortCut, nil)
	dfbMock.On("NewFromKeysAndRange", testutils.AnyContext, shortCut.Keys, testTimeBegin, testTimeEnd, fr.request.Progress).Return(df, nil)
	actualDf, err := fr.doKeys(context.Background(), testShortcutKey, testTimeBegin, testTimeEnd)

	require.NoError(t, err)
	require.Equal(t, df, actualDf)
}

func TestDoCalc_InvalidFormulaCompact_ReturnsError(t *testing.T) {

	_, _, fr := frameRequestForTest(t)
	_, err := fr.doCalc(context.Background(), `sum(filter(`, testTimeBegin, testTimeEnd)
	require.Error(t, err)
}

func TestDoCalc_ValidFormulaInvalidQueryCompact_ReturnsError(t *testing.T) {

	_, _, fr := frameRequestForTest(t)
	_, err := fr.doCalc(context.Background(), `sum(filter("this is not a valid query"))`, testTimeBegin, testTimeEnd)
	require.Error(t, err)
}

func TestDoCalc_ValidQueryCompact_ReturnsDataFrameWithCalculatedTraces(t *testing.T) {

	dfbMock, df, fr := frameRequestForTest(t)
	dfbMock.On("NewNFromQuery", testutils.AnyContext, testTimeEnd, mock.Anything, fr.request.NumCommits, fr.request.Progress).Return(df, nil)

	actualDf, err := fr.doCalc(context.Background(), `sum(filter("arch=x86"))`, testTimeBegin, testTimeEnd)
	require.NoError(t, err)
	assert.Equal(t, actualDf.TraceSet[`sum(filter("arch=x86"))`], types.Trace{3, 6, 9})
}

func TestDoCalc_ValidQueryTimeRange_ReturnsDataFrameWithCalculatedTraces(t *testing.T) {

	dfbMock, df, fr := frameRequestForTest(t)

	fr.request.RequestType = REQUEST_TIME_RANGE

	dfbMock.On("NewFromQueryAndRange", testutils.AnyContext, testTimeBegin, testTimeEnd, mock.Anything, fr.request.Progress).Return(df, nil)

	actualDf, err := fr.doCalc(context.Background(), `sum(filter("arch=x86"))`, testTimeBegin, testTimeEnd)
	require.NoError(t, err)
	assert.Equal(t, actualDf.TraceSet[`sum(filter("arch=x86"))`], types.Trace{3, 6, 9})
}

func TestDoCalc_ValidFormulaInvalidShortcutCompact_ReturnsError(t *testing.T) {

	_, _, fr := frameRequestForTest(t)
	ssMock := fr.shortcutStore.(*shortcutStoreMock.Store)
	ssMock.On("Get", testutils.AnyContext, testShortcutKey).Return(nil, errTestError)

	_, err := fr.doCalc(context.Background(), fmt.Sprintf(`shortcut("%s")`, testShortcutKey), testTimeBegin, testTimeEnd)
	require.Error(t, err)
}

func TestDoCalc_ValidFormulaValidShortcutCompact_ReturnsDataFrameWithCalculatedTracesFromShortcut(t *testing.T) {

	dfbMock, df, fr := frameRequestForTest(t)
	ssMock := fr.shortcutStore.(*shortcutStoreMock.Store)

	// Create valid shortCut.Shortcut for "Get" to return.
	shortCutKeys := []string{}
	copy(shortCutKeys, df.ParamSet.Keys())
	sort.Strings(shortCutKeys)
	shortCut := &shortcut.Shortcut{
		Keys: shortCutKeys,
	}

	ssMock.On("Get", testutils.AnyContext, testShortcutKey).Return(shortCut, nil)
	dfbMock.On("NewNFromKeys", testutils.AnyContext, testTimeEnd, shortCut.Keys, fr.request.NumCommits, fr.request.Progress).Return(df, nil)

	formula := fmt.Sprintf(`sum(shortcut("%s"))`, testShortcutKey)
	actualDf, err := fr.doCalc(context.Background(), formula, testTimeBegin, testTimeEnd)
	require.NoError(t, err)
	require.Equal(t, actualDf.TraceSet[formula], types.Trace{3, 6, 9})
}

func TestDoCalc_ValidFormulaValidShortcutTimeRange_ReturnsDataFrameWithCalculatedTracesFromShortcut(t *testing.T) {

	dfbMock, df, fr := frameRequestForTest(t)

	fr.request.RequestType = REQUEST_TIME_RANGE

	ssMock := fr.shortcutStore.(*shortcutStoreMock.Store)

	// Create valid shortCut.Shortcut for "Get" to return.
	shortCutKeys := []string{}
	copy(shortCutKeys, df.ParamSet.Keys())
	sort.Strings(shortCutKeys)
	shortCut := &shortcut.Shortcut{
		Keys: shortCutKeys,
	}

	ssMock.On("Get", testutils.AnyContext, testShortcutKey).Return(shortCut, nil)
	dfbMock.On("NewFromKeysAndRange", testutils.AnyContext, shortCut.Keys, testTimeBegin, testTimeEnd, fr.request.Progress).Return(df, nil)

	formula := fmt.Sprintf(`sum(shortcut("%s"))`, testShortcutKey)
	actualDf, err := fr.doCalc(context.Background(), formula, testTimeBegin, testTimeEnd)
	require.NoError(t, err)
	require.Equal(t, actualDf.TraceSet[formula], types.Trace{3, 6, 9})
}

func TestRun_QueryAndThenPivot_ReturnsPivotedDataFrame(t *testing.T) {

	dfbMock, df, fr := frameRequestForTest(t)
	fr.request.Pivot = &pivot.Request{
		GroupBy:   []string{"config"},
		Operation: pivot.Sum,
	}
	fr.request.Begin = int(testTimeBegin.Unix())
	fr.request.End = int(testTimeEnd.Unix())

	dfbMock.On("NewNFromQuery", testutils.AnyContext, testTimeEnd, mock.Anything, fr.request.NumCommits, fr.request.Progress).Return(df, nil)

	actualDf, err := fr.run(context.Background())
	require.NoError(t, err)
	// You can tell this succeeded since the keys are changed to just include the pivot GroupBy keys.
	require.Equal(t, actualDf.TraceSet[",config=565,"], types.Trace{2, 4, 6})
	require.Equal(t, actualDf.TraceSet[",config=8888,"], types.Trace{1, 2, 3})
}

func TestRun_ValidQueryAndThenInvalidPivot_ReturnsError(t *testing.T) {

	dfbMock, df, fr := frameRequestForTest(t)
	fr.request.Pivot = &pivot.Request{
		GroupBy:   []string{"config"},
		Operation: pivot.Operation("this-is-not-a-valid-operation"),
	}
	fr.request.Begin = int(testTimeBegin.Unix())
	fr.request.End = int(testTimeEnd.Unix())

	dfbMock.On("NewNFromQuery", testutils.AnyContext, testTimeEnd, mock.Anything, fr.request.NumCommits, fr.request.Progress).Return(df, nil)

	_, err := fr.run(context.Background())
	require.Error(t, err)
}

func TestRun_KeysAndThenPivot_ReturnsPivotedDataFrame(t *testing.T) {

	dfbMock, df, fr := frameRequestForTest(t)
	fr.request.Pivot = &pivot.Request{
		GroupBy:   []string{"config"},
		Operation: pivot.Sum,
	}
	fr.request.Begin = int(testTimeBegin.Unix())
	fr.request.End = int(testTimeEnd.Unix())
	fr.request.Queries = []string{}
	fr.request.Keys = testShortcutKey

	// Create valid shortCut.Shortcut for "Get" to return.
	shortCutKeys := []string{}
	copy(shortCutKeys, df.ParamSet.Keys())
	sort.Strings(shortCutKeys)
	shortCut := &shortcut.Shortcut{
		Keys: shortCutKeys,
	}

	ssMock := fr.shortcutStore.(*shortcutStoreMock.Store)
	ssMock.On("Get", testutils.AnyContext, testShortcutKey).Return(shortCut, nil)
	dfbMock.On("NewNFromKeys", testutils.AnyContext, testTimeEnd, shortCut.Keys, fr.request.NumCommits, fr.request.Progress).Return(df, nil)

	actualDf, err := fr.run(context.Background())
	require.NoError(t, err)
	// You can tell this succeeded since the keys are changed to just include the pivot GroupBy keys.
	require.Equal(t, actualDf.TraceSet[",config=565,"], types.Trace{2, 4, 6})
	require.Equal(t, actualDf.TraceSet[",config=8888,"], types.Trace{1, 2, 3})
}

func TestResponseFromDataFrame_NullPivot_ReturnsDisplayModePlot(t *testing.T) {
	_, df, _ := frameRequestForTest(t)
	resp, err := ResponseFromDataFrame(context.Background(), nil, df, nil, false, progress.New())
	require.NoError(t, err)
	require.Equal(t, DisplayPlot, resp.DisplayMode)
}

func TestResponseFromDataFrame_ValidPivotRequestForPlot_ReturnsDisplayModePivotPlot(t *testing.T) {
	_, df, _ := frameRequestForTest(t)
	pivotRequest := &pivot.Request{
		GroupBy:   []string{"config"},
		Operation: pivot.Sum,
	}
	resp, err := ResponseFromDataFrame(context.Background(), pivotRequest, df, nil, false, progress.New())
	require.NoError(t, err)
	require.Equal(t, DisplayPivotPlot, resp.DisplayMode)
}

func TestResponseFromDataFrame_ValidPivotRequestForPivotTable_ReturnsDisplayModePivotTable(t *testing.T) {
	_, df, _ := frameRequestForTest(t)
	pivotRequest := &pivot.Request{
		GroupBy:   []string{"config"},
		Operation: pivot.Sum,
		Summary:   []pivot.Operation{pivot.Avg},
	}
	resp, err := ResponseFromDataFrame(context.Background(), pivotRequest, df, nil, false, progress.New())
	require.NoError(t, err)
	require.Equal(t, DisplayPivotTable, resp.DisplayMode)
}

func TestAddAnomaliesToResponse_GotAnomalies_Success(t *testing.T) {
	resp := buildResponse(t)

	mockChromePerf := chromeperfMock.NewAnomalyApiClient(t)
	mockChromePerf.On("GetAnomalies", testutils.AnyContext, traceNames, startCommitPosition, endCommitPosition).Return(chromePerfAnomalyMap, nil)

	anomayStore, err := cache.New(mockChromePerf)
	require.NoError(t, err)

	addRevisionBasedAnomaliesToResponse(ctx, resp, anomayStore, nil)

	expectedAnomalyMap := chromeperf.AnomalyMap{
		traceName1: map[types.CommitNumber]chromeperf.Anomaly{12: anomaly1},
		traceName2: map[types.CommitNumber]chromeperf.Anomaly{15: anomaly2},
	}
	assert.Equal(t, expectedAnomalyMap, resp.AnomalyMap)
}

func TestAddAnomaliesToResponse_ErrorGetAnomalies_GotEmptyAnomalyMap(t *testing.T) {
	resp := buildResponse(t)

	mockChromePerf := chromeperfMock.NewAnomalyApiClient(t)
	mockChromePerf.On("GetAnomalies", testutils.AnyContext, traceNames, startCommitPosition, endCommitPosition).Return(nil, errMock)

	anomayStore, err := cache.New(mockChromePerf)
	require.NoError(t, err)

	addRevisionBasedAnomaliesToResponse(ctx, resp, anomayStore, nil)
	assert.Equal(t, chromeperf.AnomalyMap{}, resp.AnomalyMap)
}

func buildResponse(t *testing.T) *FrameResponse {
	_, df, _ := frameRequestForTest(t)
	df.TraceSet = traceSet
	df.Header[0].Offset = types.CommitNumber(startCommitPosition)
	df.Header[len(df.Header)-1].Offset = types.CommitNumber(endCommitPosition)
	pivotRequest := &pivot.Request{
		GroupBy:   []string{"config"},
		Operation: pivot.Sum,
	}
	resp, err := ResponseFromDataFrame(context.Background(), pivotRequest, df, nil, false, progress.New())
	require.NoError(t, err)
	return resp
}
