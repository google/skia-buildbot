package validation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/validation/data_manager"
	"go.skia.org/infra/golden/go/validation/data_manager/mocks"
)

func TestUpdateExpiry_Expectations_NoRows_Success(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dataManager := mocks.NewExpiryDataManager(t)
	dataManager.On("GetExpiringExpectations", ctx).Return([]data_manager.ExpectationKey{}, nil)
	monitor := NewExpirationMonitor(dataManager)

	err := monitor.UpdateTriagedExpectationsExpiry(ctx)
	require.NoError(t, err)
	dataManager.AssertNumberOfCalls(t, "UpdateExpectationsExpiry", 0)
}

func TestUpdateExpiry_Expectations_Rows_Success(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dataManager := mocks.NewExpiryDataManager(t)
	expiringExpectations := []data_manager.ExpectationKey{
		{
			Digest:     "sampleDigest1",
			Groupingid: "sampleGrouping1",
		},
		{
			Digest:     "sampleDigest2",
			Groupingid: "sampleGrouping2",
		},
		{
			Digest:     "sampleDigest1",
			Groupingid: "sampleGrouping2",
		},
	}
	dataManager.On("GetExpiringExpectations", ctx).Return(expiringExpectations, nil)
	dataManager.On("UpdateExpectationsExpiry", ctx, expiringExpectations, mock.Anything).Return(nil)
	monitor := NewExpirationMonitor(dataManager)

	err := monitor.UpdateTriagedExpectationsExpiry(ctx)
	require.NoError(t, err)

	dataManager.AssertNumberOfCalls(t, "UpdateExpectationsExpiry", 1)
}

func TestUpdateExpiry_GetExpectations_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dataManager := mocks.NewExpiryDataManager(t)
	dataManager.On("GetExpiringExpectations", ctx).Return(nil, skerr.Fmt("GetExpectations error"))
	monitor := NewExpirationMonitor(dataManager)

	err := monitor.UpdateTriagedExpectationsExpiry(ctx)
	require.Error(t, err)
	dataManager.AssertNumberOfCalls(t, "UpdateExpectationsExpiry", 0)
}

func TestUpdateExpiry_UpdatetExpectations_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dataManager := mocks.NewExpiryDataManager(t)
	expiringExpectations := []data_manager.ExpectationKey{
		{
			Digest:     "sampleDigest1",
			Groupingid: "sampleGrouping1",
		},
		{
			Digest:     "sampleDigest2",
			Groupingid: "sampleGrouping2",
		},
		{
			Digest:     "sampleDigest1",
			Groupingid: "sampleGrouping2",
		},
	}
	dataManager.On("GetExpiringExpectations", ctx).Return(expiringExpectations, nil)
	dataManager.On("UpdateExpectationsExpiry", ctx, expiringExpectations, mock.Anything).Return(skerr.Fmt("Updating error"))
	monitor := NewExpirationMonitor(dataManager)

	err := monitor.UpdateTriagedExpectationsExpiry(ctx)
	require.Error(t, err)
	dataManager.AssertNumberOfCalls(t, "UpdateExpectationsExpiry", 1)
}
