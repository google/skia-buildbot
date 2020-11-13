package fs_metricsstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/types"
)

func TestToStoreEntryToDiffMetrics(t *testing.T) {
	unittest.SmallTest(t)
	expectedDiffMetrics := makeDiffMetrics(100)
	entry := toStoreEntry(expectedDiffMetrics)
	actualDiffMetrics := entry.toDiffMetrics()
	assert.Equal(t, expectedDiffMetrics, actualDiffMetrics)
}

func TestPutGetDiffMetrics(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(context.Background(), t)
	defer cleanup()

	// Create Firestore-backed MetricsStore instance.
	f := New(c)

	// Create test DiffMetrics instances.
	id1 := "abc-def"
	expectedDiffMetrics1 := makeDiffMetrics(100)
	id2 := "ghi-jkl"
	expectedDiffMetrics2 := makeDiffMetrics(200)

	// Not found.
	ctx := context.Background()
	m, err := f.LoadDiffMetrics(ctx, []string{id1, id2})
	require.NoError(t, err)
	assert.Nil(t, m[0])
	assert.Nil(t, m[1])

	// Save them.
	err = f.SaveDiffMetrics(ctx, id1, expectedDiffMetrics1)
	assert.NoError(t, err)
	err = f.SaveDiffMetrics(ctx, id2, expectedDiffMetrics2)
	assert.NoError(t, err)

	// Load them.
	actual, err := f.LoadDiffMetrics(ctx, []string{id1, id2})
	require.NoError(t, err)

	// Assert that the right diff metrics were returned.
	assert.Equal(t, expectedDiffMetrics1, actual[0])
	assert.Equal(t, expectedDiffMetrics2, actual[1])
}

func TestPurge(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(context.Background(), t)
	defer cleanup()

	// Create Firestore-backed MetricsStore instance.
	f := New(c)

	// Purge non-existent digest.
	ctx := context.Background()
	err := f.PurgeDiffMetrics(ctx, types.DigestSlice{types.Digest("abc")})
	assert.NoError(t, err)

	// Add metrics.
	leftId := types.Digest("abc")
	rightId := types.Digest("def")
	diffId := string(leftId + "-" + rightId)
	expected := makeDiffMetrics(100)
	assert.NoError(t, f.SaveDiffMetrics(ctx, diffId, expected))

	// Purging by coercing the diffId as a types.Digest does nothing.
	err = f.PurgeDiffMetrics(ctx, types.DigestSlice{types.Digest(diffId)})
	assert.NoError(t, err)
	dm, err := f.LoadDiffMetrics(ctx, []string{diffId})
	assert.NoError(t, err)
	assert.Equal(t, expected, dm[0])

	// Purging by leftId works.
	err = f.PurgeDiffMetrics(ctx, types.DigestSlice{leftId})
	assert.NoError(t, err)
	dm, err = f.LoadDiffMetrics(ctx, []string{diffId})
	assert.NoError(t, err)
	assert.Nil(t, dm[0])

	// Re-add metric.
	assert.NoError(t, f.SaveDiffMetrics(ctx, diffId, expected))

	// Purging by rightId works.
	err = f.PurgeDiffMetrics(ctx, types.DigestSlice{rightId})
	assert.NoError(t, err)
	dm, err = f.LoadDiffMetrics(ctx, []string{diffId})
	assert.NoError(t, err)
	assert.Nil(t, dm[0])
}

func TestPurgeMultiple(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(context.Background(), t)
	defer cleanup()

	// Create Firestore-backed MetricsStore instance.
	f := New(c)

	// Add multiple metrics.
	ctx := context.Background()
	assert.NoError(t, f.SaveDiffMetrics(ctx, "aaa-bbb", makeDiffMetrics(100)))
	assert.NoError(t, f.SaveDiffMetrics(ctx, "aaa-ccc", makeDiffMetrics(200)))
	assert.NoError(t, f.SaveDiffMetrics(ctx, "aaa-ddd", makeDiffMetrics(300)))
	assert.NoError(t, f.SaveDiffMetrics(ctx, "aaa-eee", makeDiffMetrics(400)))
	assert.NoError(t, f.SaveDiffMetrics(ctx, "bbb-ccc", makeDiffMetrics(500)))
	assert.NoError(t, f.SaveDiffMetrics(ctx, "bbb-ddd", makeDiffMetrics(600)))
	assert.NoError(t, f.SaveDiffMetrics(ctx, "bbb-eee", makeDiffMetrics(700)))
	assert.NoError(t, f.SaveDiffMetrics(ctx, "ccc-ddd", makeDiffMetrics(800)))
	assert.NoError(t, f.SaveDiffMetrics(ctx, "ccc-eee", makeDiffMetrics(900)))
	assert.NoError(t, f.SaveDiffMetrics(ctx, "ddd-eee", makeDiffMetrics(1000)))

	// Purge some but not all.
	err := f.PurgeDiffMetrics(ctx, types.DigestSlice{
		types.Digest("aaa"),
		types.Digest("bbb"),
	})
	assert.NoError(t, err)

	// Assert that the expected metrics were purged.
	purged := []string{
		"aaa-bbb",
		"aaa-ccc",
		"aaa-ddd",
		"aaa-eee",
		"bbb-ccc",
		"bbb-ddd",
		"bbb-eee",
	}
	xdm, err := f.LoadDiffMetrics(ctx, purged)
	require.NoError(t, err)
	for _, dm := range xdm {
		assert.Nil(t, dm)
	}

	// Assert that the expected metrics remain in the store.
	notPurged := map[string]*diff.DiffMetrics{
		"ccc-ddd": makeDiffMetrics(800),
		"ccc-eee": makeDiffMetrics(900),
		"ddd-eee": makeDiffMetrics(1000),
	}
	for id, expectedDiffMetrics := range notPurged {
		actualDiffMetrics, err := f.LoadDiffMetrics(ctx, []string{id})
		assert.NoError(t, err)
		assert.Equal(t, expectedDiffMetrics, actualDiffMetrics[0])
	}
}

func TestCancelledContext(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(context.Background(), t)
	defer cleanup()

	// Create Firestore-backed MetricsStore instance.
	f := New(c)

	// Cancellable context.
	ctx, cancelFn := context.WithCancel(context.Background())

	// Create and save test DiffMetrics instance.
	id := "abc-def"
	diffMetrics := makeDiffMetrics(100)
	assert.NoError(t, f.SaveDiffMetrics(ctx, id, diffMetrics))

	// Cancel context. The most common scenario where this would happen is during a web request to
	// retrieve metrics which gets interrupted mid-flight, e.g. if the user closes the browser tab.
	cancelFn()

	// Try all MetricsStore methods with the cancelled context, assert that they fail.
	_, err := f.LoadDiffMetrics(ctx, []string{id})
	assert.Error(t, err)
	assert.Error(t, f.SaveDiffMetrics(ctx, id, diffMetrics))
	assert.Error(t, f.PurgeDiffMetrics(ctx, types.DigestSlice{types.Digest(id)}))
}

func makeDiffMetrics(numDiffPixels int) *diff.DiffMetrics {
	diffMetrics := &diff.DiffMetrics{
		NumDiffPixels:    numDiffPixels,
		PixelDiffPercent: 0.5,
		MaxRGBADiffs:     [4]int{2, 3, 5, 7},
		DimDiffer:        true,
	}
	diffMetrics.CombinedMetric = diff.CombinedDiffMetric(diffMetrics.MaxRGBADiffs, diffMetrics.PixelDiffPercent)
	return diffMetrics
}
