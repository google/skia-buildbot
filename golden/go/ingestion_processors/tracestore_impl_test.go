package ingestion_processors

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vcsinfo"
	mock_vcs "go.skia.org/infra/go/vcsinfo/mocks"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/tracestore"
	"go.skia.org/infra/golden/go/types"
)

// TestTraceStoreProcessorSunnyDay tests that the ingestion makes the right calls
// to the VCS and the Tracestore for the sample file in testdata/.
func TestTraceStoreProcessorSunnyDay(t *testing.T) {
	unittest.MediumTest(t)

	mts := &mocks.TraceStore{}
	mvs := &mock_vcs.VCS{}
	defer mts.AssertExpectations(t)
	defer mvs.AssertExpectations(t)

	commit := &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash:    testCommitHash,
			Subject: "Really big code change",
		},
		// arbitrary time
		Timestamp: time.Date(2019, time.June, 16, 3, 23, 17, 0, time.UTC),
	}
	mvs.On("Details", testutils.AnyContext, testCommitHash, false).Return(commit, nil)
	// arbitrary result
	mvs.On("IndexOf", testutils.AnyContext, testCommitHash).Return(12, nil)

	// There are 3 entries in the file, but one of them is pdf, which should
	// be ignored by this ingester.
	expectedEntries := []*tracestore.Entry{
		{
			Params: map[string]string{
				"arch":                "x86_64",
				"compiler":            "MSVC",
				"config":              "pipe-8888",
				"configuration":       "Debug",
				"cpu_or_gpu":          "CPU",
				"cpu_or_gpu_value":    "AVX2",
				"model":               "ShuttleB",
				types.PrimaryKeyField: "aaclip",
				"os":                  "Win8",
				types.CorpusField:     "gm",
			},
			Options: map[string]string{
				"ext": "png",
			},
			Digest: "fa3c371d201d6f88f7a47b41862e2e85",
		},
		{
			Params: map[string]string{
				"arch":                "x86_64",
				"compiler":            "MSVC",
				"config":              "pipe-8888",
				"configuration":       "Debug",
				"cpu_or_gpu":          "CPU",
				"cpu_or_gpu_value":    "AVX2",
				"model":               "ShuttleB",
				types.PrimaryKeyField: "clipcubic",
				"os":                  "Win8",
				types.CorpusField:     "gm",
			},
			Options: map[string]string{
				"ext": "png",
			},
			Digest: "64e446d96bebba035887dd7dda6db6c4",
		},
	}

	mts.On("Put", testutils.AnyContext, testCommitHash, expectedEntries, mock.AnythingOfType("time.Time")).Return(nil)

	src := fakeGCSSourceFromFile(t, "dm.json")
	p := &btProcessor{
		ts:     mts,
		vcs:    mvs,
		source: src,
	}
	err := p.Process(context.Background(), "whatever")
	assert.NoError(t, err)
}

const (
	testCommitHash = "02cb37309c01506e2552e931efa9c04a569ed266"
)

type fakeGCSSource struct {
	content []byte
}

func fakeGCSSourceFromFile(t *testing.T, file string) *fakeGCSSource {
	fp := filepath.Join(testutils.TestDataDir(t), file)
	b, err := ioutil.ReadFile(fp)
	require.NoError(t, err)
	return &fakeGCSSource{content: b}
}

func (f *fakeGCSSource) GetReader(_ context.Context, _ string) (io.ReadCloser, error) {
	return ioutil.NopCloser(bytes.NewReader(f.content)), nil
}

func (f *fakeGCSSource) HandlesFile(_ string) bool {
	return true
}
