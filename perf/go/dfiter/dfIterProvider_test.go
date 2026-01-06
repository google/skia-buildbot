package dfiter

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/dataframe/mocks"
	"go.skia.org/infra/perf/go/progress"
)

func TestDfProvider_GetDataFrame_CacheMiss(t *testing.T) {
	provider := NewDfProvider()
	ctx := context.Background()
	mockBuilder := mocks.NewDataFrameBuilder(t)
	q, _ := query.NewFromString("a=b")
	endTime := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	n := int32(100)
	config.Config = &config.InstanceConfig{}
	config.Config.Experiments = config.Experiments{ProgressUseRedisCache: false}
	prog := progress.New()

	expectedDf := &dataframe.DataFrame{
		Header: []*dataframe.ColumnHeader{
			{Offset: 1, Timestamp: 1234567890},
		},
	}

	mockBuilder.On("NewNFromQuery", testutils.AnyContext, endTime, q, n, prog).Return(expectedDf, nil).Once()

	df, err := provider.GetDataFrame(ctx, mockBuilder, q, endTime, n, prog)
	assert.NoError(t, err)
	assert.Equal(t, expectedDf, df)

	// Verify that it is in cache now
	key := key(q, endTime, n)
	provider.mutex.RLock()
	cachedDf, ok := provider.dfCache[key]
	provider.mutex.RUnlock()
	assert.True(t, ok)
	assert.Equal(t, expectedDf, cachedDf)
}

func TestDfProvider_GetDataFrame_CacheHit(t *testing.T) {
	provider := NewDfProvider()
	ctx := context.Background()
	mockBuilder := mocks.NewDataFrameBuilder(t)
	q, _ := query.NewFromString("a=b")
	endTime := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	n := int32(100)
	config.Config = &config.InstanceConfig{}
	config.Config.Experiments = config.Experiments{ProgressUseRedisCache: false}
	prog := progress.New()

	expectedDf := &dataframe.DataFrame{
		Header: []*dataframe.ColumnHeader{
			{Offset: 1, Timestamp: 1234567890},
		},
	}

	// Pre-populate cache
	key := key(q, endTime, n)
	provider.dfCache[key] = expectedDf

	// Builder should NOT be called.
	// Since we passed a mockBuilder, if it were called unexpectedly, the test would fail automatically
	// because we didn't set up any expectations.

	df, err := provider.GetDataFrame(ctx, mockBuilder, q, endTime, n, prog)
	assert.NoError(t, err)
	assert.Equal(t, expectedDf, df)
}

func TestDfProvider_GetDataFrame_BuilderError(t *testing.T) {
	provider := NewDfProvider()
	ctx := context.Background()
	mockBuilder := mocks.NewDataFrameBuilder(t)
	q, _ := query.NewFromString("a=b")
	endTime := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	n := int32(100)
	config.Config = &config.InstanceConfig{}
	config.Config.Experiments = config.Experiments{ProgressUseRedisCache: false}
	prog := progress.New()

	expectedErr := errors.New("builder error")

	mockBuilder.On("NewNFromQuery", testutils.AnyContext, endTime, q, n, prog).Return(nil, expectedErr).Once()

	df, err := provider.GetDataFrame(ctx, mockBuilder, q, endTime, n, prog)
	assert.Error(t, err)
	assert.Nil(t, df)
	assert.Equal(t, expectedErr, err)

	// Verify that it is NOT in cache
	key := key(q, endTime, n)
	provider.mutex.RLock()
	_, ok := provider.dfCache[key]
	provider.mutex.RUnlock()
	assert.False(t, ok)
}

func TestKey(t *testing.T) {
	q, _ := query.NewFromString("a=b")
	endTime := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	n := int32(100)

	k := key(q, endTime, n)
	// Expected format: "%s_%s_%d" -> "a=b_2021-01-01 00:00:00 +0000 UTC_100"
	// Note: String representation of time might vary slightly based on environment/go version,
	// but generally it's standard. We check if it contains the components.
	assert.Contains(t, k, "a=[b]")
	assert.Contains(t, k, "2021-01-01")
	assert.Contains(t, k, "100")
}

func TestDfProvider_GetDataFrame_SingleFlight(t *testing.T) {
	provider := NewDfProvider()
	ctx := context.Background()
	mockBuilder := mocks.NewDataFrameBuilder(t)
	q, _ := query.NewFromString("a=b")
	endTime := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	n := int32(100)
	prog := progress.New()

	expectedDf := &dataframe.DataFrame{
		Header: []*dataframe.ColumnHeader{
			{Offset: 1, Timestamp: 1234567890},
		},
	}

	// This should only be called ONCE even with concurrent requests
	mockBuilder.On("NewNFromQuery", testutils.AnyContext, endTime, q, n, prog).
		Return(expectedDf, nil).
		Once().
		Run(func(args mock.Arguments) {
			// Simulate some work time to ensure overlap
			time.Sleep(50 * time.Millisecond)
		})

	var wg sync.WaitGroup
	numRoutines := 5
	wg.Add(numRoutines)

	for i := 0; i < numRoutines; i++ {
		go func() {
			defer wg.Done()
			df, err := provider.GetDataFrame(ctx, mockBuilder, q, endTime, n, prog)
			assert.NoError(t, err)
			assert.Equal(t, expectedDf, df)
		}()
	}

	wg.Wait()
	mockBuilder.AssertExpectations(t)
}
