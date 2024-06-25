package psrefresh

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	mockCache "go.skia.org/infra/perf/go/cache/mock"
	"go.skia.org/infra/perf/go/config"
	dfb "go.skia.org/infra/perf/go/dataframe/mocks"
	"go.skia.org/infra/perf/go/psrefresh/mocks"
	"go.skia.org/infra/perf/go/types"
)

func TestPopulateCache_Success(t *testing.T) {
	pf := getPsRefresher()
	cache := mockCache.NewCache(t)
	cache.On("SetValue", mock.Anything, "8888", mock.Anything).Return(nil)
	refresher := NewCachedParamSetRefresher(pf, cache)

	ctx := context.Background()
	refresher.PopulateCache(ctx)
	cache.AssertNumberOfCalls(t, "SetValue", 1)
}

func TestPopulateCache_InvalidValue(t *testing.T) {
	pf := getPsRefresher()
	pf.qConfig.RedisConfig.Level1Values = []string{"NonExistingValue"}
	cache := mockCache.NewCache(t)
	refresher := NewCachedParamSetRefresher(pf, cache)

	ctx := context.Background()
	refresher.PopulateCache(ctx)
	cache.AssertNotCalled(t, "SetValue")
}

func getPsRefresher() *ParamSetRefresher {
	op := &mocks.OPSProvider{}
	tileNumber := types.TileNumber(100)
	op.On("GetLatestTile", testutils.AnyContext).Return(tileNumber, nil)

	ps1 := paramtools.ReadOnlyParamSet{
		"config": []string{"8888", "565"},
	}
	op.On("GetParamSet", testutils.AnyContext, tileNumber).Return(ps1, nil)

	dfbMock := &dfb.DataFrameBuilder{}
	dfbMock.On("PreflightQuery", mock.Anything, mock.Anything, mock.Anything).Return(
		int64(10), paramtools.ParamSet{"config": []string{"8888"}}, nil)

	qConfig := config.QueryConfig{
		RedisConfig: config.RedisConfig{
			Project:      "testProject",
			Zone:         "testZone",
			Instance:     "testInstance",
			Level1Key:    "config",
			Level1Values: []string{"8888"},
			Enabled:      true,
		},
	}
	pf := NewParamSetRefresher(op, 1, dfbMock, qConfig)
	_ = pf.Start(time.Minute)
	return pf
}
