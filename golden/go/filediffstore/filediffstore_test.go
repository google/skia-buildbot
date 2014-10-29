package filediffstore

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"skia.googlesource.com/buildbot.git/go/util"
	"skia.googlesource.com/buildbot.git/golden/go/diff"
)

import (
	"github.com/stretchr/testify/assert"
)

const (
	TESTDATA_DIR   = "testdata"
	TEST_DIGEST1   = "11069776588985027208"
	TEST_DIGEST2   = "5024150605949408692"
	TEST_DIGEST3   = "10552995703607727960"
	MISSING_DIGEST = "abc"
)

var (
	// DiffMetrics between TEST_DIGEST1 and TEST_DIGEST2.
	expectedDiffMetrics1_2 = &diff.DiffMetrics{
		NumDiffPixels:     2233,
		PixelDiffPercent:  0.8932,
		PixelDiffFilePath: filepath.Join(os.TempDir(), fmt.Sprintf("%s-%s.%s", TEST_DIGEST1, TEST_DIGEST2, DIFF_EXTENSION)),
		MaxRGBDiffs:       []int{0, 0, 1}}
	// DiffMetrics between TEST_DIGEST1 and TEST_DIGEST3.
	expectedDiffMetrics1_3 = &diff.DiffMetrics{
		NumDiffPixels:     250000,
		PixelDiffPercent:  100,
		PixelDiffFilePath: filepath.Join(os.TempDir(), fmt.Sprintf("%s-%s.%s", TEST_DIGEST3, TEST_DIGEST1, DIFF_EXTENSION)),
		MaxRGBDiffs:       []int{248, 91, 132}}
)

func getTestFileDiffStore(localImgDir, localDiffMetricsDir string) *FileDiffStore {
	Init()
	client := util.NewTimeoutClient()
	return &FileDiffStore{
		client:              client,
		localImgDir:         localImgDir,
		localDiffDir:        os.TempDir(),
		localDiffMetricsDir: localDiffMetricsDir,
		gsBucketName:        "chromium-skia-gm",
		storageBaseDir:      "testdata",
		lock:                sync.Mutex{},
	}
}

func TestNewFileDiffStore(t *testing.T) {
	// This test merely ensures that the NewFileDiffStore constructor codepath
	// is exercised.
	NewFileDiffStore(nil, TESTDATA_DIR, "chromium-skia-gm")
}

func TestFindDigestFromDir(t *testing.T) {
	digestsToExpectedResults := map[string]bool{
		TEST_DIGEST1:   true,
		TEST_DIGEST2:   true,
		MISSING_DIGEST: false,
	}
	fds := getTestFileDiffStore(filepath.Join(TESTDATA_DIR, "images"), filepath.Join(TESTDATA_DIR, "diffs"))

	for digest, expectedValue := range digestsToExpectedResults {
		ret, err := fds.isDigestInCache(digest)
		if err != nil {
			t.Error("Unexpected error: ", err)
		}
		assert.Equal(t, expectedValue, ret)
	}
}

func TestGetDiffMetricFromDir(t *testing.T) {
	digestsToExpectedResults := map[[2]string]*diff.DiffMetrics{
		[2]string{TEST_DIGEST1, TEST_DIGEST2}:   expectedDiffMetrics1_2,
		[2]string{TEST_DIGEST2, TEST_DIGEST1}:   expectedDiffMetrics1_2,
		[2]string{MISSING_DIGEST, TEST_DIGEST2}: nil,
		[2]string{TEST_DIGEST1, MISSING_DIGEST}: nil,
	}
	fds := getTestFileDiffStore(filepath.Join(TESTDATA_DIR, "images"), filepath.Join(TESTDATA_DIR, "diffmetrics"))

	for digests, expectedValue := range digestsToExpectedResults {
		ret, err := fds.getDiffMetricsFromCache(digests[0], digests[1])
		if err != nil {
			t.Error("Unexpected error: ", err)
		}
		assert.Equal(t, expectedValue, ret)
	}
}

func TestOpenDiffMetrics(t *testing.T) {

	diffMetrics, err := openDiffMetrics(
		filepath.Join("testdata", "diffmetrics",
			fmt.Sprintf("%s-%s.%s", TEST_DIGEST1, TEST_DIGEST2, DIFFMETRICS_EXTENSION)))
	if err != nil {
		t.Error("Unexpected error: ", err)
	}

	assert.Equal(t, expectedDiffMetrics1_2, diffMetrics)
}

