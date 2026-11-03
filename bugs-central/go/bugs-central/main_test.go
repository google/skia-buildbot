package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/bugs-central/go/types"
	db_mocks "go.skia.org/infra/bugs-central/go/types/mocks"
)

func TestPopulateChartsDataCache_MockClientData_NoErrors(t *testing.T) {

	ctx := context.Background()
	testRunID1 := "test-run1"
	testRecognizedRunIDS := map[string]bool{testRunID1: true}
	testClient := types.RecognizedClient("testClient")
	testSource := types.IssueSource("testSource")
	testQuery := "test-query"
	retClients := map[types.RecognizedClient]map[types.IssueSource]map[string]bool{
		testClient: {
			testSource: {
				testQuery: true,
			},
		},
	}
	retCountsData := &types.IssueCountsData{
		OpenCount: 5,
		P1Count:   2,
	}
	retQueryData1 := []*types.QueryData{
		{
			RunId:      testRunID1,
			CountsData: retCountsData,
		},
	}

	// Create mocks.
	dbMock := &db_mocks.BugsDB{}
	defer dbMock.AssertExpectations(t)
	dbMock.On("GetClientsFromDB", ctx).Return(retClients, nil).Once()
	dbMock.On("GetAllRecognizedRunIds", ctx).Return(testRecognizedRunIDS, nil).Once()
	dbMock.On("GetQueryDataFromDB", ctx, testClient, testSource, testQuery).Return(retQueryData1, nil).Once()

	srv := Server{
		dbClient: dbMock,
	}
	err := srv.populateChartsDataCache(ctx)
	require.NoError(t, err)
	// Including the empty client/source/query there should be 4 entries returned.
	require.Equal(t, 4, len(clientsToChartsDataCache))
	// Assert charts data of all 4 entries.
	require.Equal(t, map[string]*types.IssueCountsData{testRunID1: retCountsData}, clientsToChartsDataCache[getClientsKey(testClient, testSource, testQuery)])
	require.Equal(t, map[string]*types.IssueCountsData{testRunID1: retCountsData}, clientsToChartsDataCache[getClientsKey(testClient, testSource, "")])
	require.Equal(t, map[string]*types.IssueCountsData{testRunID1: retCountsData}, clientsToChartsDataCache[getClientsKey(testClient, "", "")])
	require.Equal(t, map[string]*types.IssueCountsData{testRunID1: retCountsData}, clientsToChartsDataCache[getClientsKey("", "", "")])
}
