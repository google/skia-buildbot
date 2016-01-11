package perfingestion

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/testutils"
	tracedb "go.skia.org/infra/go/trace/db"
)

// TestTrybotBenchData tests parsing and processing of a single trybot file.
func TestTrybotBenchData(t *testing.T) {
	// Load the sample data file as BenchData.
	r, err := os.Open(filepath.Join(TEST_DATA_DIR, "trybot.json"))
	assert.Nil(t, err)

	benchData, err := parseBenchDataFromReader(r)
	assert.Nil(t, err)

	assert.Equal(t, "x86_64:Clang:GPU:GeForce320M:MacMini4.1:Mac10.8", benchData.keyPrefix())
	assert.Equal(t, "1467533002", benchData.Issue)
	assert.Equal(t, "1", benchData.PatchSet)
}

func TestTrybotPerfIngestion(t *testing.T) {
	b, err := ioutil.ReadFile(filepath.Join("testdata", "rietveld_response.txt"))
	assert.NoError(t, err)
	m := mockhttpclient.NewURLMock()
	m.Mock("https://codereview.chromium.org/api/1467533002/1", b)

	server, serverAddr := ingestion.StartTraceDBTestServer(t, "./trybot_test_trace.db")
	defer server.Stop()
	defer testutils.Remove(t, "./trybot_test_trace.db")

	ingesterConf := &sharedconfig.IngesterConfig{
		ExtraParams: map[string]string{
			CONFIG_TRACESERVICE: serverAddr,
		},
	}
	processor, err := newPerfTrybotProcessor(nil, ingesterConf, nil)
	assert.Nil(t, err)

	processor.(*perfTrybotProcessor).review = rietveld.New("https://codereview.chromium.org", m.Client())

	fsResult, err := ingestion.FileSystemResult(filepath.Join(TEST_DATA_DIR, "trybot.json"))
	assert.Nil(t, err)
	err = processor.Process(fsResult)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(processor.(*perfTrybotProcessor).cache))

	// Steal the traceDB used by the processor to verify the results.
	traceDB := processor.(*perfTrybotProcessor).traceDB

	startTime := time.Time{}
	commitIDs, err := traceDB.List(startTime, time.Now())
	assert.Nil(t, err)

	assert.Equal(t, 1, len(commitIDs))
	assert.Equal(t, &tracedb.CommitID{
		Timestamp: 1448036640,
		ID:        "1",
		Source:    "https://codereview.chromium.org/1467533002",
	}, commitIDs[0])

	// Get a tile and make sure we have the right number of traces.
	tile, _, err := traceDB.TileFromCommits(commitIDs)
	assert.Nil(t, err)

	traces := tile.Traces
	assert.Equal(t, 2, len(traces))

	assert.Nil(t, traceDB.Close())
}
