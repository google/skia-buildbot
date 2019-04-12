package goldclient

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/gold-client/go/mocks"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/types"

	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
)

// test data processing of the known hashes input.
func TestLoadKnownHashes(t *testing.T) {
	testutils.SmallTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	auth, httpClient, uploader := makeMocks()
	defer auth.AssertExpectations(t)
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse([]byte(mockHashesTxt), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	expectations := httpResponse([]byte("{}"), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/abcd1234?issue=867").Return(expectations, nil)

	goldClient, err := makeCloudClient(auth, false, wd)
	assert.NoError(t, err)
	// Check that the baseline was loaded correctly
	baseline := goldClient.resultState.Expectations
	assert.Empty(t, baseline, "No expectations loaded")

	knownHashes := goldClient.resultState.KnownHashes
	assert.Len(t, knownHashes, 4, "4 hashes loaded")
	// spot check
	assert.Contains(t, knownHashes, "a9e1481ebc45c1c4f6720d1119644c20")
	assert.NotContains(t, knownHashes, "notInThere")
}

// Test data processing of the baseline input
func TestLoadBaseline(t *testing.T) {
	testutils.SmallTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	auth, httpClient, uploader := makeMocks()
	defer auth.AssertExpectations(t)
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse([]byte("none"), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	expectations := httpResponse([]byte(mockBaselineJSON), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/abcd1234?issue=867").Return(expectations, nil)

	goldClient, err := makeCloudClient(auth, false, wd)
	assert.NoError(t, err)
	// Check that the baseline was loaded correctly
	baseline := goldClient.resultState.Expectations
	assert.Len(t, baseline, 1, "only one test")
	digests := baseline["ThisIsTheOnlyTest"]
	assert.Len(t, digests, 2, "two previously seen images")
	assert.Equal(t, types.NEGATIVE, digests["badbadbad1325855590527db196112e0"])
	assert.Equal(t, types.POSITIVE, digests["beef00d3a1527db19619ec12a4e0df68"])

	knownHashes := goldClient.resultState.KnownHashes
	assert.Empty(t, knownHashes, "No hashes loaded")
}

// Report an image that does not match any previous digests.
func TestNewReportNormal(t *testing.T) {
	testutils.SmallTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer fmt.Printf("checkout %s %p\n", wd, cleanup)

	imgData := []byte("some bytes")
	imgHash := "9d0568469d206c1aedf1b71f12f474bc"

	auth, httpClient, uploader := makeMocks()
	defer auth.AssertExpectations(t)
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse([]byte("none"), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	expectations := httpResponse([]byte("{}"), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/abcd1234?issue=867").Return(expectations, nil)

	expectedUploadPath := "gs://skia-gold-testing/dm-images-v1/" + imgHash + ".png"
	uploader.On("UploadBytes", imgData, testImgPath, expectedUploadPath).Return(nil)

	// Notice the JSON is not uploaded if we are not in passfail mode - a client
	// would need to call finalize first.
	goldClient, err := makeCloudClient(auth, false /*=passFail*/, wd)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, string, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	pass, err := goldClient.Test("first-test", testImgPath)
	assert.NoError(t, err)
	// true is always returned if we are not on passFail mode.
	assert.True(t, pass)
}

// Report an image that does not match any previous digests.
func TestFinalizeNormal(t *testing.T) {
	// This test reads and writes a small amount of data from/to disk
	testutils.MediumTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	auth, httpClient, uploader := makeMocks()
	defer auth.AssertExpectations(t)
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	j := resultState{
		PerTestPassFail: false,
		InstanceID:      "testing",
		GoldURL:         "https://testing-gold.skia.org",
		Bucket:          "skia-gold-testing",
		GoldResults: &jsonio.GoldResults{
			GitHash: "cadbed23562",
			Key: map[string]string{
				"os":  "TestOS",
				"cpu": "z80",
			},
			Results: []*jsonio.Result{
				{
					Key: map[string]string{
						"name":        "first-test",
						"source_type": "default",
					},
					Options: map[string]string{
						"ext": "png",
					},
					Digest: "9d0568469d206c1aedf1b71f12f474bc",
				},
			},
		},
	}
	fmt.Printf("results %#v\n", j)

	hashesResp := httpResponse([]byte("none"), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	expectations := httpResponse([]byte("{}"), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/abcd1234?issue=867").Return(expectations, nil)

	// Notice the JSON is not uploaded if we are not in passfail mode - a client
	// would need to call finalize first.
	goldClient, err := makeCloudClient(auth, false /*=passFail*/, wd)
	assert.NoError(t, err)

	err = goldClient.Finalize()
	assert.NoError(t, err)

}

func TestNewReportPassFail(t *testing.T) {
	testutils.SmallTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	imgData := []byte("some bytes")
	imgHash := "9d0568469d206c1aedf1b71f12f474bc"
	testName := "TestNotSeenBefore"

	auth, httpClient, uploader := makeMocks()
	defer auth.AssertExpectations(t)
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse([]byte("none"), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	expectations := httpResponse([]byte("{}"), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/abcd1234?issue=867").Return(expectations, nil)

	expectedUploadPath := "gs://skia-gold-testing/dm-images-v1/" + imgHash + ".png"
	uploader.On("UploadBytes", imgData, testImgPath, expectedUploadPath).Return(nil)

	expectedJSONPath := "skia-gold-testing/trybot/dm-json-v1/2019/04/02/19/abcd1234/117/1554234843/dm-1554234843000000000.json"
	c := uploader.On("UploadJSON", mock.AnythingOfType("*jsonio.GoldResults"), filepath.Join(wd, jsonTempFile), expectedJSONPath)
	c.Run(func(args mock.Arguments) {
		json := args.Get(0).(*jsonio.GoldResults)
		// spot check some of the properties
		assert.Equal(t, "abcd1234", json.GitHash)
		assert.Equal(t, testBuildBucketID, json.BuildBucketID)
		assert.Equal(t, "WinTest", json.Key["os"])

		results := json.Results
		assert.Len(t, results, 1)
		r := results[0]
		assert.Equal(t, imgHash, r.Digest)
		assert.Equal(t, testName, r.Key["name"])
		// Since we did not specify a source_type it defaults to the instance name, which is
		// "testing"
		assert.Equal(t, "testing", r.Key["source_type"])

		assert.Equal(t, "png", r.Options["ext"])
	}).Return(nil)

	goldClient, err := makeCloudClient(auth, true /*=passFail*/, wd)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, string, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	pass, err := goldClient.Test(testName, testImgPath)
	assert.NoError(t, err)
	// Returns false because the test name has never been seen before
	// (and the digest is brand new)
	assert.False(t, pass)
}

func TestNegativePassFail(t *testing.T) {
	testutils.SmallTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	imgData := []byte("some bytes")
	// These are defined in mockBaselineJSON
	imgHash := "badbadbad1325855590527db196112e0"
	testName := "ThisIsTheOnlyTest"

	auth, httpClient, uploader := makeMocks()
	defer auth.AssertExpectations(t)
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse([]byte(imgHash), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	expectations := httpResponse([]byte(mockBaselineJSON), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/abcd1234?issue=867").Return(expectations, nil)

	// No upload expected because the bytes were already seen in json/hashes.

	expectedJSONPath := "skia-gold-testing/trybot/dm-json-v1/2019/04/02/19/abcd1234/117/1554234843/dm-1554234843000000000.json"
	uploader.On("UploadJSON", mock.AnythingOfType("*jsonio.GoldResults"), filepath.Join(wd, jsonTempFile), expectedJSONPath).Return(nil)

	goldClient, err := makeCloudClient(auth, true /*=passFail*/, wd)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, string, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	pass, err := goldClient.Test(testName, testImgPath)
	assert.NoError(t, err)
	// Returns false because the test is negative
	assert.False(t, pass)
}

func TestPositivePassFail(t *testing.T) {
	testutils.SmallTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	imgData := []byte("some bytes")
	// These are defined in mockBaselineJSON
	imgHash := "beef00d3a1527db19619ec12a4e0df68"
	testName := "ThisIsTheOnlyTest"

	auth, httpClient, uploader := makeMocks()
	defer auth.AssertExpectations(t)
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse([]byte(imgHash), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	expectations := httpResponse([]byte(mockBaselineJSON), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/abcd1234?issue=867").Return(expectations, nil)

	// No upload expected because the bytes were already seen in json/hashes.

	expectedJSONPath := "skia-gold-testing/trybot/dm-json-v1/2019/04/02/19/abcd1234/117/1554234843/dm-1554234843000000000.json"
	uploader.On("UploadJSON", mock.AnythingOfType("*jsonio.GoldResults"), filepath.Join(wd, jsonTempFile), expectedJSONPath).Return(nil)

	goldClient, err := makeCloudClient(auth, true /*=passFail*/, wd)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, string, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	pass, err := goldClient.Test(testName, testImgPath)
	assert.NoError(t, err)
	// Returns true because this test has been seen before and the digest was
	// previously triaged positive.
	assert.True(t, pass)
}

func TestInitGSUtil(t *testing.T) {
	// writes to disk (but not a lot)
	testutils.MediumTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()
	err := InitGSUtil(wd)
	assert.NoError(t, err)
	auth, err := LoadAuthOpt(wd)
	assert.NoError(t, err)

	assert.True(t, auth.GSUtil)
	assert.NoError(t, auth.Validate())
	// ensure the file exists
	assert.True(t, fileutil.FileExists(filepath.Join(wd, authFile)))

	auth, err = LoadAuthOpt(wd)
	// should still be GSUtil
	assert.NoError(t, err)
	assert.True(t, auth.GSUtil)
	assert.NoError(t, auth.Validate())
}

func TestInitLUCIAuth(t *testing.T) {
	// writes to disk (but not a lot)
	testutils.MediumTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()
	err := InitLUCIAuth(wd)
	assert.NoError(t, err)
	auth, err := LoadAuthOpt(wd)
	assert.NoError(t, err)

	assert.True(t, auth.Luci)
	assert.NoError(t, auth.Validate())
	// ensure the file exists
	assert.True(t, fileutil.FileExists(filepath.Join(wd, authFile)))

	auth, err = LoadAuthOpt(wd)
	// should still be Luci
	assert.NoError(t, err)
	assert.True(t, auth.Luci)
	assert.NoError(t, auth.Validate())
}

func TestInitServiceAccountAuth(t *testing.T) {
	// writes to disk (but not a lot)
	testutils.MediumTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()
	serviceLoc := "/non/existing/file.json"
	err := InitServiceAccountAuth(serviceLoc, wd)
	assert.NoError(t, err)
	auth, err := LoadAuthOpt(wd)
	assert.NoError(t, err)

	assert.Equal(t, serviceLoc, auth.ServiceAccount)
	assert.NoError(t, auth.Validate())
	// ensure the file exists
	assert.True(t, fileutil.FileExists(filepath.Join(wd, authFile)))

	auth, err = LoadAuthOpt(wd)
	// should still be ServiceAccount
	assert.NoError(t, err)
	assert.Equal(t, serviceLoc, auth.ServiceAccount)
	assert.NoError(t, auth.Validate())
}

func makeMocks() (*MockAuthOpt, *mocks.HTTPClient, *mocks.GoldUploader) {
	mh := mocks.HTTPClient{}
	mg := mocks.GoldUploader{}
	ma := MockAuthOpt{}
	ma.On("Validate").Return(nil).Maybe()
	ma.On("GetHTTPClient").Return(&mh, nil).Maybe()
	ma.On("GetGoldUploader").Return(&mg, nil).Maybe()
	return &ma, &mh, &mg
}

func makeCloudClient(auth AuthOpt, passFail bool, workDir string) (*cloudClient, error) {
	config := &GoldClientConfig{
		InstanceID:   testInstanceID,
		WorkDir:      workDir,
		PassFailStep: passFail,
	}

	gr := &jsonio.GoldResults{
		GitHash: "abcd1234",
		Key: map[string]string{
			"os":  "WinTest",
			"gpu": "GPUTest",
		},
		Issue:         testIssueID,
		Patchset:      testPatchsetID,
		BuildBucketID: testBuildBucketID,
	}
	c, err := NewCloudClient(auth, config, gr)
	if err != nil {
		return nil, err
	}
	c.now = func() time.Time {
		return time.Date(2019, time.April, 2, 19, 54, 3, 0, time.UTC)
	}
	return c, nil
}

func overrideLoadAndHashImage(c *cloudClient, testFn func(path string) ([]byte, string, error)) {
	c.loadAndHashImage = testFn
}

func httpResponse(body []byte, status string, statusCode int) *http.Response {
	return &http.Response{
		Body:       &respBodyCloser{bytes.NewReader(body)},
		Status:     status,
		StatusCode: statusCode,
	}
}

// respBodyCloser is a wrapper which lets us pretend to implement io.ReadCloser
// by wrapping a bytes.Reader.
type respBodyCloser struct {
	io.Reader
}

// Close is a stub method which lets us pretend to implement io.ReadCloser.
func (r respBodyCloser) Close() error {
	return nil
}

const (
	testInstanceID    = "testing"
	testIssueID       = int64(867)
	testPatchsetID    = int64(5309)
	testBuildBucketID = int64(117)
	testImgPath       = "/path/to/images/fake.png"
)

// An example baseline that has a single test at a single commit with a good
// image and a bad image.
const mockBaselineJSON = `
{
  "startCommit": {
    "commit_time": 1554696093,
    "hash": "abcd1234",
    "author": "Example User (user@example.com)"
  },
  "endCommit": {
    "commit_time": 1554696093,
    "hash": "abcd1234",
    "author": "Example User (user@example.com)"
  },
  "commitDelta": 0,
  "total": 3,
  "filled": 1,
  "md5": "7e4081337b3258555906970002a04a59",
  "master": {
    "ThisIsTheOnlyTest": {
      "beef00d3a1527db19619ec12a4e0df68": 1,
      "badbadbad1325855590527db196112e0": 2
    }
  },
  "Issue": 0
}`

const mockHashesTxt = `a9e1481ebc45c1c4f6720d1119644c20
c156c5e4b634a3b8cc96e16055197f8b
4a434407218e198faf2054645fe0ff73
303a5fd488361214f246004530e24273`
