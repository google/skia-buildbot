package goldclient

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
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
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/gold-client/go/imgmatching"
	"go.skia.org/infra/gold-client/go/mocks"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/image/text"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/sql"
	one_by_five "go.skia.org/infra/golden/go/testutils/data_one_by_five"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/web/frontend"
)

// test data processing of the known hashes input.
func TestLoadKnownHashes(t *testing.T) {
	unittest.SmallTest(t)

	wd := t.TempDir()

	ctx, httpClient, uploader, _ := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse(mockHashesTxt, "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v1/hashes").Return(hashesResp, nil)

	exp := httpResponse("{}", "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v2/expectations?issue=867&crs=gerrit").Return(exp, nil)

	goldClient, err := makeGoldClient(false /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(ctx, makeTestSharedConfig(), false)
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

	wd := t.TempDir()

	ctx, httpClient, uploader, _ := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse("none", "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v1/hashes").Return(hashesResp, nil)

	exp := httpResponse(mockBaselineJSON, "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v2/expectations?issue=867&crs=gerrit").Return(exp, nil)

	goldClient, err := makeGoldClient(false /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(ctx, makeTestSharedConfig(), false)
	assert.NoError(t, err)

	// Check that the baseline was loaded correctly
	bl := goldClient.resultState.Expectations
	assert.Len(t, bl, 1, "only one test")
	digests := bl["ThisIsTheOnlyTest"]
	assert.Len(t, digests, 2, "two previously seen images")
	assert.Equal(t, expectations.Negative, digests["badbadbad1325855590527db196112e0"])
	assert.Equal(t, expectations.Positive, digests["beef00d3a1527db19619ec12a4e0df68"])

	assert.Equal(t, testIssueID, goldClient.resultState.SharedConfig.ChangelistID)

	knownHashes := goldClient.resultState.KnownHashes
	assert.Empty(t, knownHashes, "No hashes loaded")
}

// TestLoadBaselineMaster loads the baseline for the master branch.
func TestLoadBaselineMaster(t *testing.T) {
	unittest.SmallTest(t)

	wd := t.TempDir()

	ctx, httpClient, uploader, _ := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse("none", "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v1/hashes").Return(hashesResp, nil)

	exp := httpResponse(mockBaselineJSON, "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v2/expectations").Return(exp, nil)

	goldClient, err := makeGoldClient(false /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(ctx, jsonio.GoldResults{
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

	assert.Equal(t, "", goldClient.resultState.SharedConfig.ChangelistID)

	knownHashes := goldClient.resultState.KnownHashes
	assert.Empty(t, knownHashes, "No hashes loaded")
}

// Test that the working dir has the correct JSON after initializing.
// This is effectively a test for "goldctl imgtest init"
func TestInit(t *testing.T) {
	// This test reads and writes a small amount of data from/to disk
	unittest.MediumTest(t)

	wd := t.TempDir()

	ctx, httpClient, uploader, _ := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse(mockHashesTxt, "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v1/hashes").Return(hashesResp, nil)

	exp := httpResponse(mockBaselineJSON, "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v2/expectations?issue=867&crs=gerrit").Return(exp, nil)

	// no uploader calls

	goldClient, err := makeGoldClient(true /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(ctx, makeTestSharedConfig(), false)
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
	assert.Equal(t, makeTestSharedConfig(), state.SharedConfig)

	_, err = loadStateFromJSON("/tmp/some-file-guaranteed-not-to-exist")
	assert.Error(t, err)
}

// TestInitInvalidKeys fails if the SharedConfig would not pass validation (e.g. keys are malformed)
func TestInitInvalidKeys(t *testing.T) {
	unittest.SmallTest(t)

	wd := t.TempDir()

	ctx, _, _, _ := makeMocks()

	goldClient, err := makeGoldClient(true /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	conf := makeTestSharedConfig()
	conf.Key["blank"] = ""
	err = goldClient.SetSharedConfig(ctx, conf, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `invalid configuration`)
}

// Test that the client does not fetch from the server if UploadOnly is set.
// This is effectively a test for "goldctl imgtest init --upload-only"
func TestInitUploadOnly(t *testing.T) {
	// This test reads and writes a small amount of data from/to disk
	unittest.MediumTest(t)

	wd := t.TempDir()

	ctx, httpClient, uploader, _ := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	// no calls of any kind

	config := GoldClientConfig{
		InstanceID:   "fuchsia",
		WorkDir:      wd,
		PassFailStep: false,
		UploadOnly:   true,
	}

	goldClient, err := NewCloudClient(config)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(ctx, makeTestSharedConfig(), false)
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

func TestAddResult_Success(t *testing.T) {
	unittest.SmallTest(t)

	const expectedTraceID = "673031cf116813b91be9c4ac14d62412"
	_, tb := sql.SerializeMap(map[string]string{"alpha": "beta", "gamma": "delta", "name": "my_test", "source_type": "my_corpus"})
	require.Equal(t, expectedTraceID, hex.EncodeToString(tb))

	goldClient := CloudClient{
		resultState: &resultState{
			InstanceID: "my_instance", // Should be ignored.
			SharedConfig: jsonio.GoldResults{
				Key: map[string]string{
					"alpha":           "beta",
					types.CorpusField: "my_corpus",
				},
			},
		},
	}

	traceId := goldClient.addResult("my_test", "9d0568469d206c1aedf1b71f12f474bc", map[string]string{"gamma": "delta"}, map[string]string{"epsilon": "zeta"})
	assert.Equal(t, []jsonio.Result{
		{
			Digest: "9d0568469d206c1aedf1b71f12f474bc",
			Key: map[string]string{
				"gamma": "delta",
				"name":  "my_test",
			},
			Options: map[string]string{
				"epsilon": "zeta",
				"ext":     "png",
			},
		},
	}, goldClient.resultState.SharedConfig.Results)
	assert.Equal(t, tiling.TraceIDV2(expectedTraceID), traceId)
}

func TestAddResult_NoCorpusSpecified_UsesInstanceIdAsCorpus_Success(t *testing.T) {
	unittest.SmallTest(t)

	const expectedTraceID = "b8bb20640d45f2fa4f2b52d1acb11abd"
	_, tb := sql.SerializeMap(map[string]string{"alpha": "beta", "gamma": "delta", "name": "my_test", "source_type": "my_instance"})
	require.Equal(t, expectedTraceID, hex.EncodeToString(tb))

	goldClient := CloudClient{
		resultState: &resultState{
			InstanceID: "my_instance",
			SharedConfig: jsonio.GoldResults{
				Key: map[string]string{
					"alpha": "beta",
					// No corpus specified, therefore the instance name is used as the default.
				},
			},
		},
	}

	traceId := goldClient.addResult("my_test", "9d0568469d206c1aedf1b71f12f474bc", map[string]string{"gamma": "delta"}, map[string]string{"epsilon": "zeta"})
	assert.Equal(t, []jsonio.Result{
		{
			Digest: "9d0568469d206c1aedf1b71f12f474bc",
			Key: map[string]string{
				"gamma":       "delta",
				"name":        "my_test",
				"source_type": "my_instance",
			},
			Options: map[string]string{
				"epsilon": "zeta",
				"ext":     "png",
			},
		},
	}, goldClient.resultState.SharedConfig.Results)
	assert.Equal(t, tiling.TraceIDV2(expectedTraceID), traceId)
}

// Report an image that does not match any previous digests.
// This is effectively a test for "goldctl imgtest add"
func TestNewReportNormal(t *testing.T) {
	unittest.SmallTest(t)

	wd := t.TempDir()

	imgData := []byte("some bytes")
	imgHash := types.Digest("9d0568469d206c1aedf1b71f12f474bc")

	ctx, httpClient, uploader, _ := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse("none", "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v1/hashes").Return(hashesResp, nil)

	exp := httpResponse("{}", "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v2/expectations?issue=867&crs=gerrit").Return(exp, nil)

	expectedUploadPath := string("gs://skia-gold-testing/dm-images-v1/" + imgHash + ".png")
	uploader.On("UploadBytes", testutils.AnyContext, imgData, testImgPath, expectedUploadPath).Return(nil)

	// Notice the JSON is not uploaded if we are not in passfail mode - a client
	// would need to call finalize first.
	goldClient, err := makeGoldClient(false /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(ctx, makeTestSharedConfig(), false)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	pass, err := goldClient.Test(ctx, "first-test", testImgPath, "", nil, nil)
	assert.NoError(t, err)
	// true is always returned if we are not on passFail mode.
	assert.True(t, pass)
}

// Report an image using the supplied digest
// This is effectively a test for "goldctl imgtest add --png-digest ... --png-file ..."
func TestNewReportDigestAndImageSupplied(t *testing.T) {
	unittest.SmallTest(t)

	wd := t.TempDir()

	imgData := []byte("some bytes")
	// This is the digest the client has calculated
	const precalculatedDigest = types.Digest("00000000000000111111111111111222")

	ctx, httpClient, uploader, _ := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse("none", "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v1/hashes").Return(hashesResp, nil)

	exp := httpResponse("{}", "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v2/expectations?issue=867&crs=gerrit").Return(exp, nil)

	expectedUploadPath := string("gs://skia-gold-testing/dm-images-v1/" + precalculatedDigest + ".png")
	uploader.On("UploadBytes", testutils.AnyContext, imgData, testImgPath, expectedUploadPath).Return(nil)

	// Notice the JSON is not uploaded if we are not in passfail mode - a client
	// would need to call finalize first.
	goldClient, err := makeGoldClient(false /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(ctx, makeTestSharedConfig(), false)
	assert.NoError(t, err)

	// This returns the "wrong" hash, i.e. the one we don't want to use.
	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, "ffffffffffffffffffffffffffffffff", nil
	})

	pass, err := goldClient.Test(ctx, "first-test", testImgPath, precalculatedDigest, nil, nil)
	assert.NoError(t, err)
	// true is always returned if we are not on passFail mode.
	assert.True(t, pass)
}

// Report an image using only the supplied digest (no png to upload).
// This is effectively a test for "goldctl imgtest add --png-digest ..."
func TestNewReportDigestSupplied(t *testing.T) {
	unittest.SmallTest(t)

	wd := t.TempDir()

	// This is the digest the client has calculated
	const precalculatedDigest = "00000000000000111111111111111222"

	ctx, httpClient, uploader, _ := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse(precalculatedDigest, "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v1/hashes").Return(hashesResp, nil)

	exp := httpResponse("{}", "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v2/expectations?issue=867&crs=gerrit").Return(exp, nil)

	// Notice the JSON is not uploaded if we are not in passfail mode - a client
	// would need to call finalize first.
	goldClient, err := makeGoldClient(false /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(ctx, makeTestSharedConfig(), false)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Fail(t, "call not expected")
		return nil, "", errors.New("not expected")
	})

	pass, err := goldClient.Test(ctx, "first-test", "", precalculatedDigest, nil, nil)
	assert.NoError(t, err)
	// true is always returned if we are not on passFail mode.
	assert.True(t, pass)
}

// TestNewReportNormalBadKeys tests the case when bad keys are passed in, which should not upload
// because the jsonio.GoldResults would be invalid.
func TestNewReportNormalBadKeys(t *testing.T) {
	unittest.SmallTest(t)

	wd := t.TempDir()

	imgData := []byte("some bytes")
	imgHash := types.Digest("9d0568469d206c1aedf1b71f12f474bc")

	ctx, httpClient, _, _ := makeMocks()
	defer httpClient.AssertExpectations(t)

	hashesResp := httpResponse("none", "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v1/hashes").Return(hashesResp, nil)

	exp := httpResponse("{}", "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v2/expectations?issue=867&crs=gerrit").Return(exp, nil)

	// Notice the JSON is not uploaded if we are not in passfail mode - a client
	// would need to call finalize first.
	goldClient, err := makeGoldClient(false /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(ctx, makeTestSharedConfig(), false)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	_, err = goldClient.Test(ctx, "first-test", testImgPath, "", map[string]string{"empty": ""}, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid test config")
}

// Test the uploading of JSON after two tests/images have been seen.
// This is effectively a test for "goldctl imgtest finalize"
func TestFinalizeNormal(t *testing.T) {
	// This test reads and writes a small amount of data from/to disk
	unittest.MediumTest(t)

	wd := t.TempDir()

	ctx, httpClient, uploader, _ := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	// handcrafted state that has two tests in it
	j := resultState{
		PerTestPassFail: false,
		InstanceID:      "testing",
		GoldURL:         "https://testing-gold.skia.org",
		Bucket:          "skia-gold-testing",
		SharedConfig: jsonio.GoldResults{
			GitHash:  "cadbed23562",
			TryJobID: "Test-Z80-Debug",
			Key: map[string]string{
				"os":  "TestOS",
				"cpu": "z80",
			},
			Results: []jsonio.Result{
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

	expectedJSONPath := "skia-gold-testing/dm-json-v1/2019/04/02/19/cadbed23562/Test-Z80-Debug/dm-1554234843000000000.json"
	grm := mock.MatchedBy(func(gr jsonio.GoldResults) bool {
		assertdeep.Equal(t, j.SharedConfig, gr)
		return true
	})
	uploader.On("UploadJSON", testutils.AnyContext, grm, filepath.Join(wd, jsonTempFile), expectedJSONPath).Return(nil)

	jsonToWrite := testutils.MarshalJSON(t, &j)
	testutils.WriteFile(t, filepath.Join(wd, stateFile), jsonToWrite)

	goldClient, err := LoadCloudClient(wd)
	assert.NoError(t, err)

	// We don't need to call SetSharedConfig because the state should be
	// loaded from disk

	err = goldClient.Finalize(ctx)
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

	wd := t.TempDir()

	imgData := []byte("some bytes")
	const firstHash = types.Digest("9d0568469d206c1aedf1b71f12f474bc")
	const secondHash = types.Digest("29d0568469d206c1aedf1b71f12f474b")

	ctx, httpClient, uploader, _ := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	expectedUploadPath := string("gs://skia-gold-testing/dm-images-v1/" + firstHash + ".png")
	uploader.On("UploadBytes", testutils.AnyContext, imgData, testImgPath, expectedUploadPath).Return(nil).Once()
	expectedUploadPath = string("gs://skia-gold-testing/dm-images-v1/" + secondHash + ".png")
	uploader.On("UploadBytes", testutils.AnyContext, imgData, testImgPath, expectedUploadPath).Return(nil).Once()

	// Notice the JSON is not uploaded if we are not in passfail mode - a client
	// would need to call finalize first.
	goldClient, err := makeGoldClient(false /*=passFail*/, true /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(ctx, makeTestSharedConfig(), false)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, firstHash, nil
	})

	pass, err := goldClient.Test(ctx, "first-test", testImgPath, "", map[string]string{
		"config": "canvas",
	}, map[string]string{
		"alpha_type": "Premul",
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
	assert.Equal(t, "testing", r.Key[types.CorpusField])
	assert.Equal(t, "Premul", r.Options["alpha_type"])
	assert.Equal(t, firstHash, r.Digest)

	// Now read the state from disk to make sure results are still there
	goldClient, err = LoadCloudClient(wd)
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
	pass, err = goldClient.Test(ctx, "second-test", testImgPath, "", map[string]string{
		"config": "svg",
	}, nil)
	assert.NoError(t, err)
	// true is always returned if we are not on passFail mode.
	assert.True(t, pass)

	// Now read the state again from disk to make sure results are still there
	goldClient, err = LoadCloudClient(wd)
	assert.NoError(t, err)

	expectedJSONPath := "skia-gold-testing/trybot/dm-json-v1/2019/04/02/19/867__5309/117/dm-1554234843000000000.json"
	grm := mock.MatchedBy(func(gr jsonio.GoldResults) bool {
		assert.Len(t, gr.Results, 2)
		r := gr.Results[0]
		assert.Equal(t, "first-test", r.Key["name"])
		assert.Equal(t, firstHash, r.Digest)
		assert.Equal(t, "canvas", r.Key["config"])
		assert.Equal(t, "testing", r.Key[types.CorpusField])
		assert.Equal(t, "Premul", r.Options["alpha_type"])
		r = gr.Results[1]
		assert.Equal(t, "second-test", r.Key["name"])
		assert.Equal(t, secondHash, r.Digest)
		assert.Equal(t, "svg", r.Key["config"])
		assert.Equal(t, "testing", r.Key[types.CorpusField])
		return true
	})
	uploader.On("UploadJSON", testutils.AnyContext, grm, filepath.Join(wd, jsonTempFile), expectedJSONPath).Return(nil)

	err = goldClient.Finalize(ctx)
	assert.NoError(t, err)
}

// TestNewReportPassFail ensures that a brand new test/digest returns false in pass-fail mode.
func TestNewReportPassFail(t *testing.T) {
	unittest.MediumTest(t)

	wd := t.TempDir()

	imgData := []byte("some bytes")
	imgHash := types.Digest("9d0568469d206c1aedf1b71f12f474bc")
	testName := types.TestName("TestNotSeenBefore")

	ctx, httpClient, uploader, _ := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse("none", "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v1/hashes").Return(hashesResp, nil)

	exp := httpResponse("{}", "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v2/expectations?issue=867&crs=gerrit").Return(exp, nil)

	expectedUploadPath := string("gs://skia-gold-testing/dm-images-v1/" + imgHash + ".png")
	uploader.On("UploadBytes", testutils.AnyContext, imgData, testImgPath, expectedUploadPath).Return(nil)

	expectedJSONPath := "skia-gold-testing/trybot/dm-json-v1/2019/04/02/19/867__5309/117/dm-1554234843000000000.json"
	checkResults := func(g jsonio.GoldResults) bool {
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
		assert.Equal(t, jsonio.Result{
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

	goldClient, err := makeGoldClient(true /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(ctx, makeTestSharedConfig(), false)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	pass, err := goldClient.Test(ctx, testName, testImgPath, "", nil, nil)
	assert.NoError(t, err)
	// Returns false because the test name has never been seen before
	// (and the digest is brand new)
	assert.False(t, pass)

	b, err := ioutil.ReadFile(filepath.Join(wd, failureLog))
	assert.NoError(t, err)
	assert.Equal(t, "https://testing-gold.skia.org/detail?test=TestNotSeenBefore&digest=9d0568469d206c1aedf1b71f12f474bc&changelist_id=867&crs=gerrit\n", string(b))
}

// TestReportPassFailPassWithCorpus test that when we set the corpus via the initial config
// // it properly gets overridden.
func TestReportPassFailPassWithCorpusInInit(t *testing.T) {
	unittest.MediumTest(t)

	wd := t.TempDir()

	imgData := []byte("some bytes")
	// These are defined in mockBaselineJSON
	const imgHash = "beef00d3a1527db19619ec12a4e0df68"
	const testName = types.TestName("ThisIsTheOnlyTest")

	const overRiddenCorpus = "gtest-pixeltests"

	ctx, httpClient, uploader, _ := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse(imgHash, "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v1/hashes").Return(hashesResp, nil)

	exp := httpResponse(mockBaselineJSON, "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v2/expectations?issue=867&crs=gerrit").Return(exp, nil)

	expectedJSONPath := "skia-gold-testing/trybot/dm-json-v1/2019/04/02/19/867__5309/117/dm-1554234843000000000.json"
	checkResults := func(g jsonio.GoldResults) bool {
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
		assert.Equal(t, jsonio.Result{
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

	goldClient, err := makeGoldClient(true /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	config := makeTestSharedConfig()
	config.Key[types.CorpusField] = overRiddenCorpus
	err = goldClient.SetSharedConfig(ctx, config, false)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	extraKeys := map[string]string{
		"another_notch": "emeril",
	}

	pass, err := goldClient.Test(ctx, testName, testImgPath, "", extraKeys, nil)
	assert.NoError(t, err)
	// Returns true because the test has been seen before and marked positive.
	assert.True(t, pass)
}

// TestReportPassFailPassWithCorpusInKeys test that when we set the corpus via additional keys,
// it properly gets overridden.
func TestReportPassFailPassWithCorpusInKeys(t *testing.T) {
	unittest.MediumTest(t)

	wd := t.TempDir()

	imgData := []byte("some bytes")
	// These are defined in mockBaselineJSON
	const imgHash = "beef00d3a1527db19619ec12a4e0df68"
	const testName = types.TestName("ThisIsTheOnlyTest")

	const overRiddenCorpus = "gtest-pixeltests"

	ctx, httpClient, uploader, _ := makeMocks()

	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse(imgHash, "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v1/hashes").Return(hashesResp, nil)

	exp := httpResponse(mockBaselineJSON, "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v2/expectations?issue=867&crs=gerrit").Return(exp, nil)

	expectedJSONPath := "skia-gold-testing/trybot/dm-json-v1/2019/04/02/19/867__5309/117/dm-1554234843000000000.json"
	checkResults := func(g jsonio.GoldResults) bool {
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
		assert.Equal(t, jsonio.Result{
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

	goldClient, err := makeGoldClient(true /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(ctx, makeTestSharedConfig(), false)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	extraKeys := map[string]string{
		"source_type":   overRiddenCorpus,
		"another_notch": "emeril",
	}

	pass, err := goldClient.Test(ctx, testName, testImgPath, "", extraKeys, nil)
	assert.NoError(t, err)
	// Returns true because the test has been seen before and marked positive.
	assert.True(t, pass)
}

// TestReportPassFailPassWithFuzzyMatching tests that a non-exact image matching algorithm is used
// when one is specified via the optional keys. Specifically, the user adds an image with digest
// "111...", which is deemed to be an approximate match to the latest positive digest "222...".
// Because it is deemed to be a match, we expect to see an RPC to triage "111..." as positive.
func TestReportPassFailPassWithFuzzyMatching(t *testing.T) {
	unittest.MediumTest(t)

	wd := t.TempDir()

	// The test name is defined in mockBaselineJSON. The image hashes below are not.
	testName := types.TestName("ThisIsTheOnlyTest")

	latestPositiveImageBytes := imageToPngBytes(t, text.MustToNRGBA(`! SKTEXTSIMPLE
	2 2
	0x00000000 0x00000000
	0x00000000 0x00000000`))
	const latestPositiveImageHash = types.Digest("22222222222222222222222222222222")

	// New image differs from the latest positive image by one pixel, but the difference is below the
	// fuzzy matching thresholds.
	newImageBytes := imageToPngBytes(t, text.MustToNRGBA(`! SKTEXTSIMPLE
	2 2
	0xFFFFFFFF 0x00000000
	0x00000000 0x00000000`))
	const newImageHash = "11111111111111111111111111111111"

	// Fuzzy matching with big thresholds to ensure the images above are always deemed equivalent.
	const imageMatchingAlgorithm = imgmatching.FuzzyMatching
	const maxDifferentPixels = "99999999"
	const pixelDeltaThreshold = "1020"

	overRiddenCorpus := "gtest-pixeltests"

	ctx, httpClient, uploader, downloader := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)
	defer downloader.AssertExpectations(t)

	// Mock out getting the list of known hashes.
	hashesResp := httpResponse(newImageHash, "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v1/hashes").Return(hashesResp, nil)

	// Mock out getting the test baselines.
	exp := httpResponse(mockBaselineJSON, "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v2/expectations?issue=867&crs=gerrit").Return(exp, nil)

	// Mock out retrieving the latest positive image hash for ThisIsTheOnlyTest.
	// This hash comes from an MD5 hash of the key/values in the trace
	const latestPositiveDigestRpcUrl = "https://testing-gold.skia.org/json/v2/latestpositivedigest/84c1168e85de827b0b958c8994485e83"
	const latestPositiveDigestResponse = `{"digest":"` + string(latestPositiveImageHash) + `"}`
	httpClient.On("Get", latestPositiveDigestRpcUrl).Return(httpResponse(latestPositiveDigestResponse, "200 OK", http.StatusOK), nil)

	// Mock out downloading the latest positive digest returned by the previous mocked RPC.
	downloader.On("DownloadImage", testutils.AnyContext, "https://testing-gold.skia.org", latestPositiveImageHash).Return(latestPositiveImageBytes, nil)

	// Mock out RPC to automatically triage the new image as positive.
	bodyMatcher := mock.MatchedBy(func(r io.Reader) bool {
		b, err := ioutil.ReadAll(r)
		assert.NoError(t, err)
		if len(b) == 0 {
			// This matcher can get called a second time during AssertExpectations. This check makes sure
			// we don't erroniously fail.
			return false
		}
		tr := frontend.TriageRequest{}
		assert.NoError(t, json.Unmarshal(b, &tr))
		assert.Equal(t, frontend.TriageRequest{
			TestDigestStatus: map[types.TestName]map[types.Digest]expectations.Label{
				"ThisIsTheOnlyTest": {
					newImageHash: expectations.Positive,
				},
			},
			ChangelistID:           "867",
			CodeReviewSystem:       "gerrit",
			ImageMatchingAlgorithm: "fuzzy",
		}, tr)
		return true
	})
	httpClient.On("Post", "https://testing-gold.skia.org/json/v2/triage", "application/json", bodyMatcher).Return(httpResponse("", "200 OK", http.StatusOK), nil)

	// Mock out uploading the JSON file with the test results to Gold.
	expectedJSONPath := "skia-gold-testing/trybot/dm-json-v1/2019/04/02/19/867__5309/117/dm-1554234843000000000.json"
	checkResults := func(g jsonio.GoldResults) bool {
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
		assert.Equal(t, jsonio.Result{
			Digest: newImageHash,
			Options: map[string]string{
				"ext":                                   "png",
				imgmatching.AlgorithmNameOptKey:         string(imageMatchingAlgorithm),
				string(imgmatching.MaxDifferentPixels):  maxDifferentPixels,
				string(imgmatching.PixelDeltaThreshold): pixelDeltaThreshold,
			},
			Key: map[string]string{
				"name":          string(testName),
				"another_notch": "emeril",
			},
		}, r)
		return true
	}
	uploader.On("UploadJSON", testutils.AnyContext, mock.MatchedBy(checkResults), filepath.Join(wd, jsonTempFile), expectedJSONPath).Return(nil)

	goldClient, err := makeGoldClient(true /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	config := makeTestSharedConfig()
	config.Key[types.CorpusField] = overRiddenCorpus
	err = goldClient.SetSharedConfig(ctx, config, false)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return newImageBytes, newImageHash, nil
	})

	extraKeys := map[string]string{
		"another_notch": "emeril",
	}

	optionalKeys := map[string]string{
		imgmatching.AlgorithmNameOptKey:         string(imageMatchingAlgorithm),
		string(imgmatching.MaxDifferentPixels):  maxDifferentPixels,
		string(imgmatching.PixelDeltaThreshold): pixelDeltaThreshold,
	}

	pass, err := goldClient.Test(ctx, testName, testImgPath, "", extraKeys, optionalKeys)
	assert.NoError(t, err)
	// Returns true because the test has been seen before and marked positive.
	assert.True(t, pass)
}

// TestNegativePassFail ensures that a digest marked negative returns false in pass-fail mode.
func TestNegativePassFail(t *testing.T) {
	unittest.MediumTest(t)

	wd := t.TempDir()

	imgData := []byte("some bytes")
	// These are defined in mockBaselineJSON
	const imgHash = "badbadbad1325855590527db196112e0"
	const testName = types.TestName("ThisIsTheOnlyTest")

	ctx, httpClient, uploader, _ := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse(imgHash, "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v1/hashes").Return(hashesResp, nil)

	exp := httpResponse(mockBaselineJSON, "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v2/expectations?issue=867&crs=gerrit").Return(exp, nil)

	// No upload expected because the bytes were already seen in json/hashes.

	expectedJSONPath := "skia-gold-testing/trybot/dm-json-v1/2019/04/02/19/867__5309/117/dm-1554234843000000000.json"
	uploader.On("UploadJSON", testutils.AnyContext, mock.AnythingOfType("jsonio.GoldResults"), filepath.Join(wd, jsonTempFile), expectedJSONPath).Return(nil)

	goldClient, err := makeGoldClient(true /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(ctx, makeTestSharedConfig(), false)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	pass, err := goldClient.Test(ctx, testName, testImgPath, "", nil, nil)
	assert.NoError(t, err)
	// Returns false because the test is negative
	assert.False(t, pass)

	// Run it again to make sure the failure log isn't truncated
	pass, err = goldClient.Test(ctx, testName, testImgPath, "", nil, nil)
	assert.NoError(t, err)
	// Returns false because the test is negative
	assert.False(t, pass)

	b, err := ioutil.ReadFile(filepath.Join(wd, failureLog))
	assert.NoError(t, err)
	assert.Equal(t, `https://testing-gold.skia.org/detail?test=ThisIsTheOnlyTest&digest=badbadbad1325855590527db196112e0&changelist_id=867&crs=gerrit
https://testing-gold.skia.org/detail?test=ThisIsTheOnlyTest&digest=badbadbad1325855590527db196112e0&changelist_id=867&crs=gerrit
`, string(b))
}

// TestPositivePassFail ensures that a positively marked digest returns true in pass-fail mode.
func TestPositivePassFail(t *testing.T) {
	unittest.MediumTest(t)

	wd := t.TempDir()

	imgData := []byte("some bytes")
	// These are defined in mockBaselineJSON
	const imgHash = "beef00d3a1527db19619ec12a4e0df68"
	const testName = types.TestName("ThisIsTheOnlyTest")

	ctx, httpClient, uploader, _ := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer uploader.AssertExpectations(t)

	hashesResp := httpResponse(imgHash, "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v1/hashes").Return(hashesResp, nil)

	exp := httpResponse(mockBaselineJSON, "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v2/expectations?issue=867&crs=gerrit").Return(exp, nil)

	// No upload expected because the bytes were already seen in json/hashes.

	expectedJSONPath := "skia-gold-testing/trybot/dm-json-v1/2019/04/02/19/867__5309/117/dm-1554234843000000000.json"
	uploader.On("UploadJSON", testutils.AnyContext, mock.AnythingOfType("jsonio.GoldResults"), filepath.Join(wd, jsonTempFile), expectedJSONPath).Return(nil)

	goldClient, err := makeGoldClient(true /*=passFail*/, false /*=uploadOnly*/, wd)
	assert.NoError(t, err)
	err = goldClient.SetSharedConfig(ctx, makeTestSharedConfig(), false)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	pass, err := goldClient.Test(ctx, testName, testImgPath, "", nil, nil)
	assert.NoError(t, err)
	// Returns true because this test has been seen before and the digest was
	// previously triaged positive.
	assert.True(t, pass)
}

// TestCheckSunnyDay emulates running goldctl auth; goldctl imgtest check ... where the
// passed in image matches something on the baseline
func TestCheckSunnyDay(t *testing.T) {
	unittest.MediumTest(t)

	wd := t.TempDir()

	imgData := []byte("some bytes")
	// These are defined in mockBaselineJSON
	const imgHash = "beef00d3a1527db19619ec12a4e0df68"
	const testName = types.TestName("ThisIsTheOnlyTest")

	ctx, httpClient, _, _ := makeMocks()
	defer httpClient.AssertExpectations(t)

	hashesResp := httpResponse(imgHash, "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v1/hashes").Return(hashesResp, nil)

	exp := httpResponse(mockBaselineJSON, "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v2/expectations").Return(exp, nil)

	config := GoldClientConfig{
		WorkDir:    wd,
		InstanceID: "testing",
	}
	goldClient, err := NewCloudClient(config)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	pass, err := goldClient.Check(ctx, testName, testImgPath, nil, nil)
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

	wd := t.TempDir()

	imgData := []byte("some bytes")
	// These are defined in mockBaselineJSON
	const imgHash = "beef00d3a1527db19619ec12a4e0df68"
	const testName = types.TestName("ThisIsTheOnlyTest")
	const githubCRS = "github"
	const changelistID = "abc"

	ctx, httpClient, _, _ := makeMocks()
	defer httpClient.AssertExpectations(t)

	hashesResp := httpResponse(imgHash, "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v1/hashes").Return(hashesResp, nil)

	exp := httpResponse(mockBaselineJSON, "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v2/expectations?issue=abc&crs=github").Return(exp, nil)

	config := GoldClientConfig{
		WorkDir:    wd,
		InstanceID: "testing",
	}
	goldClient, err := NewCloudClient(config)
	assert.NoError(t, err)

	gr := jsonio.GoldResults{
		CodeReviewSystem: githubCRS,
		ChangelistID:     changelistID,
		GitHash:          "HEAD",
	}
	err = goldClient.SetSharedConfig(ctx, gr, true)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	pass, err := goldClient.Check(ctx, testName, testImgPath, nil, nil)
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

	wd := t.TempDir()

	imgData := []byte("some bytes")
	// imgHash is not seen in expectations
	const imgHash = "4043142d1ec36177e8c6c4d31af0c6de"
	const testName = types.TestName("ThisIsTheOnlyTest")

	ctx, httpClient, _, _ := makeMocks()
	defer httpClient.AssertExpectations(t)

	hashesResp := httpResponse(imgHash, "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v1/hashes").Return(hashesResp, nil)

	exp := httpResponse(mockBaselineJSON, "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v2/expectations").Return(exp, nil)

	config := GoldClientConfig{
		WorkDir:    wd,
		InstanceID: "testing",
	}
	goldClient, err := NewCloudClient(config)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	pass, err := goldClient.Check(ctx, testName, testImgPath, nil, nil)
	assert.NoError(t, err)
	assert.False(t, pass)
}

// TestCheckLoad emulates running goldctl auth; goldctl imgtest check ...; goldctl imgtest check...
// specifically focusing on loading from disk after the first check and not querying the
// backend every time.
func TestCheckLoad(t *testing.T) {
	unittest.MediumTest(t)

	wd := t.TempDir()

	imgData := []byte("some bytes")
	// These are defined in mockBaselineJSON
	const imgHash = "beef00d3a1527db19619ec12a4e0df68"
	const testName = types.TestName("ThisIsTheOnlyTest")

	ctx, httpClient, _, _ := makeMocks()
	defer httpClient.AssertExpectations(t)

	hashesResp := httpResponse(imgHash, "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v1/hashes").Return(hashesResp, nil).Once()

	exp := httpResponse(mockBaselineJSON, "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v2/expectations").Return(exp, nil).Once()

	config := GoldClientConfig{
		WorkDir:    wd,
		InstanceID: "testing",
	}
	goldClient, err := NewCloudClient(config)
	assert.NoError(t, err)

	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})

	pass, err := goldClient.Check(ctx, testName, testImgPath, nil, nil)
	assert.NoError(t, err)
	assert.True(t, pass)

	// Reload saved state from disk
	goldClient, err = LoadCloudClient(wd)
	assert.NoError(t, err)
	overrideLoadAndHashImage(goldClient, func(path string) ([]byte, types.Digest, error) {
		assert.Equal(t, testImgPath, path)
		return imgData, imgHash, nil
	})
	pass, err = goldClient.Check(ctx, testName, testImgPath, nil, nil)
	assert.NoError(t, err)
	assert.True(t, pass)
}

// TestCheckLoadFails make sure that if we load from an empty directory, we fail to initialize
// a GoldClient.
func TestCheckLoadFails(t *testing.T) {
	unittest.MediumTest(t)

	wd := t.TempDir()

	// This should not work
	_, err := LoadCloudClient(wd)
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

	wd := t.TempDir()
	outDir := filepath.Join(wd, "out")

	inputPath := filepath.Join(wd, "input.png")
	input, err := os.Create(inputPath)
	require.NoError(t, err)
	require.NoError(t, png.Encode(input, image1))
	require.NoError(t, input.Close())

	ctx, httpClient, _, dlr := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer dlr.AssertExpectations(t)

	digests := httpResponse(mockDigestsJSON, "200 OK", http.StatusOK)
	httpClient.On("Get", "https://testing-gold.skia.org/json/v2/digests?grouping=name%3DThis%2BIsTheOnly%2BTest%26source_type%3DThis%2BHas%2Bspaces").Return(digests, nil)

	img2 := imageToPngBytes(t, image2)
	dlr.On("DownloadImage", testutils.AnyContext, "https://testing-gold.skia.org", types.Digest(rightHash)).Return(img2, nil)
	img3 := imageToPngBytes(t, image3)
	dlr.On("DownloadImage", testutils.AnyContext, "https://testing-gold.skia.org", types.Digest(otherHash)).Return(img3, nil)

	config := GoldClientConfig{
		WorkDir:    wd,
		InstanceID: "testing",
	}
	goldClient, err := NewCloudClient(config)
	require.NoError(t, err)

	grouping := paramtools.Params{
		types.CorpusField:     corpus,
		types.PrimaryKeyField: string(testName),
	}
	err = goldClient.Diff(ctx, grouping, inputPath, outDir)
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
	const rightHash = types.Digest("bbb0dc56d0429ef3586629787666ce09")
	const otherHash = types.Digest("ccc2912653148661835084a809fee263")

	wd := t.TempDir()
	outDir := filepath.Join(wd, "out")

	inputPath := filepath.Join(wd, "input.png")
	input, err := os.Create(inputPath)
	require.NoError(t, err)
	require.NoError(t, png.Encode(input, image1))
	require.NoError(t, input.Close())

	ctx, httpClient, _, dlr := makeMocks()
	defer httpClient.AssertExpectations(t)
	defer dlr.AssertExpectations(t)

	httpClient.On("Get", "https://testing-gold.skia.org/json/v2/digests?grouping=name%3DThisIsTheOnlyTest%26source_type%3Dwhatever").Return(func(_ string) *http.Response {
		// return a fresh response each time Diff is called
		return httpResponse(mockDigestsJSON, "200 OK", http.StatusOK)
	}, nil).Twice()

	img2 := imageToPngBytes(t, image2)
	dlr.On("DownloadImage", testutils.AnyContext, "https://testing-gold.skia.org", rightHash).Return(img2, nil)
	img3 := imageToPngBytes(t, image3)
	dlr.On("DownloadImage", testutils.AnyContext, "https://testing-gold.skia.org", otherHash).Return(img3, nil)

	config := GoldClientConfig{
		WorkDir:    wd,
		InstanceID: "testing",
	}
	goldClient, err := NewCloudClient(config)
	require.NoError(t, err)

	grouping := paramtools.Params{
		types.CorpusField:     corpus,
		types.PrimaryKeyField: string(testName),
	}

	err = goldClient.Diff(ctx, grouping, inputPath, outDir)
	require.NoError(t, err)

	// Call it twice to make sure we only hit GCS once per file
	err = goldClient.Diff(ctx, grouping, inputPath, outDir)
	require.NoError(t, err)
}

func TestMakeResultKeyAndTraceId_Success(t *testing.T) {
	unittest.SmallTest(t)

	const instanceId = "my_instance"
	const testName = types.TestName("my_test")

	tests := []struct {
		name              string
		sharedConfig      jsonio.GoldResults
		additionalKeys    map[string]string
		expectedResultKey map[string]string
		expectedTrace     paramtools.Params
	}{
		{
			name: "no additional keys, nil shared config, corpus set to instance ID",
			expectedResultKey: map[string]string{
				types.PrimaryKeyField: "my_test",
				types.CorpusField:     instanceId,
			},
			expectedTrace: paramtools.Params{"name": "my_test", "source_type": instanceId},
		},
		{
			name:         "no additional keys, empty shared config, corpus set to instance ID",
			sharedConfig: jsonio.GoldResults{},
			expectedResultKey: map[string]string{
				types.PrimaryKeyField: "my_test",
				types.CorpusField:     instanceId,
			},
			expectedTrace: paramtools.Params{"name": "my_test", "source_type": instanceId},
		},
		{
			name: "no additional keys, shared config with corpus, uses corpus from shared config",
			sharedConfig: jsonio.GoldResults{
				Key: map[string]string{types.CorpusField: "my_corpus"},
			},
			expectedResultKey: map[string]string{
				types.PrimaryKeyField: "my_test",
			},
			expectedTrace: paramtools.Params{"name": "my_test", "source_type": "my_corpus"},
		},
		{
			name:           "additional keys with corpus, empty shared config, uses corpus from additional keys",
			sharedConfig:   jsonio.GoldResults{},
			additionalKeys: map[string]string{types.CorpusField: "my_corpus"},
			expectedResultKey: map[string]string{
				types.PrimaryKeyField: "my_test",
				types.CorpusField:     "my_corpus",
			},
			expectedTrace: paramtools.Params{"name": "my_test", "source_type": "my_corpus"},
		},
		{
			name: "additional keys with corpus, shared config with corpus, uses corpus from additional keys",
			sharedConfig: jsonio.GoldResults{
				Key: map[string]string{types.CorpusField: "corpus_from_shared_config"},
			},
			additionalKeys: map[string]string{types.CorpusField: "my_corpus"},
			expectedResultKey: map[string]string{
				types.PrimaryKeyField: "my_test",
				types.CorpusField:     "my_corpus",
			},
			expectedTrace: paramtools.Params{"name": "my_test", "source_type": "my_corpus"},
		},
		{
			name: "overlapping shared and additional keys, additional keys take precedence",
			sharedConfig: jsonio.GoldResults{
				Key: map[string]string{
					types.CorpusField: "corpus_from_shared_config",
					"overlapping_key": "alpha",
					"shared_key":      "foo",
				},
			},
			additionalKeys: map[string]string{
				types.CorpusField: "my_corpus",
				"overlapping_key": "beta",
				"additional_key":  "bar",
			},
			expectedResultKey: map[string]string{
				types.PrimaryKeyField: "my_test",
				types.CorpusField:     "my_corpus",
				"overlapping_key":     "beta",
				"additional_key":      "bar",
			},
			expectedTrace: paramtools.Params{
				types.PrimaryKeyField: "my_test",
				types.CorpusField:     "my_corpus",
				"shared_key":          "foo",
				"overlapping_key":     "beta",
				"additional_key":      "bar",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			goldClient := CloudClient{
				resultState: newResultState(tc.sharedConfig, &GoldClientConfig{
					InstanceID: instanceId,
				}),
			}

			resultKey, traceID := goldClient.makeResultKeyAndTraceId(testName, tc.additionalKeys)
			assert.Equal(t, tc.expectedResultKey, resultKey)
			_, tb := sql.SerializeMap(tc.expectedTrace)
			expectedTraceID := tiling.TraceIDV2(hex.EncodeToString(tb))
			assert.Equal(t, expectedTraceID, traceID)
		})
	}
}

func TestCloudClient_MatchImageAgainstBaseline_NoAlgorithmSpecified_DefaultsToExactMatching_Success(t *testing.T) {
	unittest.MediumTest(t) // This test reads/writes a small amount of data from/to disk.

	const testName = types.TestName("my_test")
	const digest = types.Digest("11111111111111111111111111111111")
	const unlabeled = expectations.Label("unlabeled") // Sentinel value.

	test := func(name string, label expectations.Label, want bool) {
		t.Run(name, func(t *testing.T) {
			goldClient, ctx, _, _ := makeGoldClientForMatchImageAgainstBaselineTests(t)

			if label != unlabeled {
				goldClient.resultState.Expectations = expectations.Baseline{
					testName: {
						digest: label,
					},
				}
			}

			// Parameters traceId and imageBytes are not used in exact matching.
			got, algorithmName, err := goldClient.matchImageAgainstBaseline(ctx, testName, "" /* =traceId */, []byte{} /* =imageBytes */, digest, nil /* =optionalKeys */)

			assert.NoError(t, err)
			assert.Equal(t, imgmatching.ExactMatching, algorithmName)
			assert.Equal(t, want, got)
		})
	}

	test("image label positive, returns true", expectations.Positive, true)
	test("image label negative, returns false", expectations.Negative, false)
	test("image label untriaged, returns false", expectations.Untriaged, false)
	test("image unlabeled, returns false", unlabeled, false)
}

func TestCloudClient_MatchImageAgainstBaseline_ExactMatching_Success(t *testing.T) {
	unittest.MediumTest(t) // This test reads/writes a small amount of data from/to disk.

	const testName = types.TestName("my_test")
	const digest = types.Digest("11111111111111111111111111111111")
	const unlabeled = expectations.Label("unlabeled") // Sentinel value.

	test := func(name string, label expectations.Label, want bool) {
		t.Run(name, func(t *testing.T) {
			goldClient, ctx, _, _ := makeGoldClientForMatchImageAgainstBaselineTests(t)

			if label != unlabeled {
				goldClient.resultState.Expectations = expectations.Baseline{
					testName: {
						digest: label,
					},
				}
			}

			optionalKeys := map[string]string{
				imgmatching.AlgorithmNameOptKey: string(imgmatching.ExactMatching),
			}

			// Parameters traceId and imageBytes are not used in exact matching.
			got, algorithmName, err := goldClient.matchImageAgainstBaseline(ctx, testName, "" /* =traceId */, []byte{} /* =imageBytes */, digest, optionalKeys)

			assert.NoError(t, err)
			assert.Equal(t, imgmatching.ExactMatching, algorithmName)
			assert.Equal(t, want, got)
		})
	}

	test("image labeled positive, returns true", expectations.Positive, true)
	test("image labeled negative, returns false", expectations.Negative, false)
	test("image labeled untriaged, returns false", expectations.Untriaged, false)
	test("image unlabeled, returns false", unlabeled, false)
}

func TestCloudClient_MatchImageAgainstBaseline_FuzzyMatching_ImageAlreadyLabeled_Success(t *testing.T) {
	unittest.MediumTest(t) // This test reads/writes a small amount of data from/to disk.

	test := func(name string, label expectations.Label, want bool) {
		t.Run(name, func(t *testing.T) {
			goldClient, ctx, _, _ := makeGoldClientForMatchImageAgainstBaselineTests(t)

			const testName = types.TestName("my_test")
			const digest = types.Digest("11111111111111111111111111111111")
			optionalKeys := map[string]string{
				imgmatching.AlgorithmNameOptKey: string(imgmatching.FuzzyMatching),
				// These optionalKeys do not matter because the algorithm is not exercised by this test.
				string(imgmatching.MaxDifferentPixels):  "0",
				string(imgmatching.PixelDeltaThreshold): "0",
			}

			goldClient.resultState.Expectations = expectations.Baseline{
				testName: {
					digest: label,
				},
			}

			got, algorithmName, err := goldClient.matchImageAgainstBaseline(ctx, testName, "" /* =traceId */, nil /* =imageBytes */, digest, optionalKeys)
			assert.NoError(t, err)
			assert.Equal(t, imgmatching.ExactMatching, algorithmName)
			assert.Equal(t, want, got)
		})
	}

	test("labeled positive, returns true", expectations.Positive, true)
	test("labeled negative, returns false", expectations.Negative, false)
}

func TestCloudClient_MatchImageAgainstBaseline_FuzzyMatching_UntriagedImage_Success(t *testing.T) {
	unittest.MediumTest(t) // This test reads/writes a small amount of data from/to disk.

	const testName = types.TestName("my_test")
	const traceId = tiling.TraceIDV2("1234567890abcdef1234567890abcdef")
	const digest = types.Digest("11111111111111111111111111111111")

	const latestPositiveDigestRpcUrl = "https://testing-gold.skia.org/json/v2/latestpositivedigest/1234567890abcdef1234567890abcdef"
	const latestPositiveDigestResponse = `{"digest":"22222222222222222222222222222222"}`
	const latestPositiveDigest = types.Digest("22222222222222222222222222222222")
	latestPositiveImageBytes := imageToPngBytes(t, text.MustToNRGBA(`! SKTEXTSIMPLE
	2 2
	0x00000000 0x00000000
	0x00000000 0x00000000`))

	// FuzzyMatching algorithm parameters.
	const maxDifferentPixels = "2"
	const pixelDeltaThreshold = "10"

	tests := []struct {
		name       string
		imageBytes []byte
		expected   bool
	}{
		{
			name: "identical images, returns true",
			imageBytes: imageToPngBytes(t, text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`)),
			expected: true,
		},
		{
			name: "images different below threshold, returns true",
			imageBytes: imageToPngBytes(t, text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000505 0x00000001
			0x00000000 0x00000000`)),
			expected: true,
		},
		{
			name: "images different above threshold, returns false",
			imageBytes: imageToPngBytes(t, text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000506 0x00000001
			0x00000001 0x00000000`)),
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			goldClient, ctx, httpClient, dlr := makeGoldClientForMatchImageAgainstBaselineTests(t)
			defer httpClient.AssertExpectations(t)
			defer dlr.AssertExpectations(t)

			httpClient.On("Get", latestPositiveDigestRpcUrl).Return(httpResponse(latestPositiveDigestResponse, "200 OK", http.StatusOK), nil)
			dlr.On("DownloadImage", testutils.AnyContext, "https://testing-gold.skia.org", latestPositiveDigest).Return(latestPositiveImageBytes, nil)

			optionalKeys := map[string]string{
				imgmatching.AlgorithmNameOptKey:         string(imgmatching.FuzzyMatching),
				string(imgmatching.MaxDifferentPixels):  maxDifferentPixels,
				string(imgmatching.PixelDeltaThreshold): pixelDeltaThreshold,
			}

			actual, algorithmName, err := goldClient.matchImageAgainstBaseline(ctx, testName, traceId, tc.imageBytes, digest, optionalKeys)
			assert.NoError(t, err)
			assert.Equal(t, imgmatching.FuzzyMatching, algorithmName)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestCloudClient_MatchImageAgainstBaseline_FuzzyMatching_InvalidParameters_ReturnsError(t *testing.T) {
	unittest.MediumTest(t) // This test reads/writes a small amount of data from/to disk.

	tests := []struct {
		name         string
		optionalKeys map[string]string
		error        string
	}{
		{
			name: "insufficient parameters: no parameter specified",
			optionalKeys: map[string]string{
				imgmatching.AlgorithmNameOptKey: string(imgmatching.FuzzyMatching),
			},
			error: "required image matching parameter not found",
		},
		{
			name: "insufficient parameters: only some parameters specified",
			optionalKeys: map[string]string{
				imgmatching.AlgorithmNameOptKey:        string(imgmatching.FuzzyMatching),
				string(imgmatching.MaxDifferentPixels): "0",
			},
			error: "required image matching parameter not found",
		},
		{
			name: "invalid parameters",
			optionalKeys: map[string]string{
				imgmatching.AlgorithmNameOptKey:         string(imgmatching.FuzzyMatching),
				string(imgmatching.MaxDifferentPixels):  "not a number",
				string(imgmatching.PixelDeltaThreshold): "not a number",
			},
			error: "parsing integer value",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			goldClient, ctx, _, _ := makeGoldClientForMatchImageAgainstBaselineTests(t)

			_, _, err := goldClient.matchImageAgainstBaseline(ctx, "my_test", "" /* =traceId */, nil /* =imageBytes */, "11111111111111111111111111111111", tc.optionalKeys)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.error)
		})
	}
}

func TestCloudClient_MatchImageAgainstBaseline_FuzzyMatching_NoRecentPositiveDigests_ReturnsFalse(t *testing.T) {
	unittest.MediumTest(t) // This test reads/writes a small amount of data from/to disk.

	const testName = types.TestName("my_test")
	const traceId = tiling.TraceIDV2("1234567890abcdef1234567890abcdef")
	const digest = types.Digest("11111111111111111111111111111111")
	imageBytes := imageToPngBytes(t, text.MustToNRGBA(`! SKTEXTSIMPLE
	1 1
	0x00000000`))

	const latestPositiveDigestRpcUrl = "https://testing-gold.skia.org/json/v2/latestpositivedigest/1234567890abcdef1234567890abcdef"
	const latestPositiveDigestResponse = `{"digest":""}`

	goldClient, ctx, httpClient, _ := makeGoldClientForMatchImageAgainstBaselineTests(t)
	defer httpClient.AssertExpectations(t)

	httpClient.On("Get", latestPositiveDigestRpcUrl).Return(httpResponse(latestPositiveDigestResponse, "200 OK", http.StatusOK), nil)

	optionalKeys := map[string]string{
		imgmatching.AlgorithmNameOptKey:         string(imgmatching.FuzzyMatching),
		string(imgmatching.MaxDifferentPixels):  "0",
		string(imgmatching.PixelDeltaThreshold): "0",
	}

	matched, algorithmName, err := goldClient.matchImageAgainstBaseline(ctx, testName, traceId, imageBytes, digest, optionalKeys)
	assert.NoError(t, err)
	assert.False(t, matched)
	assert.Equal(t, imgmatching.FuzzyMatching, algorithmName)
}

func TestCloudClient_MatchImageAgainstBaseline_SobelFuzzyMatching_ImageAlreadyLabeled_Success(t *testing.T) {
	unittest.MediumTest(t) // This test reads/writes a small amount of data from/to disk.

	test := func(name string, label expectations.Label, want bool) {
		t.Run(name, func(t *testing.T) {
			goldClient, ctx, _, _ := makeGoldClientForMatchImageAgainstBaselineTests(t)

			const testName = types.TestName("my_test")
			const digest = types.Digest("11111111111111111111111111111111")
			optionalKeys := map[string]string{
				imgmatching.AlgorithmNameOptKey: string(imgmatching.SobelFuzzyMatching),
				// These optionalKeys do not matter because the algorithm is not exercised by this test.
				string(imgmatching.EdgeThreshold):       "0",
				string(imgmatching.MaxDifferentPixels):  "0",
				string(imgmatching.PixelDeltaThreshold): "0",
			}

			goldClient.resultState.Expectations = expectations.Baseline{
				testName: {
					digest: label,
				},
			}

			got, algorithmName, err := goldClient.matchImageAgainstBaseline(ctx, testName, "" /* =traceId */, nil /* =imageBytes */, digest, optionalKeys)
			assert.NoError(t, err)
			assert.Equal(t, imgmatching.ExactMatching, algorithmName)
			assert.Equal(t, want, got)
		})
	}

	test("labeled positive, returns true", expectations.Positive, true)
	test("labeled negative, returns false", expectations.Negative, false)
}

func TestCloudClient_MatchImageAgainstBaseline_SobelFuzzyMatching_UntriagedImage_Success(t *testing.T) {
	unittest.MediumTest(t) // This test reads/writes a small amount of data from/to disk.

	const testName = types.TestName("my_test")
	const traceId = tiling.TraceIDV2("1234567890abcdef1234567890abcdef")
	const digest = types.Digest("11111111111111111111111111111111")
	testImageBytes := imageToPngBytes(t, text.MustToNRGBA(`! SKTEXTSIMPLE
	8 8
	0x44 0x44 0x44 0x44 0x44 0x44 0x49 0x83
	0x44 0x44 0x44 0x44 0x44 0x49 0x83 0x88
	0x44 0x44 0x47 0x49 0x55 0x83 0x88 0x88
	0x44 0x44 0x49 0x4D 0x7F 0x87 0x88 0x88
	0x44 0x44 0x55 0x7F 0x88 0x88 0x88 0x88
	0x44 0x49 0x83 0x87 0x88 0x88 0x88 0x88
	0x49 0x83 0x88 0x88 0x88 0x88 0x88 0x88
	0x83 0x88 0x88 0x88 0x88 0x88 0x88 0x88`))

	const latestPositiveDigestRpcUrl = "https://testing-gold.skia.org/json/v2/latestpositivedigest/1234567890abcdef1234567890abcdef"
	const latestPositiveDigestResponse = `{"digest":"22222222222222222222222222222222"}`
	const latestPositiveDigest = types.Digest("22222222222222222222222222222222")
	latestPositiveImageBytes := imageToPngBytes(t, text.MustToNRGBA(`! SKTEXTSIMPLE
	8 8
	0x44 0x44 0x44 0x44 0x44 0x44 0x49 0x83
	0x44 0x44 0x44 0x44 0x44 0x49 0x83 0x88
	0x44 0x44 0x44 0x44 0x49 0x83 0x88 0x88
	0x44 0x44 0x44 0x49 0x83 0x88 0x88 0x88
	0x44 0x44 0x49 0x83 0x88 0x88 0x88 0x88
	0x44 0x49 0x83 0x88 0x88 0x88 0x88 0x88
	0x49 0x83 0x88 0x88 0x88 0x88 0x88 0x88
	0x83 0x88 0x88 0x88 0x88 0x88 0x88 0x88`))

	test := func(name, edgeThreshold string, expected bool) {
		t.Run(name, func(t *testing.T) {
			goldClient, ctx, httpClient, dlr := makeGoldClientForMatchImageAgainstBaselineTests(t)
			defer httpClient.AssertExpectations(t)
			defer dlr.AssertExpectations(t)

			httpClient.On("Get", latestPositiveDigestRpcUrl).Return(httpResponse(latestPositiveDigestResponse, "200 OK", http.StatusOK), nil)
			dlr.On("DownloadImage", testutils.AnyContext, "https://testing-gold.skia.org", latestPositiveDigest).Return(latestPositiveImageBytes, nil)

			optionalKeys := map[string]string{
				imgmatching.AlgorithmNameOptKey:         string(imgmatching.SobelFuzzyMatching),
				string(imgmatching.MaxDifferentPixels):  "2",
				string(imgmatching.PixelDeltaThreshold): "10",
				string(imgmatching.EdgeThreshold):       edgeThreshold,
			}

			actual, algorithmName, err := goldClient.matchImageAgainstBaseline(ctx, testName, traceId, testImageBytes, digest, optionalKeys)
			assert.NoError(t, err)
			assert.Equal(t, imgmatching.SobelFuzzyMatching, algorithmName)
			assert.Equal(t, expected, actual)
		})
	}

	test("differences under threshold, returns true", "0x66", true)
	test("differences above threshold, returns false", "0xAA", false)
}

func TestCloudClient_MatchImageAgainstBaseline_SobelFuzzyMatching_InvalidParameters_ReturnsError(t *testing.T) {
	unittest.MediumTest(t) // This test reads/writes a small amount of data from/to disk.

	tests := []struct {
		name         string
		optionalKeys map[string]string
		error        string
	}{
		{
			name: "insufficient parameters: no parameter specified",
			optionalKeys: map[string]string{
				imgmatching.AlgorithmNameOptKey: string(imgmatching.SobelFuzzyMatching),
			},
			error: "required image matching parameter not found",
		},

		{
			name: "insufficient parameters: only SobelFuzzyMatching-specific parameter specified",
			optionalKeys: map[string]string{
				imgmatching.AlgorithmNameOptKey:   string(imgmatching.SobelFuzzyMatching),
				string(imgmatching.EdgeThreshold): "0",
			},
			error: "required image matching parameter not found",
		},
		{
			name: "insufficient parameters: only FuzzyMatching-specific parameter specified",
			optionalKeys: map[string]string{
				imgmatching.AlgorithmNameOptKey:         string(imgmatching.SobelFuzzyMatching),
				string(imgmatching.MaxDifferentPixels):  "0",
				string(imgmatching.PixelDeltaThreshold): "0",
			},
			error: "required image matching parameter not found",
		},
		{
			name: "invalid parameters",
			optionalKeys: map[string]string{
				imgmatching.AlgorithmNameOptKey:         string(imgmatching.SobelFuzzyMatching),
				string(imgmatching.EdgeThreshold):       "not a number",
				string(imgmatching.MaxDifferentPixels):  "not a number",
				string(imgmatching.PixelDeltaThreshold): "not a number",
			},
			error: "parsing integer value",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			goldClient, ctx, _, _ := makeGoldClientForMatchImageAgainstBaselineTests(t)

			_, _, err := goldClient.matchImageAgainstBaseline(ctx, "my_test", "" /* =traceId */, nil /* =imageBytes */, "11111111111111111111111111111111", tc.optionalKeys)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.error)
		})
	}
}

func TestCloudClient_MatchImageAgainstBaseline_UnknownAlgorithm_ReturnsError(t *testing.T) {
	unittest.MediumTest(t) // This test reads/writes a small amount of data from/to disk.

	goldClient, ctx, _, _ := makeGoldClientForMatchImageAgainstBaselineTests(t)

	optionalKeys := map[string]string{
		imgmatching.AlgorithmNameOptKey: "unknown algorithm",
	}

	_, _, err := goldClient.matchImageAgainstBaseline(ctx, "" /* =testName */, "" /* =traceId */, nil /* =imageBytes */, "" /* =digest */, optionalKeys)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unrecognized image matching algorithm")
}

func TestCloudClient_GetDigestFromCacheOrGCS_NotInCache_DownloadsImageFromGCS_Success(t *testing.T) {
	unittest.MediumTest(t) // This tests reads/writes a small amount of data from/to disk.

	wd := t.TempDir()

	ctx, _, _, gcsDownloader := makeMocks()
	gcsDownloader.AssertExpectations(t)

	goldClient, err := NewCloudClient(GoldClientConfig{
		WorkDir:    wd,
		InstanceID: "testing",
	})
	assert.NoError(t, err)

	const digest = types.Digest("11111111111111111111111111111111")
	digestImage := image1
	digestBytes := imageToPngBytes(t, image1)

	gcsDownloader.On("DownloadImage", testutils.AnyContext, "https://testing-gold.skia.org", digest).Return(digestBytes, nil)

	actualImage, actualBytes, err := goldClient.getDigestFromCacheOrGCS(ctx, digest)
	assert.NoError(t, err)
	assert.Equal(t, digestImage, actualImage)
	assert.Equal(t, digestBytes, actualBytes)
}

func TestCloudClient_GetDigestFromCacheOrGCS_NotInCache_DownloadsCorruptedImageFromGCS_Failure(t *testing.T) {
	unittest.MediumTest(t) // This tests reads/writes a small amount of data from/to disk.

	wd := t.TempDir()

	ctx, _, _, gcsDownloader := makeMocks()
	gcsDownloader.AssertExpectations(t)

	goldClient, err := NewCloudClient(GoldClientConfig{
		WorkDir:    wd,
		InstanceID: "testing",
	})
	assert.NoError(t, err)

	const digest = types.Digest("11111111111111111111111111111111")
	digestBytes := []byte("corrupted image")

	gcsDownloader.On("DownloadImage", testutils.AnyContext, "https://testing-gold.skia.org", digest).Return(digestBytes, nil)

	_, _, err = goldClient.getDigestFromCacheOrGCS(ctx, digest)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decoding PNG file at "+filepath.Join(wd, digestsDirectory, string(digest))+".png")
}

func TestCloudClient_GetDigestFromCacheOrGCS_InCache_ReadsImageFromDisk_Success(t *testing.T) {
	unittest.MediumTest(t) // This tests reads/writes a small amount of data from/to disk.

	wd := t.TempDir()

	ctx, _, _, _ := makeMocks()

	goldClient, err := NewCloudClient(GoldClientConfig{
		WorkDir:    wd,
		InstanceID: "testing",
	})
	assert.NoError(t, err)

	const digest = types.Digest("11111111111111111111111111111111")
	digestImage := image1
	digestBytes := imageToPngBytes(t, image1)

	// Make cache directory that will contain the cached digest.
	err = os.MkdirAll(filepath.Join(wd, digestsDirectory), os.ModePerm)
	assert.NoError(t, err)

	// Write cached digest to disk.
	err = ioutil.WriteFile(filepath.Join(wd, digestsDirectory, string(digest)+".png"), digestBytes, os.ModePerm)
	assert.NoError(t, err)

	actualImage, actualBytes, err := goldClient.getDigestFromCacheOrGCS(ctx, digest)
	assert.NoError(t, err)
	assert.Equal(t, digestImage, actualImage)
	assert.Equal(t, digestBytes, actualBytes)
}

func TestCloudClient_GetDigestFromCacheOrGCS_InCache_ReadsCorruptedImageFromDisk_Failure(t *testing.T) {
	unittest.MediumTest(t) // This tests reads/writes a small amount of data from/to disk.

	wd := t.TempDir()

	ctx, _, _, _ := makeMocks()

	goldClient, err := NewCloudClient(GoldClientConfig{
		WorkDir:    wd,
		InstanceID: "testing",
	})
	assert.NoError(t, err)

	const digest = types.Digest("11111111111111111111111111111111")
	digestBytes := []byte("corrupted image")

	// Make cache directory that will contain the cached digest.
	err = os.MkdirAll(filepath.Join(wd, digestsDirectory), os.ModePerm)
	assert.NoError(t, err)

	// Write cached digest to disk.
	err = ioutil.WriteFile(filepath.Join(wd, digestsDirectory, string(digest)+".png"), digestBytes, os.ModePerm)
	assert.NoError(t, err)

	_, _, err = goldClient.getDigestFromCacheOrGCS(ctx, digest)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decoding PNG file at "+filepath.Join(wd, digestsDirectory, string(digest))+".png")
}

func TestCloudClient_Whoami_Success(t *testing.T) {
	// This test reads and writes a small amount of data from/to disk.
	unittest.MediumTest(t)

	wd := t.TempDir()

	ctx, httpClient, _, _ := makeMocks()
	defer httpClient.AssertExpectations(t)

	config := GoldClientConfig{
		WorkDir:    wd,
		InstanceID: "testing",
	}
	goldClient, err := NewCloudClient(config)
	assert.NoError(t, err)

	url := "https://testing-gold.skia.org/json/v1/whoami"
	response := `{"whoami": "test@example.com"}`
	httpClient.On("Get", url).Return(httpResponse(response, "200 OK", http.StatusOK), nil)

	email, err := goldClient.Whoami(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "test@example.com", email)
}

func TestCloudClient_Whoami_InternalServerError_Failure(t *testing.T) {
	// This test takes >30 seconds to retry a bunch.
	unittest.LargeTest(t)

	wd := t.TempDir()

	ctx, httpClient, _, _ := makeMocks()
	defer httpClient.AssertExpectations(t)

	config := GoldClientConfig{
		WorkDir:    wd,
		InstanceID: "testing",
	}
	goldClient, err := NewCloudClient(config)
	assert.NoError(t, err)

	url := "https://testing-gold.skia.org/json/v1/whoami"
	httpClient.On("Get", url).Return(httpResponse("", "500 Internal Server Error", http.StatusInternalServerError), nil)

	_, err = goldClient.Whoami(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestCloudClient_TriageAsPositive_NoCL_Success(t *testing.T) {
	// This test reads and writes a small amount of data from/to disk.
	unittest.MediumTest(t)

	wd := t.TempDir()

	// Pretend "goldctl imgtest init" was called.
	j := resultState{
		GoldURL:      "https://testing-gold.skia.org",
		SharedConfig: jsonio.GoldResults{},
	}
	jsonToWrite := testutils.MarshalJSON(t, &j)
	testutils.WriteFile(t, filepath.Join(wd, stateFile), jsonToWrite)

	ctx, httpClient, _, _ := makeMocks()
	defer httpClient.AssertExpectations(t)

	goldClient, err := LoadCloudClient(wd)
	assert.NoError(t, err)

	url := "https://testing-gold.skia.org/json/v2/triage"
	contentType := "application/json"
	bodyMatcher := mock.MatchedBy(func(r io.Reader) bool {
		b, err := ioutil.ReadAll(r)
		assert.NoError(t, err)
		if len(b) == 0 {
			// This matcher can get called a second time during AssertExpectations. This check makes sure
			// we don't erroniously fail.
			return false
		}
		tr := frontend.TriageRequest{}
		assert.NoError(t, json.Unmarshal(b, &tr))
		assert.Equal(t, frontend.TriageRequest{
			TestDigestStatus: map[types.TestName]map[types.Digest]expectations.Label{
				"MyTest": {
					"deadbeefcafefe771d61bf0ed3d84bc2": expectations.Positive,
				},
			},
			ImageMatchingAlgorithm: "fuzzy",
		}, tr)
		return true
	})
	httpClient.On("Post", url, contentType, bodyMatcher).Return(httpResponse("", "200 OK", http.StatusOK), nil)

	err = goldClient.TriageAsPositive(ctx, "MyTest", "deadbeefcafefe771d61bf0ed3d84bc2", "fuzzy")
	assert.NoError(t, err)
}

func TestCloudClient_TriageAsPositive_WithCL_Success(t *testing.T) {
	// This test reads and writes a small amount of data from/to disk.
	unittest.MediumTest(t)

	wd := t.TempDir()

	// Pretend "goldctl imgtest init" was called.
	j := resultState{
		GoldURL: "https://testing-gold.skia.org",
		SharedConfig: jsonio.GoldResults{
			CodeReviewSystem: "gerrit",
			ChangelistID:     "123456",
		},
	}
	jsonToWrite := testutils.MarshalJSON(t, &j)
	testutils.WriteFile(t, filepath.Join(wd, stateFile), jsonToWrite)

	ctx, httpClient, _, _ := makeMocks()
	defer httpClient.AssertExpectations(t)

	goldClient, err := LoadCloudClient(wd)
	assert.NoError(t, err)

	url := "https://testing-gold.skia.org/json/v2/triage"
	contentType := "application/json"
	bodyMatcher := mock.MatchedBy(func(r io.Reader) bool {
		b, err := ioutil.ReadAll(r)
		assert.NoError(t, err)
		if len(b) == 0 {
			// This matcher can get called a second time during AssertExpectations. This check makes sure
			// we don't erroniously fail.
			return false
		}
		tr := frontend.TriageRequest{}
		assert.NoError(t, json.Unmarshal(b, &tr))
		assert.Equal(t, frontend.TriageRequest{
			TestDigestStatus: map[types.TestName]map[types.Digest]expectations.Label{
				"MyTest": {
					"deadbeefcafefe771d61bf0ed3d84bc2": expectations.Positive,
				},
			},
			CodeReviewSystem:       "gerrit",
			ChangelistID:           "123456",
			ImageMatchingAlgorithm: "fuzzy",
		}, tr)
		return true
	})
	httpClient.On("Post", url, contentType, bodyMatcher).Return(httpResponse("", "200 OK", http.StatusOK), nil)

	err = goldClient.TriageAsPositive(ctx, "MyTest", "deadbeefcafefe771d61bf0ed3d84bc2", "fuzzy")
	assert.NoError(t, err)
}

func TestCloudClient_TriageAsPositive_InternalServerError_Failure(t *testing.T) {
	// This test takes >30 seconds to retry a bunch.
	unittest.LargeTest(t)

	wd := t.TempDir()

	// Pretend "goldctl imgtest init" was called.
	j := resultState{
		GoldURL:      "https://testing-gold.skia.org",
		SharedConfig: jsonio.GoldResults{},
	}
	jsonToWrite := testutils.MarshalJSON(t, &j)
	testutils.WriteFile(t, filepath.Join(wd, stateFile), jsonToWrite)

	ctx, httpClient, _, _ := makeMocks()
	defer httpClient.AssertExpectations(t)

	goldClient, err := LoadCloudClient(wd)
	assert.NoError(t, err)

	url := "https://testing-gold.skia.org/json/v2/triage"
	contentType := "application/json"
	httpClient.On("Post", url, contentType, mock.Anything).Return(httpResponse("", "500 Internal Server Error", http.StatusInternalServerError), nil)

	err = goldClient.TriageAsPositive(ctx, "MyTest", "deadbeefcafefe771d61bf0ed3d84bc2", "fuzzy")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestCloudClient_MostRecentPositiveDigest_Success(t *testing.T) {
	// This test reads and writes a small amount of data from/to disk.
	unittest.MediumTest(t)

	wd := t.TempDir()

	ctx, httpClient, _, _ := makeMocks()
	defer httpClient.AssertExpectations(t)

	config := GoldClientConfig{
		WorkDir:    wd,
		InstanceID: "testing",
	}
	goldClient, err := NewCloudClient(config)
	assert.NoError(t, err)

	const traceId = tiling.TraceIDV2("1234567890abcdef1234567890abcdef")
	const url = "https://testing-gold.skia.org/json/v2/latestpositivedigest/1234567890abcdef1234567890abcdef"
	const response = `{"digest":"deadbeefcafefe771d61bf0ed3d84bc2"}`
	const expectedDigest = types.Digest("deadbeefcafefe771d61bf0ed3d84bc2")

	httpClient.On("Get", url).Return(httpResponse(response, "200 OK", http.StatusOK), nil)

	actualDigest, err := goldClient.MostRecentPositiveDigest(ctx, traceId)
	assert.NoError(t, err)
	assert.Equal(t, expectedDigest, actualDigest)
}

func TestCloudClient_MostRecentPositiveDigest_NonJSONResponse_Failure(t *testing.T) {
	// This test reads and writes a small amount of data from/to disk.
	unittest.MediumTest(t)

	wd := t.TempDir()

	ctx, httpClient, _, _ := makeMocks()
	defer httpClient.AssertExpectations(t)

	config := GoldClientConfig{
		WorkDir:    wd,
		InstanceID: "testing",
	}
	goldClient, err := NewCloudClient(config)
	assert.NoError(t, err)

	const traceId = tiling.TraceIDV2("1234567890abcdef1234567890abcdef")
	const url = "https://testing-gold.skia.org/json/v2/latestpositivedigest/1234567890abcdef1234567890abcdef"
	const response = "Not JSON"

	httpClient.On("Get", url).Return(httpResponse(response, "200 OK", http.StatusOK), nil)

	_, err = goldClient.MostRecentPositiveDigest(ctx, traceId)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshalling JSON response")
}

func TestCloudClient_MostRecentPositiveDigest_InternalServerError_Failure(t *testing.T) {
	// This test takes a while to deal with retries
	unittest.LargeTest(t)

	wd := t.TempDir()

	ctx, httpClient, _, _ := makeMocks()
	defer httpClient.AssertExpectations(t)

	config := GoldClientConfig{
		WorkDir:    wd,
		InstanceID: "testing",
	}
	goldClient, err := NewCloudClient(config)
	assert.NoError(t, err)

	const traceId = tiling.TraceIDV2("1234567890abcdef1234567890abcdef")
	const url = "https://testing-gold.skia.org/json/v2/latestpositivedigest/1234567890abcdef1234567890abcdef"

	httpClient.On("Get", url).Return(httpResponse("", "500 Internal Server Error", http.StatusInternalServerError), nil)

	_, err = goldClient.MostRecentPositiveDigest(ctx, traceId)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func makeMocks() (context.Context, *mocks.HTTPClient, *mocks.GCSUploader, *mocks.ImageDownloader) {
	mh := &mocks.HTTPClient{}
	mg := &mocks.GCSUploader{}
	md := &mocks.ImageDownloader{}
	fakeNow := time.Date(2019, time.April, 2, 19, 54, 3, 0, time.UTC)
	ctx := WithContext(context.Background(), mg, mh, md)
	ctx = context.WithValue(ctx, now.ContextKey, fakeNow)
	return ctx, mh, mg, md
}

// makeGoldClient will create new cloud client from scratch (using a
// set configuration), and return it.
func makeGoldClient(passFail bool, uploadOnly bool, workDir string) (*CloudClient, error) {
	config := GoldClientConfig{
		InstanceID:   testInstanceID,
		WorkDir:      workDir,
		FailureFile:  filepath.Join(workDir, failureLog),
		PassFailStep: passFail,
		UploadOnly:   uploadOnly,
	}

	return NewCloudClient(config)
}

// makeGoldClientForMatchImageAgainstBaselineTests returns a new CloudClient to be used in
// CloudClient#matchImageAgainstBaseline() tests.
func makeGoldClientForMatchImageAgainstBaselineTests(t *testing.T) (*CloudClient, context.Context, *mocks.HTTPClient, *mocks.ImageDownloader) {
	wd := t.TempDir()
	ctx, httpClient, _, gcsDownloader := makeMocks()
	goldClient, err := NewCloudClient(GoldClientConfig{
		WorkDir:    wd,
		InstanceID: "testing",
	})
	require.NoError(t, err)
	return goldClient, ctx, httpClient, gcsDownloader
}

func overrideLoadAndHashImage(c *CloudClient, testFn func(path string) ([]byte, types.Digest, error)) {
	c.loadAndHashImage = testFn
}

const (
	testInstanceID    = "testing"
	testIssueID       = "867"
	testPatchsetOrder = 5309
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
  "primary": {
    "ThisIsTheOnlyTest": {
      "beef00d3a1527db19619ec12a4e0df68": "positive",
      "badbadbad1325855590527db196112e0": "negative"
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
		ChangelistID:                testIssueID,
		PatchsetOrder:               testPatchsetOrder,
		CodeReviewSystem:            "gerrit",
		TryJobID:                    testBuildBucketID,
		ContinuousIntegrationSystem: "buildbucket",
	}
}

func imageToPngBytes(t *testing.T, img image.Image) []byte {
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
