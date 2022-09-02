package psrefresh

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/psrefresh/mocks"
	"go.skia.org/infra/perf/go/types"
)

var (
	errMyMockError = errors.New("my mock error")
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

	pf := NewParamSetRefresher(op, 2)
	err := pf.Start(time.Minute)
	assert.NoError(t, err)
	assert.Equal(t, []string{"565", "8888", "gles"}, pf.Get()["config"])
	op.AssertExpectations(t)
}

func TestRefresher_GetLatestTileReturnsError_ReturnsError(t *testing.T) {

	op := &mocks.OPSProvider{}
	tileNumber := types.TileNumber(100)
	op.On("GetLatestTile", testutils.AnyContext).Return(tileNumber, errMyMockError)

	pf := NewParamSetRefresher(op, 2)
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

	pf := NewParamSetRefresher(op, 3)
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

	pf := NewParamSetRefresher(op, 2)
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

	pf := NewParamSetRefresher(op, 2)
	err := pf.Start(time.Minute)
	assert.Error(t, err)
	op.AssertExpectations(t)
}
