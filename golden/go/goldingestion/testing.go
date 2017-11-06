package goldingestion

import (
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/testhelpers"
	"google.golang.org/grpc"
)

// RunGoldTrybotProcessor sets up the necessary data stores based on the values of traceDBFile and shareDBDir. It then
// ingests the content of ingestionFile by using the code review system at reviewURL (will not be called).
// After the successful ingestion it returns the instance of running GRPC server and the server address.
// When all tests are done it's the responsibility of the caller to call server.Stop() and remove all
// data directories.
func RunGoldTrybotProcessor(t assert.TestingT, traceDBFile, shareDBDir, ingestionFile, rootDir, rietveldReviewURL string, gerritReviewURL string) (*grpc.Server, string) {
	shareDBDir, err := fileutil.EnsureDirExists(shareDBDir)
	assert.NoError(t, err)

	// Extract the commits from the file.
	fsResult, err := ingestion.FileSystemResult(ingestionFile, rootDir)
	assert.NoError(t, err)

	r, err := fsResult.Open()
	assert.NoError(t, err)
	dmResults, err := ParseDMResultsFromReader(r, fsResult.Name())
	assert.NoError(t, err)

	now := time.Now()
	testCommits := []*vcsinfo.LongCommit{
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    dmResults.GitHash,
				Subject: "Really big code change",
			},
			Timestamp: now.Add(-time.Second * 10).Add(-time.Nanosecond * time.Duration(now.Nanosecond())),
			Branches:  map[string]bool{"master": true},
		},
	}

	// Set up mock VCS and run a servcer with the given data directory.
	vcs := ingestion.MockVCS(testCommits, nil)
	server, serverAddr := testhelpers.StartTraceDBTestServer(t, traceDBFile, shareDBDir)

	ingesterConf := &sharedconfig.IngesterConfig{
		ExtraParams: map[string]string{
			CONFIG_TRACESERVICE:             serverAddr,
			CONFIG_RIETVELD_CODE_REVIEW_URL: rietveldReviewURL,
			CONFIG_GERRIT_CODE_REVIEW_URL:   gerritReviewURL,
		},
	}

	ingestionStore, err := NewIngestionStore(serverAddr)
	assert.NoError(t, err)
	defer func() { assert.NoError(t, ingestionStore.Close()) }()

	// Set up the processor.
	processor, err := newGoldTrybotProcessor(vcs, ingesterConf, nil)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, processor.(*goldTrybotProcessor).traceDB.Close())
		assert.NoError(t, processor.(*goldTrybotProcessor).ingestionStore.Close())
	}()

	// Load the example file and process it.
	fsResult, err = ingestion.FileSystemResult(ingestionFile, "./")
	assert.NoError(t, err)
	err = processor.Process(fsResult)
	assert.NoError(t, err)

	// Make sure recorded that correct master/builder/build_number was recorded.
	assert.True(t, ingestionStore.IsIngested(config.CONSTRUCTOR_GOLD, dmResults.Master, dmResults.Builder, dmResults.BuildNumber))

	// Make sure we get false for arbitrary build information.
	assert.False(t, ingestionStore.IsIngested(config.CONSTRUCTOR_GOLD, "client.skia", "Test-Win8-MSVC-ShuttleB-CPU-AVX2-x86-Debug", 9944))

	return server, serverAddr
}