func TestCacheImageFromGS(t *testing.T) {
	imgFilePath := filepath.Join(os.TempDir(), fmt.Sprintf("%s.%s", TEST_DIGEST3, IMG_EXTENSION))
	defer os.Remove(imgFilePath)

	fds := getTestFileDiffStore(os.TempDir(), filepath.Join(TESTDATA_DIR, "diffmetrics"))
	err := fds.cacheImageFromGS(TEST_DIGEST3)
	if err != nil {
		t.Error("Unexpected error: ", err)
	}
	if _, err := os.Stat(imgFilePath); err != nil {
		t.Errorf("File %s was not created!", imgFilePath)
	}
	assert.Equal(t, 1, downloadSuccessCount.Count())

	// Test error and assert the download failures map.
	for i := 1; i < 6; i++ {
		if err := fds.cacheImageFromGS(MISSING_DIGEST); err == nil {
			t.Error("Was expecting 404 error for missing digest")
		}
		assert.Equal(t, 1, downloadSuccessCount.Count())
		assert.Equal(t, i, downloadFailureCount.Count())
	}
}

func TestDiff(t *testing.T) {
	fds := getTestFileDiffStore(filepath.Join(TESTDATA_DIR, "images"), os.TempDir())
	diffFilePath := filepath.Join(os.TempDir(), fmt.Sprintf("%s-%s.%s", TEST_DIGEST1, TEST_DIGEST2, DIFF_EXTENSION))
	defer os.Remove(diffFilePath)
	diffMetrics, err := fds.diff(TEST_DIGEST1, TEST_DIGEST2)
	if err != nil {
		t.Error("Unexpected error: ", err)
	}
	// Assert that the diff file was created.
	if _, err := os.Stat(diffFilePath); err != nil {
		t.Errorf("Diff file %s was not created!", diffFilePath)
	}
	// Assert that the DiffMetrics are as expected.
	assert.Equal(t, expectedDiffMetrics1_2, diffMetrics)
}

func assertFileExists(filePath string, t *testing.T) {
	if _, err := os.Stat(filePath); err != nil {
		t.Errorf("File %s does not exist!", filePath)
	}
}

func TestAbsPath(t *testing.T) {
	imagesDir := filepath.Join(TESTDATA_DIR, "images")
	fds := getTestFileDiffStore(imagesDir, filepath.Join(TESTDATA_DIR, "diffmetrics"))
	paths, err := fds.AbsPath([]string{TEST_DIGEST1, TEST_DIGEST2})
	if err != nil {
		t.Error("Unexpected error: ", err)
	}
	assert.Equal(t, 2, len(paths))
	assert.Equal(t, filepath.Join(imagesDir, fmt.Sprintf("%s.%s", TEST_DIGEST1, IMG_EXTENSION)), paths[0])
	assert.Equal(t, filepath.Join(imagesDir, fmt.Sprintf("%s.%s", TEST_DIGEST2, IMG_EXTENSION)), paths[1])
}

