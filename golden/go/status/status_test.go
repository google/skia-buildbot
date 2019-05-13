package status

import (
	"context"
	"math/rand"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gcs/gcs_testutils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/golden/go/digeststore"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/types"
)

const (
	// Directory with testdata.
	TEST_DATA_DIR = "./testdata"

	// Local file location of the test data.
	TEST_DATA_PATH = TEST_DATA_DIR + "/goldentile.json.zip"

	// Folder in the testdata bucket. See go/testutils for details.
	TEST_DATA_STORAGE_PATH = "gold-testdata/goldentile.json.gz"
)

func TestStatusWatcher(t *testing.T) {
	unittest.LargeTest(t)

	err := gcs_testutils.DownloadTestDataFile(t, gcs_testutils.TEST_DATA_BUCKET, TEST_DATA_STORAGE_PATH, TEST_DATA_PATH)
	assert.NoError(t, err, "Unable to download testdata.")
	defer testutils.RemoveAll(t, TEST_DATA_DIR)

	tileBuilder := mocks.NewMockTileBuilderFromJson(t, TEST_DATA_PATH)
	testStatusWatcher(t, tileBuilder)
}

func BenchmarkStatusWatcher(b *testing.B) {
	ctx := context.Background()
	// Get the TEST_TILE environment variable that points to the
	// tile to read.
	tileBuilder := mocks.GetTileBuilderFromEnv(b, ctx)

	storages := &storage.Storage{
		MasterTileBuilder: tileBuilder,
	}

	// Load the tile into memory and reset the timer to avoid measuring
	// disk load time.
	_, err := storages.GetLastTileTrimmed()
	assert.NoError(b, err)
	b.ResetTimer()
	testStatusWatcher(b, tileBuilder)
}

func testStatusWatcher(t assert.TestingT, tileBuilder tracedb.MasterTileBuilder) {
	eventBus := eventbus.New()
	storages := &storage.Storage{
		ExpectationsStore: expstorage.NewMemExpectationsStore(eventBus),
		MasterTileBuilder: tileBuilder,
		EventBus:          eventBus,
	}

	watcher, err := New(storages)
	assert.NoError(t, err)

	// Go through all corpora and change all the Items to positive.
	status := watcher.GetStatus()
	assert.NotNil(t, status)

	for idx, corpStatus := range status.CorpStatus {
		assert.False(t, corpStatus.OK)
		cpxTile, err := storages.GetLastTileTrimmed()
		assert.NoError(t, err)

		changes := types.Expectations{}
		posOrNeg := []types.Label{types.POSITIVE, types.NEGATIVE}
		for _, trace := range cpxTile.GetTile(types.ExcludeIgnoredTraces).Traces {
			if trace.Params()[types.CORPUS_FIELD] == corpStatus.Name {
				gTrace := trace.(*types.GoldenTrace)
				testName := gTrace.TestName()
				for _, digest := range gTrace.Digests {
					if _, ok := changes[testName]; !ok {
						changes[testName] = map[types.Digest]types.Label{}
					}
					changes[testName][digest] = posOrNeg[rand.Int()%2]
				}
			}
		}

		// Update the expectations and wait for the status to change.
		assert.NoError(t, storages.ExpectationsStore.AddChange(changes, ""))
		time.Sleep(1 * time.Second)
		assert.NoError(t, storages.ExpectationsStore.AddChange(changes, ""))
		time.Sleep(1 * time.Second)

		// Make sure the current corpus is now ok.
		newStatus := watcher.GetStatus()
		assert.True(t, newStatus.CorpStatus[idx].OK)
	}

	// All corpora are ok therefore the overall status should be ok.
	newStatus := watcher.GetStatus()
	assert.True(t, newStatus.OK)
}

type MockDigestStore struct {
	issueIDs []int
}

func (m *MockDigestStore) Get(testName, digest string) (*digeststore.DigestInfo, bool, error) {
	return &digeststore.DigestInfo{
		IssueIDs: m.issueIDs,
	}, true, nil
}

func (m *MockDigestStore) Update([]*digeststore.DigestInfo) error {
	return nil
}
