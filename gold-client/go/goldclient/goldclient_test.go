package goldclient

import (
	"bytes"
	"io"
	"net/http"
	"path/filepath"
	"testing"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/gold-client/go/mocks"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/types"

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
func TestNewReportNoPassFail(t *testing.T) {
	testutils.SmallTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

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

	goldClient, err := makeCloudClient(auth, false, wd)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, string, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	pass, err := goldClient.Test("first-test", testImgPath)
	assert.NoError(t, err)
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
		PassFailStep: false,
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
	return NewCloudClient(auth, config, gr)
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
