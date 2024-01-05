package cache

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/chromeperf"
	anomalies_chrome_mock "go.skia.org/infra/perf/go/chromeperf/mock"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/types"

	"github.com/stretchr/testify/assert"
)

const (
	traceName1 = ",benchmark=Blazor,bot=MacM1,master=ChromiumPerf,test=test1,"
	traceName2 = ",benchmark=Blazor,bot=MacM1,master=ChromiumPerf,test=test2,"
	traceName3 = ",benchmark=Blazor,bot=MacM1,master=ChromiumPerf,test=test3,"
	traceName4 = ",benchmark=Blazor,bot=MacM1,master=ChromiumPerf,test=test4,"

	testPath1 = "ChromiumPerf/MacM1/Blazor/test1"
	testPath2 = "ChromiumPerf/MacM1/Blazor/test2"
	testPath3 = "ChromiumPerf/MacM1/Blazor/test3"
	testPath4 = "ChromiumPerf/MacM1/Blazor/test4"

	startCommitPosition = 11
	endCommitPosition   = 21
)

var anomaly1 = chromeperf.Anomaly{
	Id:            111,
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
	Id:            222,
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
var anomaly3 = chromeperf.Anomaly{
	Id:            333,
	TestPath:      testPath3,
	StartRevision: startCommitPosition,
	EndRevision:   endCommitPosition,
	IsImprovement: false,
	Recovered:     false,
	State:         "unknown",
	Statistics:    "avg",
	Unit:          "ms",
	PValue:        3.3,
}
var anomaly4 = chromeperf.Anomaly{
	Id:            444,
	TestPath:      testPath4,
	StartRevision: startCommitPosition,
	EndRevision:   endCommitPosition,
	IsImprovement: false,
	Recovered:     false,
	State:         "unknown",
	Statistics:    "avg",
	Unit:          "ms",
	PValue:        4.4,
}

var errMock = errors.New("this is my mock test error")

var traceNames = []string{traceName1, traceName2}

var testPathes = []string{testPath1, testPath2}

var traceSet = types.TraceSet{
	traceName1: types.Trace([]float32{1.2, 2.1}),
	traceName2: types.Trace([]float32{1.3, 3.1}),
}

var chromePerfAnomalyMap = chromeperf.AnomalyMap{
	traceName1: map[types.CommitNumber]chromeperf.Anomaly{12: anomaly1},
	traceName2: map[types.CommitNumber]chromeperf.Anomaly{15: anomaly2},
}

var header = []*dataframe.ColumnHeader{
	{Offset: startCommitPosition, Timestamp: 1},
	{Offset: endCommitPosition, Timestamp: 1},
}

var dataFrame = &dataframe.DataFrame{
	TraceSet: traceSet,
	Header:   header,
}

var ctx = context.Background()

func TestGetAnomalies_FromChromePerf_Success(t *testing.T) {
	mockChromePerf := anomalies_chrome_mock.NewAnomalyApiClient(t)
	mockChromePerf.On("GetAnomalies", ctx, traceNames, startCommitPosition, endCommitPosition).Return(chromePerfAnomalyMap, nil)
	anomayStore := getAnomalyStore(t, mockChromePerf)

	expectedAnomalyMap := chromeperf.AnomalyMap{
		traceName1: map[types.CommitNumber]chromeperf.Anomaly{12: anomaly1},
		traceName2: map[types.CommitNumber]chromeperf.Anomaly{15: anomaly2},
	}
	am, err := anomayStore.GetAnomalies(ctx, traceNames, startCommitPosition, endCommitPosition)
	require.NoError(t, err)
	assert.Equal(t, expectedAnomalyMap, am)

	assert.True(t, anomayStore.testsCache.Contains(getAnomalyCacheKey(traceName1, startCommitPosition, endCommitPosition)))
	assert.True(t, anomayStore.testsCache.Contains(getAnomalyCacheKey(traceName2, startCommitPosition, endCommitPosition)))
}

func TestGetAnomalies_FromChromePerfAndCache_Success(t *testing.T) {
	mockChromePerf := anomalies_chrome_mock.NewAnomalyApiClient(t)
	mockChromePerf.On("GetAnomalies", ctx, traceNames, startCommitPosition, endCommitPosition).Return(chromePerfAnomalyMap, nil)
	anomayStore := getAnomalyStore(t, mockChromePerf)

	expectedAnomalyMap1 := chromeperf.AnomalyMap{
		traceName1: map[types.CommitNumber]chromeperf.Anomaly{12: anomaly1},
		traceName2: map[types.CommitNumber]chromeperf.Anomaly{15: anomaly2},
	}

	am, err := anomayStore.GetAnomalies(ctx, traceNames, startCommitPosition, endCommitPosition)
	require.NoError(t, err)
	assert.Equal(t, expectedAnomalyMap1, am)

	assert.True(t, anomayStore.testsCache.Contains(getAnomalyCacheKey(traceName1, startCommitPosition, endCommitPosition)))
	assert.True(t, anomayStore.testsCache.Contains(getAnomalyCacheKey(traceName2, startCommitPosition, endCommitPosition)))
	assert.False(t, anomayStore.testsCache.Contains(getAnomalyCacheKey(traceName3, startCommitPosition, endCommitPosition)))
	assert.False(t, anomayStore.testsCache.Contains(getAnomalyCacheKey(traceName4, startCommitPosition, endCommitPosition)))

	expectedAnomalyMap2 := chromeperf.AnomalyMap{
		traceName1: map[types.CommitNumber]chromeperf.Anomaly{12: anomaly1},
		traceName2: map[types.CommitNumber]chromeperf.Anomaly{15: anomaly2},
		traceName3: map[types.CommitNumber]chromeperf.Anomaly{17: anomaly3},
		traceName4: map[types.CommitNumber]chromeperf.Anomaly{20: anomaly4},
	}

	traceNames2 := []string{traceName3, traceName4}
	chromePerfAnomalyMap2 := chromeperf.AnomalyMap{
		traceName3: map[types.CommitNumber]chromeperf.Anomaly{17: anomaly3},
		traceName4: map[types.CommitNumber]chromeperf.Anomaly{20: anomaly4},
	}
	mockChromePerf.On("GetAnomalies", ctx, traceNames2, startCommitPosition, endCommitPosition).Return(chromePerfAnomalyMap2, nil)

	traceNames3 := []string{traceName1, traceName2, traceName3, traceName4}
	am, err = anomayStore.GetAnomalies(ctx, traceNames3, startCommitPosition, endCommitPosition)
	require.NoError(t, err)
	assert.Equal(t, expectedAnomalyMap2, am)

	assert.True(t, anomayStore.testsCache.Contains(getAnomalyCacheKey(traceName1, startCommitPosition, endCommitPosition)))
	assert.True(t, anomayStore.testsCache.Contains(getAnomalyCacheKey(traceName2, startCommitPosition, endCommitPosition)))
	assert.True(t, anomayStore.testsCache.Contains(getAnomalyCacheKey(traceName3, startCommitPosition, endCommitPosition)))
	assert.True(t, anomayStore.testsCache.Contains(getAnomalyCacheKey(traceName4, startCommitPosition, endCommitPosition)))
}

func TestGetAnomalies_GetErrorFromChromePerf_EmptyAnomalyMap(t *testing.T) {
	mockChromePerf := anomalies_chrome_mock.NewAnomalyApiClient(t)
	mockChromePerf.On("GetAnomalies", ctx, traceNames, startCommitPosition, endCommitPosition).Return(nil, errMock)
	anomayStore := getAnomalyStore(t, mockChromePerf)

	expectedAnomalyMap := chromeperf.AnomalyMap{}
	am, err := anomayStore.GetAnomalies(ctx, traceNames, startCommitPosition, endCommitPosition)
	require.NoError(t, err)
	assert.Equal(t, expectedAnomalyMap, am)

	assert.False(t, anomayStore.testsCache.Contains(getAnomalyCacheKey(traceName1, startCommitPosition, endCommitPosition)))
	assert.False(t, anomayStore.testsCache.Contains(getAnomalyCacheKey(traceName2, startCommitPosition, endCommitPosition)))
}

func TestGetAnomalies_GetEmptyResultFromChromePerf_EmptyAnomalyMap(t *testing.T) {
	mockChromePerf := anomalies_chrome_mock.NewAnomalyApiClient(t)
	mockChromePerf.On("GetAnomalies", ctx, traceNames, startCommitPosition, endCommitPosition).Return(nil, nil)
	anomayStore := getAnomalyStore(t, mockChromePerf)

	expectedAnomalyMap := chromeperf.AnomalyMap{}
	am, err := anomayStore.GetAnomalies(ctx, traceNames, startCommitPosition, endCommitPosition)
	require.NoError(t, err)
	assert.Equal(t, expectedAnomalyMap, am)

	assert.False(t, anomayStore.testsCache.Contains(getAnomalyCacheKey(traceName1, startCommitPosition, endCommitPosition)))
	assert.False(t, anomayStore.testsCache.Contains(getAnomalyCacheKey(traceName2, startCommitPosition, endCommitPosition)))
}

func TestGetTracesAroundRevision_FromChromePerf_Success(t *testing.T) {
	mockChromePerf := anomalies_chrome_mock.NewAnomalyApiClient(t)
	revisionNum := 1234
	paramsFromChromePerf := map[string][]string{
		"master":    {"ChromePerf"},
		"bot":       {"linux-perf"},
		"benchmark": {"b1"},
		"test":      {"t1", "t2"},
		"subtest_1": {"s1"},
	}
	anomaliesExpected := []chromeperf.AnomalyForRevision{
		{
			StartRevision: 123,
			EndRevision:   456,
			Params:        paramsFromChromePerf,
			Anomaly:       chromeperf.Anomaly{},
			TestPath:      "some/test/path",
		},
	}

	mockChromePerf.On("GetAnomaliesAroundRevision", ctx, revisionNum).Return(anomaliesExpected, nil)

	anomalyStore := getAnomalyStore(t, mockChromePerf)
	params, err := anomalyStore.GetAnomaliesAroundRevision(ctx, revisionNum)
	assert.Nil(t, err)
	assert.Equal(t, anomaliesExpected, params)
	cachedEntries, ok := anomalyStore.revisionCache.Get(revisionNum)
	assert.True(t, ok)
	assert.Equal(t, anomaliesExpected, cachedEntries.([]chromeperf.AnomalyForRevision))
}

func getAnomalyStore(t *testing.T, mockChromePerf *anomalies_chrome_mock.AnomalyApiClient) *store {
	anomayStore, err := New(mockChromePerf)
	require.NoError(t, err)
	return anomayStore
}
