package psrefresh

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/psrefresh/mocks"
	"go.skia.org/infra/perf/go/types"
)

func TestRefresher(t *testing.T) {
	unittest.SmallTest(t)

	op := &mocks.OPSProvider{}
	tileNumber := types.TileNumber(100)
	tileNumber2 := tileNumber.Prev()
	op.On("GetLatestTile", testutils.AnyContext).Return(tileNumber, nil)

	ps1 := paramtools.NewOrderedParamSet()
	ps1.Update(paramtools.ParamSet{
		"config": []string{"8888", "565"},
	})
	ps2 := paramtools.NewOrderedParamSet()
	ps2.Update(paramtools.ParamSet{
		"config": []string{"gles"},
	})
	op.On("GetOrderedParamSet", testutils.AnyContext, tileNumber).Return(ps1, nil)
	op.On("GetOrderedParamSet", testutils.AnyContext, tileNumber2).Return(ps2, nil)

	pf := NewParamSetRefresher(op)
	err := pf.Start(time.Minute)
	assert.NoError(t, err)
	assert.Equal(t, []string{"565", "8888", "gles"}, pf.Get()["config"])
}

func TestRefresherFailure(t *testing.T) {
	unittest.SmallTest(t)

	op := &mocks.OPSProvider{}
	tileNumber := types.TileNumber(100)
	op.On("GetLatestTile", testutils.AnyContext).Return(tileNumber, fmt.Errorf("Something happened"))

	pf := NewParamSetRefresher(op)
	err := pf.Start(time.Minute)
	assert.Error(t, err)
}
