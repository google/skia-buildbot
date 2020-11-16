package psrefresh

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/psrefresh/mocks"
	"go.skia.org/infra/perf/go/types"
)

func TestRefresher(t *testing.T) {
	unittest.SmallTest(t)

	op := &mocks.OPSProvider{}
	tileNumber := types.TileNumber(100)
	tileNumber2 := tileNumber.Prev()
	op.On("GetLatestTile", mock.Anything).Return(tileNumber, nil)

	ps1 := paramtools.NewOrderedParamSet()
	ps1.Update(paramtools.ParamSet{
		"config": []string{"8888", "565"},
	})
	ps2 := paramtools.NewOrderedParamSet()
	ps2.Update(paramtools.ParamSet{
		"config": []string{"8888", "565", "gles"},
	})
	op.On("GetOrderedParamSet", mock.Anything, tileNumber).Return(ps1, nil)
	op.On("GetOrderedParamSet", mock.Anything, tileNumber2).Return(ps2, nil)

	pf := NewParamSetRefresher(op)
	err := pf.Start(time.Minute)
	assert.NoError(t, err)
	assert.Len(t, pf.Get()["config"], 3)
}

func TestRefresherFailure(t *testing.T) {
	unittest.SmallTest(t)

	op := &mocks.OPSProvider{}
	tileNumber := types.TileNumber(100)
	op.On("GetLatestTile", mock.Anything).Return(tileNumber, fmt.Errorf("Something happened"))

	pf := NewParamSetRefresher(op)
	err := pf.Start(time.Minute)
	assert.Error(t, err)
}
