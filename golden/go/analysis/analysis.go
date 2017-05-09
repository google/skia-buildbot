package analysis

import "go.skia.org/infra/go/tiling"

// All the stats about the current tile.
type TileStats struct {
	Commits []*tiling.Commit
	Uniques []int
	Total   []int
	New     []int
}

// Analysis contains a complete description of one test.
type Analysis struct{}

type Analyzer interface {
	// Adds a new tile to the underlying datastore.
	AddTile(tile *tiling.Tile) error

	// Get the Stats about the current tile.
	TileStats(tile *tiling.Tile) (*TileStats, error)

	// Look up digests by commit it.
	NewDigestsByCommit(commitHash string) ([]string, error)

	// TileForTest returns a tile for a given test that contains all
	// traces and digests that were created for the given digest.
	TileForTest(testName string) (*tiling.Tile, error)

	// AnalyzeTest returns a complete analysis for a specific test.
	AnalyzeTest(testName string) (*Analysis, error)
}

type analyzerImpl struct {
	*analyzerStore
}

func New(baseDir string) (Analyzer, error) {
	store, err := newAnalyzerStore(baseDir)
	if err != nil {
		return nil, err
	}
	return &analyzerImpl{
		analyzerStore: store,
	}, nil
}

func (a *analyzerImpl) TileStats(tile *tiling.Tile) (*TileStats, error) {
	return nil, nil
}

func (a *analyzerImpl) NewDigestsByCommit(commitHash string) ([]string, error) {
	return nil, nil
}

func (a *analyzerImpl) TileForTest(testName string) (*tiling.Tile, error) {
	return nil, nil
}

func (a *analyzerImpl) AnalyzeTest(testName string) (*Analysis, error) {
	return nil, nil
}
