package diffstore

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

const (
	// Test digests for GoldDiffStoreMapper.
	TEST_GOLD_LEFT  = "098f6bcd4621d373cade4e832627b4f6"
	TEST_GOLD_RIGHT = "1660f0783f4076284bc18c5f4bdc9608"

	// PNG extension.
	DOT_EXT = ".png"
)

func TestGoldDiffStoreMapper(t *testing.T) {
	testutils.SmallTest(t)

	mapper := GoldDiffStoreMapper{}

	// Test DiffID and SplitDiffID
	expectedDiffID := TEST_GOLD_LEFT + DIFF_IMG_SEPARATOR + TEST_GOLD_RIGHT
	actualDiffID := mapper.DiffID(TEST_GOLD_LEFT, TEST_GOLD_RIGHT)
	actualLeft, actualRight := mapper.SplitDiffID(expectedDiffID)
	assert.Equal(t, expectedDiffID, actualDiffID)
	assert.Equal(t, TEST_GOLD_LEFT, actualLeft)
	assert.Equal(t, TEST_GOLD_RIGHT, actualRight)

	// Test DiffPath
	twoLevelRadix := TEST_GOLD_LEFT[0:2] + "/" + TEST_GOLD_LEFT[2:4] + "/"
	expectedDiffPath := twoLevelRadix + TEST_GOLD_LEFT + "-" +
		TEST_GOLD_RIGHT + DOT_EXT
	actualDiffPath := mapper.DiffPath(TEST_GOLD_LEFT, TEST_GOLD_RIGHT)
	assert.Equal(t, expectedDiffPath, actualDiffPath)

	// Test ImagePaths
	expectedLocalPath := twoLevelRadix + TEST_GOLD_LEFT + DOT_EXT
	expectedGSPath := TEST_GOLD_LEFT + DOT_EXT
	localPath, bucket, gsPath := mapper.ImagePaths(TEST_GOLD_LEFT)
	assert.Equal(t, expectedLocalPath, localPath)
	assert.Equal(t, "", bucket)
	assert.Equal(t, expectedGSPath, gsPath)

	// Test IsValidDiffImgID
	// Trim the two level radix path and image extension first
	expectedDiffImgID := expectedDiffPath[len(twoLevelRadix) : len(expectedDiffPath)-len(DOT_EXT)]
	assert.True(t, mapper.IsValidDiffImgID(expectedDiffImgID))

	// Test IsValidImgID
	assert.True(t, mapper.IsValidImgID(TEST_GOLD_LEFT))
	assert.True(t, mapper.IsValidImgID(TEST_GOLD_RIGHT))
}

func TestGCSSupport(t *testing.T) {
	testutils.SmallTest(t)

	mapper := GoldDiffStoreMapper{}

	// Test for GCS paths.
	gcsImgID1 := GCSPathToImageID(TEST_GCS_SECONDARY_BUCKET, TEST_PATH_IMG_1)
	diffID := mapper.DiffID(gcsImgID1, TEST_GOLD_LEFT)
	assert.Equal(t, diffID, mapper.DiffID(TEST_GOLD_LEFT, gcsImgID1))

	id1, id2 := mapper.SplitDiffID(diffID)
	if id1 > id2 {
		id1, id2 = id2, id1
	}
	assert.Equal(t, id1, TEST_GOLD_LEFT)
	assert.Equal(t, id2, gcsImgID1)

	diffPath := mapper.DiffPath(TEST_GOLD_LEFT, gcsImgID1)
	exp := "gs/skia-infra-testdata/gold-testdata/filediffstore-testdata/" + diffID + "." + IMG_EXTENSION
	assert.Equal(t, exp, diffPath)

	localPath, gsBucket, gsPath := mapper.ImagePaths(gcsImgID1)
	exp = GS_PREFIX + "/" + TEST_GCS_SECONDARY_BUCKET + "/" + TEST_PATH_IMG_1
	assert.Equal(t, exp, localPath)
	assert.Equal(t, gsBucket, TEST_GCS_SECONDARY_BUCKET)
	assert.Equal(t, gsPath, TEST_PATH_IMG_1)

	assert.True(t, mapper.IsValidDiffImgID(diffID))
	assert.True(t, mapper.IsValidImgID(gcsImgID1))
}

func TestCodec(t *testing.T) {
	testutils.MediumTest(t)

	baseDir := TEST_DATA_BASE_DIR + "-codec"
	client, _ := getSetupAndTile(t, baseDir)
	defer testutils.RemoveAll(t, baseDir)

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

func TestBase62Encoding(t *testing.T) {
	testutils.SmallTest(t)

	randStr := "2Sde/4p8A/ziA/CI+WIzO/myfTvoyOHu0h5P2s8Hw0V8VE5VCPajRsFb/JzXIpRlG2FJ+:Mm5YJeN9V/"
	for n := 0; n <= len(randStr); n++ {
		rStr := randStr[:n]
		encodedStr := encodeBase62(rStr)
		found := decodeBase62(encodedStr)
		assert.Equal(t, rStr, found)
	}
}

func TestGCSImageIDs(t *testing.T) {
	testutils.SmallTest(t)

	imgID1 := GCSPathToImageID(TEST_GCS_SECONDARY_BUCKET, TEST_PATH_IMG_1)
	bucket, path := ImageIDToGCSPath(imgID1)
	assert.Equal(t, TEST_GCS_SECONDARY_BUCKET, bucket)
	assert.Equal(t, TEST_PATH_IMG_1, path)
	assert.True(t, ValidGCSImageID(imgID1))
	assert.False(t, ValidGCSImageID(GCSPathToImageID("", "some/path/img.png")))
	assert.False(t, ValidGCSImageID(GCSPathToImageID("some_bucket", "")))
	decImgID1 := decodeBase62(imgID1[len(GS_PREFIX):])
	assert.Equal(t, TEST_GCS_SECONDARY_BUCKET+"/"+TEST_PATH_IMG_1, decImgID1+"."+IMG_EXTENSION)
}
