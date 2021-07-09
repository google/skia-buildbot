package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.skia.org/infra/go/now"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/gold-client/go/gcsuploader"
	"go.skia.org/infra/gold-client/go/goldclient"
	"go.skia.org/infra/gold-client/go/httpclient"
	"go.skia.org/infra/gold-client/go/imagedownloader"
	"go.skia.org/infra/gold-client/go/imgmatching"
	"go.skia.org/infra/gold-client/go/mocks"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/web/frontend"
)

const (
	// These images are copied from the datakitchensink set
	blankDigest = "00000000000000000000000000000000" // all pixels blank
	a01Digest   = "a01a01a01a01a01a01a01a01a01a01a0" // has a square drawn
	a05Digest   = "a05a05a05a05a05a05a05a05a05a05a0" // small difference from a01
	a09Digest   = "a09a09a09a09a09a09a09a09a09a09a0" // large difference from a01
)

var (
	timeOne = time.Date(2021, time.January, 23, 22, 21, 20, 19, time.UTC)
	timeTwo = time.Date(2021, time.January, 23, 22, 22, 0, 0, time.UTC)
)

func TestImgTest_Init_LoadKeysFromDisk_WritesProperResultState(t *testing.T) {
	unittest.MediumTest(t)

	workDir := t.TempDir()
	setupAuthWithGSUtil(t, workDir)

	keysFile := filepath.Join(workDir, "keys.json")
	require.NoError(t, ioutil.WriteFile(keysFile, []byte(`{"os": "Android"}`), 0644))

	mh := mockRPCResponses("https://my-instance-gold.skia.org").Positive("pixel-tests", blankDigest).
		Negative("other-test", blankDigest).
		Known("11111111111111111111111111111111").Build()

	// Call imgtest init with the following flags. We expect it to load the baseline expectations
	// and the known hashes (both empty).
	ctx, output, exit := testContext(nil, mh, nil, nil)
	env := imgTest{
		gitHash:      "1234567890123456789012345678901234567890",
		corpus:       "my_corpus",
		instanceID:   "my-instance",
		keysFile:     keysFile,
		passFailStep: true,
		workDir:      workDir,
	}
	runUntilExit(t, func() {
		env.Init(ctx)
	})
	exit.AssertWasCalledWithCode(t, 0, output.String())

	b, err := ioutil.ReadFile(filepath.Join(workDir, "result-state.json"))
	require.NoError(t, err)
	resultState := string(b)
	assert.Contains(t, resultState, `"key":{"os":"Android","source_type":"my_corpus"}`)
	assert.Contains(t, resultState, `"KnownHashes":{"00000000000000000000000000000000":true,"11111111111111111111111111111111":true}`)
	assert.Contains(t, resultState, `"Expectations":{"other-test":{"00000000000000000000000000000000":"negative"},"pixel-tests":{"00000000000000000000000000000000":"positive"}}`)
	assert.Contains(t, resultState, `"gitHash":"1234567890123456789012345678901234567890"`)
}

func TestImgTest_Init_CommitIDAndMetadataSet_WritesProperResultState(t *testing.T) {
	unittest.MediumTest(t)

	workDir := t.TempDir()
	setupAuthWithGSUtil(t, workDir)

	keysFile := filepath.Join(workDir, "keys.json")
	require.NoError(t, ioutil.WriteFile(keysFile, []byte(`{"os": "Android"}`), 0644))

	mh := mockRPCResponses("https://my-instance-gold.skia.org").Positive("pixel-tests", blankDigest).
		Negative("other-test", blankDigest).
		Known("11111111111111111111111111111111").Build()

	// Call imgtest init with the following flags. We expect it to load the baseline expectations
	// and the known hashes (both empty).
	ctx, output, exit := testContext(nil, mh, nil, nil)
	env := imgTest{
		commitID:       "92.103234.1.123456",
		commitMetadata: "http://example.com/92.103234.1.123456.xml",
		corpus:         "my_corpus",
		instanceID:     "my-instance",
		keysFile:       keysFile,
		passFailStep:   true,
		workDir:        workDir,
	}
	runUntilExit(t, func() {
		env.Init(ctx)
	})
	exit.AssertWasCalledWithCode(t, 0, output.String())

	b, err := ioutil.ReadFile(filepath.Join(workDir, "result-state.json"))
	require.NoError(t, err)
	resultState := string(b)
	assert.Contains(t, resultState, `"key":{"os":"Android","source_type":"my_corpus"}`)
	assert.Contains(t, resultState, `"KnownHashes":{"00000000000000000000000000000000":true,"11111111111111111111111111111111":true}`)
	assert.Contains(t, resultState, `"Expectations":{"other-test":{"00000000000000000000000000000000":"negative"},"pixel-tests":{"00000000000000000000000000000000":"positive"}}`)
	assert.Contains(t, resultState, `"commit_id":"92.103234.1.123456","commit_metadata":"http://example.com/92.103234.1.123456.xml"`)
}

