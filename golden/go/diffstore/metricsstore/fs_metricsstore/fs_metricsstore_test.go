package fs_metricsstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/types"
)

func TestToStoreEntryToDiffMetrics(t *testing.T) {
	expectedDiffMetrics := makeDiffMetrics()
	entry := toStoreEntry(expectedDiffMetrics)
	actualDiffMetrics := entry.toDiffMetrics()
	assert.Equal(t, expectedDiffMetrics, actualDiffMetrics)
}

func TestPutGetDiffMetrics(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	// Create Firestore-backed MetricsStore instance.
	f := New(c)

	// Create test DiffMetrics instance.
	id := "abc-def"
	diffMetrics := makeDiffMetrics()

	// Not found.
	m, err := f.LoadDiffMetrics(id)
	assert.NoError(t, err)
	assert.Nil(t, m)

	// Save it.
	err = f.SaveDiffMetrics(id, diffMetrics)
	assert.NoError(t, err)

	// Load it.
	m, err = f.LoadDiffMetrics(id)
	assert.NoError(t, err)
	assert.Equal(t, diffMetrics, m)
}

func TestPurge(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	// Create Firestore-backed MetricsStore instance.
	f := New(c)

	// Purge non-existent digest.
	err := f.PurgeDiffMetrics(types.DigestSlice{types.Digest("abc")})
	assert.NoError(t, err)

	// Add metrics.
	leftId := types.Digest("abc")
	rightId := types.Digest("def")
	diffId := string(leftId + "-" + rightId)
	expected := makeDiffMetrics()
	assert.NoError(t, f.SaveDiffMetrics(diffId, expected))

	// Purging by coercing the diffId as a types.Digest does nothing.
	err = f.PurgeDiffMetrics(types.DigestSlice{types.Digest(diffId)})
	assert.NoError(t, err)
	dm, err := f.LoadDiffMetrics(diffId)
	assert.NoError(t, err)
	assert.Equal(t, expected, dm)

	// Purging by leftId works.
	err = f.PurgeDiffMetrics(types.DigestSlice{leftId})
	assert.NoError(t, err)
	dm, err = f.LoadDiffMetrics(diffId)
	assert.NoError(t, err)
	assert.Nil(t, dm)

	// Re-add metric.
	assert.NoError(t, f.SaveDiffMetrics(diffId, expected))

	// Purging by rightId works.
	err = f.PurgeDiffMetrics(types.DigestSlice{rightId})
	assert.NoError(t, err)
	dm, err = f.LoadDiffMetrics(diffId)
	assert.NoError(t, err)
	assert.Nil(t, dm)
}

func TestPurgeMultiple(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	// Create Firestore-backed MetricsStore instance.
	f := New(c)

	// Add multiple metrics.
	assert.NoError(t, f.SaveDiffMetrics("aaa-bbb", makeDiffMetrics()))
	assert.NoError(t, f.SaveDiffMetrics("aaa-ccc", makeDiffMetrics()))
	assert.NoError(t, f.SaveDiffMetrics("aaa-ddd", makeDiffMetrics()))
	assert.NoError(t, f.SaveDiffMetrics("aaa-eee", makeDiffMetrics()))
	assert.NoError(t, f.SaveDiffMetrics("bbb-ccc", makeDiffMetrics()))
	assert.NoError(t, f.SaveDiffMetrics("bbb-ddd", makeDiffMetrics()))
	assert.NoError(t, f.SaveDiffMetrics("bbb-eee", makeDiffMetrics()))
	assert.NoError(t, f.SaveDiffMetrics("ccc-ddd", makeDiffMetrics()))
	assert.NoError(t, f.SaveDiffMetrics("ccc-eee", makeDiffMetrics()))
	assert.NoError(t, f.SaveDiffMetrics("ddd-eee", makeDiffMetrics()))

	// Purge some but not all.
	err := f.PurgeDiffMetrics(types.DigestSlice{
		types.Digest("aaa"),
		types.Digest("bbb"),
	})
	assert.NoError(t, err)

	// Assert that the expected metrics were purged.
	purged := []string {
		"aaa-bbb",
		"aaa-ccc",
		"aaa-ddd",
		"aaa-eee",
		"bbb-ccc",
		"bbb-ddd",
		"bbb-eee",
	}
	for _, id := range purged {
		dm, err := f.LoadDiffMetrics(id)
		assert.NoError(t, err)
		assert.Nil(t, dm)
	}

	// Assert that the expected metrics remain in the store.
	notPurged := []string {
		"ccc-ddd",
		"ccc-eee",
		"ddd-eee",
	}
	for _, id := range notPurged {
		dm, err := f.LoadDiffMetrics(id)
		assert.NoError(t, err)
		assert.NotNil(t, dm)
	}
}

func makeDiffMetrics() *diff.DiffMetrics {
	diffMetrics := &diff.DiffMetrics{
		NumDiffPixels: 100,
		PixelDiffPercent: 0.5,
		MaxRGBADiffs: []int{2, 3, 5, 7},
		DimDiffer: true,
		Diffs: map[string]float32{
			diff.METRIC_PERCENT: 0.5,
			diff.METRIC_PIXEL: float32(100),
		},
	}
	diffMetrics.Diffs[diff.METRIC_COMBINED] = diff.CombinedDiffMetric(diffMetrics, nil, nil)
	return diffMetrics
}