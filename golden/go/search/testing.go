package search

import (
	"os"
	"time"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/serialize"
	"go.skia.org/infra/golden/go/storage"
)

func GetStoragesAndIndexerFromTile(t assert.TestingT, path string) (*storage.Storage, *expstorage.Expectations, *indexer.Indexer) {
	loadTimer := timer.New("Loading sample tile")
	sampledState := LoadSample(t, path)
	tileBuilder := mocks.NewMockTileBuilderFromTile(t, sampledState.Tile)
	eventBus := eventbus.New()
	expStore := expstorage.NewMemExpectationsStore(eventBus)
	loadTimer.Stop()

	err := expStore.AddChange(sampledState.Expectations.Tests, "testuser")
	assert.NoError(t, err)

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

	ixr, err := indexer.New(storages, 240*time.Minute)
	assert.NoError(t, err)

	return storages, sampledState.Expectations, ixr
}

func LoadSample(t assert.TestingT, fileName string) *serialize.Sample {
	file, err := os.Open(fileName)
	assert.NoError(t, err)

	sample, err := serialize.DeserializeSample(file)
	assert.NoError(t, err)

	return sample
}