func TestImgTest_Init_ChangeListWithoutCommitHash_WritesProperResultState(t *testing.T) {
	unittest.MediumTest(t)

	workDir := t.TempDir()
	setupAuthWithGSUtil(t, workDir)

	keysFile := filepath.Join(workDir, "keys.json")
	require.NoError(t, ioutil.WriteFile(keysFile, []byte(`{"os": "Android"}`), 0644))

	mh := mockRPCResponses("https://my-instance-gold.skia.org").Positive("pixel-tests", blankDigest).
		Negative("other-test", blankDigest).
		Known("11111111111111111111111111111111").BuildForCL("my_CRS", "my_CL")

	// Call imgtest init with the following flags. We expect it to load the baseline expectations
	// and the known hashes (both empty).
	ctx, output, exit := testContext(nil, mh, nil, nil)
	env := imgTest{
		corpus:                      "my_corpus",
		instanceID:                  "my-instance",
		keysFile:                    keysFile,
		passFailStep:                true,
		workDir:                     workDir,
		codeReviewSystem:            "my_CRS",
		changelistID:                "my_CL",
		patchsetID:                  "some_patchset",
		continuousIntegrationSystem: "my_CIS",
		tryJobID:                    "some_tryjob",
	}
	runUntilExit(t, func() {
		env.Init(ctx)
	})
	exit.AssertWasCalledWithCode(t, 0, output.String())

	b, err := ioutil.ReadFile(filepath.Join(workDir, "result-state.json"))
	require.NoError(t, err)
	resultState := string(b)
	assert.Contains(t, resultState, `"key":{"os":"Android","source_type":"my_corpus"}`)
	assert.Contains(t, resultState, `"KnownHashes":{"00000000000000000000000000000000":true,"11111111111111111111111111111111":true}`)
	assert.Contains(t, resultState, `"Expectations":{"other-test":{"00000000000000000000000000000000":"negative"},"pixel-tests":{"00000000000000000000000000000000":"positive"}}`)
	assert.Contains(t, resultState, `"change_list_id":"my_CL","patch_set_order":0,"patch_set_id":"some_patchset","crs":"my_CRS","try_job_id":"some_tryjob","cis":"my_CIS"`)
}

func TestImgTest_Init_NoChangeListNorCommitHash_NonzeroExitCode(t *testing.T) {
	unittest.MediumTest(t)

	workDir := t.TempDir()
	setupAuthWithGSUtil(t, workDir)

	keysFile := filepath.Join(workDir, "keys.json")
	require.NoError(t, ioutil.WriteFile(keysFile, []byte(`{"os": "Android"}`), 0644))

	// Call imgtest init with the following flags. We expect it to fail because we need to provide
	// a commit or CL info
	ctx, output, exit := testContext(nil, nil, nil, nil)
	env := imgTest{
		corpus:       "my_corpus",
		instanceID:   "my-instance",
		keysFile:     keysFile,
		passFailStep: true,
		workDir:      workDir,
	}
	runUntilExit(t, func() {
		env.Init(ctx)
	})
	outStr := output.String()
	exit.AssertWasCalledWithCode(t, 1, outStr)
	assert.Contains(t, outStr, `invalid configuration: field "gitHash", "commit_id", or "change_list_id" must be set`)
}

func TestImgTest_InitAdd_StreamingPassFail_DoesNotMatchExpectations_NonzeroExitCode(t *testing.T) {
	unittest.MediumTest(t)

	workDir := t.TempDir()
	setupAuthWithGSUtil(t, workDir)
	td := testutils.TestDataDir(t)

	mh := mockRPCResponses("https://my-instance-gold.skia.org").Build()

	// Call imgtest init with the following flags. We expect it to load the baseline expectations
	// and the known hashes (both empty).
	ctx, output, exit := testContext(nil, mh, nil, nil)
	env := imgTest{
		gitHash:         "1234567890123456789012345678901234567890",
		corpus:          "my_corpus",
		instanceID:      "my-instance",
		passFailStep:    true,
		failureFile:     filepath.Join(workDir, "failures.txt"),
		workDir:         workDir,
		testKeysStrings: []string{"os:Android"},
	}
	runUntilExit(t, func() {
		env.Init(ctx)
	})
	exit.AssertWasCalledWithCode(t, 0, output.String())

	mg := &mocks.GCSUploader{}
	resultsMatcher := mock.MatchedBy(func(results jsonio.GoldResults) bool {
		assert.Equal(t, jsonio.GoldResults{
			GitHash: "1234567890123456789012345678901234567890",
			Key: map[string]string{
				"os":          "Android",
				"source_type": "my_corpus",
			},
			Results: []jsonio.Result{{
				Key:     map[string]string{"name": "pixel-tests", "device": "angler"},
				Options: map[string]string{"some_option": "is optional", "ext": "png"},
				Digest:  blankDigest,
			}},
		}, results)
		return true
	})
	mg.On("UploadJSON", testutils.AnyContext, resultsMatcher, mock.Anything,
		`skia-gold-my-instance/dm-json-v1/2021/01/23/22/1234567890123456789012345678901234567890/waterfall/dm-1611440480000000019.json`).Return(nil)
	bytesMatcher := mock.MatchedBy(func(b []byte) bool {
		assert.Len(t, b, 78) // spot check length
		return true
	})
	mg.On("UploadBytes", testutils.AnyContext, bytesMatcher, mock.Anything,
		`gs://skia-gold-my-instance/dm-images-v1/00000000000000000000000000000000.png`).Return(nil)

	// Now call imgtest add with the following flags. This is simulating a test uploading a single
	// result for a test called pixel-tests.
	ctx, output, exit = testContext(mg, nil, nil, &timeOne)
	env = imgTest{
		workDir:                 workDir,
		testName:                "pixel-tests",
		pngFile:                 filepath.Join(td, "00000000000000000000000000000000.png"),
		pngDigest:               blankDigest,
		testKeysStrings:         []string{"device:angler"},
		testOptionalKeysStrings: []string{"some_option:is optional"},
	}
	runUntilExit(t, func() {
		env.Add(ctx)
	})
	logs := output.String()
	exit.AssertWasCalledWithCode(t, 1, logs)
	mg.AssertExpectations(t)
	assert.Contains(t, logs, `Untriaged or negative image: https://my-instance-gold.skia.org/detail?test=pixel-tests&digest=00000000000000000000000000000000`)
	assert.Contains(t, logs, `Test: pixel-tests FAIL`)

	fb, err := ioutil.ReadFile(filepath.Join(workDir, "failures.txt"))
	require.NoError(t, err)
	assert.Contains(t, string(fb), "https://my-instance-gold.skia.org/detail?test=pixel-tests&digest=00000000000000000000000000000000")
}

