package bolt_metricsstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	d_utils "go.skia.org/infra/golden/go/diffstore/testutils"
	"go.skia.org/infra/golden/go/types"
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

func TestPurge(t *testing.T) {
	unittest.MediumTest(t)

	w, cleanup := testutils.TempDir(t)
	defer cleanup()

	ms, err := New(w, util.JSONCodec(d_utils.DummyDiffMetrics{}))
	assert.NoError(t, err)

	// Purge non-existent digest.
	err = ms.PurgeDiffMetrics(types.DigestSlice{"abc"})
	assert.NoError(t, err)

	// Add metrics.
	leftId := types.Digest("abc")
	rightId := types.Digest("def")
	diffId := string(leftId + "-" + rightId)
	expected := &d_utils.DummyDiffMetrics{
		NumDiffPixels:     3,
		PercentDiffPixels: 0.3,
	}
	assert.NoError(t, ms.SaveDiffMetrics(diffId, expected))

	// Purging by coercing the diffId as a types.Digest does nothing.
	err = ms.PurgeDiffMetrics(types.DigestSlice{types.Digest(diffId)})
	assert.NoError(t, err)
	dm, err := ms.LoadDiffMetrics(diffId)
	assert.NoError(t, err)
	assert.Equal(t, expected, dm)

	// Purging by leftId works.
	err = ms.PurgeDiffMetrics(types.DigestSlice{leftId})
	assert.NoError(t, err)
	dm, err = ms.LoadDiffMetrics(diffId)
	assert.NoError(t, err)
	assert.Nil(t, dm)

	// Re-add metric.
	assert.NoError(t, ms.SaveDiffMetrics(diffId, expected))

	// Purging by rightId works.
	err = ms.PurgeDiffMetrics(types.DigestSlice{rightId})
	assert.NoError(t, err)
	dm, err = ms.LoadDiffMetrics(diffId)
	assert.NoError(t, err)
	assert.Nil(t, dm)
}
