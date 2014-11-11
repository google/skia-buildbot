package filediffstore

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"skia.googlesource.com/buildbot.git/go/util"
	"skia.googlesource.com/buildbot.git/golden/go/diff"
)

import (
	assert "github.com/stretchr/testify/require"
)

const (
	TESTDATA_DIR         = "testdata"
	MASSIVE_TESTDATA_DIR = "concur-testdata"
	TEST_DIGEST1         = "11069776588985027208"
	TEST_DIGEST2         = "5024150605949408692"
	TEST_DIGEST3         = "10552995703607727960"
	MISSING_DIGEST       = "abc"
)

var (
	// DiffMetrics between TEST_DIGEST1 and TEST_DIGEST2.
	expectedDiffMetrics1_2 = &diff.DiffMetrics{
		NumDiffPixels:     2233,
		PixelDiffPercent:  0.8932,
		PixelDiffFilePath: filepath.Join(os.TempDir(), fmt.Sprintf("%s-%s.%s", TEST_DIGEST1, TEST_DIGEST2, DIFF_EXTENSION)),
		MaxRGBDiffs:       []int{0, 0, 1},
		DimDiffer:         false,
	}
	// DiffMetrics between TEST_DIGEST1 and TEST_DIGEST3.
	expectedDiffMetrics1_3 = &diff.DiffMetrics{
		NumDiffPixels:     250000,
		PixelDiffPercent:  100,
		PixelDiffFilePath: filepath.Join(os.TempDir(), fmt.Sprintf("%s-%s.%s", TEST_DIGEST3, TEST_DIGEST1, DIFF_EXTENSION)),
		MaxRGBDiffs:       []int{248, 90, 113},
		DimDiffer:         true,
	}
)

func getTestFileDiffStore(localImgDir, localDiffMetricsDir, storageBaseDir string) *FileDiffStore {
	Init()
	client := util.NewTimeoutClient()
	fs := &FileDiffStore{
		client:              client,
		localImgDir:         localImgDir,
		localDiffDir:        os.TempDir(),
		localDiffMetricsDir: localDiffMetricsDir,
		gsBucketName:        "chromium-skia-gm",
		storageBaseDir:      storageBaseDir,
		diffDirLock:         sync.Mutex{},
		digestDirLock:       sync.Mutex{},
	}
	fs.activateWorkers(RECOMMENDED_WORKER_POOL_SIZE)
	return fs
}

func TestNewFileDiffStore(t *testing.T) {
	// This test merely ensures that the NewFileDiffStore constructor codepath
	// is exercised.
	NewFileDiffStore(nil, TESTDATA_DIR, "chromium-skia-gm", RECOMMENDED_WORKER_POOL_SIZE)
}

func TestFindDigestFromDir(t *testing.T) {
	digestsToExpectedResults := map[string]bool{
		TEST_DIGEST1:   true,
		TEST_DIGEST2:   true,
		MISSING_DIGEST: false,
	}
	fds := getTestFileDiffStore(filepath.Join(TESTDATA_DIR, "images"), filepath.Join(TESTDATA_DIR, "diffs"), TESTDATA_DIR)

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
	fds := getTestFileDiffStore(filepath.Join(TESTDATA_DIR, "images"), filepath.Join(TESTDATA_DIR, "diffmetrics"), TESTDATA_DIR)

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

	fds := getTestFileDiffStore(os.TempDir(), filepath.Join(TESTDATA_DIR, "diffmetrics"), TESTDATA_DIR)
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
	fds := getTestFileDiffStore(filepath.Join(TESTDATA_DIR, "images"), os.TempDir(), TESTDATA_DIR)
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
	fds := getTestFileDiffStore(imagesDir, filepath.Join(TESTDATA_DIR, "diffmetrics"), TESTDATA_DIR)
	digestToPaths := fds.AbsPath([]string{TEST_DIGEST1, TEST_DIGEST2})
	assert.Equal(t, 2, len(digestToPaths))
	assert.Equal(t, filepath.Join(imagesDir, fmt.Sprintf("%s.%s", TEST_DIGEST1, IMG_EXTENSION)), digestToPaths[TEST_DIGEST1])
	assert.Equal(t, filepath.Join(imagesDir, fmt.Sprintf("%s.%s", TEST_DIGEST2, IMG_EXTENSION)), digestToPaths[TEST_DIGEST2])

	digestToPaths = fds.AbsPath([]string{})
	assert.Equal(t, 0, len(digestToPaths))
}

func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	fmt.Printf("%s took %s", name, elapsed)
}