func TestImgTest_InitAdd_OverwriteBucketAndURL_ProperLinks(t *testing.T) {
	unittest.MediumTest(t)

	workDir := t.TempDir()
	setupAuthWithGSUtil(t, workDir)
	td := testutils.TestDataDir(t)

	mh := mockRPCResponses("https://my-custom-gold-url.example.com").Build()

	// Call imgtest init with the following flags. We expect it to load the baseline expectations
	// and the known hashes (both empty).
	ctx, output, exit := testContext(nil, mh, nil, nil)
	env := imgTest{
		bucketOverride:  "my-custom-bucket",
		gitHash:         "1234567890123456789012345678901234567890",
		corpus:          "my_corpus",
		instanceID:      "my-instance",
		passFailStep:    true,
		failureFile:     filepath.Join(workDir, "failures.txt"),
		workDir:         workDir,
		testKeysStrings: []string{"os:Android"},
		urlOverride:     "https://my-custom-gold-url.example.com",
	}
	runUntilExit(t, func() {
		env.Init(ctx)
	})
	exit.AssertWasCalledWithCode(t, 0, output.String())

	mg := &mocks.GCSUploader{}
	resultsMatcher := mock.MatchedBy(func(results jsonio.GoldResults) bool {
		assert.Equal(t, jsonio.GoldResults{
			GitHash: "1234567890123456789012345678901234567890",
			Key: map[string]string{
				"os":          "Android",
				"source_type": "my_corpus",
			},
			Results: []jsonio.Result{{
				Key:     map[string]string{"name": "pixel-tests", "device": "angler"},
				Options: map[string]string{"some_option": "is optional", "ext": "png"},
				Digest:  blankDigest,
			}},
		}, results)
		return true
	})
	mg.On("UploadJSON", testutils.AnyContext, resultsMatcher, mock.Anything,
		`my-custom-bucket/dm-json-v1/2021/01/23/22/1234567890123456789012345678901234567890/waterfall/dm-1611440480000000019.json`).Return(nil)
	bytesMatcher := mock.MatchedBy(func(b []byte) bool {
		assert.Len(t, b, 78) // spot check length
		return true
	})
	mg.On("UploadBytes", testutils.AnyContext, bytesMatcher, mock.Anything,
		`gs://my-custom-bucket/dm-images-v1/00000000000000000000000000000000.png`).Return(nil)

	// Now call imgtest add with the following flags. This is simulating a test uploading a single
	// result for a test called pixel-tests.
	ctx, output, exit = testContext(mg, nil, nil, &timeOne)
	env = imgTest{
		workDir:                 workDir,
		testName:                "pixel-tests",
		pngFile:                 filepath.Join(td, "00000000000000000000000000000000.png"),
		pngDigest:               blankDigest,
		testKeysStrings:         []string{"device:angler"},
		testOptionalKeysStrings: []string{"some_option:is optional"},
	}
	runUntilExit(t, func() {
		env.Add(ctx)
	})
	logs := output.String()
	exit.AssertWasCalledWithCode(t, 1, logs)
	mg.AssertExpectations(t)
	assert.Contains(t, logs, `Untriaged or negative image: https://my-custom-gold-url.example.com/detail?test=pixel-tests&digest=00000000000000000000000000000000`)
	assert.Contains(t, logs, `Test: pixel-tests FAIL`)

	fb, err := ioutil.ReadFile(filepath.Join(workDir, "failures.txt"))
	require.NoError(t, err)
	assert.Contains(t, string(fb), "https://my-custom-gold-url.example.com/detail?test=pixel-tests&digest=00000000000000000000000000000000")
}

