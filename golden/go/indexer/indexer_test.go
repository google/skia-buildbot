package indexer

import (
	"flag"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/skia-dev/glog"
	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/database/testutil"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/gs"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/tiling"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/golden/go/db"
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
)

// Flags used by benchmarks. Everything else uses reasonable assumptions based
// on a local setup of tracdb and skiaingestion.
var (
	traceService = flag.String("trace_service", "localhost:9001", "The address of the traceservice endpoint.")
	dbName       = flag.String("db_name", "gold_skiacorrectness", "The name of the databased to use. User 'readwrite' and local test settings are assumed.")
)

func TestIndexer(t *testing.T) {
	testutils.SkipIfShort(t)

	err := gs.DownloadTestDataFile(t, gs.TEST_DATA_BUCKET, TEST_DATA_STORAGE_PATH, TEST_DATA_PATH)
	assert.NoError(t, err, "Unable to download testdata.")
	defer testutils.RemoveAll(t, TEST_DATA_DIR)

	tileBuilder := mocks.NewMockTileBuilderFromJson(t, TEST_DATA_PATH)
	eventBus := eventbus.New(nil)
	expStore := expstorage.NewMemExpectationsStore(eventBus)

	storages := &storage.Storage{
		ExpectationsStore: expStore,
		MasterTileBuilder: tileBuilder,
		DigestStore: &mocks.MockDigestStore{
			FirstSeen: time.Now().Unix(),
			OkValue:   true,
		},
		DiffStore: mocks.NewMockDiffStore(),
		EventBus:  eventBus,
	}

	ixr, err := New(storages, time.Minute)
	assert.NoError(t, err)

	idxOne := ixr.GetIndex()

	// Change the classifications.
	changes := getChanges(t, idxOne.tilePair.Tile)
	assert.NoError(t, storages.ExpectationsStore.AddChange(changes, ""))

	// Wait for the re-indexing.
	time.Sleep(time.Second)
	idxTwo := ixr.GetIndex()

	assert.NotEqual(t, idxOne, idxTwo)
}

func getChanges(t *testing.T, tile *tiling.Tile) map[string]types.TestClassification {
	ret := map[string]types.TestClassification{}
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
	storages, expStore := setupStorages(b)
	defer testutils.RemoveAll(b, REPO_DIR)

	// Build the initial index.
	b.ResetTimer()
	_, err := New(storages, time.Minute*15)
	assert.NoError(b, err)

	// Update the expectations.
	changes, err := expStore.Get()
	assert.NoError(b, err)

	glog.Infof("Got %d tests", len(changes.Tests))

	// Wait for the indexTests to complete when we change the expectations.
	var wg sync.WaitGroup
	wg.Add(1)
	storages.EventBus.SubscribeAsync(EV_INDEX_UPDATED, func(state interface{}) {
		wg.Done()
	})
	assert.NoError(b, storages.ExpectationsStore.AddChange(changes.Tests, ""))
	wg.Wait()
}

func setupStorages(t assert.TestingT) (*storage.Storage, expstorage.ExpectationsStore) {
	flag.Parse()

	// Set up the database configuration.
	dbConf := testutil.LocalTestDatabaseConfig(db.MigrationSteps())
	dbConf.User = DB_USER
	dbConf.Name = *dbName

	// Set up the diff store, the event bus and the DB connection.
	diffStore := mocks.NewMockDiffStore()
	evt := eventbus.New(nil)
	vdb, err := dbConf.NewVersionedDB()
	assert.NoError(t, err)
	expStore := expstorage.NewCachingExpectationStore(expstorage.NewSQLExpectationStore(vdb), evt)

	git, err := gitinfo.CloneOrUpdate(REPO_URL, REPO_DIR, false)
	assert.NoError(t, err)

	traceDB, err := tracedb.NewTraceServiceDBFromAddress(*traceService, types.GoldenTraceBuilder)
	assert.NoError(t, err)

	masterTileBuilder, err := tracedb.NewMasterTileBuilder(traceDB, git, N_COMMITS, evt)
	assert.NoError(t, err)

	ret := &storage.Storage{
		DiffStore:         diffStore,
		ExpectationsStore: expstorage.NewMemExpectationsStore(evt),
		MasterTileBuilder: masterTileBuilder,
		BranchTileBuilder: nil,
		DigestStore:       &mocks.MockDigestStore{IssueIDs: []int{}, OkValue: true},
		NCommits:          N_COMMITS,
		EventBus:          evt,
		TrybotResults:     nil,
		RietveldAPI:       nil,
	}

	ret.IgnoreStore = ignore.NewSQLIgnoreStore(vdb, ret.ExpectationsStore, ret.GetTileStreamNow(time.Minute))

	_, err = ret.GetLastTileTrimmed()
	assert.NoError(t, err)
	return ret, expStore
}
