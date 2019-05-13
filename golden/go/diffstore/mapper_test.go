package diffstore

import (
	"path"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/types"
)

const (
	// Test digests for GoldDiffStoreMapper.
	TEST_GOLD_LEFT  = types.Digest("098f6bcd4621d373cade4e832627b4f6")
	TEST_GOLD_RIGHT = types.Digest("1660f0783f4076284bc18c5f4bdc9608")

	// PNG extension.
	DOT_EXT = ".png"
)

func TestGoldDiffStoreMapper(t *testing.T) {
	unittest.SmallTest(t)

	mapper := GoldDiffStoreMapper{}

	// Test DiffID and SplitDiffID
	expectedDiffID := string(TEST_GOLD_LEFT + DIFF_IMG_SEPARATOR + TEST_GOLD_RIGHT)
	actualDiffID := mapper.DiffID(TEST_GOLD_LEFT, TEST_GOLD_RIGHT)
	actualLeft, actualRight := mapper.SplitDiffID(expectedDiffID)
	assert.Equal(t, expectedDiffID, actualDiffID)
	assert.Equal(t, TEST_GOLD_LEFT, actualLeft)
	assert.Equal(t, TEST_GOLD_RIGHT, actualRight)

	// Test DiffPath
	twoLevelRadix := TEST_GOLD_LEFT[0:2] + "/" + TEST_GOLD_LEFT[2:4] + "/"
	expectedDiffPath := string(twoLevelRadix + TEST_GOLD_LEFT + "-" +
		TEST_GOLD_RIGHT + DOT_EXT)
	actualDiffPath := mapper.DiffPath(TEST_GOLD_LEFT, TEST_GOLD_RIGHT)
	assert.Equal(t, expectedDiffPath, actualDiffPath)

	// Test ImagePaths
	expectedLocalPath := string(twoLevelRadix + TEST_GOLD_LEFT + DOT_EXT)
	expectedGSPath := string(TEST_GOLD_LEFT + DOT_EXT)
	localPath, bucket, gsPath := mapper.ImagePaths(TEST_GOLD_LEFT)
	assert.Equal(t, expectedLocalPath, localPath)
	assert.Equal(t, "", bucket)
	assert.Equal(t, expectedGSPath, gsPath)

	// Test IsValidDiffImgID
	// Trim the two level radix path and image extension first
	expectedDiffImgID := expectedDiffPath[len(twoLevelRadix) : len(expectedDiffPath)-len(DOT_EXT)]
	assert.True(t, mapper.IsValidDiffImgID(expectedDiffImgID))

	// Test IsValidImgID
	assert.True(t, mapper.IsValidImgID(string(TEST_GOLD_LEFT)))
	assert.True(t, mapper.IsValidImgID(string(TEST_GOLD_RIGHT)))
}

func TestCodec(t *testing.T) {
	unittest.MediumTest(t)

	w, cleanup := testutils.TempDir(t)
	defer cleanup()
	baseDir := path.Join(w, TEST_DATA_BASE_DIR+"-codec")
	client, _ := getSetupAndTile(t, baseDir)

	// Instantiate a new MemDiffStore with a codec for the test struct defined above.
	mapper := NewGoldDiffStoreMapper(&DummyDiffMetrics{})
	diffStore, err := NewMemDiffStore(client, baseDir, []string{TEST_GCS_BUCKET_NAME}, TEST_GCS_IMAGE_DIR, 10, mapper)
	assert.NoError(t, err)
	memDiffStore := diffStore.(*MemDiffStore)

	diffID := mapper.DiffID(TEST_GOLD_LEFT, TEST_GOLD_RIGHT)
	diffMetrics := &DummyDiffMetrics{
		NumDiffPixels:     100,
		PercentDiffPixels: 0.5,
	}
	err = memDiffStore.metricsStore.saveDiffMetrics(diffID, diffMetrics)
	assert.NoError(t, err)

	// Verify the returned diff metrics object has the same type and same contents
	// as the object that was saved to the metricsStore.
	metrics, err := memDiffStore.metricsStore.loadDiffMetrics(diffID)
	assert.NoError(t, err)
	assert.Equal(t, diffMetrics, metrics)
}
