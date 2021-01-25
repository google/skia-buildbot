package main

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"path/filepath"
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
	"go.skia.org/infra/golden/go/jsonio"
)

func TestImgTest_InitAdd_StreamingPassFail_DoesNotMatchExpectations_NonzeroExitCode(t *testing.T) {
	unittest.MediumTest(t)

	workDir := t.TempDir()
	setupAuthWithGSUtil(t, workDir)
	td, err := testutils.TestDataDir()
	require.NoError(t, err)

	mh := &mocks.HTTPClient{}
	mh.On("Get", "https://my-instance-gold.skia.org/json/v2/expectations").Return(
		httpResponse("{}", "200 OK", http.StatusOK), nil)
	mh.On("Get", "https://my-instance-gold.skia.org/json/v1/hashes").Return(
		httpResponse("", "200 OK", http.StatusOK), nil)

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
	resultsMatcher := mock.MatchedBy(func(results *jsonio.GoldResults) bool {
		assert.Equal(t, &jsonio.GoldResults{
			GitHash: "1234567890123456789012345678901234567890",
			Key: map[string]string{
				"os":          "Android",
				"source_type": "my_corpus",
			},
			Results: []*jsonio.Result{{
				Key:     map[string]string{"name": "pixel-tests", "device": "angler"},
				Options: map[string]string{"some_option": "is optional", "ext": "png"},
				Digest:  "00000000000000000000000000000000",
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
	ctx, output, exit = testContext(mg, nil, nil, mockTime(
		time.Date(2021, time.January, 23, 22, 21, 20, 19, time.UTC)))
	env = imgTest{
		workDir:                 workDir,
		testName:                "pixel-tests",
		pngFile:                 filepath.Join(td, "00000000000000000000000000000000.png"),
		pngDigest:               "00000000000000000000000000000000",
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
