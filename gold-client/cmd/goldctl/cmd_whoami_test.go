package main

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/gold-client/go/goldclient"
	"go.skia.org/infra/gold-client/go/mocks"
)

func TestWhoami_AuthedWithGSUtil_Success(t *testing.T) {
	unittest.MediumTest(t)

	workDir := t.TempDir()
	setupAuthWithGSUtil(t, workDir)

	who := whoamiEnv{
		workDir:    workDir,
		instanceID: "my-test-instance",
	}
	output := bytes.Buffer{}
	exit := &exitCodeRecorder{}
	ctx := executionContext(context.Background(), &output, &output, exit.ExitWithCode)

	mh := &mocks.HTTPClient{}
	url := "https://my-test-instance-gold.skia.org/json/v1/whoami"
	response := `{"whoami": "test@example.com"}`
	mh.On("Get", url).Return(httpResponse(response, "200 OK", http.StatusOK), nil)

	ctx = goldclient.WithContext(ctx, nil, mh, nil)

	runUntilExit(t, func() {
		who.WhoAmI(ctx)
	})
	exit.AssertWasCalledWithCode(t, 0, output.String())
	assert.Contains(t, output.String(), `Logged in as "test@example.com"`)
}

func TestWhoami_ReallyPollServer_NotLoggedIn(t *testing.T) {
	unittest.LargeTest(t) // This test really makes a network request to skia-infra.gold.skia.org

	workDir := t.TempDir()
	setupAuthWithGSUtil(t, workDir)

	who := whoamiEnv{
		workDir:    workDir,
		instanceID: "skia-infra",
	}
	output := bytes.Buffer{}
	exit := &exitCodeRecorder{}
	ctx := executionContext(context.Background(), &output, &output, exit.ExitWithCode)

	runUntilExit(t, func() {
		who.WhoAmI(ctx)
	})
	exit.AssertWasCalledWithCode(t, 0, output.String())
	assert.Contains(t, output.String(), `Logged in as ""`)
}

func httpResponse(body, status string, statusCode int) *http.Response {
	return &http.Response{
		Body:       ioutil.NopCloser(strings.NewReader(body)),
		Status:     status,
		StatusCode: statusCode,
	}
}
