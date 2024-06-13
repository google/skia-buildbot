package psrefresh

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/config"
	dfb "go.skia.org/infra/perf/go/dataframe/mocks"
	"go.skia.org/infra/perf/go/psrefresh/mocks"
	"go.skia.org/infra/perf/go/types"
)

var (
	errMyMockError = errors.New("my mock error")
	qConfig        = config.QueryConfig{
		RedisConfig: config.RedisConfig{},
	}
)

func TestRefresher_TwoTiles_Success(t *testing.T) {

	op := &mocks.OPSProvider{}
	tileNumber := types.TileNumber(100)
	tileNumber2 := tileNumber.Prev()
	op.On("GetLatestTile", testutils.AnyContext).Return(tileNumber, nil)

	ps1 := paramtools.ReadOnlyParamSet{
		"config": []string{"8888", "565"},
	}
	ps2 := paramtools.ReadOnlyParamSet{
		"config": []string{"8888", "565", "gles"},
	}
	op.On("GetParamSet", testutils.AnyContext, tileNumber).Return(ps1, nil)
	op.On("GetParamSet", testutils.AnyContext, tileNumber2).Return(ps2, nil)

	dfbMock := &dfb.DataFrameBuilder{}

	pf := NewParamSetRefresher(op, 2, dfbMock, qConfig)
	err := pf.Start(time.Minute)
	assert.NoError(t, err)
	assert.Equal(t, []string{"565", "8888", "gles"}, pf.Get()["config"])
	op.AssertExpectations(t)
}

func TestRefresher_TwoTiles_Success_WithCache(t *testing.T) {

	op := &mocks.OPSProvider{}
	tileNumber := types.TileNumber(100)
	tileNumber2 := tileNumber.Prev()
	op.On("GetLatestTile", testutils.AnyContext).Return(tileNumber, nil)

	ps1 := paramtools.ReadOnlyParamSet{
		"config": []string{"8888", "565"},
	}
	ps2 := paramtools.ReadOnlyParamSet{
		"config": []string{"8888", "565", "gles"},
	}
	op.On("GetParamSet", testutils.AnyContext, tileNumber).Return(ps1, nil)
	op.On("GetParamSet", testutils.AnyContext, tileNumber2).Return(ps2, nil)

	dfbMock := &dfb.DataFrameBuilder{}
	dfbMock.On("PreflightQuery", mock.Anything, mock.Anything, mock.Anything).Return(
		int64(10), paramtools.ParamSet{"config": []string{"8888"}}, nil)
	qConfig.RedisConfig.Enabled = true
	qConfig.RedisConfig.Level1Key = "config"
	qConfig.RedisConfig.Level1Values = []string{"8888"}
	pf := NewParamSetRefresher(op, 2, dfbMock, qConfig)
	err := pf.Start(time.Minute)
	assert.NoError(t, err)
	op.AssertExpectations(t)
	err = pf.oneStep(true)
	assert.Equal(t, pf.countCache["8888"], int64(10))
	assert.Equal(t, pf.queryCache["8888"], paramtools.ReadOnlyParamSet{"config": []string{"8888"}})
	assert.NoError(t, err)
}

