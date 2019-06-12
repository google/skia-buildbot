package goldclient

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/gold-client/go/mocks"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/types"

	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
)

// test data processing of the known hashes input.
func TestLoadKnownHashes(t *testing.T) {
	unittest.SmallTest(t)

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

	goldClient, err := makeGoldClient(auth, false /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(testSharedConfig)
	assert.NoError(t, err)
	// Check that the baseline was loaded correctly
	baseline := goldClient.resultState.Expectations
	assert.Empty(t, baseline, "No expectations loaded")

	knownHashes := goldClient.resultState.KnownHashes
	assert.Len(t, knownHashes, 4, "4 hashes loaded")
	// spot check
	assert.Contains(t, knownHashes, types.Digest("a9e1481ebc45c1c4f6720d1119644c20"))
	assert.NotContains(t, knownHashes, "notInThere")
}

// TestLoadBaseline loads a baseline for an issue (testSharedConfig defaults to being
// an configured for a tryjob).
func TestLoadBaseline(t *testing.T) {
	unittest.SmallTest(t)

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

	goldClient, err := makeGoldClient(auth, false /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(testSharedConfig)
	assert.NoError(t, err)

	// Check that the baseline was loaded correctly
	bl := goldClient.resultState.Expectations
	assert.Len(t, bl, 1, "only one test")
	digests := bl["ThisIsTheOnlyTest"]
	assert.Len(t, digests, 2, "two previously seen images")
	assert.Equal(t, types.NEGATIVE, digests["badbadbad1325855590527db196112e0"])
	assert.Equal(t, types.POSITIVE, digests["beef00d3a1527db19619ec12a4e0df68"])

	assert.Equal(t, testIssueID, goldClient.resultState.SharedConfig.Issue)

	knownHashes := goldClient.resultState.KnownHashes
	assert.Empty(t, knownHashes, "No hashes loaded")
}

// TestLoadBaselineMaster loads the baseline for the master branch.
func TestLoadBaselineMaster(t *testing.T) {
	unittest.SmallTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	auth, httpClient, uploader := makeMocks()
	defer auth.AssertExpectations(t)
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse([]byte("none"), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	expectations := httpResponse([]byte(mockBaselineJSON), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/abcd1234").Return(expectations, nil)

	goldClient, err := makeGoldClient(auth, false /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(jsonio.GoldResults{
		GitHash: "abcd1234",
		Key: map[string]string{
			"os":  "WinTest",
			"gpu": "GPUTest",
		},
		Issue: types.MasterBranch,
	})
	assert.NoError(t, err)

	// Check that the baseline was loaded correctly
	bl := goldClient.resultState.Expectations
	assert.Len(t, bl, 1, "only one test")
	digests := bl["ThisIsTheOnlyTest"]
	assert.Len(t, digests, 2, "two previously seen images")
	assert.Equal(t, types.NEGATIVE, digests["badbadbad1325855590527db196112e0"])
	assert.Equal(t, types.POSITIVE, digests["beef00d3a1527db19619ec12a4e0df68"])

	assert.Equal(t, types.MasterBranch, goldClient.resultState.SharedConfig.Issue)

	knownHashes := goldClient.resultState.KnownHashes
	assert.Empty(t, knownHashes, "No hashes loaded")
}

// Test that the working dir has the correct JSON after initializing.
// This is effectively a test for "goldctl imgtest init"
func TestInit(t *testing.T) {
	// This test reads and writes a small amount of data from/to disk
	unittest.MediumTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	auth, httpClient, uploader := makeMocks()
	defer auth.AssertExpectations(t)
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse([]byte(mockHashesTxt), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	expectations := httpResponse([]byte(mockBaselineJSON), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/abcd1234?issue=867").Return(expectations, nil)

	// no uploader calls

	goldClient, err := makeGoldClient(auth, true /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(testSharedConfig)
	assert.NoError(t, err)

	outFile := filepath.Join(wd, stateFile)
	assert.True(t, fileutil.FileExists(outFile))

	state, err := loadStateFromJson(outFile)
	assert.NoError(t, err)
	assert.True(t, state.PerTestPassFail)
	assert.False(t, state.UploadOnly)
	assert.Equal(t, "testing", state.InstanceID)
	assert.Equal(t, "https://testing-gold.skia.org", state.GoldURL)
	assert.Equal(t, "skia-gold-testing", state.Bucket)
	assert.Len(t, state.KnownHashes, 4) // these should be saved to disk
	assert.Len(t, state.Expectations, 1)
	assert.Len(t, state.Expectations["ThisIsTheOnlyTest"], 2)
	assert.Equal(t, testSharedConfig, *state.SharedConfig)
}

// Test that the client does not fetch from the server if UploadOnly is set.
// This is effectively a test for "goldctl imgtest init --upload-only"
func TestInitUploadOnly(t *testing.T) {
	// This test reads and writes a small amount of data from/to disk
	unittest.MediumTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	auth, httpClient, uploader := makeMocks()
	defer auth.AssertExpectations(t)
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	// no calls of any kind

	config := GoldClientConfig{
		InstanceID:   "fuchsia",
		WorkDir:      wd,
		PassFailStep: false,
		UploadOnly:   true,
	}

	goldClient, err := NewCloudClient(auth, config)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(testSharedConfig)
	assert.NoError(t, err)

	outFile := filepath.Join(wd, stateFile)
	assert.True(t, fileutil.FileExists(outFile))

	state, err := loadStateFromJson(outFile)
	assert.NoError(t, err)
	assert.False(t, state.PerTestPassFail)
	assert.True(t, state.UploadOnly)
	assert.Equal(t, "fuchsia", state.InstanceID)
	assert.Equal(t, "https://fuchsia-gold.corp.goog", state.GoldURL)
	assert.Equal(t, "skia-gold-fuchsia", state.Bucket)
}

// Report an image that does not match any previous digests.
// This is effectively a test for "goldctl imgtest add"
func TestNewReportNormal(t *testing.T) {
	unittest.SmallTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	imgData := []byte("some bytes")
	imgHash := types.Digest("9d0568469d206c1aedf1b71f12f474bc")

	auth, httpClient, uploader := makeMocks()
	defer auth.AssertExpectations(t)
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse([]byte("none"), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	expectations := httpResponse([]byte("{}"), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/abcd1234?issue=867").Return(expectations, nil)

	expectedUploadPath := string("gs://skia-gold-testing/dm-images-v1/" + imgHash + ".png")
	uploader.On("UploadBytes", imgData, testImgPath, expectedUploadPath).Return(nil)

	// Notice the JSON is not uploaded if we are not in passfail mode - a client
	// would need to call finalize first.
	goldClient, err := makeGoldClient(auth, false /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(testSharedConfig)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	pass, err := goldClient.Test("first-test", testImgPath, nil)
	assert.NoError(t, err)
	// true is always returned if we are not on passFail mode.
	assert.True(t, pass)
}

// Test the uploading of JSON after two tests/images have been seen.
// This is effectively a test for "goldctl imgtest finalize"
func TestFinalizeNormal(t *testing.T) {
	// This test reads and writes a small amount of data from/to disk
	unittest.MediumTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	auth, httpClient, uploader := makeMocks()
	defer auth.AssertExpectations(t)
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	// handcrafted state that has two tests in it
	j := resultState{
		PerTestPassFail: false,
		InstanceID:      "testing",
		GoldURL:         "https://testing-gold.skia.org",
		Bucket:          "skia-gold-testing",
		SharedConfig: &jsonio.GoldResults{
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
				{
					Key: map[string]string{
						"name":         "second-test",
						"optional_key": "frobulator",
						"source_type":  "default",
					},
					Options: map[string]string{
						"ext": "png",
					},
					Digest: "94ba66590d3da0f9ea3a2f2c43132464",
				},
			},
		},
	}

	// no calls to httpclient because expectations and baseline should be
	// loaded from disk.

	expectedJSONPath := "skia-gold-testing/dm-json-v1/2019/04/02/19/cadbed23562/0/1554234843/dm-1554234843000000000.json"
	c := uploader.On("UploadJSON", mock.AnythingOfType("*jsonio.GoldResults"), filepath.Join(wd, jsonTempFile), expectedJSONPath)
	c.Run(func(args mock.Arguments) {
		uploaded := args.Get(0).(*jsonio.GoldResults)
		deepequal.AssertDeepEqual(t, j.SharedConfig, uploaded)
	}).Return(nil)

	jsonToWrite := testutils.MarshalJSON(t, &j)
	testutils.WriteFile(t, filepath.Join(wd, stateFile), jsonToWrite)

	goldClient, err := loadGoldClient(auth, wd)
	assert.NoError(t, err)

	// We don't need to call SetSharedConfig because the state should be
	// loaded from disk

	err = goldClient.Finalize()
	assert.NoError(t, err)
}

// "End to End" test of the non-pass-fail mode
// We init the setup, write a test, re-load the client from disk, write a test, re-load
// the client from disk and then finalize.
//
// This is essentially a test for
//   goldctl imgtest init
//   goldctl imgtest add
//   goldctl imgtest add
//   goldctl imgtest finalize
func TestInitAddFinalize(t *testing.T) {
	// We read and write to disk a little
	unittest.MediumTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	imgData := []byte("some bytes")
	firstHash := types.Digest("9d0568469d206c1aedf1b71f12f474bc")
	secondHash := types.Digest("29d0568469d206c1aedf1b71f12f474b")

	auth, httpClient, uploader := makeMocks()
	defer auth.AssertExpectations(t)
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	expectedUploadPath := string("gs://skia-gold-testing/dm-images-v1/" + firstHash + ".png")
	uploader.On("UploadBytes", imgData, testImgPath, expectedUploadPath).Return(nil).Once()
	expectedUploadPath = string("gs://skia-gold-testing/dm-images-v1/" + secondHash + ".png")
	uploader.On("UploadBytes", imgData, testImgPath, expectedUploadPath).Return(nil).Once()

	// Notice the JSON is not uploaded if we are not in passfail mode - a client
	// would need to call finalize first.
	goldClient, err := makeGoldClient(auth, false /*=passFail*/, true /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(testSharedConfig)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, firstHash, nil
	})

	pass, err := goldClient.Test("first-test", testImgPath, map[string]string{
		"config": "canvas",
	})
	assert.NoError(t, err)
	// true is always returned if we are not on passFail mode.
	assert.True(t, pass)

	// Check that the goldClient's in-memory representation is good
	results := goldClient.resultState.SharedConfig.Results
	assert.Len(t, results, 1)
	r := results[0]
	assert.Equal(t, "first-test", r.Key["name"])
	assert.Equal(t, "canvas", r.Key["config"])
	assert.Equal(t, "testing", r.Key[types.CORPUS_FIELD])
	assert.Equal(t, firstHash, r.Digest)

	// Now read the state from disk to make sure results are still there
	goldClient, err = loadGoldClient(auth, wd)
	assert.NoError(t, err)

	results = goldClient.resultState.SharedConfig.Results
	assert.Len(t, results, 1)
	r = results[0]
	assert.Equal(t, "first-test", r.Key["name"])
	assert.Equal(t, firstHash, r.Digest)

	// Add a second test with the same hash
	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, secondHash, nil
	})
	pass, err = goldClient.Test("second-test", testImgPath, map[string]string{
		"config": "svg",
	})
	assert.NoError(t, err)
	// true is always returned if we are not on passFail mode.
	assert.True(t, pass)

	// Now read the state again from disk to make sure results are still there
	goldClient, err = loadGoldClient(auth, wd)
	assert.NoError(t, err)

	expectedJSONPath := "skia-gold-testing/trybot/dm-json-v1/2019/04/02/19/abcd1234/117/1554234843/dm-1554234843000000000.json"
	c := uploader.On("UploadJSON", mock.AnythingOfType("*jsonio.GoldResults"), filepath.Join(wd, jsonTempFile), expectedJSONPath)
	c.Run(func(args mock.Arguments) {
		uploaded := args.Get(0).(*jsonio.GoldResults)
		results := uploaded.Results
		assert.Len(t, results, 2)
		r := results[0]
		assert.Equal(t, "first-test", r.Key["name"])
		assert.Equal(t, firstHash, r.Digest)
		assert.Equal(t, "canvas", r.Key["config"])
		assert.Equal(t, "testing", r.Key[types.CORPUS_FIELD])
		r = results[1]
		assert.Equal(t, "second-test", r.Key["name"])
		assert.Equal(t, secondHash, r.Digest)
		assert.Equal(t, "svg", r.Key["config"])
		assert.Equal(t, "testing", r.Key[types.CORPUS_FIELD])
	}).Return(nil)

	err = goldClient.Finalize()
	assert.NoError(t, err)
}

// TestNewReportPassFail ensures that a brand new test/digest returns false in pass-fail mode.
func TestNewReportPassFail(t *testing.T) {
	unittest.MediumTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	imgData := []byte("some bytes")
	imgHash := types.Digest("9d0568469d206c1aedf1b71f12f474bc")
	testName := types.TestName("TestNotSeenBefore")

	auth, httpClient, uploader := makeMocks()
	defer auth.AssertExpectations(t)
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse([]byte("none"), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	expectations := httpResponse([]byte("{}"), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/abcd1234?issue=867").Return(expectations, nil)

	expectedUploadPath := string("gs://skia-gold-testing/dm-images-v1/" + imgHash + ".png")
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
		assert.Equal(t, string(testName), r.Key["name"])
		// Since we did not specify a source_type it defaults to the instance name, which is
		// "testing"
		assert.Equal(t, "testing", r.Key["source_type"])

		assert.Equal(t, "png", r.Options["ext"])
	}).Return(nil)

	goldClient, err := makeGoldClient(auth, true /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(testSharedConfig)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	pass, err := goldClient.Test(testName, testImgPath, nil)
	assert.NoError(t, err)
	// Returns false because the test name has never been seen before
	// (and the digest is brand new)
	assert.False(t, pass)

	bytes, err := ioutil.ReadFile(filepath.Join(wd, failureLog))
	assert.NoError(t, err)
	assert.Equal(t, "https://testing-gold.skia.org/detail?test=TestNotSeenBefore&digest=9d0568469d206c1aedf1b71f12f474bc\n", string(bytes))
}

// TestNegativePassFail ensures that a digest marked negative returns false in pass-fail mode.
func TestNegativePassFail(t *testing.T) {
	unittest.MediumTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	imgData := []byte("some bytes")
	// These are defined in mockBaselineJSON
	imgHash := types.Digest("badbadbad1325855590527db196112e0")
	testName := types.TestName("ThisIsTheOnlyTest")

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

	goldClient, err := makeGoldClient(auth, true /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(testSharedConfig)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	pass, err := goldClient.Test(testName, testImgPath, nil)
	assert.NoError(t, err)
	// Returns false because the test is negative
	assert.False(t, pass)

	// Run it again to make sure the failure log isn't truncated
	pass, err = goldClient.Test(testName, testImgPath, nil)
	assert.NoError(t, err)
	// Returns false because the test is negative
	assert.False(t, pass)

	bytes, err := ioutil.ReadFile(filepath.Join(wd, failureLog))
	assert.NoError(t, err)
	assert.Equal(t, `https://testing-gold.skia.org/detail?test=ThisIsTheOnlyTest&digest=badbadbad1325855590527db196112e0
https://testing-gold.skia.org/detail?test=ThisIsTheOnlyTest&digest=badbadbad1325855590527db196112e0
`, string(bytes))
}

// TestPositivePassFail ensures that a positively marked digest returns true in pass-fail mode.
func TestPositivePassFail(t *testing.T) {
	unittest.MediumTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	imgData := []byte("some bytes")
	// These are defined in mockBaselineJSON
	imgHash := types.Digest("beef00d3a1527db19619ec12a4e0df68")
	testName := types.TestName("ThisIsTheOnlyTest")

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

	goldClient, err := makeGoldClient(auth, true /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(testSharedConfig)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	pass, err := goldClient.Test(testName, testImgPath, nil)
	assert.NoError(t, err)
	// Returns true because this test has been seen before and the digest was
	// previously triaged positive.
	assert.True(t, pass)

	// Failure file exists, but is empty if no failures
	bytes, err := ioutil.ReadFile(filepath.Join(wd, failureLog))
	assert.NoError(t, err)
	assert.Equal(t, "", string(bytes))
}

// Tests service account authentication is properly setup in the working directory.
// This (and the rest of TestInit*) are effectively tests of "goldctl auth".
func TestInitServiceAccountAuth(t *testing.T) {
	// writes to disk (but not a lot)
	unittest.MediumTest(t)

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

// Tests gsutil authentication is properly setup in the working directory.
func TestInitGSUtil(t *testing.T) {
	// writes to disk (but not a lot)
	unittest.MediumTest(t)

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

// Tests LUCI authentication is properly setup in the working directory.
func TestInitLUCIAuth(t *testing.T) {
	// writes to disk (but not a lot)
	unittest.MediumTest(t)

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

func TestGetWithRetriesSunnyDay(t *testing.T) {
	unittest.MediumTest(t)

	mh := &mocks.HTTPClient{}
	defer mh.AssertExpectations(t)

	url := "example.com"

	mh.On("Get", url).Return(httpResponse([]byte("this is example"), "200 OK", http.StatusOK), nil).Once()

	bytes, err := getWithRetries(mh, url)
	assert.NoError(t, err)
	assert.Equal(t, []byte("this is example"), bytes)
}

func TestGetWithRetriesRecover(t *testing.T) {
	unittest.MediumTest(t)

	mh := &mocks.HTTPClient{}
	defer mh.AssertExpectations(t)

	url := "example.com"

	mh.On("Get", url).Return(nil, errors.New("bork")).Once()
	mh.On("Get", url).Return(httpResponse([]byte("should not see"), "500 oops", http.StatusInternalServerError), nil).Once()
	mh.On("Get", url).Return(httpResponse([]byte("this is example"), "200 OK", http.StatusOK), nil).Once()

	bytes, err := getWithRetries(mh, url)
	assert.NoError(t, err)
	assert.Equal(t, []byte("this is example"), bytes)
}

func TestGetWithRetriesTooMany(t *testing.T) {
	unittest.LargeTest(t)

	mh := &mocks.HTTPClient{}
	defer mh.AssertExpectations(t)

	url := "example.com"

	mh.On("Get", url).Return(httpResponse([]byte("should not see"), "404 Not found", http.StatusNotFound), nil)

	_, err := getWithRetries(mh, url)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")
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

// loadGoldClient will load the cloudClient off the disk and returns it
// after stubbing out the time. Tests calling this should likely be
// medium sized due to disk reading.
func loadGoldClient(auth AuthOpt, workDir string) (*CloudClient, error) {
	c, err := LoadCloudClient(auth, workDir)
	if err != nil {
		return nil, err
	}
	c.now = func() time.Time {
		return time.Date(2019, time.April, 2, 19, 54, 3, 0, time.UTC)
	}
	return c, nil
}

// makeGoldClient will create new cloud client from scratch (using a
// set configuration), stub out time handling and return it.
func makeGoldClient(auth AuthOpt, passFail bool, uploadOnly bool, workDir string) (*CloudClient, error) {
	config := GoldClientConfig{
		InstanceID:   testInstanceID,
		WorkDir:      workDir,
		FailureFile:  filepath.Join(workDir, failureLog),
		PassFailStep: passFail,
		UploadOnly:   uploadOnly,
	}

	c, err := NewCloudClient(auth, config)
	if err != nil {
		return nil, err
	}
	c.now = func() time.Time {
		return time.Date(2019, time.April, 2, 19, 54, 3, 0, time.UTC)
	}
	return c, nil
}

func overrideLoadAndHashImage(c *CloudClient, testFn func(path string) ([]byte, types.Digest, error)) {
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

	failureLog = "failures.log"
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

var testSharedConfig = jsonio.GoldResults{
	GitHash: "abcd1234",
	Key: map[string]string{
		"os":  "WinTest",
		"gpu": "GPUTest",
	},
	Issue:         testIssueID,
	Patchset:      testPatchsetID,
	BuildBucketID: testBuildBucketID,
}
