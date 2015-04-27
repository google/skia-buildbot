package status

import (
	"math/rand"
	"os"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/digeststore"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/types"
	pconfig "go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/filetilestore"
	ptypes "go.skia.org/infra/perf/go/types"
)

var (
	// Directory with testdata.
	TEST_DATA_DIR = "./testdata"

	// Local file location of the test data.
	TEST_DATA_PATH = TEST_DATA_DIR + "/goldentile.json.zip"

	// Folder in the testdata bucket. See go/testutils for details.
	TEST_DATA_STORAGE_PATH = "gold-testdata/goldentile.json.gz"
)

func TestStatusWatcher(t *testing.T) {
	testutils.SkipIfShort(t)

	err := testutils.DownloadTestDataFile(t, TEST_DATA_STORAGE_PATH, TEST_DATA_PATH)
	assert.Nil(t, err, "Unable to download testdata.")
	defer testutils.RemoveAll(t, TEST_DATA_DIR)

	tileStore := mocks.NewMockTileStoreFromJson(t, TEST_DATA_PATH)
	testStatusWatcher(t, tileStore)
}

func BenchmarkStatusWatcher(b *testing.B) {
	// Get the TEST_TILE environment variable that points to the
	// tile to read.
	tileDir := os.Getenv("TEST_TILE_DIR")
	assert.NotEqual(b, "", tileDir, "Please define the TEST_TILE_DIR environment variable to point to a live tile store.")
	tileStore := filetilestore.NewFileTileStore(tileDir, pconfig.DATASET_GOLD, 2*time.Minute)

	storages := &storage.Storage{
		TileStore: tileStore,
	}

	// Load the tile into memory and reset the timer to avoid measuring
	// disk load time.
	_, err := storages.GetLastTileTrimmed(true)
	assert.Nil(b, err)
	b.ResetTimer()
	testStatusWatcher(b, tileStore)
}

func testStatusWatcher(t assert.TestingT, tileStore ptypes.TileStore) {
	eventBus := eventbus.New()
	storages := &storage.Storage{
		ExpectationsStore: expstorage.NewMemExpectationsStore(eventBus),
		TileStore:         tileStore,
		DigestStore:       &MockDigestStore{},
		EventBus:          eventBus,
	}

	watcher, err := New(storages)
	assert.Nil(t, err)

	// Go through all corpora and change all the Items to positive.
	status := watcher.GetStatus()
	assert.NotNil(t, status)

	for corpus, corpStatus := range status.CorpStatus {
		// Make sure no digests has any issues attached.
		storages.DigestStore.(*MockDigestStore).issueIDs = nil

		assert.False(t, corpStatus.OK)
		tile, err := storages.GetLastTileTrimmed(true)
		assert.Nil(t, err)

		changes := map[string]types.TestClassification{}
		posOrNeg := []types.Label{types.POSITIVE, types.NEGATIVE}
		for _, trace := range tile.Traces {
			if trace.Params()[types.CORPUS_FIELD] == corpus {
				gTrace := trace.(*ptypes.GoldenTrace)
				testName := gTrace.Params()[types.PRIMARY_KEY_FIELD]
				for _, digest := range gTrace.Values {
					if _, ok := changes[testName]; !ok {
						changes[testName] = map[string]types.Label{}
					}
					changes[testName][digest] = posOrNeg[rand.Int()%2]
				}
			}
		}

		// Update the expecations and wait for the status to change.
		assert.Nil(t, storages.ExpectationsStore.AddChange(changes, ""))
		time.Sleep(1 * time.Second)
		newStatus := watcher.GetStatus()
		assert.False(t, newStatus.CorpStatus[corpus].OK)
		assert.False(t, newStatus.OK)

		// Make sure all tests have an issue attached to each DigestInfo and
		// trigger another expectations update.
		storages.DigestStore.(*MockDigestStore).issueIDs = []int{1}
		assert.Nil(t, storages.ExpectationsStore.AddChange(changes, ""))
		time.Sleep(1 * time.Second)

		// Make sure the current corpus is now ok.
		newStatus = watcher.GetStatus()
		assert.True(t, newStatus.CorpStatus[corpus].OK)
	}

	// All corpora are ok therefore the overall status should be ok.
	newStatus := watcher.GetStatus()
	assert.True(t, newStatus.OK)
}

type MockDigestStore struct {
	issueIDs []int
}

func (m *MockDigestStore) GetDigestInfo(testName, digest string) (*digeststore.DigestInfo, bool, error) {
	return &digeststore.DigestInfo{
		IssueIDs: m.issueIDs,
	}, true, nil
}

func (m *MockDigestStore) UpdateDigestTimeStamps(testName, digest string, commit *ptypes.Commit) (*digeststore.DigestInfo, error) {
	ret, _, err := m.GetDigestInfo(testName, digest)
	return ret, err
}
