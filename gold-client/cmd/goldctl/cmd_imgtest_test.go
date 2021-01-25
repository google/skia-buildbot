package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/gold-client/go/gcsuploader"
	"go.skia.org/infra/gold-client/go/goldclient"
	"go.skia.org/infra/gold-client/go/httpclient"
	"go.skia.org/infra/gold-client/go/imagedownloader"
	"go.skia.org/infra/gold-client/go/mocks"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/types"
)

const (
	blankDigest = "00000000000000000000000000000000"
)

var (
	timeOne = time.Date(2021, time.January, 23, 22, 21, 20, 19, time.UTC)
	timeTwo = time.Date(2021, time.January, 23, 22, 22, 0, 0, time.UTC)
)

func TestImgTest_InitAdd_StreamingPassFail_DoesNotMatchExpectations_NonzeroExitCode(t *testing.T) {
	unittest.MediumTest(t)

	workDir := t.TempDir()
	setupAuthWithGSUtil(t, workDir)
	td, err := testutils.TestDataDir()
	require.NoError(t, err)

	mh := mockRPCResponses().Build()

	// Call imgtest init with the following flags. We expect it to load the baseline expectations
	// and the known hashes (both empty).
	ctx, output, exit := testContext(nil, mh, nil, nil)
	env := imgTest{
		commitHash:      "1234567890123456789012345678901234567890",
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
			Results: []*jsonio.Result{{
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
	ctx, output, exit = testContext(mg, nil, nil, mockTime(timeOne))
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

func TestImgTest_InitAdd_StreamingPassFail_MatchesExpectations_ZeroExitCode(t *testing.T) {
	unittest.MediumTest(t)

	workDir := t.TempDir()
	setupAuthWithGSUtil(t, workDir)
	td, err := testutils.TestDataDir()
	require.NoError(t, err)

	mh := mockRPCResponses().Positive("pixel-tests", blankDigest).Build()

	// Call imgtest init with the following flags. We expect it to load the baseline expectations
	// and the known hashes.
	ctx, output, exit := testContext(nil, mh, nil, nil)
	env := imgTest{
		commitHash:      "1234567890123456789012345678901234567890",
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
			Results: []*jsonio.Result{{
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
	ctx, output, exit = testContext(mg, nil, nil, mockTime(timeOne))
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

	fb, err := ioutil.ReadFile(filepath.Join(workDir, "failures.txt"))
	require.NoError(t, err)
	assert.Empty(t, fb)
}

func TestImgTest_InitAdd_StreamingPassFail_SuccessiveCalls_ProperJSONUploaded(t *testing.T) {
	unittest.MediumTest(t)

	workDir := t.TempDir()
	setupAuthWithGSUtil(t, workDir)
	td, err := testutils.TestDataDir()
	require.NoError(t, err)

	mh := mockRPCResponses().Positive("pixel-tests", blankDigest).Build()

	// Call imgtest init with the following flags. We expect it to load the baseline expectations
	// and the known hashes.
	ctx, output, exit := testContext(nil, mh, nil, nil)
	env := imgTest{
		commitHash:      "1234567890123456789012345678901234567890",
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
			Results: []*jsonio.Result{{
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
	ctx, output, exit = testContext(mg, nil, nil, mockTime(timeOne))
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
			Results: []*jsonio.Result{{
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
	ctx, output, exit = testContext(mg, nil, nil, mockTime(timeTwo))
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
	td, err := testutils.TestDataDir()
	require.NoError(t, err)

	keysFile := filepath.Join(workDir, "keys.json")
	require.NoError(t, ioutil.WriteFile(keysFile, []byte(`{"os": "Android"}`), 0644))

	mh := mockRPCResponses().Positive("pixel-tests", blankDigest).Build()

	mg := &mocks.GCSUploader{}
	resultsMatcher := mock.MatchedBy(func(results jsonio.GoldResults) bool {
		assert.Equal(t, jsonio.GoldResults{
			GitHash: "1234567890123456789012345678901234567890",
			Key: map[string]string{
				"os": "Android",
			},
			Results: []*jsonio.Result{{
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
	ctx, output, exit := testContext(mg, mh, nil, mockTime(timeOne))
	env := imgTest{
		commitHash:              "1234567890123456789012345678901234567890",
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

	fb, err := ioutil.ReadFile(filepath.Join(workDir, "failures.txt"))
	require.NoError(t, err)
	assert.Empty(t, fb)
}

func testContext(g gcsuploader.GCSUploader, h httpclient.HTTPClient, i imagedownloader.ImageDownloader, n goldclient.NowSource) (context.Context, *bytes.Buffer, *exitCodeRecorder) {
	output := &bytes.Buffer{}
	exit := &exitCodeRecorder{}

	ctx := executionContext(context.Background(), output, output, exit.ExitWithCode)
	ctx = context.WithValue(ctx, goldclient.NowSourceKey, n)
	return goldclient.WithContext(ctx, g, h, i), output, exit
}

func mockTime(ts time.Time) goldclient.NowSource {
	mt := mocks.NowSource{}
	mt.On("Now").Return(ts)
	return &mt
}

type rpcResponsesBuilder struct {
	knownDigests []string
	exp          *expectations.Expectations
}

func mockRPCResponses() *rpcResponsesBuilder {
	return &rpcResponsesBuilder{
		exp: &expectations.Expectations{},
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

func (r *rpcResponsesBuilder) Build() *mocks.HTTPClient {
	mh := &mocks.HTTPClient{}
	knownResp := strings.Join(r.knownDigests, "\n")
	mh.On("Get", "https://my-instance-gold.skia.org/json/v1/hashes").Return(httpResponse(knownResp, "200 OK", http.StatusOK), nil)

	exp, err := json.Marshal(baseline.Baseline{
		MD5:              "somemd5",
		Expectations:     r.exp.AsBaseline(),
		ChangelistID:     "", // TODO(kjlubick)
		CodeReviewSystem: "", // TODO(kjlubick)
	})
	if err != nil {
		panic(err)
	}
	mh.On("Get", "https://my-instance-gold.skia.org/json/v2/expectations").Return(
		httpResponse(string(exp), "200 OK", http.StatusOK), nil)

	return mh
}

func TestRPCResponsesBuilder_Default_ReturnsBlankValues(t *testing.T) {
	unittest.SmallTest(t)

	mh := mockRPCResponses().Build()
	resp, err := mh.Get("https://my-instance-gold.skia.org/json/v1/hashes")
	require.NoError(t, err)
	b, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "", string(b))

	resp, err = mh.Get("https://my-instance-gold.skia.org/json/v2/expectations")
	require.NoError(t, err)
	b, err = ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, `{"md5":"somemd5"}`, string(b))
}

func TestRPCResponsesBuilder_WithValues_ReturnsValidListsAndJSON(t *testing.T) {
	unittest.SmallTest(t)

	mh := mockRPCResponses().
		Known("first_digest").
		Positive("alpha test", "second_digest").
		Positive("beta test", "third_digest").
		Negative("alpha test", "fourth_digest").
		Known("fifth_digest").
		Build()
	resp, err := mh.Get("https://my-instance-gold.skia.org/json/v1/hashes")
	require.NoError(t, err)
	b, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, `first_digest
second_digest
third_digest
fourth_digest
fifth_digest`, string(b))

	resp, err = mh.Get("https://my-instance-gold.skia.org/json/v2/expectations")
	require.NoError(t, err)
	b, err = ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, `{"md5":"somemd5","primary":{"alpha test":{"fourth_digest":"negative","second_digest":"positive"},"beta test":{"third_digest":"positive"}}}`, string(b))
}
