package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/testutils"
	anomalygroup_mocks "go.skia.org/infra/perf/go/anomalygroup/mocks"
	"go.skia.org/infra/perf/go/chromeperf"
	"go.skia.org/infra/perf/go/config"
	culprit_mocks "go.skia.org/infra/perf/go/culprit/mocks"
	"go.skia.org/infra/perf/go/dataframe"
	perfgit_mocks "go.skia.org/infra/perf/go/git/mocks"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/regression"
	reg_mocks "go.skia.org/infra/perf/go/regression/mocks"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

func setupAnomaliesApiWithMocks(t *testing.T) (anomaliesApi, *anomalygroup_mocks.Store, *culprit_mocks.Store, *reg_mocks.Store) {
	anomalygroupStore := anomalygroup_mocks.NewStore(t)
	culpritStore := culprit_mocks.NewStore(t)
	regStore := reg_mocks.NewStore(t)

	api := anomaliesApi{
		anomalygroupStore: anomalygroupStore,
		culpritStore:      culpritStore,
		regStore:          regStore,
	}
	return api, anomalygroupStore, culpritStore, regStore
}

func TestGetGroupReportByBugId(t *testing.T) {
	api, anomalygroupStore, culpritStore, regStore := setupAnomaliesApiWithMocks(t)

	ctx := context.Background()
	bugId := "12345"
	anomalyIds := []string{"anomaly-id-1", "anomaly-id-2", "anomaly-improvement"}
	culrpitAnomalyIds := []string{"anomaly-id-4"}
	allAnomalyIds := append(anomalyIds, culrpitAnomalyIds...)
	traceset := ",arch=x86,bot=linux,benchmark=jetstream2,test=score,config=default,master=main,"

	anomalyGroupIds := []string{"agid-1"}
	culpritStore.On("GetAnomalyGroupIdsForIssueId", mock.Anything, bugId).Return(anomalyGroupIds, nil).Once()
	anomalygroupStore.On("GetAnomalyIdsByAnomalyGroupIds", mock.Anything, anomalyGroupIds).Return(culrpitAnomalyIds, nil).Once()

	anomalygroupStore.On("GetAnomalyIdsByIssueId", mock.Anything, bugId).Return(anomalyIds, nil)

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
		{
			Id: "anomaly-id-4",
			Frame: &frame.FrameResponse{
				DataFrame: &dataframe.DataFrame{
					TraceSet: types.TraceSet{
						traceset: []float32{2.0},
					},
				},
			},
		},
	}
	regStore.On("GetByIDs", mock.Anything, allAnomalyIds).Return(regressions, nil)

	// Create the request.
	req := GetGroupReportRequest{
		BugID: bugId,
	}

	// Call the function under test.
	resp, err := api.getGroupReportByBugId(ctx, req)

	// Assert the results.
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Len(t, resp.Anomalies, len(regressions))
	// Note: compat.ConvertRegressionToAnomalies is not mocked, so we can't check all fields.
	// We are just checking the Id.
	for _, id := range allAnomalyIds {
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
	api, anomalygroupStore, _, regStore := setupAnomaliesApiWithMocks(t)

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
	api, anomalygroupStore, _, regStore := setupAnomaliesApiWithMocks(t)

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

func TestGetGroupReportByRevision(t *testing.T) {
	api, anomalygroupStore, _, regStore := setupAnomaliesApiWithMocks(t)

	ctx := context.Background()
	anomalyIds := []string{"anom-id-1", "anom-id-2"}
	traceset := ",arch=x86,bot=linux,benchmark=jetstream2,test=score,config=default,master=main,"
	revision := "1"

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
	regStore.On("GetByRevision", ctx, revision).Return(regressions, nil).Once()

	// Create the request.
	req := GetGroupReportRequest{
		Revison: revision,
	}

	// Call the function under test.
	resp, err := api.getGroupReportByRevision(ctx, req)

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

func TestGetGroupReportByRevision_InvalidRevisionsAreRejected(t *testing.T) {
	api, anomalygroupStore, _, regStore := setupAnomaliesApiWithMocks(t)

	ctx := context.Background()

	badRevision := "not-a-number"

	// Create the request.
	req := GetGroupReportRequest{
		Revison: badRevision,
	}

	regStore.On("GetByRevision", ctx, badRevision).Return(nil, skerr.Fmt("error"))

	// Call the function under test.
	resp, err := api.getGroupReportByRevision(ctx, req)

	// Assert the results.
	require.Error(t, err)
	require.Nil(t, resp)
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

func TestGetTimeRangeMap(t *testing.T) {
	ctx := context.Background()

	// Create a mock perfgit.Git instance.
	mockGit := &perfgit_mocks.Git{}

	// Define the expected behavior of the mock.
	startCommit := provider.Commit{Timestamp: 1672531200} // 2023-01-01 00:00:00
	endCommit := provider.Commit{Timestamp: 1672617600}   // 2023-01-02 00:00:00
	mockGit.On("CommitFromCommitNumber", ctx, types.CommitNumber(12345)).Return(startCommit, nil)
	mockGit.On("CommitFromCommitNumber", ctx, types.CommitNumber(54321)).Return(endCommit, nil)

	// Create an instance of anomaliesApi with the mockGit.
	api := anomaliesApi{
		perfGit: mockGit,
	}

	// Create sample anomalies.
	anomalies := []chromeperf.Anomaly{
		{
			Id:            "anomaly1",
			StartRevision: 12345,
			EndRevision:   54321,
		},
	}

	// Call the function under test.
	timerangeMap, err := api.getTimerangeMap(ctx, anomalies)

	// Assert the results.
	assert.NoError(t, err)
	assert.NotNil(t, timerangeMap)
	assert.Len(t, timerangeMap, 1)

	expectedTimerange := Timerange{
		Begin: startCommit.Timestamp,
		End:   time.Unix(endCommit.Timestamp, 0).AddDate(0, 0, 1).Unix(),
	}
	assert.Equal(t, expectedTimerange, timerangeMap["anomaly1"])

	// Verify that the mock's expectations were met.
	mockGit.AssertExpectations(t)
}