// Remove the 'Massive' prefix to run with massive test, takes around 13
// seconds to run.
func MassiveTestGet_45Digests(t *testing.T) {
	defer timeTrack(time.Now(), "MassiveTestGet")

	workingDir := filepath.Join(os.TempDir(), MASSIVE_TESTDATA_DIR)
	defer os.RemoveAll(workingDir)
	os.Mkdir(workingDir, 0777)
	fds := getTestFileDiffStore(workingDir, workingDir, MASSIVE_TESTDATA_DIR)
	diffMetricsMap, err := fds.Get(
		"0ff8bf090c7bcfa6e1333f1b27de34a2",
		[]string{MISSING_DIGEST, "0f35601a05e4b70e571d383531d6475d", "0f38c862a94642632a7e1418ce0322dc", "0f422eb209256e4e94442b8bc7216fc4", "0f448bf24d6b1a2d59e8d61ca1864a40", "0f47abec25acbba9fcd8a9fffcc89db4", "0f4d6addbbf439d8a5d43880d06aad2b", "0f50439a1bfcea213b7cb53e64dc8c41", "0f5964ac9eeb3e830c2af590f5a5b417", "0f5ab81728a3fc617374dd01b5e9139c",
			"0f6227320e2ca014e2df9ec9d5b1ea0e", "0f654cb1f795e4f51672474d31e54df7", "0f6fd6ff6db45243f475644cc21675ac", "0f750afae368fc5094e9e2aaf93838dc", "0f77002c0d777a55aad75060a1988054", "0f7c3d2d6daea1e14e262adfd8956703", "0f802ed345aa011b3f645d935f115b5d", "0f81c4c1d4e29887cfe377090bff1e3d", "0f87072e6c003766135c40a6665ecd6e",
			"0f880aa7f6db1e50a6bc532581b52dc8", "0f915b5931e56817287fe5c355439a1a", "0f96f63917f0c62b2c9b8110ff20badc", "0f98bfd192b64eed137f9e6772683365", "0f9aa5700e3ec10bcec5ee74f357cb9d", "0fa1dad80143172942b9ebcb16a41dbf", "0fa50dc22558dc2cc39c48fb5f17f2d0", "0faacf520d0feae4dd7933eabb31d850", "0fafdb43076e5667c38ac0864af59142",
			"0fb0442568f8d9f16da8f26435bfe612", "0fba6eb3b0577c16b76ad84a1bb0f23b", "0fbcd5335eb08911873395c00840b74b", "0fbe8c55504d8a8420c4bef6a9d078f4", "0fc082cb3ca2b72869379c3c053e51c2", "0fc528ee84845f6044e516a1276caa46", "0fc587b905523f45ef287f2f9defb844", "0fcacb142d1517474b8d09b93072f2fc", "0fcbc9417b21e95b07f59495c1d8c29e",
			"0fce6e571aac26038cea582356065e34", "0fd21ebcb59b7f9fde71bc868c2bd77b", "0fdd731115695cc1b6c912ce8ab6e7e6", "0fe58f4a759d46a60198ac1853cb1d43", "0fe7a59b8a3caf68e83ae7fa4abe5052", "0fe88d578a0b1359dbced64a6063c4e9", "0ff48464b23d47af28d8c740507a1212", "0ff864fb2bab5daa74e67fced7eac536"})
	if err != nil {
		t.Error("Unexpected error: ", err)
	}
	assert.Equal(t, 45, downloadSuccessCount.Count())
	assert.Equal(t, 1, downloadFailureCount.Count())
	assert.Equal(t, 44, len(diffMetricsMap))
}