func TestImgTest_InitAdd_StreamingPassFail_MatchesExpectations_ZeroExitCode(t *testing.T) {
	unittest.MediumTest(t)

	workDir := t.TempDir()
	setupAuthWithGSUtil(t, workDir)
	td := testutils.TestDataDir(t)

	mh := mockRPCResponses("https://my-instance-gold.skia.org").Positive("pixel-tests", blankDigest).Build()

	// Call imgtest init with the following flags. We expect it to load the baseline expectations
	// and the known hashes.
	ctx, output, exit := testContext(nil, mh, nil, nil)
	env := imgTest{
		gitHash:         "1234567890123456789012345678901234567890",
		corpus:          "my_corpus",
		instanceID:      "my-instance",
		passFailStep:    true,
		failureFile:     filepath.Join(workDir, "failures.txt"),
		workDir:         workDir,
		testKeysStrings: []string{"os:Android"},
	}
	runUntilExit(t, func() {
		env.Init(ctx)
	})
	exit.AssertWasCalledWithCode(t, 0, output.String())

	mg := &mocks.GCSUploader{}
	resultsMatcher := mock.MatchedBy(func(results jsonio.GoldResults) bool {
		assert.Equal(t, jsonio.GoldResults{
			GitHash: "1234567890123456789012345678901234567890",
			Key: map[string]string{
				"os":          "Android",
				"source_type": "my_corpus",
			},
			Results: []jsonio.Result{{
				Key:     map[string]string{"name": "pixel-tests", "device": "angler"},
				Options: map[string]string{"some_option": "is optional", "ext": "png"},
				Digest:  blankDigest,
			}},
		}, results)
		return true
	})
	mg.On("UploadJSON", testutils.AnyContext, resultsMatcher, mock.Anything,
		`skia-gold-my-instance/dm-json-v1/2021/01/23/22/1234567890123456789012345678901234567890/waterfall/dm-1611440480000000019.json`).Return(nil)

	// Now call imgtest add with the following flags. This is simulating a test uploading a single
	// result for a test called pixel-tests. The digest has already been triaged positive.
	ctx, output, exit = testContext(mg, nil, nil, &timeOne)
	env = imgTest{
		workDir:                 workDir,
		testName:                "pixel-tests",
		pngFile:                 filepath.Join(td, "00000000000000000000000000000000.png"),
		pngDigest:               blankDigest,
		testKeysStrings:         []string{"device:angler"},
		testOptionalKeysStrings: []string{"some_option:is optional"},
	}
	runUntilExit(t, func() {
		env.Add(ctx)
	})
	logs := output.String()
	exit.AssertWasCalledWithCode(t, 0, logs)
	mg.AssertExpectations(t)
}

func TestImgTest_InitAdd_StreamingPassFail_SuccessiveCalls_ProperJSONUploaded(t *testing.T) {
	unittest.MediumTest(t)

	workDir := t.TempDir()
	setupAuthWithGSUtil(t, workDir)
	td := testutils.TestDataDir(t)

	mh := mockRPCResponses("https://my-instance-gold.skia.org").Positive("pixel-tests", blankDigest).Build()

	// Call imgtest init with the following flags. We expect it to load the baseline expectations
	// and the known hashes.
	ctx, output, exit := testContext(nil, mh, nil, nil)
	env := imgTest{
		gitHash:         "1234567890123456789012345678901234567890",
		corpus:          "my_corpus",
		instanceID:      "my-instance",
		passFailStep:    true,
		workDir:         workDir,
		testKeysStrings: []string{"os:Android"},
	}
	runUntilExit(t, func() {
		env.Init(ctx)
	})
	exit.AssertWasCalledWithCode(t, 0, output.String())

	mg := &mocks.GCSUploader{}
	resultsMatcher := mock.MatchedBy(func(results jsonio.GoldResults) bool {
		assert.Equal(t, jsonio.GoldResults{
			GitHash: "1234567890123456789012345678901234567890",
			Key: map[string]string{
				"os":          "Android",
				"source_type": "my_corpus",
			},
			Results: []jsonio.Result{{
				Key:     map[string]string{"name": "pixel-tests", "device": "angler"},
				Options: map[string]string{"some_option": "is optional", "ext": "png"},
				Digest:  blankDigest,
			}},
		}, results)
		return true
	})
	mg.On("UploadJSON", testutils.AnyContext, resultsMatcher, mock.Anything,
		`skia-gold-my-instance/dm-json-v1/2021/01/23/22/1234567890123456789012345678901234567890/waterfall/dm-1611440480000000019.json`).Return(nil)

	// Now call imgtest add with the following flags. This is simulating a test uploading a single
	// result for a test called pixel-tests. The digest has already been triaged positive.
	ctx, output, exit = testContext(mg, nil, nil, &timeOne)
	env = imgTest{
		workDir:                 workDir,
		testName:                "pixel-tests",
		pngFile:                 filepath.Join(td, "00000000000000000000000000000000.png"),
		pngDigest:               blankDigest,
		testKeysStrings:         []string{"device:angler"},
		testOptionalKeysStrings: []string{"some_option:is optional"},
	}
	runUntilExit(t, func() {
		env.Add(ctx)
	})
	logs := output.String()
	exit.AssertWasCalledWithCode(t, 0, logs)
	mg.AssertExpectations(t)

	mg = &mocks.GCSUploader{}
	resultsMatcher = mock.MatchedBy(func(results jsonio.GoldResults) bool {
		assert.Equal(t, jsonio.GoldResults{
			GitHash: "1234567890123456789012345678901234567890",
			Key: map[string]string{
				"os":          "Android",
				"source_type": "my_corpus",
			},
			Results: []jsonio.Result{{
				Key:     map[string]string{"name": "pixel-tests", "device": "bullhead"},
				Options: map[string]string{"some_option": "is VERY DIFFERENT", "ext": "png"},
				Digest:  blankDigest,
			}},
		}, results)
		return true
	})
	mg.On("UploadJSON", testutils.AnyContext, resultsMatcher, mock.Anything,
		`skia-gold-my-instance/dm-json-v1/2021/01/23/22/1234567890123456789012345678901234567890/waterfall/dm-1611440520000000000.json`).Return(nil)

	// Call imgtest add for a second device running the same test as above.
	ctx, output, exit = testContext(mg, nil, nil, &timeTwo)
	env = imgTest{
		workDir:                 workDir,
		testName:                "pixel-tests",
		pngFile:                 filepath.Join(td, "00000000000000000000000000000000.png"),
		pngDigest:               blankDigest,
		testKeysStrings:         []string{"device:bullhead"},
		testOptionalKeysStrings: []string{"some_option:is VERY DIFFERENT"},
	}
	runUntilExit(t, func() {
		env.Add(ctx)
	})
	logs = output.String()
	exit.AssertWasCalledWithCode(t, 0, logs)
	mg.AssertExpectations(t)
}

