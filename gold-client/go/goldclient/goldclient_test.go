package goldclient

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/gold-client/go/mocks"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/image/text"
	"go.skia.org/infra/golden/go/jsonio"
	one_by_five "go.skia.org/infra/golden/go/testutils/data_one_by_five"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/types/expectations"
)

// test data processing of the known hashes input.
func TestLoadKnownHashes(t *testing.T) {
	unittest.SmallTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	auth, httpClient, uploader, _ := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse([]byte(mockHashesTxt), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	exp := httpResponse([]byte("{}"), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/abcd1234?issue=867").Return(exp, nil)

	goldClient, err := makeGoldClient(auth, false /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(makeTestSharedConfig(), false)
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

	auth, httpClient, uploader, _ := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse([]byte("none"), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	exp := httpResponse([]byte(mockBaselineJSON), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/abcd1234?issue=867").Return(exp, nil)

	goldClient, err := makeGoldClient(auth, false /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(makeTestSharedConfig(), false)
	assert.NoError(t, err)

	// Check that the baseline was loaded correctly
	bl := goldClient.resultState.Expectations
	assert.Len(t, bl, 1, "only one test")
	digests := bl["ThisIsTheOnlyTest"]
	assert.Len(t, digests, 2, "two previously seen images")
	assert.Equal(t, expectations.Negative, digests["badbadbad1325855590527db196112e0"])
	assert.Equal(t, expectations.Positive, digests["beef00d3a1527db19619ec12a4e0df68"])

	assert.Equal(t, testIssueID, goldClient.resultState.SharedConfig.ChangeListID)

	knownHashes := goldClient.resultState.KnownHashes
	assert.Empty(t, knownHashes, "No hashes loaded")
}

// TestLoadBaselineMaster loads the baseline for the master branch.
func TestLoadBaselineMaster(t *testing.T) {
	unittest.SmallTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	auth, httpClient, uploader, _ := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse([]byte("none"), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	exp := httpResponse([]byte(mockBaselineJSON), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/abcd1234").Return(exp, nil)

	goldClient, err := makeGoldClient(auth, false /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(jsonio.GoldResults{
		GitHash: "abcd1234",
		Key: map[string]string{
			"os":  "WinTest",
			"gpu": "GPUTest",
		},
		// defaults to master branch
	}, false)
	assert.NoError(t, err)

	// Check that the baseline was loaded correctly
	bl := goldClient.resultState.Expectations
	assert.Len(t, bl, 1, "only one test")
	digests := bl["ThisIsTheOnlyTest"]
	assert.Len(t, digests, 2, "two previously seen images")
	assert.Equal(t, expectations.Negative, digests["badbadbad1325855590527db196112e0"])
	assert.Equal(t, expectations.Positive, digests["beef00d3a1527db19619ec12a4e0df68"])

	assert.Equal(t, "", goldClient.resultState.SharedConfig.ChangeListID)

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

	auth, httpClient, uploader, _ := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse([]byte(mockHashesTxt), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	exp := httpResponse([]byte(mockBaselineJSON), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/abcd1234?issue=867").Return(exp, nil)

	// no uploader calls

	goldClient, err := makeGoldClient(auth, true /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(makeTestSharedConfig(), false)
	assert.NoError(t, err)

	outFile := filepath.Join(wd, stateFile)
	assert.True(t, fileutil.FileExists(outFile))

	state, err := loadStateFromJSON(outFile)
	assert.NoError(t, err)
	assert.True(t, state.PerTestPassFail)
	assert.False(t, state.UploadOnly)
	assert.Equal(t, "testing", state.InstanceID)
	assert.Equal(t, "https://testing-gold.skia.org", state.GoldURL)
	assert.Equal(t, "skia-gold-testing", state.Bucket)
	assert.Len(t, state.KnownHashes, 4) // these should be saved to disk
	assert.Len(t, state.Expectations, 1)
	assert.Len(t, state.Expectations["ThisIsTheOnlyTest"], 2)
	assert.Equal(t, makeTestSharedConfig(), *state.SharedConfig)

	state, err = loadStateFromJSON("/tmp/some-file-guaranteed-not-to-exist")
	assert.Error(t, err)
}

// TestInitInvalidKeys fails if the SharedConfig would not pass validation (e.g. keys are malformed)
func TestInitInvalidKeys(t *testing.T) {
	unittest.SmallTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	auth, _, _, _ := makeMocks()

	goldClient, err := makeGoldClient(auth, true /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	conf := makeTestSharedConfig()
	conf.Key["blank"] = ""
	err = goldClient.SetSharedConfig(conf, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `invalid configuration`)
}

// Test that the client does not fetch from the server if UploadOnly is set.
// This is effectively a test for "goldctl imgtest init --upload-only"
func TestInitUploadOnly(t *testing.T) {
	// This test reads and writes a small amount of data from/to disk
	unittest.MediumTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	auth, httpClient, uploader, _ := makeMocks()
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
	err = goldClient.SetSharedConfig(makeTestSharedConfig(), false)
	assert.NoError(t, err)

	outFile := filepath.Join(wd, stateFile)
	assert.True(t, fileutil.FileExists(outFile))

	state, err := loadStateFromJSON(outFile)
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

	auth, httpClient, uploader, _ := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse([]byte("none"), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	exp := httpResponse([]byte("{}"), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/abcd1234?issue=867").Return(exp, nil)

	expectedUploadPath := string("gs://skia-gold-testing/dm-images-v1/" + imgHash + ".png")
	uploader.On("UploadBytes", testutils.AnyContext, imgData, testImgPath, expectedUploadPath).Return(nil)

	// Notice the JSON is not uploaded if we are not in passfail mode - a client
	// would need to call finalize first.
	goldClient, err := makeGoldClient(auth, false /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(makeTestSharedConfig(), false)
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

// TestNewReportNormalBadKeys tests the case when bad keys are passed in, which should not upload
// because the jsonio.GoldResults would be invalid.
func TestNewReportNormalBadKeys(t *testing.T) {
	unittest.SmallTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	imgData := []byte("some bytes")
	imgHash := types.Digest("9d0568469d206c1aedf1b71f12f474bc")

	auth, httpClient, _, _ := makeMocks()
	defer httpClient.AssertExpectations(t)

	hashesResp := httpResponse([]byte("none"), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	exp := httpResponse([]byte("{}"), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/abcd1234?issue=867").Return(exp, nil)

	// Notice the JSON is not uploaded if we are not in passfail mode - a client
	// would need to call finalize first.
	goldClient, err := makeGoldClient(auth, false /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(makeTestSharedConfig(), false)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	_, err = goldClient.Test("first-test", testImgPath, map[string]string{"empty": ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid test config")
}

// Test the uploading of JSON after two tests/images have been seen.
// This is effectively a test for "goldctl imgtest finalize"
func TestFinalizeNormal(t *testing.T) {
	// This test reads and writes a small amount of data from/to disk
	unittest.MediumTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	auth, httpClient, uploader, _ := makeMocks()
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

	expectedJSONPath := "skia-gold-testing/dm-json-v1/2019/04/02/19/cadbed23562/waterfall/1554234843/dm-1554234843000000000.json"
	grm := mock.MatchedBy(func(gr *jsonio.GoldResults) bool {
		assertdeep.Equal(t, j.SharedConfig, gr)
		return true
	})
	uploader.On("UploadJSON", testutils.AnyContext, grm, filepath.Join(wd, jsonTempFile), expectedJSONPath).Return(nil)

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

	auth, httpClient, uploader, _ := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	expectedUploadPath := string("gs://skia-gold-testing/dm-images-v1/" + firstHash + ".png")
	uploader.On("UploadBytes", testutils.AnyContext, imgData, testImgPath, expectedUploadPath).Return(nil).Once()
	expectedUploadPath = string("gs://skia-gold-testing/dm-images-v1/" + secondHash + ".png")
	uploader.On("UploadBytes", testutils.AnyContext, imgData, testImgPath, expectedUploadPath).Return(nil).Once()

	// Notice the JSON is not uploaded if we are not in passfail mode - a client
	// would need to call finalize first.
	goldClient, err := makeGoldClient(auth, false /*=passFail*/, true /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(makeTestSharedConfig(), false)
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
	grm := mock.MatchedBy(func(gr *jsonio.GoldResults) bool {
		assert.Len(t, gr.Results, 2)
		r := gr.Results[0]
		assert.Equal(t, "first-test", r.Key["name"])
		assert.Equal(t, firstHash, r.Digest)
		assert.Equal(t, "canvas", r.Key["config"])
		assert.Equal(t, "testing", r.Key[types.CORPUS_FIELD])
		r = gr.Results[1]
		assert.Equal(t, "second-test", r.Key["name"])
		assert.Equal(t, secondHash, r.Digest)
		assert.Equal(t, "svg", r.Key["config"])
		assert.Equal(t, "testing", r.Key[types.CORPUS_FIELD])
		return true
	})
	uploader.On("UploadJSON", testutils.AnyContext, grm, filepath.Join(wd, jsonTempFile), expectedJSONPath).Return(nil)

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

	auth, httpClient, uploader, _ := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse([]byte("none"), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	exp := httpResponse([]byte("{}"), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/abcd1234?issue=867").Return(exp, nil)

	expectedUploadPath := string("gs://skia-gold-testing/dm-images-v1/" + imgHash + ".png")
	uploader.On("UploadBytes", testutils.AnyContext, imgData, testImgPath, expectedUploadPath).Return(nil)

	expectedJSONPath := "skia-gold-testing/trybot/dm-json-v1/2019/04/02/19/abcd1234/117/1554234843/dm-1554234843000000000.json"
	checkResults := func(g *jsonio.GoldResults) bool {
		// spot check some of the properties
		assert.Equal(t, "abcd1234", g.GitHash)
		assert.Equal(t, testBuildBucketID, g.TryJobID)
		assert.Equal(t, map[string]string{
			"os":  "WinTest",
			"gpu": "GPUTest",
		}, g.Key)

		results := g.Results
		assert.Len(t, results, 1)
		r := results[0]
		assert.Equal(t, &jsonio.Result{
			Digest: imgHash,
			Options: map[string]string{
				"ext": "png",
			},
			Key: map[string]string{
				"name": string(testName),
				// Since we did not specify a source_type it defaults to the instance name, which is
				// "testing"
				"source_type": "testing",
			},
		}, r)
		return true
	}

	uploader.On("UploadJSON", testutils.AnyContext, mock.MatchedBy(checkResults), filepath.Join(wd, jsonTempFile), expectedJSONPath).Return(nil)

	goldClient, err := makeGoldClient(auth, true /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(makeTestSharedConfig(), false)
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

	b, err := ioutil.ReadFile(filepath.Join(wd, failureLog))
	assert.NoError(t, err)
	assert.Equal(t, "https://testing-gold.skia.org/detail?test=TestNotSeenBefore&digest=9d0568469d206c1aedf1b71f12f474bc&issue=867\n", string(b))
}

// TestReportPassFailPassWithCorpus test that when we set the corpus via the initial config
// // it properly gets overridden.
func TestReportPassFailPassWithCorpusInInit(t *testing.T) {
	unittest.MediumTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	imgData := []byte("some bytes")
	// These are defined in mockBaselineJSON
	imgHash := types.Digest("beef00d3a1527db19619ec12a4e0df68")
	testName := types.TestName("ThisIsTheOnlyTest")

	overRiddenCorpus := "gtest-pixeltests"

	auth, httpClient, uploader, _ := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse([]byte(imgHash), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	exp := httpResponse([]byte(mockBaselineJSON), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/abcd1234?issue=867").Return(exp, nil)

	expectedJSONPath := "skia-gold-testing/trybot/dm-json-v1/2019/04/02/19/abcd1234/117/1554234843/dm-1554234843000000000.json"
	checkResults := func(g *jsonio.GoldResults) bool {
		// spot check some of the properties
		assert.Equal(t, "abcd1234", g.GitHash)
		assert.Equal(t, testBuildBucketID, g.TryJobID)
		assert.Equal(t, map[string]string{
			"os":          "WinTest",
			"gpu":         "GPUTest",
			"source_type": overRiddenCorpus,
		}, g.Key)

		results := g.Results
		assert.Len(t, results, 1)
		r := results[0]
		assert.Equal(t, &jsonio.Result{
			Digest: imgHash,
			Options: map[string]string{
				"ext": "png",
			},
			Key: map[string]string{
				"name":          string(testName),
				"another_notch": "emeril",
			},
		}, r)
		return true
	}

	uploader.On("UploadJSON", testutils.AnyContext, mock.MatchedBy(checkResults), filepath.Join(wd, jsonTempFile), expectedJSONPath).Return(nil)

	goldClient, err := makeGoldClient(auth, true /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	config := makeTestSharedConfig()
	config.Key[types.CORPUS_FIELD] = overRiddenCorpus
	err = goldClient.SetSharedConfig(config, false)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	extraKeys := map[string]string{
		"another_notch": "emeril",
	}

	pass, err := goldClient.Test(testName, testImgPath, extraKeys)
	assert.NoError(t, err)
	// Returns true because the test has been seen before and marked positive.
	assert.True(t, pass)
}

// TestReportPassFailPassWithCorpusInKeys test that when we set the corpus via additional keys,
// it properly gets overridden.
func TestReportPassFailPassWithCorpusInKeys(t *testing.T) {
	unittest.MediumTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	imgData := []byte("some bytes")
	// These are defined in mockBaselineJSON
	imgHash := types.Digest("beef00d3a1527db19619ec12a4e0df68")
	testName := types.TestName("ThisIsTheOnlyTest")

	overRiddenCorpus := "gtest-pixeltests"

	auth, httpClient, uploader, _ := makeMocks()

	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse([]byte(imgHash), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	exp := httpResponse([]byte(mockBaselineJSON), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/abcd1234?issue=867").Return(exp, nil)

	expectedJSONPath := "skia-gold-testing/trybot/dm-json-v1/2019/04/02/19/abcd1234/117/1554234843/dm-1554234843000000000.json"
	checkResults := func(g *jsonio.GoldResults) bool {
		// spot check some of the properties
		assert.Equal(t, "abcd1234", g.GitHash)
		assert.Equal(t, testBuildBucketID, g.TryJobID)
		assert.Equal(t, map[string]string{
			"os":  "WinTest",
			"gpu": "GPUTest",
		}, g.Key)

		results := g.Results
		assert.Len(t, results, 1)
		r := results[0]
		assert.Equal(t, &jsonio.Result{
			Digest: imgHash,
			Options: map[string]string{
				"ext": "png",
			},
			Key: map[string]string{
				"name":          string(testName),
				"source_type":   overRiddenCorpus,
				"another_notch": "emeril",
			},
		}, r)
		return true
	}

	uploader.On("UploadJSON", testutils.AnyContext, mock.MatchedBy(checkResults), filepath.Join(wd, jsonTempFile), expectedJSONPath).Return(nil)

	goldClient, err := makeGoldClient(auth, true /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(makeTestSharedConfig(), false)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	extraKeys := map[string]string{
		"source_type":   overRiddenCorpus,
		"another_notch": "emeril",
	}

	pass, err := goldClient.Test(testName, testImgPath, extraKeys)
	assert.NoError(t, err)
	// Returns true because the test has been seen before and marked positive.
	assert.True(t, pass)
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

	auth, httpClient, uploader, _ := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse([]byte(imgHash), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	exp := httpResponse([]byte(mockBaselineJSON), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/abcd1234?issue=867").Return(exp, nil)

	// No upload expected because the bytes were already seen in json/hashes.

	expectedJSONPath := "skia-gold-testing/trybot/dm-json-v1/2019/04/02/19/abcd1234/117/1554234843/dm-1554234843000000000.json"
	uploader.On("UploadJSON", testutils.AnyContext, mock.AnythingOfType("*jsonio.GoldResults"), filepath.Join(wd, jsonTempFile), expectedJSONPath).Return(nil)

	goldClient, err := makeGoldClient(auth, true /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(makeTestSharedConfig(), false)
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

	b, err := ioutil.ReadFile(filepath.Join(wd, failureLog))
	assert.NoError(t, err)
	assert.Equal(t, `https://testing-gold.skia.org/detail?test=ThisIsTheOnlyTest&digest=badbadbad1325855590527db196112e0&issue=867
https://testing-gold.skia.org/detail?test=ThisIsTheOnlyTest&digest=badbadbad1325855590527db196112e0&issue=867
`, string(b))
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

	auth, httpClient, uploader, _ := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse([]byte(imgHash), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	exp := httpResponse([]byte(mockBaselineJSON), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/abcd1234?issue=867").Return(exp, nil)

	// No upload expected because the bytes were already seen in json/hashes.

	expectedJSONPath := "skia-gold-testing/trybot/dm-json-v1/2019/04/02/19/abcd1234/117/1554234843/dm-1554234843000000000.json"
	uploader.On("UploadJSON", testutils.AnyContext, mock.AnythingOfType("*jsonio.GoldResults"), filepath.Join(wd, jsonTempFile), expectedJSONPath).Return(nil)

	goldClient, err := makeGoldClient(auth, true /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(makeTestSharedConfig(), false)
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
	b, err := ioutil.ReadFile(filepath.Join(wd, failureLog))
	assert.NoError(t, err)
	assert.Equal(t, "", string(b))
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

	b, err := getWithRetries(mh, url)
	assert.NoError(t, err)
	assert.Equal(t, []byte("this is example"), b)
}

func TestGetWithRetriesRecover(t *testing.T) {
	unittest.MediumTest(t) // this takes a few seconds waiting before retry

	mh := &mocks.HTTPClient{}
	defer mh.AssertExpectations(t)

	url := "example.com"

	mh.On("Get", url).Return(nil, errors.New("bork")).Once()
	mh.On("Get", url).Return(httpResponse([]byte("should not see"), "500 oops", http.StatusInternalServerError), nil).Once()
	mh.On("Get", url).Return(httpResponse([]byte("this is example"), "200 OK", http.StatusOK), nil).Once()

	b, err := getWithRetries(mh, url)
	assert.NoError(t, err)
	assert.Equal(t, []byte("this is example"), b)
}

func TestGetWithRetriesTooMany(t *testing.T) {
	unittest.LargeTest(t) // this takes several seconds waiting before retry

	mh := &mocks.HTTPClient{}
	defer mh.AssertExpectations(t)

	url := "example.com"

	mh.On("Get", url).Return(httpResponse([]byte("should not see"), "404 Not found", http.StatusNotFound), nil)

	_, err := getWithRetries(mh, url)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

// TestCheckSunnyDay emulates running goldctl auth; goldctl imgtest check ... where the
// passed in image matches something on the baseline
func TestCheckSunnyDay(t *testing.T) {
	unittest.MediumTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	imgData := []byte("some bytes")
	// These are defined in mockBaselineJSON
	imgHash := types.Digest("beef00d3a1527db19619ec12a4e0df68")
	testName := types.TestName("ThisIsTheOnlyTest")

	auth, httpClient, _, _ := makeMocks()
	defer httpClient.AssertExpectations(t)

	hashesResp := httpResponse([]byte(imgHash), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	exp := httpResponse([]byte(mockBaselineJSON), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/HEAD").Return(exp, nil)

	config := GoldClientConfig{
		WorkDir:    wd,
		InstanceID: "testing",
	}
	goldClient, err := NewCloudClient(auth, config)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	pass, err := goldClient.Check(testName, testImgPath)
	assert.NoError(t, err)
	assert.True(t, pass)

	baselineBytes, err := ioutil.ReadFile(goldClient.getResultStatePath())
	assert.NoError(t, err)
	// spot check that the expectations were written to disk
	assert.Contains(t, string(baselineBytes), imgHash)
	assert.Contains(t, string(baselineBytes), "badbadbad1325855590527db196112e0")
}

// TestCheckIssue emulates running goldctl auth; goldctl imgtest check ... where the
// passed in image matches something on the baseline for a changelist.
func TestCheckIssue(t *testing.T) {
	unittest.MediumTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	imgData := []byte("some bytes")
	// These are defined in mockBaselineJSON
	imgHash := types.Digest("beef00d3a1527db19619ec12a4e0df68")
	testName := types.TestName("ThisIsTheOnlyTest")
	changeListID := "abc"

	auth, httpClient, _, _ := makeMocks()
	defer httpClient.AssertExpectations(t)

	hashesResp := httpResponse([]byte(imgHash), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	exp := httpResponse([]byte(mockBaselineJSON), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/HEAD?issue=abc").Return(exp, nil)

	config := GoldClientConfig{
		WorkDir:    wd,
		InstanceID: "testing",
	}
	goldClient, err := NewCloudClient(auth, config)
	assert.NoError(t, err)

	gr := jsonio.GoldResults{
		ChangeListID: changeListID,
		GitHash:      "HEAD",
	}
	err = goldClient.SetSharedConfig(gr, true)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	pass, err := goldClient.Check(testName, testImgPath)
	assert.NoError(t, err)
	assert.True(t, pass)

	baselineBytes, err := ioutil.ReadFile(goldClient.getResultStatePath())
	assert.NoError(t, err)
	// spot check that the expectations were written to disk
	assert.Contains(t, string(baselineBytes), imgHash)
	assert.Contains(t, string(baselineBytes), "badbadbad1325855590527db196112e0")
}

// TestCheckSunnyDayNegative emulates running goldctl auth; goldctl imgtest check ... where the
// passed in image does not match something on the baseline.
func TestCheckSunnyDayNegative(t *testing.T) {
	unittest.SmallTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	imgData := []byte("some bytes")
	// imgHash is not seen in expectations
	imgHash := types.Digest("4043142d1ec36177e8c6c4d31af0c6de")
	testName := types.TestName("ThisIsTheOnlyTest")

	auth, httpClient, _, _ := makeMocks()
	defer httpClient.AssertExpectations(t)

	hashesResp := httpResponse([]byte(imgHash), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil)

	exp := httpResponse([]byte(mockBaselineJSON), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/HEAD").Return(exp, nil)

	config := GoldClientConfig{
		WorkDir:    wd,
		InstanceID: "testing",
	}
	goldClient, err := NewCloudClient(auth, config)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	pass, err := goldClient.Check(testName, testImgPath)
	assert.NoError(t, err)
	assert.False(t, pass)
}

// TestCheckLoad emulates running goldctl auth; goldctl imgtest check ...; goldctl imgtest check...
// specifically focusing on loading from disk after the first check and not querying the
// backend every time.
func TestCheckLoad(t *testing.T) {
	unittest.MediumTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	imgData := []byte("some bytes")
	// These are defined in mockBaselineJSON
	imgHash := types.Digest("beef00d3a1527db19619ec12a4e0df68")
	testName := types.TestName("ThisIsTheOnlyTest")

	auth, httpClient, _, _ := makeMocks()
	defer httpClient.AssertExpectations(t)

	hashesResp := httpResponse([]byte(imgHash), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/hashes").Return(hashesResp, nil).Once()

	exp := httpResponse([]byte(mockBaselineJSON), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/expectations/commit/HEAD").Return(exp, nil).Once()

	config := GoldClientConfig{
		WorkDir:    wd,
		InstanceID: "testing",
	}
	goldClient, err := NewCloudClient(auth, config)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	pass, err := goldClient.Check(testName, testImgPath)
	assert.NoError(t, err)
	assert.True(t, pass)

	// Reload saved state from disk
	goldClient, err = LoadCloudClient(auth, wd)
	assert.NoError(t, err)
	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})
	pass, err = goldClient.Check(testName, testImgPath)
	assert.NoError(t, err)
	assert.True(t, pass)
}

// TestCheckLoadFails make sure that if we load from an empty directory, we fail to initialize
// a GoldClient.
func TestCheckLoadFails(t *testing.T) {
	unittest.MediumTest(t)

	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	auth, _, _, _ := makeMocks()

	// This should not work
	_, err := LoadCloudClient(auth, wd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "from disk")
}

// TestDiffSunnyDay shows a scenario in which the user wants to identify the closest known image
// for their given image. It simulates the Gold server having two digests for the given test,
// and makes sure that the goldClient downloads those and correctly identifies the closest image.
// It asserts that the given output directory has the original image, the closest image and
// the computed diff.
func TestDiffSunnyDay(t *testing.T) {
	unittest.MediumTest(t)

	const corpus = "This Has spaces"
	const testName = types.TestName("This IsTheOnly Test")
	// This hash is the real computed hash of the bytes from image1
	const leftHash = "f81a3bb94c02596e06e74c84d1076fff"
	// rightHash is the closest of the two images compared against. It is arbitrary.
	const rightHash = "bbb0dc56d0429ef3586629787666ce09"
	// otherHash is a digest that should be compared against, but is not the closest.
	// It is arbitrary.
	const otherHash = "ccc2912653148661835084a809fee263"

	wd, cleanup := testutils.TempDir(t)
	outDir := filepath.Join(wd, "out")
	defer cleanup()

	inputPath := filepath.Join(wd, "input.png")
	input, err := os.Create(inputPath)
	require.NoError(t, err)
	require.NoError(t, png.Encode(input, image1))
	require.NoError(t, input.Close())

	auth, httpClient, _, dlr := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer dlr.AssertExpectations(t)

	digests := httpResponse([]byte(mockDigestsJSON), "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/digests?test=This+IsTheOnly+Test&corpus=This+Has+spaces").Return(digests, nil)

	img2 := asEncodedBytes(t, image2)
	dlr.On("Download", testutils.AnyContext, "gs://skia-gold-testing/dm-images-v1/"+rightHash+".png", mock.Anything).Return(img2, nil)
	img3 := asEncodedBytes(t, image3)
	dlr.On("Download", testutils.AnyContext, "gs://skia-gold-testing/dm-images-v1/"+otherHash+".png", mock.Anything).Return(img3, nil)

	config := GoldClientConfig{
		WorkDir:    wd,
		InstanceID: "testing",
	}
	goldClient, err := NewCloudClient(auth, config)
	require.NoError(t, err)

	err = goldClient.Diff(context.Background(), testName, corpus, inputPath, outDir)
	require.NoError(t, err)

	leftImg, err := openNRGBAFromFile(filepath.Join(outDir, "input-"+leftHash+".png"))
	require.NoError(t, err)
	assert.Equal(t, leftImg, image1)

	rightImg, err := openNRGBAFromFile(filepath.Join(outDir, "closest-"+rightHash+".png"))
	require.NoError(t, err)
	assert.Equal(t, rightImg, image2)

	diffImg, err := openNRGBAFromFile(filepath.Join(outDir, "diff.png"))
	require.NoError(t, err)
	assert.Equal(t, diffImg, diff12)
}

// TestDiffCaching makes sure we cache the images we download from GCS and only download them
// once if we try calling Diff multiple times.
func TestDiffCaching(t *testing.T) {
	unittest.MediumTest(t)

	const corpus = "whatever"
	const testName = types.TestName("ThisIsTheOnlyTest")
	const rightHash = "bbb0dc56d0429ef3586629787666ce09"
	const otherHash = "ccc2912653148661835084a809fee263"

	wd, cleanup := testutils.TempDir(t)
	outDir := filepath.Join(wd, "out")
	defer cleanup()

	inputPath := filepath.Join(wd, "input.png")
	input, err := os.Create(inputPath)
	require.NoError(t, err)
	require.NoError(t, png.Encode(input, image1))
	require.NoError(t, input.Close())

	auth, httpClient, _, dlr := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer dlr.AssertExpectations(t)

	httpClient.On("Get", "https://testing-gold.skia.org/json/digests?test=ThisIsTheOnlyTest&corpus=whatever").Return(func(_ string) *http.Response {
		// return a fresh response each time Diff is called
		return httpResponse([]byte(mockDigestsJSON), "200 OK", http.StatusOK)
	}, nil).Twice()

	img2 := asEncodedBytes(t, image2)
	dlr.On("Download", testutils.AnyContext, "gs://skia-gold-testing/dm-images-v1/"+rightHash+".png", mock.Anything).Return(img2, nil).Once()
	img3 := asEncodedBytes(t, image3)
	dlr.On("Download", testutils.AnyContext, "gs://skia-gold-testing/dm-images-v1/"+otherHash+".png", mock.Anything).Return(img3, nil).Once()

	config := GoldClientConfig{
		WorkDir:    wd,
		InstanceID: "testing",
	}
	goldClient, err := NewCloudClient(auth, config)
	require.NoError(t, err)

	err = goldClient.Diff(context.Background(), testName, corpus, inputPath, outDir)
	require.NoError(t, err)

	// Call it twice to make sure we only hit GCS once per file
	err = goldClient.Diff(context.Background(), testName, corpus, inputPath, outDir)
	require.NoError(t, err)
}

func makeMocks() (*MockAuthOpt, *mocks.HTTPClient, *mocks.GCSUploader, *mocks.GCSDownloader) {
	mh := mocks.HTTPClient{}
	mg := mocks.GCSUploader{}
	md := mocks.GCSDownloader{}
	ma := MockAuthOpt{}
	ma.On("Validate").Return(nil)
	ma.On("GetHTTPClient").Return(&mh, nil)
	ma.On("GetGCSUploader").Return(&mg, nil)
	ma.On("GetGCSDownloader").Return(&md, nil)
	return &ma, &mh, &mg, &md
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
		Body:       ioutil.NopCloser(bytes.NewReader(body)),
		Status:     status,
		StatusCode: statusCode,
	}
}

const (
	testInstanceID    = "testing"
	testIssueID       = "867"
	testPatchsetID    = 5309
	testBuildBucketID = "117"
	testImgPath       = "/path/to/images/fake.png"

	failureLog = "failures.log"
)

// These images (of type *image.NRGBA) are assumed to be used in a read-only manner
// throughout the tests.
var image1 = text.MustToNRGBA(one_by_five.ImageOne)
var image2 = text.MustToNRGBA(one_by_five.ImageTwo)
var image3 = text.MustToNRGBA(one_by_five.ImageSix)
var diff12 = text.MustToNRGBA(one_by_five.DiffImageOneAndTwo)

// An example baseline that has a single test at a single commit with a good
// image and a bad image.
const mockBaselineJSON = `
{
  "md5": "7e4081337b3258555906970002a04a59",
  "master": {
    "ThisIsTheOnlyTest": {
      "beef00d3a1527db19619ec12a4e0df68": 1,
      "badbadbad1325855590527db196112e0": 2
    }
  },
  "Issue": -1
}`

const mockHashesTxt = `a9e1481ebc45c1c4f6720d1119644c20
c156c5e4b634a3b8cc96e16055197f8b
4a434407218e198faf2054645fe0ff73
303a5fd488361214f246004530e24273`

const mockDigestsJSON = `
{
  "digests": ["bbb0dc56d0429ef3586629787666ce09", "ccc2912653148661835084a809fee263"]
}`

func makeTestSharedConfig() jsonio.GoldResults {
	return jsonio.GoldResults{
		GitHash: "abcd1234",
		Key: map[string]string{
			"os":  "WinTest",
			"gpu": "GPUTest",
		},
		ChangeListID:                testIssueID,
		PatchSetOrder:               testPatchsetID,
		CodeReviewSystem:            "gerrit",
		TryJobID:                    testBuildBucketID,
		ContinuousIntegrationSystem: "buildbucket",
	}
}

func asEncodedBytes(t *testing.T, img image.Image) []byte {
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// openNRGBAFromFile opens the given file path to a PNG file and returns the image as image.NRGBA.
func openNRGBAFromFile(fileName string) (*image.NRGBA, error) {
	var img *image.NRGBA
	err := util.WithReadFile(fileName, func(r io.Reader) error {
		im, err := png.Decode(r)
		if err != nil {
			return err
		}
		img = diff.GetNRGBA(im)
		return nil
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return img, nil
}
