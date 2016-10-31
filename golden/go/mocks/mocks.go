package mocks

import (
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/skia-dev/glog"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/tiling"
	tracedb "go.skia.org/infra/go/trace/db"
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

func (m MockDiffStore) Get(priority int64, dMain string, dRest []string) (map[string]*diff.DiffMetrics, error) {
	result := map[string]*diff.DiffMetrics{}
	for _, d := range dRest {
		if dMain != d {
			result[d] = &diff.DiffMetrics{
				NumDiffPixels:    10,
				PixelDiffPercent: 1.0,
				MaxRGBADiffs:     []int{5, 3, 4, 0},
				DimDiffer:        false,
				Diffs: map[string]float32{
					diff.METRIC_COMBINED: rand.Float32(),
					diff.METRIC_PERCENT:  rand.Float32(),
				},
			}
		}
	}
	return result, nil
}

func (m MockDiffStore) UnavailableDigests() map[string]*diff.DigestFailure                    { return nil }
func (m MockDiffStore) PurgeDigests(digests []string, purgeGS bool) error                     { return nil }
func (m MockDiffStore) ImageHandler(urlPrefix string) (http.Handler, error)                   { return nil, nil }
func (m MockDiffStore) WarmDigests(priority int64, digests []string)                          {}
func (m MockDiffStore) WarmDiffs(priority int64, leftDigests []string, rightDigests []string) {}

func NewMockDiffStore() diff.DiffStore {
	return MockDiffStore{}
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

type MockTileBuilder struct {
	t    assert.TestingT
	tile *tiling.Tile
}

func (m *MockTileBuilder) GetTile() *tiling.Tile {
	return m.tile
}

func (m *MockTileBuilder) ListLong(start, end time.Time, source string) ([]*tracedb.CommitIDLong, error) {
	return nil, nil
}

func (m *MockTileBuilder) TileFromCommits(commitIDs []*tracedb.CommitIDLong) (*tiling.Tile, []string, error) {
	return m.tile, nil, nil
}

func (m *MockTileBuilder) ListMD5(commitIDs []*tracedb.CommitID) ([]string, error) {
	return nil, nil
}

// Mock the tilestore for GoldenTraces
func NewMockTileBuilderFromTile(t assert.TestingT, tile *tiling.Tile) tracedb.MasterTileBuilder {
	return &MockTileBuilder{
		t:    t,
		tile: tile,
	}
}

// GetTileBuilderFromEnv looks at the TEST_TRACEDB_ADDRESS environement variable for the
// name of directory that contains tiles. If it's defined it will return a
// TileStore instance. If the not the calling test will fail.
func GetTileBuilderFromEnv(t assert.TestingT) tracedb.MasterTileBuilder {
	traceDBAddress := os.Getenv("TEST_TRACEDB_ADDRESS")
	assert.NotEqual(t, "", traceDBAddress, "Please define the TEST_TRACEDB_ADDRESS environment variable to point to the traceDB.")

	gitURL := os.Getenv("TEST_GIT_URL")
	assert.NotEqual(t, "", traceDBAddress, "Please define the TEST_TRACEDB_ADDRESS environment variable to point to the Git URL.")

	gitRepoDir, err := ioutil.TempDir("", "gitrepo")
	assert.NoError(t, err)

	git, err := gitinfo.CloneOrUpdate(gitURL, gitRepoDir, false)
	if err != nil {
		glog.Fatal(err)
	}

	eventBus := eventbus.New(nil)
	db, err := tracedb.NewTraceServiceDBFromAddress(traceDBAddress, types.GoldenTraceBuilder)
	assert.NoError(t, err)

	tileBuilder, err := tracedb.NewMasterTileBuilder(db, git, 50, eventBus)
	assert.NoError(t, err)
	return tileBuilder
}

// Mock the tilestore for GoldenTraces
func NewMockTileBuilder(t assert.TestingT, digests [][]string, params []map[string]string, commits []*tiling.Commit) tracedb.MasterTileBuilder {
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

	return &MockTileBuilder{
		t:    t,
		tile: tile,
	}
}

// NewMockTileStoreFromJson reads a tile that has been serialized to JSON
// and wraps an instance of MockTileStore around it.
func NewMockTileBuilderFromJson(t assert.TestingT, fname string) tracedb.MasterTileBuilder {
	f, err := os.Open(fname)
	assert.NoError(t, err)

	tile, err := types.TileFromJson(f, &types.GoldenTrace{})
	assert.NoError(t, err)

	return &MockTileBuilder{
		t:    t,
		tile: tile,
	}
}
