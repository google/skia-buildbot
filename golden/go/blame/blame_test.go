package blame

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/tiling"
	tracedb "go.skia.org/infra/go/trace/db"
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

func TestBlamerWithSyntheticData(t *testing.T) {
	testutils.SmallTest(t)
	start := time.Now().Unix()
	commits := []*tiling.Commit{
		{CommitTime: start + 10, Hash: "h1", Author: "John Doe 1"},
		{CommitTime: start + 20, Hash: "h2", Author: "John Doe 2"},
		{CommitTime: start + 30, Hash: "h3", Author: "John Doe 3"},
		{CommitTime: start + 40, Hash: "h4", Author: "John Doe 4"},
		{CommitTime: start + 50, Hash: "h5", Author: "John Doe 5"},
	}

	params := []map[string]string{
		{"name": "foo", "config": "8888", types.CORPUS_FIELD: "gm"},
		{"name": "foo", "config": "565", types.CORPUS_FIELD: "gm"},
		{"name": "foo", "config": "gpu", types.CORPUS_FIELD: "gm"},
		{"name": "bar", "config": "8888", types.CORPUS_FIELD: "gm"},
		{"name": "bar", "config": "565", types.CORPUS_FIELD: "gm"},
		{"name": "bar", "config": "gpu", types.CORPUS_FIELD: "gm"},
		{"name": "baz", "config": "565", types.CORPUS_FIELD: "gm"},
		{"name": "baz", "config": "gpu", types.CORPUS_FIELD: "gm"},
	}

	DI_1, DI_2, DI_3 := "digest1", "digest2", "digest3"
	DI_4, DI_5, DI_6 := "digest4", "digest5", "digest6"
	DI_7, DI_8, DI_9 := "digest7", "digest8", "digest9"
	MISS := types.MISSING_DIGEST

	digests := [][]string{
		{MISS, MISS, DI_1, MISS, MISS},
		{MISS, DI_1, DI_1, DI_2, MISS},
		{DI_3, MISS, MISS, MISS, MISS},
		{DI_5, DI_4, DI_5, DI_5, DI_5},
		{DI_6, MISS, DI_4, MISS, MISS},
		{MISS, MISS, MISS, MISS, MISS},
		{DI_7, DI_7, MISS, DI_8, MISS},
		{DI_7, MISS, DI_7, DI_8, MISS},
	}

	// Make sure the data are consistent and create a mock TileStore.
	assert.Equal(t, len(commits), len(digests[0]))
	assert.Equal(t, len(digests), len(params))

	eventBus := eventbus.New()
	storages := &storage.Storage{
		ExpectationsStore: expstorage.NewMemExpectationsStore(eventBus),
		MasterTileBuilder: mocks.NewMockTileBuilder(t, digests, params, commits),
		DigestStore:       &mocks.MockDigestStore{FirstSeen: start + 1000, OkValue: true},
		EventBus:          eventBus,
	}
	blamer := New(storages)
	tilePair, err := storages.GetLastTileTrimmed()
	assert.NoError(t, err)
	err = blamer.Calculate(tilePair.Tile)
	assert.NoError(t, err)

	storages.EventBus.SubscribeAsync(expstorage.EV_EXPSTORAGE_CHANGED, func(e interface{}) {
		if err := blamer.Calculate(tilePair.Tile); err != nil {
			assert.Fail(t, "Async calculate failed")
		}
	})

	// Check when completely untriaged
	blameLists, _ := blamer.GetAllBlameLists()
	assert.NotNil(t, blameLists)

	assert.Equal(t, 3, len(blameLists))
	assert.Equal(t, 3, len(blameLists["foo"]))
	assert.Equal(t, []int{1, 0, 0, 0}, blameLists["foo"][DI_1].Freq)
	assert.Equal(t, []int{1, 0}, blameLists["foo"][DI_2].Freq)
	assert.Equal(t, []int{1, 0, 0, 0, 0}, blameLists["foo"][DI_3].Freq)

	assert.Equal(t, 3, len(blameLists["bar"]))
	assert.Equal(t, []int{2, 0, 0, 0}, blameLists["bar"][DI_4].Freq)
	assert.Equal(t, []int{1, 0, 0, 0, 0}, blameLists["bar"][DI_5].Freq)
	assert.Equal(t, []int{1, 0, 0, 0, 0}, blameLists["bar"][DI_6].Freq)

	assert.Equal(t, &BlameDistribution{Freq: []int{1}}, blamer.GetBlame("foo", DI_1, commits))
	assert.Equal(t, &BlameDistribution{Freq: []int{3}}, blamer.GetBlame("foo", DI_2, commits))
	assert.Equal(t, &BlameDistribution{Freq: []int{0}}, blamer.GetBlame("foo", DI_3, commits))
	assert.Equal(t, &BlameDistribution{Freq: []int{1}}, blamer.GetBlame("bar", DI_4, commits))
	assert.Equal(t, &BlameDistribution{Freq: []int{0}}, blamer.GetBlame("bar", DI_5, commits))
	assert.Equal(t, &BlameDistribution{Freq: []int{0}}, blamer.GetBlame("bar", DI_6, commits))

	// Classify some digests and re-calculate.
	changes := types.TestExp{
		"foo": map[string]types.Label{DI_1: types.POSITIVE, DI_2: types.NEGATIVE},
		"bar": map[string]types.Label{DI_4: types.POSITIVE, DI_6: types.NEGATIVE},
	}
	assert.NoError(t, storages.ExpectationsStore.AddChange(changes, ""))

	// Wait for the change to propagate.
	waitForChange(blamer, blameLists)
	blameLists, _ = blamer.GetAllBlameLists()

	assert.Equal(t, 3, len(blameLists))
	assert.Equal(t, 1, len(blameLists["foo"]))
	assert.Equal(t, []int{1, 0, 0, 0, 0}, blameLists["foo"][DI_3].Freq)

	assert.Equal(t, 1, len(blameLists["bar"]))
	assert.Equal(t, []int{1, 0, 0, 0, 0}, blameLists["bar"][DI_5].Freq)
	assert.Equal(t, []int{1, 2, 0}, blameLists["baz"][DI_8].Freq)

	assert.Equal(t, &BlameDistribution{Freq: []int{0}}, blamer.GetBlame("foo", DI_3, commits))
	assert.Equal(t, &BlameDistribution{Freq: []int{0}}, blamer.GetBlame("bar", DI_5, commits))
	assert.Equal(t, &BlameDistribution{Freq: []int{3}}, blamer.GetBlame("baz", DI_8, commits))

	// Change the underlying tile and trigger with another change.
	tile := storages.MasterTileBuilder.GetTile()

	// Get the trace for the last parameters and set a value.
	gTrace := tile.Traces[mocks.TraceKey(params[5])].(*types.GoldenTrace)
	gTrace.Values[2] = DI_9

	assert.NoError(t, storages.ExpectationsStore.AddChange(changes, ""))

	// Wait for the change to propagate.
	waitForChange(blamer, blameLists)
	blameLists, _ = blamer.GetAllBlameLists()

	assert.Equal(t, 3, len(blameLists))
	assert.Equal(t, 1, len(blameLists["foo"]))
	assert.Equal(t, 2, len(blameLists["bar"]))
	assert.Equal(t, []int{1, 0, 0}, blameLists["bar"][DI_9].Freq)

	assert.Equal(t, &BlameDistribution{Freq: []int{2}}, blamer.GetBlame("bar", DI_9, commits))

	// Simulate the case where the digest is not found in digest store.
	storages.DigestStore.(*mocks.MockDigestStore).OkValue = false
	assert.NoError(t, storages.ExpectationsStore.AddChange(changes, ""))
	time.Sleep(10 * time.Millisecond)
	blameLists, _ = blamer.GetAllBlameLists()
	assert.Equal(t, 3, len(blameLists))
	assert.Equal(t, 1, len(blameLists["foo"]))
	assert.Equal(t, 2, len(blameLists["bar"]))
	assert.Equal(t, []int{1, 0, 0}, blameLists["bar"][DI_9].Freq)

	assert.Equal(t, &BlameDistribution{Freq: []int{2}}, blamer.GetBlame("bar", DI_9, commits))
	assert.Equal(t, &BlameDistribution{Freq: []int{1}}, blamer.GetBlame("bar", DI_9, commits[1:4]))
	assert.Equal(t, &BlameDistribution{Freq: []int{}}, blamer.GetBlame("bar", DI_9, commits[0:2]))
}

