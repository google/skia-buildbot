package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/bugs-central/go/types"
	db_mocks "go.skia.org/infra/bugs-central/go/types/mocks"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestPopulateChartsDataCache_MockClientData_NoErrors(t *testing.T) {
	unittest.SmallTest(t)

	ctx := context.Background()
	testRunID1 := "test-run1"
	testRunID2 := "test-run2"
	testRunID3 := "test-run3"
	testRunID4 := "test-run4"
	testRecognizedRunIDS := map[string]bool{testRunID1: true, testRunID2: true, testRunID3: true, testRunID4: true}
	testClient := types.RecognizedClient("testClient")
	testSource := types.IssueSource("testSource")
	testQuery := "test-query"
	retClients := map[types.RecognizedClient]map[types.IssueSource]map[string]bool{
		testClient: {
			testSource: {
				testQuery: false,
			},
		},
	}
	retCountsData := &types.IssueCountsData{}
	retQueryData1 := []*types.QueryData{
		{
			RunId:      testRunID1,
			CountsData: retCountsData,
		},
	}
	retQueryData2 := []*types.QueryData{
		{
			RunId:      testRunID2,
			CountsData: retCountsData,
		},
	}
	retQueryData3 := []*types.QueryData{
		{
			RunId:      testRunID3,
			CountsData: retCountsData,
		},
	}
	retQueryData4 := []*types.QueryData{
		{
			RunId:      testRunID4,
			CountsData: retCountsData,
		},
	}

	// Create mocks.
	dbMock := &db_mocks.BugsDB{}
	defer dbMock.AssertExpectations(t)
	retClients[testClient][testSource][testQuery] = true
	dbMock.On("GetClientsFromDB", ctx).Return(retClients, nil)
	dbMock.On("GetQueryDataFromDB", ctx, testClient, testSource, testQuery).Return(retQueryData1, nil).Once()
	dbMock.On("GetQueryDataFromDB", ctx, testClient, testSource, "").Return(retQueryData2, nil).Once()
	dbMock.On("GetQueryDataFromDB", ctx, testClient, types.IssueSource(""), "").Return(retQueryData3, nil).Once()
	dbMock.On("GetQueryDataFromDB", ctx, types.RecognizedClient(""), types.IssueSource(""), "").Return(retQueryData4, nil).Once()
	dbMock.On("GetAllRecognizedRunIds", ctx).Return(testRecognizedRunIDS, nil)

	srv := Server{
		dbClient: dbMock,
	}
	err := srv.populateChartsDataCache(ctx)
	require.NoError(t, err)
	// Including the empty client/source/query there should be 4 entries returned.
	require.Equal(t, 4, len(clientsToChartsDataCache))
	// Assert charts data of all 4 entries.
	require.Equal(t, map[string]*types.IssueCountsData{testRunID1: retCountsData}, clientsToChartsDataCache[getClientsKey(testClient, testSource, testQuery)])
	require.Equal(t, map[string]*types.IssueCountsData{testRunID2: retCountsData}, clientsToChartsDataCache[getClientsKey(testClient, testSource, "")])
	require.Equal(t, map[string]*types.IssueCountsData{testRunID3: retCountsData}, clientsToChartsDataCache[getClientsKey(testClient, "", "")])
	require.Equal(t, map[string]*types.IssueCountsData{testRunID4: retCountsData}, clientsToChartsDataCache[getClientsKey("", "", "")])
}