func TestGet_e2e(t *testing.T) {
	// 2 files that exist locally, diffmetrics exists locally as well.
	fds1 := getTestFileDiffStore(filepath.Join(TESTDATA_DIR, "images"), filepath.Join(TESTDATA_DIR, "diffmetrics"))
	diffMetricsSlice1, err := fds1.Get(TEST_DIGEST1, []string{TEST_DIGEST2})
	if err != nil {
		t.Error("Unexpected error: ", err)
	}
	assert.Equal(t, 1, len(diffMetricsSlice1))
	assert.Equal(t, expectedDiffMetrics1_2, diffMetricsSlice1[0])
	assert.Equal(t, 0, downloadSuccessCount.Count())
	assert.Equal(t, 0, downloadFailureCount.Count())

	// 2 files that exist locally but diffmetrics does not exist.
	diffBasename := fmt.Sprintf("%s-%s", TEST_DIGEST1, TEST_DIGEST2)
	diffFilePath := filepath.Join(os.TempDir(), fmt.Sprintf("%s.%s", diffBasename, DIFF_EXTENSION))
	diffMetricsFilePath := filepath.Join(os.TempDir(), fmt.Sprintf("%s.%s", diffBasename, DIFFMETRICS_EXTENSION))
	defer os.Remove(diffFilePath)
	defer os.Remove(diffMetricsFilePath)
	fds2 := getTestFileDiffStore(filepath.Join(TESTDATA_DIR, "images"), os.TempDir())
	diffMetricsSlice2, err := fds2.Get(TEST_DIGEST1, []string{TEST_DIGEST2})
	if err != nil {
		t.Error("Unexpected error: ", err)
	}
	// Verify that the diff and the diffmetrics files were created.
	assertFileExists(diffFilePath, t)
	assertFileExists(diffMetricsFilePath, t)
	assert.Equal(t, 1, len(diffMetricsSlice2))
	assert.Equal(t, expectedDiffMetrics1_2, diffMetricsSlice2[0])
	assert.Equal(t, 0, downloadSuccessCount.Count())
	assert.Equal(t, 0, downloadFailureCount.Count())

	// 1 file that exists locally and 1 file that exists in Google Storage.
	newImageFilePath := filepath.Join(TESTDATA_DIR, "images", fmt.Sprintf("%s.%s", TEST_DIGEST3, IMG_EXTENSION))
	diffBasename = fmt.Sprintf("%s-%s", TEST_DIGEST3, TEST_DIGEST1)
	diffFilePath = filepath.Join(os.TempDir(), fmt.Sprintf("%s.%s", diffBasename, DIFF_EXTENSION))
	diffMetricsFilePath = filepath.Join(os.TempDir(), fmt.Sprintf("%s.%s", diffBasename, DIFFMETRICS_EXTENSION))
	defer os.Remove(newImageFilePath)
	defer os.Remove(diffFilePath)
	defer os.Remove(diffMetricsFilePath)
	fds3 := getTestFileDiffStore(filepath.Join(TESTDATA_DIR, "images"), os.TempDir())
	diffMetricsSlice3, err := fds3.Get(TEST_DIGEST1, []string{TEST_DIGEST3})
	if err != nil {
		t.Error("Unexpected error: ", err)
	}
	// Verify that the image was downloaded successfully from Google Storage and
	// that the diff and diffmetrics files were created.
	assertFileExists(newImageFilePath, t)
	assertFileExists(diffFilePath, t)
	assertFileExists(diffMetricsFilePath, t)
	assert.Equal(t, 1, len(diffMetricsSlice3))
	assert.Equal(t, expectedDiffMetrics1_3, diffMetricsSlice3[0])
	assert.Equal(t, 1, downloadSuccessCount.Count())
	assert.Equal(t, 0, downloadFailureCount.Count())

	// 1 file that does not exist.
	fds4 := getTestFileDiffStore(filepath.Join(TESTDATA_DIR, "images"), os.TempDir())
	if _, err := fds4.Get(TEST_DIGEST1, []string{TEST_DIGEST2, TEST_DIGEST3, MISSING_DIGEST}); err == nil {
		t.Error("Was expecting 404 error for missing digest")
	}
	assert.Equal(t, 0, downloadSuccessCount.Count())
	assert.Equal(t, 1, downloadFailureCount.Count())

	// Call Get with multiple digests.
	newImageFilePath = filepath.Join(TESTDATA_DIR, "images", fmt.Sprintf("%s.%s", TEST_DIGEST3, IMG_EXTENSION))
	diffBasename = fmt.Sprintf("%s-%s", TEST_DIGEST3, TEST_DIGEST1)
	diffFilePath = filepath.Join(os.TempDir(), fmt.Sprintf("%s.%s", diffBasename, DIFF_EXTENSION))
	diffMetricsFilePath = filepath.Join(os.TempDir(), fmt.Sprintf("%s.%s", diffBasename, DIFFMETRICS_EXTENSION))
	defer os.Remove(newImageFilePath)
	defer os.Remove(diffFilePath)
	defer os.Remove(diffMetricsFilePath)
	fds5 := getTestFileDiffStore(filepath.Join(TESTDATA_DIR, "images"), os.TempDir())
	diffMetricsSlice5, err := fds5.Get(TEST_DIGEST1, []string{TEST_DIGEST2, TEST_DIGEST3})
	if err != nil {
		t.Error("Unexpected error: ", err)
	}
	// Verify that the image was downloaded successfully from Google Storage and
	// that the diff and diffmetrics files were created.
	assertFileExists(newImageFilePath, t)
	assertFileExists(diffFilePath, t)
	assertFileExists(diffMetricsFilePath, t)
	assert.Equal(t, 2, len(diffMetricsSlice5))
	assert.Equal(t, expectedDiffMetrics1_2, diffMetricsSlice5[0])
	assert.Equal(t, expectedDiffMetrics1_3, diffMetricsSlice5[1])
	assert.Equal(t, 0, downloadFailureCount.Count())
}
