package mocks

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/golden/go/diff"
	ptypes "go.skia.org/infra/perf/go/types"
)

// Mock the url generator function.
func MockUrlGenerator(path string) string {
	return path
}

// Mock the diffstore.
type MockDiffStore struct{}

func (m MockDiffStore) Get(dMain string, dRest []string) (map[string]*diff.DiffMetrics, error) {
	result := map[string]*diff.DiffMetrics{}
	for _, d := range dRest {
		result[d] = &diff.DiffMetrics{
			NumDiffPixels:     10,
			PixelDiffPercent:  1.0,
			PixelDiffFilePath: fmt.Sprintf("diffpath/%s-%s", dMain, d),
			MaxRGBADiffs:      []int{5, 3, 4, 0},
			DimDiffer:         false,
		}
	}
	return result, nil
}

func (m MockDiffStore) AbsPath(digest []string) map[string]string {
	result := map[string]string{}
	for _, d := range digest {
		result[d] = "abspath/" + d
	}
	return result
}

func (m MockDiffStore) ThumbAbsPath(digest []string) map[string]string {
	result := map[string]string{}
	for _, d := range digest {
		result[d] = "thumb/abspath/" + d
	}
	return result
}

func (m MockDiffStore) UnavailableDigests() map[string]bool {
	return nil
}

func (m MockDiffStore) CalculateDiffs([]string) {}

func NewMockDiffStore() diff.DiffStore {
	return MockDiffStore{}
}

// Mock the tilestore for GoldenTraces
func NewMockTileStore(t *testing.T, digests [][]string, params []map[string]string, commits []*ptypes.Commit) ptypes.TileStore {
	// Build the tile from the digests, params and commits.
	traces := map[string]ptypes.Trace{}

	for idx, traceDigests := range digests {
		traceParts := []string{}
		for _, v := range params[idx] {
			traceParts = append(traceParts, v)
		}
		sort.Strings(traceParts)

		traces[strings.Join(traceParts, ":")] = &ptypes.GoldenTrace{
			Params_: params[idx],
			Values:  traceDigests,
		}
	}

	tile := ptypes.NewTile()
	tile.Traces = traces
	tile.Commits = commits

	return &MockTileStore{
		t:    t,
		tile: tile,
	}
}

// NewMockTileStoreFromJson reads a tile that has been serialized to JSON
// and wraps an instance of MockTileStore around it.
func NewMockTileStoreFromJson(t *testing.T, fname string) ptypes.TileStore {
	f, err := os.Open(fname)
	assert.Nil(t, err)

	tile, err := ptypes.TileFromJson(f, &ptypes.GoldenTrace{})
	assert.Nil(t, err)

	return &MockTileStore{
		t:    t,
		tile: tile,
	}
}

type MockTileStore struct {
	t    *testing.T
	tile *ptypes.Tile
}

func (m *MockTileStore) Get(scale, index int) (*ptypes.Tile, error) {
	return m.tile, nil
}

func (m *MockTileStore) Put(scale, index int, tile *ptypes.Tile) error {
	assert.FailNow(m.t, "Should not be called.")
	return nil
}

func (m *MockTileStore) GetModifiable(scale, index int) (*ptypes.Tile, error) {
	return m.Get(scale, index)
}
