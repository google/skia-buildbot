package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/gold-client/go/mocks"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/web/frontend"
)

func TestDiff_Success(t *testing.T) {
	unittest.MediumTest(t)

	workDir := t.TempDir()
	setupAuthWithGSUtil(t, workDir)

	td, err := testutils.TestDataDir()
	require.NoError(t, err)

	mh := &mocks.HTTPClient{}
	j, err := json.Marshal(frontend.DigestListResponse{Digests: []types.Digest{a05Digest, a09Digest}})
	require.NoError(t, err)
	mh.On("Get", "https://my-instance-gold.skia.org/json/v1/digests?test=pixel-tests&corpus=my_corpus").Return(
		httpResponse(string(j), "200 OK", http.StatusOK), nil)

	a05Bytes, err := ioutil.ReadFile(filepath.Join(td, a05Digest+".png"))
	require.NoError(t, err)
	a09Bytes, err := ioutil.ReadFile(filepath.Join(td, a09Digest+".png"))
	require.NoError(t, err)
	mi := &mocks.ImageDownloader{}
	mi.On("DownloadImage", testutils.AnyContext, "https://my-instance-gold.skia.org", types.Digest(a05Digest)).Return(a05Bytes, nil)
	mi.On("DownloadImage", testutils.AnyContext, "https://my-instance-gold.skia.org", types.Digest(a09Digest)).Return(a09Bytes, nil)

	ctx, output, exit := testContext(nil, mh, mi, nil)
	env := diffEnv{
		test:       "pixel-tests",
		corpus:     "my_corpus",
		instanceID: "my-instance",
		inputFile:  filepath.Join(td, a01Digest+".png"),
		outDir:     filepath.Join(workDir, "output"),
		workDir:    workDir,
	}
	runUntilExit(t, func() {
		env.Diff(ctx)
	})
	logs := output.String()
	exit.AssertWasCalledWithCode(t, 0, logs)

	assert.Equal(t, `Going to compare f528252cd89506d50cf3b59147b8a6c1.png against 2 other images
Digest a05a05a05a05a05a05a05a05a05a05a0 was closest (combined metric of 0.207104)
`, logs)
}