// This tests calling imgtest add without calling imgtest init first.
func TestImgTest_Add_StreamingPassFail_MatchesExpectations_ZeroExitCode(t *testing.T) {
	unittest.MediumTest(t)

	workDir := t.TempDir()
	setupAuthWithGSUtil(t, workDir)
	td := testutils.TestDataDir(t)

	keysFile := filepath.Join(workDir, "keys.json")
	require.NoError(t, ioutil.WriteFile(keysFile, []byte(`{"os": "Android"}`), 0644))

	mh := mockRPCResponses("https://my-instance-gold.skia.org").Positive("pixel-tests", blankDigest).Build()

	mg := &mocks.GCSUploader{}
	resultsMatcher := mock.MatchedBy(func(results jsonio.GoldResults) bool {
		assert.Equal(t, jsonio.GoldResults{
			GitHash: "1234567890123456789012345678901234567890",
			Key: map[string]string{
				"os": "Android",
			},
			Results: []jsonio.Result{{
				Key:     map[string]string{"name": "pixel-tests", "device": "angler", "source_type": "my_corpus"},
				Options: map[string]string{"some_option": "is optional", "ext": "png"},
				Digest:  blankDigest,
			}},
		}, results)
		return true
	})
	mg.On("UploadJSON", testutils.AnyContext, resultsMatcher, mock.Anything,
		`skia-gold-my-instance/dm-json-v1/2021/01/23/22/1234567890123456789012345678901234567890/waterfall/dm-1611440480000000019.json`).Return(nil)

	// Now call imgtest add with the following flags. This is simulating a test uploading a single
	// result for a test called pixel-tests. The digest has already been triaged positive.
	ctx, output, exit := testContext(mg, mh, nil, &timeOne)
	env := imgTest{
		gitHash:                 "1234567890123456789012345678901234567890",
		corpus:                  "my_corpus",
		failureFile:             filepath.Join(workDir, "failures.txt"),
		instanceID:              "my-instance",
		keysFile:                keysFile,
		passFailStep:            true,
		pngDigest:               blankDigest,
		pngFile:                 filepath.Join(td, "00000000000000000000000000000000.png"),
		testKeysStrings:         []string{"device:angler"},
		testName:                "pixel-tests",
		testOptionalKeysStrings: []string{"some_option:is optional"},
		workDir:                 workDir,
	}
	runUntilExit(t, func() {
		env.Add(ctx)
	})
	logs := output.String()
	exit.AssertWasCalledWithCode(t, 0, logs)
	mg.AssertExpectations(t)
}

func TestImgTest_InitAddFinalize_BatchMode_ExpectationsMatch_ProperJSONUploaded(t *testing.T) {
	unittest.MediumTest(t)

	workDir := t.TempDir()
	setupAuthWithGSUtil(t, workDir)
	td := testutils.TestDataDir(t)

	mh := mockRPCResponses("https://my-instance-gold.skia.org").Positive("pixel-tests", blankDigest).Build()

	// Call imgtest init with the following flags. We expect it to load the baseline expectations
	// and the known hashes.
	ctx, output, exit := testContext(nil, mh, nil, nil)
	env := imgTest{
		gitHash:         "1234567890123456789012345678901234567890",
		corpus:          "my_corpus",
		instanceID:      "my-instance",
		workDir:         workDir,
		testKeysStrings: []string{"os:Android"},
	}
	runUntilExit(t, func() {
		env.Init(ctx)
	})
	exit.AssertWasCalledWithCode(t, 0, output.String())

	// Now call imgtest add with the following flags. This is simulating adding a result for
	// a test called pixel-tests. The digest has been triaged positive for this test.
	ctx, output, exit = testContext(nil, nil, nil, nil)
	env = imgTest{
		workDir:                 workDir,
		testName:                "pixel-tests",
		pngFile:                 filepath.Join(td, "00000000000000000000000000000000.png"),
		pngDigest:               blankDigest,
		testKeysStrings:         []string{"device:angler"},
		testOptionalKeysStrings: []string{"some_option:is optional"},
	}
	runUntilExit(t, func() {
		env.Add(ctx)
	})
	logs := output.String()
	exit.AssertWasCalledWithCode(t, 0, logs)

	// Call imgtest add for a second device running the same test as above.
	ctx, output, exit = testContext(nil, nil, nil, nil)
	env = imgTest{
		workDir:                 workDir,
		testName:                "pixel-tests",
		pngFile:                 filepath.Join(td, "00000000000000000000000000000000.png"),
		pngDigest:               blankDigest,
		testKeysStrings:         []string{"device:bullhead"},
		testOptionalKeysStrings: []string{"some_option:is VERY DIFFERENT"},
	}
	runUntilExit(t, func() {
		env.Add(ctx)
	})
	logs = output.String()
	exit.AssertWasCalledWithCode(t, 0, logs)

	mg := &mocks.GCSUploader{}
	resultsMatcher := mock.MatchedBy(func(results jsonio.GoldResults) bool {
		assert.Equal(t, jsonio.GoldResults{
			GitHash: "1234567890123456789012345678901234567890",
			Key: map[string]string{
				"os":          "Android",
				"source_type": "my_corpus",
			},
			Results: []jsonio.Result{{
				Key:     map[string]string{"name": "pixel-tests", "device": "angler"},
				Options: map[string]string{"some_option": "is optional", "ext": "png"},
				Digest:  blankDigest,
			}, {
				Key:     map[string]string{"name": "pixel-tests", "device": "bullhead"},
				Options: map[string]string{"some_option": "is VERY DIFFERENT", "ext": "png"},
				Digest:  blankDigest,
			}},
		}, results)
		return true
	})
	mg.On("UploadJSON", testutils.AnyContext, resultsMatcher, mock.Anything,
		`skia-gold-my-instance/dm-json-v1/2021/01/23/22/1234567890123456789012345678901234567890/waterfall/dm-1611440480000000019.json`).Return(nil)

	// Call imgtest finalize, expecting to see all data before uploaded.
	ctx, output, exit = testContext(mg, nil, nil, &timeOne)
	env = imgTest{
		workDir: workDir,
	}
	runUntilExit(t, func() {
		env.Finalize(ctx)
	})
	logs = output.String()
	exit.AssertWasCalledWithCode(t, 0, logs)
	mg.AssertExpectations(t)
}

