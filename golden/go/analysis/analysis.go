package analysis

import "go.skia.org/infra/go/tiling"

type TileStats struct {
	Commits []*tiling.Commit
	Uniques []int
	Total   []int
	New     []int
}

// Analysis contains a complete description of one test.
type Analysis struct{}

type Analyzer interface {
	AddTile(tile *tiling.Tile) error
	TileStats(tile *tiling.Tile) (*TileStats, error)
	NewDigestsByCommit(commitHash string) ([]string, error)
	TileForTest(testName string) error
	AnalyzeTest(testName string) (*Analysis, error)
}
