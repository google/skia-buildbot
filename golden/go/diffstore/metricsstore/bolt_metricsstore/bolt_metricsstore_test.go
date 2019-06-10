package bolt_metricsstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	d_utils "go.skia.org/infra/golden/go/diffstore/testutils"
)

func TestAddGet(t *testing.T) {
	unittest.MediumTest(t)

	w, cleanup := testutils.TempDir(t)
	defer cleanup()

	ms, err := New(w, util.JSONCodec(d_utils.DummyDiffMetrics{}))
	assert.NoError(t, err)

	id := "abc-def"

	dm, err := ms.LoadDiffMetrics(id)
	assert.NoError(t, err)
	assert.Nil(t, dm)

	expected := &d_utils.DummyDiffMetrics{
		NumDiffPixels:     3,
		PercentDiffPixels: 0.3,
	}

	assert.NoError(t, ms.SaveDiffMetrics(id, expected))

	dm, err = ms.LoadDiffMetrics(id)
	assert.NoError(t, err)
	assert.Equal(t, expected, dm)
}
