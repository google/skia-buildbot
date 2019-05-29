package mocks

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strings"

	"cloud.google.com/go/storage"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/golden/go/digeststore"
	"go.skia.org/infra/golden/go/types"
)

// TODO(kjlubick): replace these mocks with mockery mocks so as to be consistent

// Mock the url generator function.
func MockUrlGenerator(path string) string {
	return path
}

// traceKey returns the trace key used in MockTileStore generated from the
// params map.
// TODO(kjlubick): replace with version from tracestore when that lands
func traceKey(params map[string]string) tiling.TraceId {
	traceParts := make([]string, 0, len(params))
	for _, v := range params {
		traceParts = append(traceParts, v)
	}
	sort.Strings(traceParts)
	return tiling.TraceId(strings.Join(traceParts, ":"))
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
func GetTileBuilderFromEnv(t assert.TestingT, ctx context.Context) tracedb.MasterTileBuilder {
	traceDBAddress := os.Getenv("TEST_TRACEDB_ADDRESS")
	assert.NotEqual(t, "", traceDBAddress, "Please define the TEST_TRACEDB_ADDRESS environment variable to point to the traceDB.")

	gitURL := os.Getenv("TEST_GIT_URL")
	assert.NotEqual(t, "", traceDBAddress, "Please define the TEST_TRACEDB_ADDRESS environment variable to point to the Git URL.")

	gitRepoDir, err := ioutil.TempDir("", "gitrepo")
	assert.NoError(t, err)

	git, err := gitinfo.CloneOrUpdate(ctx, gitURL, gitRepoDir, false)
	if err != nil {
		sklog.Fatal(err)
	}

	eventBus := eventbus.New()
	db, err := tracedb.NewTraceServiceDBFromAddress(traceDBAddress, types.GoldenTraceBuilder)
	assert.NoError(t, err)

	tileBuilder, err := tracedb.NewMasterTileBuilder(ctx, db, git, 50, eventBus, "")
	assert.NoError(t, err)
	return tileBuilder
}

// Mock the tilestore for GoldenTraces
func NewMockTileBuilder(t assert.TestingT, digests []types.DigestSlice, params []map[string]string, commits []*tiling.Commit) tracedb.MasterTileBuilder {
	// Build the tile from the digests, params and commits.
	traces := map[tiling.TraceId]tiling.Trace{}

	for idx, traceDigests := range digests {
		traces[traceKey(params[idx])] = &types.GoldenTrace{
			Keys:    params[idx],
			Digests: traceDigests,
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

// GetHTTPClient returns a http client either from locally loading a config file
// or by querying meta data in the cloud.
func GetHTTPClient(t assert.TestingT) *http.Client {
	// Get the service account client from meta data or a local config file.
	ts, err := auth.NewJWTServiceAccountTokenSource("", auth.DEFAULT_JWT_FILENAME, storage.ScopeFullControl)
	if err != nil {
		fmt.Println("If you are running this test locally, be sure you have a service-account.json in the test folder.")
	}
	assert.NoError(t, err)
	return httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
}
