package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/anomalygroup/mocks"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/regression"
	reg_mocks "go.skia.org/infra/perf/go/regression/mocks"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

func setupAnomaliesApiWithMocks(t *testing.T) (anomaliesApi, *mocks.Store, *reg_mocks.Store) {
	anomalygroupStore := mocks.NewStore(t)
	regStore := reg_mocks.NewStore(t)

	api := anomaliesApi{
		anomalygroupStore: anomalygroupStore,
		regStore:          regStore,
	}
	return api, anomalygroupStore, regStore
}

func TestGetGroupReportByBugId(t *testing.T) {
	api, anomalygroupStore, regStore := setupAnomaliesApiWithMocks(t)

	ctx := context.Background()
	bugId := "12345"
	anomalyIds := []string{"anomaly-id-1", "anomaly-id-2", "anomaly-improvement"}
	traceset := ",arch=x86,bot=linux,benchmark=jetstream2,test=score,config=default,master=main,"

	// Mock the response from the anomalygroupStore.
	anomalygroupStore.On("GetAnomalyIdsByIssueId", ctx, bugId).Return(anomalyIds, nil)

	// Mock the response from the regStore.
	regressions := []*regression.Regression{
		{
			Id: "anomaly-id-1",
			Frame: &frame.FrameResponse{
				DataFrame: &dataframe.DataFrame{
					TraceSet: types.TraceSet{
						traceset: []float32{1.0},
					},
				},
			},
		},
		{
			Id: "anomaly-id-2",
			Frame: &frame.FrameResponse{
				DataFrame: &dataframe.DataFrame{
					TraceSet: types.TraceSet{
						traceset: []float32{2.0},
					},
				},
			},
		},
		{
			Id: "anomaly-improvement",
			Frame: &frame.FrameResponse{
				DataFrame: &dataframe.DataFrame{
					TraceSet: types.TraceSet{
						traceset: []float32{2.0},
					},
				},
			},
			IsImprovement: true,
		},
	}
	regStore.On("GetByIDs", ctx, anomalyIds).Return(regressions, nil)

	// Create the request.
	req := GetGroupReportRequest{
		BugID: bugId,
	}

	// Call the function under test.
	resp, err := api.getGroupReportByBugId(ctx, req)

	// Assert the results.
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Len(t, resp.Anomalies, 3)
	// Note: compat.ConvertRegressionToAnomalies is not mocked, so we can't check all fields.
	// We are just checking the Id.
	for _, id := range anomalyIds {
		idPresent := false
		for _, anomaly := range resp.Anomalies {
			if anomaly.Id == id {
				idPresent = true
				break
			}
		}
		require.True(t, idPresent)
	}

	// Ensure the mocks were called as expected.
	anomalygroupStore.AssertExpectations(t)
	regStore.AssertExpectations(t)
}

func TestGetGroupReportByAnomalyGroupId(t *testing.T) {
	api, anomalygroupStore, regStore := setupAnomaliesApiWithMocks(t)

	ctx := context.Background()
	anomalyGroupId := "group-id-1"
	anomalyIds := []string{"anom-id-1", "anom-id-2"}
	traceset := ",arch=x86,bot=linux,benchmark=jetstream2,test=score,config=default,master=main,"

	// Mock the response from the anomalygroupStore.
	anomalygroupStore.On("GetAnomalyIdsByAnomalyGroupId", ctx, anomalyGroupId).Return(anomalyIds, nil).Once()

	// Mock the response from the regStore.
	regressions := []*regression.Regression{
		{
			Id: "anom-id-1",
			Frame: &frame.FrameResponse{
				DataFrame: &dataframe.DataFrame{
					TraceSet: types.TraceSet{
						traceset: []float32{1.0},
					},
				},
			},
		},
		{
			Id: "anom-id-2",
			Frame: &frame.FrameResponse{
				DataFrame: &dataframe.DataFrame{
					TraceSet: types.TraceSet{
						traceset: []float32{2.0},
					},
				},
			},
		},
	}
	regStore.On("GetByIDs", ctx, anomalyIds).Return(regressions, nil).Once()

	// Create the request.
	req := GetGroupReportRequest{
		AnomalyGroupID: anomalyGroupId,
	}

	// Call the function under test.
	resp, err := api.getGroupReportByAnomalyGroupId(ctx, req)

	// Assert the results.
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Len(t, resp.Anomalies, 2)
	// Note: compat.ConvertRegressionToAnomalies is not mocked, so we can't check all fields.
	// We are just checking the Id.
	for _, id := range anomalyIds {
		idPresent := false
		for _, anomaly := range resp.Anomalies {
			if anomaly.Id == id {
				idPresent = true
				break
			}
		}
		require.True(t, idPresent)
	}
	// Ensure the mocks were called as expected.
	anomalygroupStore.AssertExpectations(t)
	regStore.AssertExpectations(t)
}

func TestGetGroupReportByAnomalyGroupId_Empty(t *testing.T) {
	api, anomalygroupStore, regStore := setupAnomaliesApiWithMocks(t)

	ctx := context.Background()
	anomalyGroupId := "group-id-1"
	anomalyIds := []string{}

	// Mock the response from the anomalygroupStore.
	anomalygroupStore.On("GetAnomalyIdsByAnomalyGroupId", ctx, anomalyGroupId).Return(anomalyIds, nil).Once()

	// Mock the response from the regStore.
	regressions := []*regression.Regression{}
	regStore.On("GetByIDs", ctx, anomalyIds).Return(regressions, nil).Once()

	// Create the request.
	req := GetGroupReportRequest{
		AnomalyGroupID: anomalyGroupId,
	}

	// Call the function under test.
	resp, err := api.getGroupReportByAnomalyGroupId(ctx, req)

	// Assert the results.
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Ensure the mocks were called as expected.
	anomalygroupStore.AssertExpectations(t)
	regStore.AssertExpectations(t)
}

func TestAnomaliesApi_CleanTestName_Default(t *testing.T) {
	configFileBytes := testutils.ReadFileBytes(t, "config.json")
	err := json.Unmarshal(configFileBytes, &config.Config)
	config.Config.InvalidParamCharRegex = ""
	require.NoError(t, err)

	// ':': allowed in config, not in default
	// '-': allowed in both.
	// '?': now allowed in both.
	testName := "master/bot/measurement/test/sub:test?1-name"
	cleanedName, err := cleanTestName(testName)

	require.Equal(t, "master/bot/measurement/test/sub_test_1-name", cleanedName)
}

func TestAnomaliesApi_CleanTestName_FromConfig(t *testing.T) {
	configFileBytes := testutils.ReadFileBytes(t, "config.json")
	err := json.Unmarshal(configFileBytes, &config.Config)
	require.NoError(t, err)

	testName := "master/bot/measurement/test/sub:test?1-name"
	cleanedName, err := cleanTestName(testName)

	require.Equal(t, "master/bot/measurement/test/sub:test_1-name", cleanedName)
}