func BenchmarkBlamer(b *testing.B) {
	ctx := context.Background()
	tileBuilder := mocks.GetTileBuilderFromEnv(b, ctx)

	// Get a tile to make sure it's cached.
	tileBuilder.GetTile()
	b.ResetTimer()
	testBlamerWithLiveData(b, tileBuilder)
}

func TestBlamerWithLiveData(t *testing.T) {
	testutils.LargeTest(t)

	err := gcs.DownloadTestDataFile(t, gcs.TEST_DATA_BUCKET, TEST_DATA_STORAGE_PATH, TEST_DATA_PATH)
	assert.NoError(t, err, "Unable to download testdata.")
	defer testutils.RemoveAll(t, TEST_DATA_DIR)

	tileStore := mocks.NewMockTileBuilderFromJson(t, TEST_DATA_PATH)
	testBlamerWithLiveData(t, tileStore)
}

func testBlamerWithLiveData(t assert.TestingT, tileBuilder tracedb.MasterTileBuilder) {
	eventBus := eventbus.New()
	storages := &storage.Storage{
		ExpectationsStore: expstorage.NewMemExpectationsStore(eventBus),
		MasterTileBuilder: tileBuilder,
		DigestStore: &mocks.MockDigestStore{
			FirstSeen: time.Now().Unix(),
			OkValue:   true,
		},
		EventBus: eventBus,
	}

	blamer := New(storages)
	tilePair, err := storages.GetLastTileTrimmed()
	assert.NoError(t, err)
	err = blamer.Calculate(tilePair.Tile)
	assert.NoError(t, err)

	storages.EventBus.SubscribeAsync(expstorage.EV_EXPSTORAGE_CHANGED, func(e interface{}) {
		if err := blamer.Calculate(tilePair.Tile); err != nil {
			assert.Fail(t, "Async calculate failed")
		}
	})

	// Wait until we have a blamelist.
	var blameLists map[string]map[string]*BlameDistribution
	for {
		blameLists, _ = blamer.GetAllBlameLists()
		if blameLists != nil {
			break
		}
	}

	tile := storages.MasterTileBuilder.GetTile()

	// Since we set the 'First' timestamp of all digest info entries
	// to Now. We should get a non-empty blamelist of all digests.
	oneTestName := ""
	oneDigest := ""
	forEachTestDigestDo(tile, func(testName, digest string) {
		assert.NotNil(t, blameLists[testName][digest])
		assert.True(t, len(blameLists[testName][digest].Freq) > 0)

		// Remember the last one for later.
		oneTestName, oneDigest = testName, digest
	})

	// Change the classification of one test and trigger the recalculation.
	changes := types.TestExp{
		oneTestName: map[string]types.Label{oneDigest: types.POSITIVE},
	}
	assert.NoError(t, storages.ExpectationsStore.AddChange(changes, ""))

	// Wait for change to propagate.
	waitForChange(blamer, blameLists)
	blameLists, _ = blamer.GetAllBlameLists()

	// Assert the correctness of the blamelists.
	forEachTestDigestDo(tile, func(testName, digest string) {
		if (testName == oneTestName) && (digest == oneDigest) {
			assert.Nil(t, blameLists[testName][digest])
		} else {
			assert.NotNil(t, blameLists[testName][digest])
			assert.True(t, len(blameLists[testName][digest].Freq) > 0)
		}
	})

	// Set 'First' for all digests in the past and trigger another
	// calculation.
	storages.DigestStore.(*mocks.MockDigestStore).FirstSeen = 0
	assert.NoError(t, storages.ExpectationsStore.AddChange(changes, ""))
	waitForChange(blamer, blameLists)
	blameLists, _ = blamer.GetAllBlameLists()

	// Randomly assign labels to the different digests and make sure
	// that the blamelists are correct.
	storages.DigestStore.(*mocks.MockDigestStore).FirstSeen = time.Now().Unix()

	changes = types.TestExp{}
	choices := []types.Label{types.POSITIVE, types.NEGATIVE, types.UNTRIAGED}
	forEachTestDigestDo(tile, func(testName, digest string) {
		targetTest := changes[testName]
		if targetTest == nil {
			targetTest = map[string]types.Label{}
		}
		// Randomly skip some digests.
		label := choices[rand.Int()%len(choices)]
		if label != types.UNTRIAGED {
			targetTest[digest] = label
		}
	})

	// Add the labels and wait for the recalculation.
	assert.NoError(t, storages.ExpectationsStore.AddChange(changes, ""))
	waitForChange(blamer, blameLists)
	blameLists, commits := blamer.GetAllBlameLists()

	expecations, err := storages.ExpectationsStore.Get()
	assert.NoError(t, err)

	// Verify that the results are plausible.
	forEachTestTraceDo(tile, func(testName string, values []string) {
		for idx, digest := range values {
			if digest != types.MISSING_DIGEST {
				label := expecations.Classification(testName, digest)
				if label == types.UNTRIAGED {
					bl := blameLists[testName][digest]
					assert.NotNil(t, bl)
					freq := bl.Freq
					assert.True(t, len(freq) > 0)
					startIdx := len(commits) - len(freq)
					assert.True(t, (startIdx >= 0) && (startIdx <= idx), fmt.Sprintf("Expected (%s): Smaller than %d but got %d.", digest, startIdx, idx))
				}
			}
		}
	})
}

func waitForChange(blamer *Blamer, oldBlameLists map[string]map[string]*BlameDistribution) {
	for {
		time.Sleep(500 * time.Millisecond)
		blameLists, _ := blamer.GetAllBlameLists()
		if !reflect.DeepEqual(blameLists, oldBlameLists) {
			return
		}
	}
}

func forEachTestDigestDo(tile *tiling.Tile, fn func(string, string)) {
	for _, trace := range tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		testName := gTrace.Params()[types.PRIMARY_KEY_FIELD]
		for _, digest := range gTrace.Values {
			if digest != types.MISSING_DIGEST {
				fn(testName, digest)
			}
		}
	}
}

func forEachTestTraceDo(tile *tiling.Tile, fn func(string, []string)) {
	tileLen := tile.LastCommitIndex() + 1
	for _, trace := range tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		testName := gTrace.Params()[types.PRIMARY_KEY_FIELD]
		fn(testName, gTrace.Values[:tileLen])
	}
}
