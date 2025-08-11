package anomalies_impl

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/perf/go/chromeperf"
	"go.skia.org/infra/perf/go/dataframe"
	gitMocks "go.skia.org/infra/perf/go/git/mocks"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/regression"
	regStoreMocks "go.skia.org/infra/perf/go/regression/mocks"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

type RegressionMap = map[types.CommitNumber]*regression.AllRegressionsForCommit

const (
	startCommit  = 100
	endCommit    = 200
	medianBefore = 30.0
	medianAfter  = 40.0
	traceKey1    = ",benchmark=Blazor,bot=MacM1,master=ChromiumPerf,test=test1,"
	traceKey2    = ",benchmark=Blazor,bot=MacM1,master=ChromiumPerf,test=test2,"
)

func setupStore(t *testing.T) (*sqlAnomaliesStore, *regStoreMocks.Store, *gitMocks.Git) {
	regStoreMock := regStoreMocks.NewStore(t)
	gitMock := gitMocks.NewGit(t)
	store, err := NewSqlAnomaliesStore(regStoreMock, gitMock)
	require.NoError(t, err, "Failed to create sqlAnomaliesStore")
	return store, regStoreMock, gitMock
}

func newTestRegression(id string, prevCommitNum, commitNum types.CommitNumber, traceKey string, isImprovement bool, medianBefore, medianAfter float32) *regression.Regression {
	return &regression.Regression{
		Id:               id,
		AlertId:          0,
		CommitNumber:     commitNum,
		PrevCommitNumber: prevCommitNum,
		Frame: &frame.FrameResponse{
			DataFrame: &dataframe.DataFrame{
				TraceSet: types.TraceSet{traceKey: {}},
			},
			Msg: "",
		},
		IsImprovement: isImprovement,
		MedianBefore:  medianBefore,
		MedianAfter:   medianAfter,
		// Other fields like ClusterType, Algorithm, etc., can be added if needed.
	}
}

func newTestRegresstionsMap() RegressionMap {
	return RegressionMap{
		types.CommitNumber(endCommit): &regression.AllRegressionsForCommit{
			ByAlertID: map[string]*regression.Regression{
				"alert_id_1": newTestRegression("alert_id_1", startCommit, endCommit, traceKey1, false, medianBefore, medianAfter),
				"alert_id_2": newTestRegression("alert_id_2", startCommit, endCommit, traceKey2, true, medianBefore*2, medianAfter*2),
			},
		},
	}
}

func TestNewSqlAnomaliesStore(t *testing.T) {
	regStoreMock := regStoreMocks.NewStore(t)
	gitMock := gitMocks.NewGit(t)

	store, err := NewSqlAnomaliesStore(regStoreMock, gitMock)

	require.NoError(t, err)
	require.NotNil(t, store)
}

func TestSqlAnomaliesStore_GetAnomalies(t *testing.T) {
	ctx := context.Background()

	t.Run("Success_NoRegressionsFound", func(t *testing.T) {
		store, regStoreMock, _ := setupStore(t)
		regStoreMock.On("Range", mock.Anything, types.CommitNumber(startCommit), types.CommitNumber(endCommit)).
			Return(RegressionMap{}, nil).Once()

		anomaliesMap, err := store.GetAnomalies(ctx, []string{}, startCommit, endCommit)
		require.NoError(t, err)
		assert.Empty(t, anomaliesMap)
		regStoreMock.AssertExpectations(t)
	})

	t.Run("ErrorFromRegressionStore", func(t *testing.T) {
		store, regStoreMock, _ := setupStore(t)
		regStoreMock.On("Range", mock.Anything, types.CommitNumber(startCommit), types.CommitNumber(endCommit)).
			Return(nil, errors.New("failed to fetch from regStore")).Once()

		anomaliesMap, err := store.GetAnomalies(ctx, []string{}, startCommit, endCommit)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch from regStore")
		assert.Empty(t, anomaliesMap)
		regStoreMock.AssertExpectations(t)
	})

	t.Run("Success_WithRegressions_NoTraceFilter", func(t *testing.T) {
		store, regStoreMock, _ := setupStore(t)
		regStoreMock.On("Range", mock.Anything, types.CommitNumber(startCommit), types.CommitNumber(endCommit)).
			Return(newTestRegresstionsMap(), nil).Once()

		anomaliesMap, err := store.GetAnomalies(ctx, []string{}, startCommit, endCommit)
		require.NoError(t, err)
		require.Len(t, anomaliesMap, 2)

		assert.Contains(t, anomaliesMap, traceKey1)
		assert.Contains(t, anomaliesMap[traceKey1], types.CommitNumber(endCommit))
		anomaly1 := anomaliesMap[traceKey1][types.CommitNumber(endCommit)]
		assert.Equal(t, "alert_id_1", anomaly1.Id)
		expectedTestPath1, err := chromeperf.TraceNameToTestPath(traceKey1, false)
		require.NoError(t, err)
		assert.Equal(t, expectedTestPath1, anomaly1.TestPath)
		assert.Equal(t, startCommit, anomaly1.StartRevision)
		assert.Equal(t, endCommit, anomaly1.EndRevision)
		assert.False(t, anomaly1.IsImprovement)
		assert.Equal(t, medianBefore, anomaly1.MedianBeforeAnomaly)
		assert.Equal(t, medianAfter, anomaly1.MedianAfterAnomaly)

		assert.Contains(t, anomaliesMap, traceKey2)
		assert.Contains(t, anomaliesMap[traceKey2], types.CommitNumber(endCommit))
		anomaly2 := anomaliesMap[traceKey2][types.CommitNumber(endCommit)]
		assert.Equal(t, "alert_id_2", anomaly2.Id)
		expectedTestPath2, err := chromeperf.TraceNameToTestPath(traceKey2, false)
		require.NoError(t, err)
		assert.Equal(t, expectedTestPath2, anomaly2.TestPath)
		assert.True(t, anomaly2.IsImprovement)
		assert.Equal(t, medianBefore*2, anomaly2.MedianBeforeAnomaly)
		assert.Equal(t, medianAfter*2, anomaly2.MedianAfterAnomaly)

		regStoreMock.AssertExpectations(t)
	})

	t.Run("Success_WithRegressions_WithTraceFilter_Matching", func(t *testing.T) {
		store, regStoreMock, _ := setupStore(t)
		regStoreMock.On("Range", mock.Anything, types.CommitNumber(startCommit), types.CommitNumber(endCommit)).
			Return(newTestRegresstionsMap(), nil).Once()

		anomaliesMap, err := store.GetAnomalies(ctx, []string{traceKey1}, startCommit, endCommit)
		require.NoError(t, err)
		require.Len(t, anomaliesMap, 1)
		assert.Contains(t, anomaliesMap, traceKey1)
		assert.NotContains(t, anomaliesMap, traceKey2)
		regStoreMock.AssertExpectations(t)
	})

	t.Run("Success_WithRegressions_WithTraceFilter_NoMatching", func(t *testing.T) {
		store, regStoreMock, _ := setupStore(t)
		regStoreMock.On("Range", mock.Anything, types.CommitNumber(startCommit), types.CommitNumber(endCommit)).
			Return(newTestRegresstionsMap(), nil).Once()

		anomaliesMap, err := store.GetAnomalies(ctx, []string{"non_existent_trace"}, startCommit, endCommit)
		require.NoError(t, err)
		assert.Empty(t, anomaliesMap)
		regStoreMock.AssertExpectations(t)
	})
}