func TestImgTest_InitAddFinalize_BatchMode_ExpectationsDoNotMatch_ProperJSONAndImageUploaded(t *testing.T) {
	unittest.MediumTest(t)

	workDir := t.TempDir()
	setupAuthWithGSUtil(t, workDir)
	td := testutils.TestDataDir(t)

	mh := mockRPCResponses("https://my-instance-gold.skia.org").Build()

	// Call imgtest init with the following flags. We expect it to load the baseline expectations
	// and the known hashes.
	ctx, output, exit := testContext(nil, mh, nil, nil)
	env := imgTest{
		gitHash:         "1234567890123456789012345678901234567890",
		corpus:          "my_corpus",
		instanceID:      "my-instance",
		workDir:         workDir,
		testKeysStrings: []string{"os:Android"},
	}
	runUntilExit(t, func() {
		env.Init(ctx)
	})
	exit.AssertWasCalledWithCode(t, 0, output.String())

	mg := &mocks.GCSUploader{}
	bytesMatcher := mock.MatchedBy(func(b []byte) bool {
		assert.Len(t, b, 78) // spot check length
		return true
	})
	mg.On("UploadBytes", testutils.AnyContext, bytesMatcher, mock.Anything,
		`gs://skia-gold-my-instance/dm-images-v1/00000000000000000000000000000000.png`).Return(nil)

	// Now call imgtest add with the following flags. This is simulating adding a result for
	// a test called pixel-tests. The digest has not been seen before.
	ctx, output, exit = testContext(mg, nil, nil, nil)
	env = imgTest{
		workDir:                 workDir,
		testName:                "pixel-tests",
		pngFile:                 filepath.Join(td, "00000000000000000000000000000000.png"),
		pngDigest:               blankDigest,
		testKeysStrings:         []string{"device:angler"},
		testOptionalKeysStrings: []string{"some_option:is optional"},
	}
	runUntilExit(t, func() {
		env.Add(ctx)
	})
	logs := output.String()
	exit.AssertWasCalledWithCode(t, 0, logs)

	// Call imgtest add for a second device running the same test as above.
	// TODO(kjlubick) Append to the known digests to prevent a duplicate upload.
	ctx, output, exit = testContext(mg, nil, nil, nil)
	env = imgTest{
		workDir:                 workDir,
		testName:                "pixel-tests",
		pngFile:                 filepath.Join(td, "00000000000000000000000000000000.png"),
		pngDigest:               blankDigest,
		testKeysStrings:         []string{"device:bullhead"},
		testOptionalKeysStrings: []string{"some_option:is VERY DIFFERENT"},
	}
	runUntilExit(t, func() {
		env.Add(ctx)
	})
	logs = output.String()
	exit.AssertWasCalledWithCode(t, 0, logs)

	mg = &mocks.GCSUploader{}
	resultsMatcher := mock.MatchedBy(func(results jsonio.GoldResults) bool {
		assert.Equal(t, jsonio.GoldResults{
			GitHash: "1234567890123456789012345678901234567890",
			Key: map[string]string{
				"os":          "Android",
				"source_type": "my_corpus",
			},
			Results: []jsonio.Result{{
				Key:     map[string]string{"name": "pixel-tests", "device": "angler"},
				Options: map[string]string{"some_option": "is optional", "ext": "png"},
				Digest:  blankDigest,
			}, {
				Key:     map[string]string{"name": "pixel-tests", "device": "bullhead"},
				Options: map[string]string{"some_option": "is VERY DIFFERENT", "ext": "png"},
				Digest:  blankDigest,
			}},
		}, results)
		return true
	})
	mg.On("UploadJSON", testutils.AnyContext, resultsMatcher, mock.Anything,
		`skia-gold-my-instance/dm-json-v1/2021/01/23/22/1234567890123456789012345678901234567890/waterfall/dm-1611440480000000019.json`).Return(nil)

	// Call imgtest finalize, expecting to see all data before uploaded.
	ctx, output, exit = testContext(mg, nil, nil, &timeOne)
	env = imgTest{
		workDir: workDir,
	}
	runUntilExit(t, func() {
		env.Finalize(ctx)
	})
	logs = output.String()
	// In Batch mode, even though the images were untriaged, we return 0 (not failing).
	exit.AssertWasCalledWithCode(t, 0, logs)
	mg.AssertExpectations(t)
}

