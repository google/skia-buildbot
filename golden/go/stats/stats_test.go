package stats

import (
	"testing"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/gs"
	"go.skia.org/infra/golden/go/serialize"
)

const (
	// Local file location of the test data.
	TEST_DATA_PATH = "./10-test-sample.tile"

	// Folder in the testdata bucket. See go/testutils for details.
	TEST_DATA_STORAGE_PATH = "gold-testdata/10-test-sample.tile"
)

func TestTileStats(t *testing.T) {
	storages := serialize.LoadSampledStorage(t, gs.TEST_DATA_BUCKET, TEST_DATA_STORAGE_PATH, TEST_DATA_PATH)
	tile := storages.MasterTileBuilder.GetTile()

	stat := NewStatistician()
	assert.NoError(t, stat.CalculateTileStats(tile))
	tileStats := stat.GetTileStats()

	// Some basic assertions to make sure we get results.
	commits := tile.Commits
	assert.True(t, len(commits) > 0)
	assert.True(t, tileStats.Digests > 0)
	assert.Equal(t, len(commits), len(tileStats.DigestsPerCommit))
	assert.True(t, tileStats.TotalCells > 0)
	assert.True(t, tileStats.UniqueDigests > 0)
	assert.Equal(t, len(commits), len(tileStats.UniqueDigestsPerCommit))
}