func TestSqlAnomaliesStore_GetAnomaliesInTimeRange(t *testing.T) {
	ctx := context.Background()
	startTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC)
	traceNames := []string{traceKey1}

	t.Run("GitNotInitialized", func(t *testing.T) {
		regStoreMock := regStoreMocks.NewStore(t)
		store, err := NewSqlAnomaliesStore(regStoreMock, nil /*perfGit*/)
		require.NoError(t, err)

		anomaliesMap, err := store.GetAnomaliesInTimeRange(ctx, traceNames, startTime, endTime)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Git provider is not initialized")
		assert.Empty(t, anomaliesMap)
	})

	t.Run("NoCommitsFoundInTimeRange", func(t *testing.T) {
		store, _, gitMock := setupStore(t)
		gitMock.On("CommitSliceFromTimeRange", mock.Anything, startTime, endTime).
			Return([]provider.Commit{}, nil).Once()

		anomaliesMap, err := store.GetAnomaliesInTimeRange(ctx, traceNames, startTime, endTime)
		require.NoError(t, err)
		assert.Empty(t, anomaliesMap)
		gitMock.AssertExpectations(t)
	})

	t.Run("Success", func(t *testing.T) {
		store, regStoreMock, gitMock := setupStore(t)
		commits := []provider.Commit{
			{CommitNumber: startCommit, Timestamp: startTime.Add(time.Hour).Unix()},
			{CommitNumber: 15, Timestamp: startTime.Add(2 * time.Hour).Unix()},
			{CommitNumber: endCommit, Timestamp: endTime.Add(-time.Hour).Unix()},
		}
		gitMock.On("CommitSliceFromTimeRange", mock.Anything, startTime, endTime).
			Return(commits, nil).Once()
		regStoreMock.On("Range", mock.Anything, types.CommitNumber(startCommit), types.CommitNumber(endCommit)).
			Return(RegressionMap{}, nil).Once()

		_, err := store.GetAnomaliesInTimeRange(ctx, traceNames, startTime, endTime)

		require.NoError(t, err)
		gitMock.AssertExpectations(t)
		regStoreMock.AssertExpectations(t)
	})
}

func TestSqlAnomaliesStore_GetAnomaliesAroundRevision(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		store, regStoreMock, _ := setupStore(t)
		// Allow any window size > 0.
		regStoreMock.On("Range",
			mock.Anything,
			mock.MatchedBy(func(cn types.CommitNumber) bool {
				return cn < types.CommitNumber(endCommit)
			}),
			mock.MatchedBy(func(cn types.CommitNumber) bool {
				return cn > types.CommitNumber(endCommit)
			})).
			Return(RegressionMap{}, nil).Once()

		_, err := store.GetAnomaliesAroundRevision(ctx, endCommit)
		require.NoError(t, err)
		regStoreMock.AssertExpectations(t)
	})
}
