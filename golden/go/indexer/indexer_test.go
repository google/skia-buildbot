package indexer

import (
	"context"
	"flag"
	"math/rand"
	"sync"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/ds"
	ds_testutil "go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/tiling"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/ignore"
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

	// REPO_URL is the url of the repo to check out.
	REPO_URL = "https://skia.googlesource.com/skia"

	// REPO_DIR contains the location of where to check out Skia for benchmarks.
	REPO_DIR = "./skia"

	// N_COMMITS is the number of commits used in benchmarks.
	N_COMMITS = 50

	// Database user used by benchmarks.
	DB_USER = "readwrite"

	// TEST_HASHES_PATH is the GCS path where the file will be written.
	TEST_HASHES_PATH = "skia-infra-testdata/hash_files/testing-known-hashes.txt"
)

// Flags used by benchmarks. Everything else uses reasonable assumptions based
// on a local setup of tracedb and skia_ingestion.
var (
	traceService = flag.String("trace_service", "localhost:9001", "The address of the traceservice endpoint.")
	dbName       = flag.String("db_name", "gold_skiacorrectness", "The name of the databased to use. User 'readwrite' and local test settings are assumed.")
)

func TestIndexer(t *testing.T) {
	testutils.LargeTest(t)

	err := gcs.DownloadTestDataFile(t, gcs.TEST_DATA_BUCKET, TEST_DATA_STORAGE_PATH, TEST_DATA_PATH)
	assert.NoError(t, err, "Unable to download testdata.")
	defer testutils.RemoveAll(t, TEST_DATA_DIR)

	tileBuilder := mocks.NewMockTileBuilderFromJson(t, TEST_DATA_PATH)
	eventBus := eventbus.New()
	expStore := expstorage.NewMemExpectationsStore(eventBus)

	opts := &storage.GSClientOptions{
		HashesGSPath: TEST_HASHES_PATH,
	}
	gsClient, err := storage.NewGStorageClient(mocks.GetHTTPClient(t), opts)
	assert.NoError(t, err)
	assert.NotNil(t, gsClient)

	storages := &storage.Storage{
		ExpectationsStore: expStore,
		MasterTileBuilder: tileBuilder,
		DigestStore: &mocks.MockDigestStore{
			FirstSeen: time.Now().Unix(),
			OkValue:   true,
		},
		DiffStore:      mocks.NewMockDiffStore(),
		EventBus:       eventBus,
		GStorageClient: gsClient,
	}

	assert.NoError(t, storages.InitBaseliner())

	ixr, err := New(storages, time.Minute)
	assert.NoError(t, err)

	idxOne := ixr.GetIndex()

	// Set up a waitgroup so we can block until the index is updated.
	var wg sync.WaitGroup
	eventBus.SubscribeAsync(EV_INDEX_UPDATED, func(ignore interface{}) {
		wg.Done()
	})
	wg.Add(1)

	// Change the classifications and wait for the indexing to propagate.
	changes := getChanges(t, idxOne.cpxTile.GetTile(false))
	assert.NoError(t, storages.ExpectationsStore.AddChange(changes, ""))
	wg.Wait()

	// Make sure the new index is different from the previous one.
	idxTwo := ixr.GetIndex()
	assert.NotEqual(t, idxOne, idxTwo)
}

func getChanges(t *testing.T, tile *tiling.Tile) types.TestExp {
	ret := types.TestExp{}
	labelVals := []types.Label{types.POSITIVE, types.NEGATIVE}
	for _, trace := range tile.Traces {
		if rand.Float32() > 0.5 {
			gTrace := trace.(*types.GoldenTrace)
			for _, digest := range gTrace.Values {
				if digest != types.MISSING_DIGEST {
					testName := gTrace.Params_[types.PRIMARY_KEY_FIELD]
					if found, ok := ret[testName]; ok {
						found[digest] = labelVals[rand.Int()%2]
					} else {
						ret[testName] = types.TestClassification{digest: labelVals[rand.Int()%2]}
					}
				}
			}
		}
	}

	assert.True(t, len(ret) > 0)
	return ret
}

func BenchmarkIndexer(b *testing.B) {
	ctx := context.Background()
	storages, expStore := setupStorages(b, ctx)
	defer testutils.RemoveAll(b, REPO_DIR)

	// Build the initial index.
	b.ResetTimer()
	_, err := New(storages, time.Minute*15)
	assert.NoError(b, err)

	// Update the expectations.
	changes, err := expStore.Get()
	assert.NoError(b, err)

	changesTestExp := changes.TestExp()

	// Wait for the indexTests to complete when we change the expectations.
	var wg sync.WaitGroup
	wg.Add(1)
	storages.EventBus.SubscribeAsync(EV_INDEX_UPDATED, func(state interface{}) {
		wg.Done()
	})
	assert.NoError(b, storages.ExpectationsStore.AddChange(changesTestExp, ""))
	wg.Wait()
}

func setupStorages(t testutils.TestingT, ctx context.Context) (*storage.Storage, expstorage.ExpectationsStore) {
	flag.Parse()

	// Set up the diff store, the event bus and the DB connection.
	diffStore := mocks.NewMockDiffStore()
	evt := eventbus.New()

	// Set up the cloud datasstore and initialize the expectations store.
	cleanup := ds_testutil.InitDatastore(t, ds.KindsToBackup[ds.GOLD_SKIA_PROD_NS]...)
	defer cleanup()

	cloudExpStore, _, err := expstorage.NewCloudExpectationsStore(ds.DS, evt)
	assert.NoError(t, err)

	expStore := expstorage.NewCachingExpectationStore(cloudExpStore, evt)

	git, err := gitinfo.CloneOrUpdate(context.Background(), REPO_URL, REPO_DIR, false)
	assert.NoError(t, err)

	traceDB, err := tracedb.NewTraceServiceDBFromAddress(*traceService, types.GoldenTraceBuilder)
	assert.NoError(t, err)

	masterTileBuilder, err := tracedb.NewMasterTileBuilder(ctx, traceDB, git, N_COMMITS, evt, "")
	assert.NoError(t, err)

	ret := &storage.Storage{
		DiffStore:         diffStore,
		ExpectationsStore: expstorage.NewMemExpectationsStore(evt),
		MasterTileBuilder: masterTileBuilder,
		DigestStore:       &mocks.MockDigestStore{IssueIDs: []int{}, OkValue: true},
		NCommits:          N_COMMITS,
		EventBus:          evt,
	}

	ret.IgnoreStore, err = ignore.NewCloudIgnoreStore(ds.DS, expStore, ret.GetTileStreamNow(time.Minute))
	assert.NoError(t, err)

	tilePair, err := ret.GetLastTileTrimmed()
	assert.NoError(t, err)

	assert.True(t, len(tilePair.IgnoreRules()) > 0)
	return ret, expStore
}
