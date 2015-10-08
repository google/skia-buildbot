package mocks

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/filetilestore"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/digeststore"
	"go.skia.org/infra/golden/go/types"
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

func (m MockDiffStore) UnavailableDigests() map[string]*diff.DigestFailure       { return nil }
func (m MockDiffStore) PurgeDigests(digests []string, purgeGS bool)              {}
func (m MockDiffStore) SetDigestSets(namedDigestSets map[string]map[string]bool) {}

func NewMockDiffStore() diff.DiffStore {
	return MockDiffStore{}
}

// Mock the tilestore for GoldenTraces
func NewMockTileStore(t assert.TestingT, digests [][]string, params []map[string]string, commits []*tiling.Commit) tiling.TileStore {
	// Build the tile from the digests, params and commits.
	traces := map[string]tiling.Trace{}

	for idx, traceDigests := range digests {
		traces[TraceKey(params[idx])] = &types.GoldenTrace{
			Params_: params[idx],
			Values:  traceDigests,
		}
	}

	tile := tiling.NewTile()
	tile.Traces = traces
	tile.Commits = commits

	return &MockTileStore{
		t:    t,
		tile: tile,
	}
}

// TraceKey returns the trace key used in MockTileStore generated from the
// params map.
func TraceKey(params map[string]string) string {
	traceParts := make([]string, 0, len(params))
	for _, v := range params {
		traceParts = append(traceParts, v)
	}
	sort.Strings(traceParts)
	return strings.Join(traceParts, ":")
}

// NewMockTileStoreFromJson reads a tile that has been serialized to JSON
// and wraps an instance of MockTileStore around it.
func NewMockTileStoreFromJson(t assert.TestingT, fname string) tiling.TileStore {
	f, err := os.Open(fname)
	assert.Nil(t, err)

	tile, err := types.TileFromJson(f, &types.GoldenTrace{})
	assert.Nil(t, err)

	return &MockTileStore{
		t:    t,
		tile: tile,
	}
}

type MockTileStore struct {
	t    assert.TestingT
	tile *tiling.Tile
}

func (m *MockTileStore) Get(scale, index int) (*tiling.Tile, error) {
	return m.tile, nil
}

func (m *MockTileStore) Put(scale, index int, tile *tiling.Tile) error {
	assert.FailNow(m.t, "Should not be called.")
	return nil
}

func (m *MockTileStore) GetModifiable(scale, index int) (*tiling.Tile, error) {
	return m.Get(scale, index)
}

type MockDigestStore struct {
	IssueIDs  []int
	FirstSeen int64
	OkValue   bool
}

func (m *MockDigestStore) Get(testName, digest string) (*digeststore.DigestInfo, bool, error) {
	return &digeststore.DigestInfo{
		TestName: testName,
		Digest:   digest,
		First:    m.FirstSeen,
	}, m.OkValue, nil
}

func (m *MockDigestStore) Update([]*digeststore.DigestInfo) error {
	m.OkValue = true
	return nil
}

// GetTileStoreFromEnv looks at the TEST_TILE_DIR environement variable for the
// name of directory that contains tiles. If it's defined it will return a
// TileStore instance. If the not the calling test will fail.
func GetTileStoreFromEnv(t assert.TestingT) tiling.TileStore {
	// Get the TEST_TILE environment variable that points to the
	// tile to read.
	tileDir := os.Getenv("TEST_TILE_DIR")
	assert.NotEqual(t, "", tileDir, "Please define the TEST_TILE_DIR environment variable to point to a live tile store.")
	return filetilestore.NewFileTileStore(tileDir, config.DATASET_GOLD, 2*time.Minute)
}
