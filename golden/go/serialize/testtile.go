package serialize

import (
	"os"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gs"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/storage"
)

// LoadSampledTile loads a complete sampled storage configuration from from disk.
// This includes expectations and ignores.
func LoadSampledStorage(t assert.TestingT, bucket, storagePath, outputPath string) *storage.Storage {
	err := gs.DownloadTestDataFile(t, bucket, storagePath, outputPath)
	assert.NoError(t, err, "Unable to download testdata.")
	defer testutils.RemoveAll(t, outputPath)

	file, err := os.Open(outputPath)
	assert.NoError(t, err)

	sample, err := DeserializeSample(file)
	assert.NoError(t, err)

	tileBuilder := mocks.NewMockTileBuilderFromTile(t, sample.Tile)
	eventBus := eventbus.New(nil)
	expStore := expstorage.NewMemExpectationsStore(eventBus)
	err = expStore.AddChange(sample.Expectations.Tests, "testuser")
	assert.NoError(t, err)

	return &storage.Storage{
		ExpectationsStore: expStore,
		MasterTileBuilder: tileBuilder,
		DigestStore: &mocks.MockDigestStore{
			FirstSeen: time.Now().Unix(),
			OkValue:   true,
		},
		DiffStore: mocks.NewMockDiffStore(),
		EventBus:  eventBus,
	}
}