func TestRefresher_TwoTiles_Success_WithCache_NoValid(t *testing.T) {

	op := &mocks.OPSProvider{}
	tileNumber := types.TileNumber(100)
	tileNumber2 := tileNumber.Prev()
	op.On("GetLatestTile", testutils.AnyContext).Return(tileNumber, nil)

	ps1 := paramtools.ReadOnlyParamSet{
		"config": []string{"8888", "565"},
	}
	ps2 := paramtools.ReadOnlyParamSet{
		"config": []string{"8888", "565", "gles"},
	}
	op.On("GetParamSet", testutils.AnyContext, tileNumber).Return(ps1, nil)
	op.On("GetParamSet", testutils.AnyContext, tileNumber2).Return(ps2, nil)

	dfbMock := &dfb.DataFrameBuilder{}
	dfbMock.On("PreflightQuery", mock.Anything, mock.Anything, mock.Anything).Return(
		int64(10), paramtools.ParamSet{"config": []string{"8888"}}, nil)
	qConfig.RedisConfig.Enabled = true
	qConfig.RedisConfig.Level1Key = "config"
	qConfig.RedisConfig.Level1Values = []string{"7777"}
	pf := NewParamSetRefresher(op, 2, dfbMock, qConfig)
	err := pf.Start(time.Minute)
	assert.NoError(t, err)
	op.AssertExpectations(t)
	err = pf.oneStep(true)
	assert.Equal(t, pf.countCache, map[string]int64{})
	assert.Equal(t, pf.queryCache, map[string]paramtools.ReadOnlyParamSet{})
	assert.NoError(t, err)
}

func TestRefresher_GetLatestTileReturnsError_ReturnsError(t *testing.T) {

	op := &mocks.OPSProvider{}
	tileNumber := types.TileNumber(100)
	op.On("GetLatestTile", testutils.AnyContext).Return(tileNumber, errMyMockError)

	dfbMock := &dfb.DataFrameBuilder{}
	pf := NewParamSetRefresher(op, 2, dfbMock, qConfig)
	err := pf.Start(time.Minute)
	assert.Error(t, err)
	op.AssertExpectations(t)
}

func TestRefresher_MulitpleTiles_Success(t *testing.T) {

	op := &mocks.OPSProvider{}
	tileNumber := types.TileNumber(100)
	op.On("GetLatestTile", testutils.AnyContext).Return(tileNumber, nil)

	ps1 := paramtools.ReadOnlyParamSet{
		"config": []string{"8888", "565"},
	}
	op.On("GetParamSet", testutils.AnyContext, mock.Anything).Return(ps1, nil).Times(3)

	dfbMock := &dfb.DataFrameBuilder{}
	pf := NewParamSetRefresher(op, 3, dfbMock, qConfig)
	err := pf.Start(time.Minute)
	assert.NoError(t, err)
	assert.Equal(t, []string{"565", "8888"}, pf.Get()["config"])
	op.AssertExpectations(t)
}

func TestRefresher_MulitpleTilesFirstTileOKSecondTileFails_Success(t *testing.T) {

	op := &mocks.OPSProvider{}
	tileNumber := types.TileNumber(100)
	tileNumber2 := tileNumber.Prev()
	op.On("GetLatestTile", testutils.AnyContext).Return(tileNumber, nil)

	ps1 := paramtools.ReadOnlyParamSet{
		"config": []string{"8888", "565"},
	}
	op.On("GetParamSet", testutils.AnyContext, tileNumber).Return(ps1, nil)
	op.On("GetParamSet", testutils.AnyContext, tileNumber2).Return(ps1, errMyMockError)

	dfbMock := &dfb.DataFrameBuilder{}
	pf := NewParamSetRefresher(op, 2, dfbMock, qConfig)
	err := pf.Start(time.Minute)
	assert.NoError(t, err)
	assert.Equal(t, []string{"565", "8888"}, pf.Get()["config"])
	op.AssertExpectations(t)
}

func TestRefresher_FailsFirstTile_ReturnsError(t *testing.T) {

	op := &mocks.OPSProvider{}
	tileNumber := types.TileNumber(100)
	op.On("GetLatestTile", testutils.AnyContext).Return(tileNumber, nil)

	ps1 := paramtools.ReadOnlyParamSet{
		"config": []string{"8888", "565"},
	}
	op.On("GetParamSet", testutils.AnyContext, tileNumber).Return(ps1, errMyMockError)

	dfbMock := &dfb.DataFrameBuilder{}
	pf := NewParamSetRefresher(op, 2, dfbMock, qConfig)
	err := pf.Start(time.Minute)
	assert.Error(t, err)
	op.AssertExpectations(t)
}