// Remove the 'Massive' prefix to run with massive test, takes around 2
// seconds to run.
func MassiveTestAbsPath_45Digests(t *testing.T) {
	defer timeTrack(time.Now(), "MassiveTestAbsPath")

	workingDir := filepath.Join(os.TempDir(), MASSIVE_TESTDATA_DIR)
	defer os.RemoveAll(workingDir)
	os.Mkdir(workingDir, 0777)
	fds := getTestFileDiffStore(workingDir, workingDir, MASSIVE_TESTDATA_DIR)
	digestsToPaths := fds.AbsPath(
		[]string{MISSING_DIGEST, "0ff8bf090c7bcfa6e1333f1b27de34a2", "0f35601a05e4b70e571d383531d6475d", "0f38c862a94642632a7e1418ce0322dc", "0f422eb209256e4e94442b8bc7216fc4", "0f448bf24d6b1a2d59e8d61ca1864a40", "0f47abec25acbba9fcd8a9fffcc89db4", "0f4d6addbbf439d8a5d43880d06aad2b", "0f50439a1bfcea213b7cb53e64dc8c41", "0f5964ac9eeb3e830c2af590f5a5b417",
			"0f5ab81728a3fc617374dd01b5e9139c", "0f6227320e2ca014e2df9ec9d5b1ea0e", "0f654cb1f795e4f51672474d31e54df7", "0f6fd6ff6db45243f475644cc21675ac", "0f750afae368fc5094e9e2aaf93838dc", "0f77002c0d777a55aad75060a1988054", "0f7c3d2d6daea1e14e262adfd8956703", "0f802ed345aa011b3f645d935f115b5d", "0f81c4c1d4e29887cfe377090bff1e3d",
			"0f87072e6c003766135c40a6665ecd6e", "0f880aa7f6db1e50a6bc532581b52dc8", "0f915b5931e56817287fe5c355439a1a", "0f96f63917f0c62b2c9b8110ff20badc", "0f98bfd192b64eed137f9e6772683365", "0f9aa5700e3ec10bcec5ee74f357cb9d", "0fa1dad80143172942b9ebcb16a41dbf", "0fa50dc22558dc2cc39c48fb5f17f2d0", "0faacf520d0feae4dd7933eabb31d850",
			"0fafdb43076e5667c38ac0864af59142", "0fb0442568f8d9f16da8f26435bfe612", "0fba6eb3b0577c16b76ad84a1bb0f23b", "0fbcd5335eb08911873395c00840b74b", "0fbe8c55504d8a8420c4bef6a9d078f4", "0fc082cb3ca2b72869379c3c053e51c2", "0fc528ee84845f6044e516a1276caa46", "0fc587b905523f45ef287f2f9defb844", "0fcacb142d1517474b8d09b93072f2fc",
			"0fcbc9417b21e95b07f59495c1d8c29e", "0fce6e571aac26038cea582356065e34", "0fd21ebcb59b7f9fde71bc868c2bd77b", "0fdd731115695cc1b6c912ce8ab6e7e6", "0fe58f4a759d46a60198ac1853cb1d43", "0fe7a59b8a3caf68e83ae7fa4abe5052", "0fe88d578a0b1359dbced64a6063c4e9", "0ff48464b23d47af28d8c740507a1212", "0ff864fb2bab5daa74e67fced7eac536"})
	assert.Equal(t, 45, downloadSuccessCount.Count())
	assert.Equal(t, 1, downloadFailureCount.Count())
	assert.Equal(t, 45, len(digestsToPaths))
}