// This test compares image a01 and a05. These images have 2 pixels different, with a maximum
// delta of 7, so the settings are close enough to let those match.
func TestImgTest_Check_CloseEnoughForFuzzyMatch_ExitCodeZero(t *testing.T) {
	unittest.MediumTest(t)

	workDir := t.TempDir()
	setupAuthWithGSUtil(t, workDir)
	td := testutils.TestDataDir(t)

	mh := mockRPCResponses("https://my-instance-gold.skia.org").Positive("pixel-tests", a01Digest).
		LatestPositive(a01Digest, paramtools.Params{
			"device": "bullhead", "name": "pixel-tests", "source_type": "my-instance",
		}).Build()

	a01Bytes, err := ioutil.ReadFile(filepath.Join(td, a01Digest+".png"))
	require.NoError(t, err)
	mi := &mocks.ImageDownloader{}
	mi.On("DownloadImage", testutils.AnyContext, "https://my-instance-gold.skia.org", types.Digest(a01Digest)).Return(a01Bytes, nil)

	ctx, output, exit := testContext(nil, mh, mi, nil)
	env := imgTest{
		workDir:         workDir,
		instanceID:      "my-instance",
		pngFile:         filepath.Join(td, a05Digest+".png"),
		testName:        "pixel-tests",
		testKeysStrings: []string{"device:bullhead"},
		testOptionalKeysStrings: []string{
			string(imgmatching.AlgorithmNameOptKey + ":" + imgmatching.FuzzyMatching),
			string(imgmatching.MaxDifferentPixels + ":2"),
			string(imgmatching.PixelDeltaThreshold + ":10"),
		},
	}
	runUntilExit(t, func() {
		env.Check(ctx)
	})
	logs := output.String()
	exit.AssertWasCalledWithCode(t, 0, logs)
	assert.Contains(t, logs, `Non-exact image comparison using algorithm "fuzzy" against most recent positive digest "a01a01a01a01a01a01a01a01a01a01a0".`)
	assert.Contains(t, logs, `Test: pixel-tests PASS`)
}

func TestImgTest_Check_TooDifferentOnChangelist_ExitCodeOne(t *testing.T) {
	unittest.MediumTest(t)

	workDir := t.TempDir()
	setupAuthWithGSUtil(t, workDir)
	td := testutils.TestDataDir(t)

	mh := mockRPCResponses("https://my-instance-gold.skia.org").Positive("pixel-tests", a01Digest).
		LatestPositive(a01Digest, paramtools.Params{
			"device": "bullhead", "name": "pixel-tests", "source_type": "my-instance",
		}).BuildForCL("gerritHub", "cl_1234")

	a01Bytes, err := ioutil.ReadFile(filepath.Join(td, a01Digest+".png"))
	require.NoError(t, err)
	mi := &mocks.ImageDownloader{}
	mi.On("DownloadImage", testutils.AnyContext, "https://my-instance-gold.skia.org", types.Digest(a01Digest)).Return(a01Bytes, nil)

	ctx, output, exit := testContext(nil, mh, mi, nil)
	env := imgTest{
		workDir:          workDir,
		changelistID:     "cl_1234",
		codeReviewSystem: "gerritHub",
		instanceID:       "my-instance",
		pngFile:          filepath.Join(td, a09Digest+".png"),
		testName:         "pixel-tests",
		testKeysStrings:  []string{"device:bullhead"},
		testOptionalKeysStrings: []string{
			string(imgmatching.AlgorithmNameOptKey + ":" + imgmatching.FuzzyMatching),
			string(imgmatching.MaxDifferentPixels + ":2"),
			string(imgmatching.PixelDeltaThreshold + ":10"),
		},
	}
	runUntilExit(t, func() {
		env.Check(ctx)
	})
	logs := output.String()
	exit.AssertWasCalledWithCode(t, 1, logs)
	assert.Contains(t, logs, `Non-exact image comparison using algorithm "fuzzy" against most recent positive digest "a01a01a01a01a01a01a01a01a01a01a0".`)
	assert.Contains(t, logs, `Test: pixel-tests FAIL`)
}

func testContext(g gcsuploader.GCSUploader, h httpclient.HTTPClient, i imagedownloader.ImageDownloader, ts *time.Time) (context.Context, *threadSafeBuffer, *exitCodeRecorder) {
	output := &threadSafeBuffer{}
	exit := &exitCodeRecorder{}

	ctx := executionContext(context.Background(), output, output, exit.ExitWithCode)
	if ts != nil {
		ctx = context.WithValue(ctx, now.ContextKey, *ts)
	}
	return goldclient.WithContext(ctx, g, h, i), output, exit
}

type threadSafeBuffer struct {
	buf   bytes.Buffer
	mutex sync.Mutex
}

func (t *threadSafeBuffer) Write(p []byte) (n int, err error) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return t.buf.Write(p)
}

func (t *threadSafeBuffer) String() string {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return t.buf.String()
}

type rpcResponsesBuilder struct {
	knownDigests    []string
	exp             *expectations.Expectations
	latestPositives map[tiling.TraceID]types.Digest
	urlBase         string
}

