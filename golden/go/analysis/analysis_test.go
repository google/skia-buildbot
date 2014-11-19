package analysis

import (
	"os"
	"testing"
	"time"

	"skia.googlesource.com/buildbot.git/go/auth"
	"skia.googlesource.com/buildbot.git/go/gs"
	"skia.googlesource.com/buildbot.git/golden/go/expstorage"
	"skia.googlesource.com/buildbot.git/golden/go/filediffstore"
	"skia.googlesource.com/buildbot.git/golden/go/types"
	"skia.googlesource.com/buildbot.git/perf/go/filetilestore"
)

import (
	// Using 'require' which is like using 'assert' but causes tests to fail.
	assert "github.com/stretchr/testify/require"
)

func init() {
	filediffstore.Init()
}

// TODO (stephana): WIP to make the analyzer more testable.
func TestAnalyzer(t *testing.T) {
	analyzer := setupAnalyzer(t)
	assert.NotNil(t, analyzer)

	waitForLoopCounter(analyzer, 1, time.Second*5)

	tileCounts, err := analyzer.GetTileCounts(map[string][]string{})
	assert.Nil(t, err)

	for testName, _ := range tileCounts.Counts {
		testDetails, err := analyzer.GetTestDetails(testName, map[string][]string{})
		assert.Nil(t, err)

		triagedTests := map[string]types.TestClassification{}
		triagedTests[testName] = map[string]types.Label{}
		posDigests := map[string]bool{}
		negDigests := map[string]bool{}
		i := 0
		for digest := range testDetails.Tests[testName].Untriaged {
			if i%2 == 0 {
				posDigests[digest] = true
				triagedTests[testName][digest] = types.POSITIVE
			} else {
				negDigests[digest] = true
				triagedTests[testName][digest] = types.NEGATIVE
			}
			i++
		}

		assert.Equal(t, len(posDigests)+len(negDigests), len(testDetails.Tests[testName].Untriaged))

		// Set the digests and check if the result is correct.
		result, err := analyzer.SetDigestLabels(triagedTests, "fakeUserId")
		assert.Nil(t, err)

		found := result.Tests[testName]
		assert.NotNil(t, found)
		assert.Equal(t, len(posDigests), len(found.Positive))
		assert.Equal(t, len(negDigests), len(found.Negative))
		assert.Equal(t, 0, len(found.Untriaged))

		for digest := range posDigests {
			assert.NotNil(t, found.Positive[digest])
		}

		for digest := range negDigests {
			assert.NotNil(t, found.Negative[digest])
		}
	}
}

func setupAnalyzer(t *testing.T) *Analyzer {
	// TODO (stephana): Needs to be cleaned up so it doesn't depend on hard
	// coded paths.
	imageDir := "../../testruns/imagediffs"
	gsBucketName := gs.GS_PROJECT_BUCKET
	tileStoreDir := "../../../../../../../checkouts/tiles"

	// Skip this test if the directories don't exist.
	if _, err := os.Stat(imageDir); os.IsNotExist(err) {
		t.Skipf("Skiping test because %s does not exist.", imageDir)
	}
	if _, err := os.Stat(tileStoreDir); os.IsNotExist(err) {
		t.Skipf("Skiping test because %s does not exist.", tileStoreDir)
	}

	oauthClient, err := auth.RunFlow(auth.DefaultOAuthConfig("./google_storage_token.data"))
	assert.Nil(t, err)

	// Get the expecations storage, the filediff storage and the tilestore.
	diffStore := filediffstore.NewFileDiffStore(oauthClient, imageDir, gsBucketName, filediffstore.RECOMMENDED_WORKER_POOL_SIZE)
	expStore := expstorage.NewMemExpectationsStore()
	tileStore := filetilestore.NewFileTileStore(tileStoreDir, "golden", -1)

	// Initialize the Analyzer
	return NewAnalyzer(expStore, tileStore, diffStore, mockUrlGenerator, 5*time.Minute)
}

func mockUrlGenerator(path string) string {
	return path
}

func waitForLoopCounter(a *Analyzer, minCount int, pollInterval time.Duration) {
	for _ = range time.Tick(pollInterval) {
		if a.loopCounter >= minCount {
			break
		}
	}
}
