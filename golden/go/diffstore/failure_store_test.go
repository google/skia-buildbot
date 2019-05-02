package diffstore

import (
	"path"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/types"
)

func TestFailureHandling(t *testing.T) {
	testutils.MediumTest(t)

	// Get a small tile and get them cached.
	w, cleanup := testutils.TempDir(t)
	defer cleanup()
	baseDir := path.Join(w, TEST_DATA_BASE_DIR+"-diffstore-failure")
	client, tile := getSetupAndTile(t, baseDir)

	mapper := NewGoldDiffStoreMapper(&diff.DiffMetrics{})
	diffStore, err := NewMemDiffStore(client, baseDir, []string{TEST_GCS_BUCKET_NAME}, TEST_GCS_IMAGE_DIR, 10, mapper)
	assert.NoError(t, err)

	validDigestSet := util.StringSet{}
	for _, trace := range tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		validDigestSet.AddLists(gTrace.Digests)
	}
	delete(validDigestSet, types.MISSING_DIGEST)

	invalidDigest_1 := "invaliddigest1"
	invalidDigest_2 := "invaliddigest2"

	validDigests := validDigestSet.Keys()
	mainDigest := validDigests[0]
	diffDigests := append(validDigests[1:6], invalidDigest_1, invalidDigest_2)

	diffs, err := diffStore.Get(diff.PRIORITY_NOW, mainDigest, diffDigests)
	assert.NoError(t, err)
	assert.Equal(t, len(diffDigests)-2, len(diffs))

	unavailableDigests := diffStore.UnavailableDigests()
	assert.Equal(t, 2, len(unavailableDigests))
	assert.NotNil(t, unavailableDigests[invalidDigest_1])
	assert.NotNil(t, unavailableDigests[invalidDigest_2])

	assert.NoError(t, diffStore.PurgeDigests([]string{invalidDigest_1, invalidDigest_2}, true))
	unavailableDigests = diffStore.UnavailableDigests()
	assert.Equal(t, 0, len(unavailableDigests))
}