func mockRPCResponses(instanceURL string) *rpcResponsesBuilder {
	return &rpcResponsesBuilder{
		exp:     &expectations.Expectations{},
		urlBase: instanceURL,
	}
}

func (r *rpcResponsesBuilder) Positive(name string, digest types.Digest) *rpcResponsesBuilder {
	r.exp.Set(types.TestName(name), digest, expectations.Positive)
	r.knownDigests = append(r.knownDigests, string(digest))
	return r
}

func (r *rpcResponsesBuilder) Negative(name string, digest types.Digest) *rpcResponsesBuilder {
	r.exp.Set(types.TestName(name), digest, expectations.Negative)
	r.knownDigests = append(r.knownDigests, string(digest))
	return r
}

func (r *rpcResponsesBuilder) Known(digest types.Digest) *rpcResponsesBuilder {
	r.knownDigests = append(r.knownDigests, string(digest))
	return r
}

func (r *rpcResponsesBuilder) LatestPositive(digest types.Digest, traceKeys paramtools.Params) *rpcResponsesBuilder {
	if len(r.latestPositives) == 0 {
		r.latestPositives = map[tiling.TraceID]types.Digest{}
	}
	traceID := tiling.TraceIDFromParams(traceKeys)
	r.latestPositives[traceID] = digest
	return r
}

func (r *rpcResponsesBuilder) Build() *mocks.HTTPClient {
	mh := &mocks.HTTPClient{}
	knownResp := strings.Join(r.knownDigests, "\n")
	mh.On("Get", r.urlBase+"/json/v1/hashes").Return(httpResponse(knownResp, "200 OK", http.StatusOK), nil)

	exp, err := json.Marshal(frontend.BaselineV2Response{
		Expectations: r.exp.AsBaseline(),
	})
	if err != nil {
		panic(err)
	}
	mh.On("Get", r.urlBase+"/json/v2/expectations").Return(
		httpResponse(string(exp), "200 OK", http.StatusOK), nil)

	for traceID, digest := range r.latestPositives {
		j, err := json.Marshal(frontend.MostRecentPositiveDigestResponse{Digest: digest})
		if err != nil {
			panic(err)
		}
		url := r.urlBase + "/json/v2/latestpositivedigest/" + string(traceID)
		mh.On("Get", url).Return(
			httpResponse(string(j), "200 OK", http.StatusOK), nil)
	}

	return mh
}

func (r *rpcResponsesBuilder) BuildForCL(crs, clID string) *mocks.HTTPClient {
	mh := &mocks.HTTPClient{}
	knownResp := strings.Join(r.knownDigests, "\n")
	mh.On("Get", r.urlBase+"/json/v1/hashes").Return(httpResponse(knownResp, "200 OK", http.StatusOK), nil)

	exp, err := json.Marshal(frontend.BaselineV2Response{
		Expectations:     r.exp.AsBaseline(),
		ChangelistID:     clID,
		CodeReviewSystem: crs,
	})
	if err != nil {
		panic(err)
	}
	url := fmt.Sprintf("%s/json/v2/expectations?issue=%s&crs=%s", r.urlBase, clID, crs)
	mh.On("Get", url).Return(
		httpResponse(string(exp), "200 OK", http.StatusOK), nil)

	for traceID, digest := range r.latestPositives {
		j, err := json.Marshal(frontend.MostRecentPositiveDigestResponse{Digest: digest})
		if err != nil {
			panic(err)
		}
		url := r.urlBase + "/json/v2/latestpositivedigest/" + string(traceID)
		mh.On("Get", url).Return(
			httpResponse(string(j), "200 OK", http.StatusOK), nil)
	}

	return mh
}

func TestRPCResponsesBuilder_Default_ReturnsBlankValues(t *testing.T) {
	unittest.SmallTest(t)

	mh := mockRPCResponses("https://my-instance-gold.skia.org").Build()
	resp, err := mh.Get("https://my-instance-gold.skia.org/json/v1/hashes")
	require.NoError(t, err)
	b, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "", string(b))

	resp, err = mh.Get("https://my-instance-gold.skia.org/json/v2/expectations")
	require.NoError(t, err)
	b, err = ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, `{}`, string(b))
}

func TestRPCResponsesBuilder_WithValues_ReturnsValidListsAndJSON(t *testing.T) {
	unittest.SmallTest(t)

	mh := mockRPCResponses("http://my-custom-url.example.com").
		Known("first_digest").
		Positive("alpha test", "second_digest").
		Positive("beta test", "third_digest").
		Negative("alpha test", "fourth_digest").
		LatestPositive("third_digest", paramtools.Params{"alpha": "beta", "gamma": "delta epsilon"}).
		Known("fifth_digest").
		Build()
	resp, err := mh.Get("http://my-custom-url.example.com/json/v1/hashes")
	require.NoError(t, err)
	b, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, `first_digest
second_digest
third_digest
fourth_digest
fifth_digest`, string(b))

	resp, err = mh.Get("http://my-custom-url.example.com/json/v2/expectations")
	require.NoError(t, err)
	b, err = ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, `{"primary":{"alpha test":{"fourth_digest":"negative","second_digest":"positive"},"beta test":{"third_digest":"positive"}}}`, string(b))

	resp, err = mh.Get("http://my-custom-url.example.com/json/v2/latestpositivedigest/,alpha=beta,gamma=delta epsilon,")
	require.NoError(t, err)
	b, err = ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, `{"digest":"third_digest"}`, string(b))
}
