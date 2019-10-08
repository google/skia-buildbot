package bolt_and_fs_metricsstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/types"
)

func TestSave(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()
	tmpDir, cleanup := testutils.TempDir(t)
	defer cleanup()

	// Create MetricsStore.
	s, err := New(tmpDir, c)
	assert.NoError(t, err)

	// Create test DiffMetrics instance.
	id := "abc-def"
	expectedDiffMetrics := makeDiffMetrics(100)

	// Save it.
	err = s.SaveDiffMetrics(id, expectedDiffMetrics)
	assert.NoError(t, err)

	// Assert that the Bolt-backed store contains it.
	actualDiffMetrics, err := s.boltStore.LoadDiffMetrics(id)
	assert.NoError(t, err)
	assert.Equal(t, expectedDiffMetrics, actualDiffMetrics)

	// Assert that the Firestore-backed store contains it.
	actualDiffMetrics, err = s.fsStore.LoadDiffMetrics(id)
	assert.NoError(t, err)
	assert.Equal(t, expectedDiffMetrics, actualDiffMetrics)
}

func TestLoad(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()
	tmpDir, cleanup := testutils.TempDir(t)
	defer cleanup()

	// Create MetricsStore.
	s, err := New(tmpDir, c)
	assert.NoError(t, err)

	// Create two different DiffMetrics instances, one for each store.
	id := "abc-def"
	boltDiffMetrics := makeDiffMetrics(100)
	firestoreDiffMetrics := makeDiffMetrics(200)

	// Save both to their respective stores.
	err = s.boltStore.SaveDiffMetrics(id, boltDiffMetrics)
	assert.NoError(t, err)
	err = s.fsStore.SaveDiffMetrics(id, firestoreDiffMetrics)
	assert.NoError(t, err)

	// Load.
	actualDiffMetrics, err := s.LoadDiffMetrics(id)
	assert.NoError(t, err)

	// Assert that we obtained the diffMetrics from the Bolt-backed implementation.
	assert.Equal(t, boltDiffMetrics, actualDiffMetrics)
}

func TestPurge(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()
	tmpDir, cleanup := testutils.TempDir(t)
	defer cleanup()

	// Create MetricsStore.
	s, err := New(tmpDir, c)
	assert.NoError(t, err)

	// Create a DiffMetrics instance.
	id := "abc-def"
	diffMetrics := makeDiffMetrics(100)

	// Save it to both stores.
	err = s.boltStore.SaveDiffMetrics(id, diffMetrics)
	assert.NoError(t, err)
	err = s.fsStore.SaveDiffMetrics(id, diffMetrics)
	assert.NoError(t, err)

	// Purge.
	err = s.PurgeDiffMetrics(types.DigestSlice{"abc"})
	assert.NoError(t, err)

	// Assert that neither store contains it.
	dm, err := s.boltStore.LoadDiffMetrics(id)
	assert.NoError(t, err)
	assert.Nil(t, dm)
	dm, err = s.fsStore.LoadDiffMetrics(id)
	assert.NoError(t, err)
	assert.Nil(t, dm)
}

func makeDiffMetrics(numDiffPixels int) *diff.DiffMetrics {
	diffMetrics := &diff.DiffMetrics{
		NumDiffPixels:    numDiffPixels,
		PixelDiffPercent: 0.5,
		MaxRGBADiffs:     []int{2, 3, 5, 7},
		DimDiffer:        true,
		Diffs: map[string]float32{
			diff.METRIC_PERCENT: 0.5,
			diff.METRIC_PIXEL:   float32(numDiffPixels),
		},
	}
	diffMetrics.Diffs[diff.METRIC_COMBINED] = diff.CombinedDiffMetric(diffMetrics, nil, nil)
	return diffMetrics
}
