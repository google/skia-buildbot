package cachepopulation

import (
	"context"
	"errors"
	"testing"

	cache_mock "go.skia.org/infra/go/cache/mock"
	"go.skia.org/infra/go/testutils"
)

const mockCacheKey = "testKey"

type mockCacheDataProvider struct {
	CacheResult string
	Error       error
}

func (m mockCacheDataProvider) GetData(ctx context.Context) (string, string, error) {
	return mockCacheKey, m.CacheResult, m.Error
}

var _ CacheDataProvider = mockCacheDataProvider{}

func TestCachePopulator_SingleJob_CorrectDataStored(t *testing.T) {
	mockJob := mockCacheDataProvider{CacheResult: "{ result: \"Sample Result\"}"}
	jobs := []CacheDataProvider{mockJob}
	cacheMock := cache_mock.NewCache(t)
	cacheMock.On("SetValue", testutils.AnyContext, mockCacheKey, mockJob.CacheResult).Return(nil)
	defer cacheMock.AssertExpectations(t)

	cachePopulator := NewCachePopulator(cacheMock, jobs)

	ctx := context.Background()
	cachePopulator.Start(ctx)
	cacheMock.AssertExpectations(t)
}

func TestCachePopulator_SingleJob_DataProvider_ReturnsError(t *testing.T) {
	mockJob := mockCacheDataProvider{Error: errors.New("Error getting data for cache.")}
	jobs := []CacheDataProvider{mockJob}

	// No cache operations expected when an error occurs.
	cacheMock := cache_mock.NewCache(t)
	defer cacheMock.AssertExpectations(t)

	cachePopulator := NewCachePopulator(cacheMock, jobs)

	ctx := context.Background()
	cachePopulator.Start(ctx)
	cacheMock.AssertExpectations(t)
}