func TestGet_e2e(t *testing.T) {
	// Empty digests to compare too.
	fdsEmpty := getTestFileDiffStore(filepath.Join(TESTDATA_DIR, "images"), filepath.Join(TESTDATA_DIR, "diffmetrics"), TESTDATA_DIR)
	diffMetricsMapEmpty, err := fdsEmpty.Get(TEST_DIGEST1, []string{})
	assert.Nil(t, err)
	assert.Equal(t, 0, len(diffMetricsMapEmpty))

	// 2 files that exist locally, diffmetrics exists locally as well.
	fds1 := getTestFileDiffStore(filepath.Join(TESTDATA_DIR, "images"), filepath.Join(TESTDATA_DIR, "diffmetrics"), TESTDATA_DIR)
	diffMetricsMap1, err := fds1.Get(TEST_DIGEST1, []string{TEST_DIGEST2})
	if err != nil {
		t.Error("Unexpected error: ", err)
	}
	assert.Equal(t, 1, len(diffMetricsMap1))
	assert.Equal(t, expectedDiffMetrics1_2, diffMetricsMap1[TEST_DIGEST2])
	assert.Equal(t, 0, downloadSuccessCount.Count())
	assert.Equal(t, 0, downloadFailureCount.Count())

	// 2 files that exist locally but diffmetrics does not exist.
	diffBasename := fmt.Sprintf("%s-%s", TEST_DIGEST1, TEST_DIGEST2)
	diffFilePath := filepath.Join(os.TempDir(), fmt.Sprintf("%s.%s", diffBasename, DIFF_EXTENSION))
	diffMetricsFilePath := filepath.Join(os.TempDir(), fmt.Sprintf("%s.%s", diffBasename, DIFFMETRICS_EXTENSION))
	defer os.Remove(diffFilePath)
	defer os.Remove(diffMetricsFilePath)
	fds2 := getTestFileDiffStore(filepath.Join(TESTDATA_DIR, "images"), os.TempDir(), TESTDATA_DIR)
	diffMetricsMap2, err := fds2.Get(TEST_DIGEST1, []string{TEST_DIGEST2})
	if err != nil {
		t.Error("Unexpected error: ", err)
	}
	// Verify that the diff and the diffmetrics files were created.
	assertFileExists(diffFilePath, t)
	assertFileExists(diffMetricsFilePath, t)
	assert.Equal(t, 1, len(diffMetricsMap2))
	assert.Equal(t, expectedDiffMetrics1_2, diffMetricsMap2[TEST_DIGEST2])
	assert.Equal(t, 0, downloadSuccessCount.Count())
	assert.Equal(t, 0, downloadFailureCount.Count())

	// 1 file that exists locally, 1 file that exists in Google Storage, 1
	// file that does not exist.
	newImageFilePath := filepath.Join(TESTDATA_DIR, "images", fmt.Sprintf("%s.%s", TEST_DIGEST3, IMG_EXTENSION))
	diffBasename = fmt.Sprintf("%s-%s", TEST_DIGEST3, TEST_DIGEST1)
	diffFilePath = filepath.Join(os.TempDir(), fmt.Sprintf("%s.%s", diffBasename, DIFF_EXTENSION))
	diffMetricsFilePath = filepath.Join(os.TempDir(), fmt.Sprintf("%s.%s", diffBasename, DIFFMETRICS_EXTENSION))
	defer os.Remove(newImageFilePath)
	defer os.Remove(diffFilePath)
	defer os.Remove(diffMetricsFilePath)
	fds3 := getTestFileDiffStore(filepath.Join(TESTDATA_DIR, "images"), os.TempDir(), TESTDATA_DIR)
	diffMetricsMap3, err := fds3.Get(TEST_DIGEST1, []string{TEST_DIGEST3, MISSING_DIGEST})
	if err != nil {
		t.Error("Unexpected error: ", err)
	}
	// Verify that the image was downloaded successfully from Google Storage and
	// that the diff and diffmetrics files were created.
	assertFileExists(newImageFilePath, t)
	assertFileExists(diffFilePath, t)
	assertFileExists(diffMetricsFilePath, t)
	assert.Equal(t, 1, len(diffMetricsMap3))
	assert.Equal(t, expectedDiffMetrics1_3, diffMetricsMap3[TEST_DIGEST3])
	assert.Equal(t, 1, downloadSuccessCount.Count())
	assert.Equal(t, 1, downloadFailureCount.Count())

	// Call Get with multiple digests.
	newImageFilePath = filepath.Join(TESTDATA_DIR, "images", fmt.Sprintf("%s.%s", TEST_DIGEST3, IMG_EXTENSION))
	diffBasename = fmt.Sprintf("%s-%s", TEST_DIGEST3, TEST_DIGEST1)
	diffFilePath = filepath.Join(os.TempDir(), fmt.Sprintf("%s.%s", diffBasename, DIFF_EXTENSION))
	diffMetricsFilePath = filepath.Join(os.TempDir(), fmt.Sprintf("%s.%s", diffBasename, DIFFMETRICS_EXTENSION))
	defer os.Remove(newImageFilePath)
	defer os.Remove(diffFilePath)
	defer os.Remove(diffMetricsFilePath)
	fds5 := getTestFileDiffStore(filepath.Join(TESTDATA_DIR, "images"), os.TempDir(), TESTDATA_DIR)
	diffMetricsMap5, err := fds5.Get(TEST_DIGEST1, []string{TEST_DIGEST2, TEST_DIGEST3, MISSING_DIGEST})
	if err != nil {
		t.Error("Unexpected error: ", err)
	}
	// Verify that the image was downloaded successfully from Google Storage and
	// that the diff and diffmetrics files were created.
	assertFileExists(newImageFilePath, t)
	assertFileExists(diffFilePath, t)
	assertFileExists(diffMetricsFilePath, t)
	assert.Equal(t, 2, len(diffMetricsMap5))
	assert.Equal(t, expectedDiffMetrics1_2, diffMetricsMap5[TEST_DIGEST2])
	assert.Equal(t, expectedDiffMetrics1_3, diffMetricsMap5[TEST_DIGEST3])
	assert.Equal(t, 1, downloadFailureCount.Count())
}

func TestReuseSameInstance(t *testing.T) {
	imagesDir := filepath.Join(TESTDATA_DIR, "images")
	fds := getTestFileDiffStore(imagesDir, filepath.Join(TESTDATA_DIR, "diffmetrics"), TESTDATA_DIR)

	// Use the instance to call Get.
	diffMetricsMap1, err := fds.Get(TEST_DIGEST1, []string{TEST_DIGEST2})
	if err != nil {
		t.Error("Unexpected error: ", err)
	}
	assert.Equal(t, 1, len(diffMetricsMap1))
	assert.Equal(t, expectedDiffMetrics1_2, diffMetricsMap1[TEST_DIGEST2])
	assert.Equal(t, 0, downloadSuccessCount.Count())
	assert.Equal(t, 0, downloadFailureCount.Count())

	// Use same instance to call AbsPath.
	digestToPaths := fds.AbsPath([]string{TEST_DIGEST1, TEST_DIGEST2})
	assert.Equal(t, 2, len(digestToPaths))
	assert.Equal(t, filepath.Join(imagesDir, fmt.Sprintf("%s.%s", TEST_DIGEST1, IMG_EXTENSION)), digestToPaths[TEST_DIGEST1])
	assert.Equal(t, filepath.Join(imagesDir, fmt.Sprintf("%s.%s", TEST_DIGEST2, IMG_EXTENSION)), digestToPaths[TEST_DIGEST2])
}
