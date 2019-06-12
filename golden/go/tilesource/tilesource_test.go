package tilesource

import (
	"context"
	"math/rand"
	"os"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs/gcs_testutils"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/serialize"
	"go.skia.org/infra/golden/go/types"
)

const (
	// Directory with testdata.
	TEST_DATA_DIR = "./testdata"

	// Local file location of the test data.
	TEST_DATA_PATH = TEST_DATA_DIR + "/10-test-sample-4bytes.tile"

	// Folder in the testdata bucket. See go/testutils for details.
	TEST_DATA_STORAGE_PATH = "gold-testdata/10-test-sample-4bytes.tile"
)

func init() {
	rand.Seed(42)
}

// TODO(kjlubick): delete this when we have the BT-based tracestore, which will compute this.
func TestCondenseTile(t *testing.T) {
	unittest.LargeTest(t)

	bucket, storagePath, outputPath := gcs_testutils.TEST_DATA_BUCKET, TEST_DATA_STORAGE_PATH, TEST_DATA_PATH

	err := gcs_testutils.DownloadTestDataFile(t, bucket, storagePath, outputPath)
	assert.NoError(t, err, "Unable to download testdata.")
	sample := loadSample(t, outputPath)
	testTile := sample.Tile

	nCommitVals := []int{
		len(testTile.Commits),
		len(testTile.Commits) / 2,
		len(testTile.Commits) / 5,
		0,
	}
	nEmptyVals := []int{
		len(testTile.Commits) / 2,
		int(float64(len(testTile.Commits)) * 0.75),
		len(testTile.Commits) / 5,
		0,
	}

	for _, nCommits := range nCommitVals {
		for _, nEmpty := range nEmptyVals {
			testCondenseForSize(t, testTile, nCommits, nEmpty)
		}
	}
}

func testCondenseForSize(t *testing.T, testTile *tiling.Tile, nCommits, nEmpty int) {
	sparseTile := testTile.Copy()

	// Make sure half the commits are empty. Some commits might empty from the get go.
	empty := map[int]bool{0: true, 1: true}
	for idx := range sparseTile.Commits {
		found := false
		for _, trace := range sparseTile.Traces {
			if (trace.(*types.GoldenTrace)).Digests[idx] != types.MISSING_DIGEST {
				found = true
				break
			}
		}
		if !found {
			empty[idx] = true
		}
	}

	tileLen := len(sparseTile.Commits)
	for len(empty) < nEmpty {
		empty[rand.Int()%tileLen] = true
	}

	sklog.Infof("STATS: %d - %d - %d", len(empty), len(sparseTile.Commits), len(sparseTile.Traces))
	sklog.Infof("Empty: %v", empty)
	// empty = map[int]bool{}

	// Iterate over the tile and set the chosen commits to empty.
	for _, trace := range sparseTile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		for idx := range empty {
			gTrace.Digests[idx] = types.MISSING_DIGEST
		}
	}

	cti := CachedTileSourceImpl{
		CachedTileSourceConfig: CachedTileSourceConfig{
			TraceDB:  &mockTDB{tile: sparseTile.Copy()},
			VCS:      newMockVCS(sparseTile.Commits),
			NCommits: nCommits,
		},
	}

	ctx := context.Background()
	denseTile, sparseCommits, cardinalities, err := cti.getCondensedTile(ctx, nil)
	assert.NoError(t, err)

	// If we don't want any commits then we just make sure we got none.
	if nCommits <= 0 {
		assert.Equal(t, 0, len(denseTile.Commits))
		return
	}

	// verify the right commits were selected
	assert.Equal(t, sparseCommits[0], denseTile.Commits[0])
	denseIdx := 0
	sparseStart := len(sparseTile.Commits) - len(sparseCommits)
	for idx, commit := range sparseCommits {
		if !empty[sparseStart+idx] {
			assert.Equal(t, commit, denseTile.Commits[denseIdx])
			denseIdx++
		} else {
			assert.Equal(t, 0, cardinalities[idx])
		}
	}

	// Remove all leading empty commits and make sure the sparseCommits are correct.
	startIdx := 0
	for empty[startIdx] {
		startIdx++
	}
	expAllCommits := sparseTile.Commits[len(sparseTile.Commits)-len(sparseCommits):]
	sklog.Infof("Length: %d  %d", len(expAllCommits), len(sparseCommits))
	assert.Equal(t, expAllCommits, sparseCommits)
}

// mockTBD implements the TileFromCommits function of the tracedb.DB interface.
// TODO(kjlubick): Replace this with a mockery based implementation
type mockTDB struct {
	tile *tiling.Tile
}

func (m *mockTDB) List(begin, end time.Time) ([]*tracedb.CommitID, error) { return nil, nil }
func (m *mockTDB) Close() error                                           { return nil }
func (m *mockTDB) Add(commitID *tracedb.CommitID, values map[tiling.TraceId]*tracedb.Entry) error {
	return nil
}

func (m *mockTDB) TileFromCommits(commitIDs []*tracedb.CommitID) (*tiling.Tile, []string, error) {
	// comMap maps hashes from commitIDs to indices in the commits of the test tile.
	comMap := make(map[string]int, len(commitIDs))
	for _, commitID := range commitIDs {
		targetIdx := -1
		for idx, commit := range m.tile.Commits {
			if commit.Hash == commitID.ID {
				targetIdx = idx
				break
			}
		}
		// If a requested commit does not exist in the tile we fail.
		if targetIdx < 0 {
			return nil, nil, skerr.Fmt("Hash %s not found in test tile", commitID.ID)
		}
		comMap[commitID.ID] = targetIdx
	}

	ret := tiling.NewTile()

	// Add the commits to the new tile.
	ret.Commits = make([]*tiling.Commit, len(commitIDs))
	for idx, commitID := range commitIDs {
		ret.Commits[idx] = m.tile.Commits[comMap[commitID.ID]]
	}

	// Assemble the traces.
	paramSet := paramtools.ParamSet{}
	for traceID, trace := range m.tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		newTrace := types.NewGoldenTraceN(len(commitIDs))

		// Copy the params
		for k, v := range gTrace.Keys {
			newTrace.Keys[k] = v
		}
		paramSet.AddParams(gTrace.Keys)

		// Set the values in the order of the commit IDs.
		for idx, commitID := range commitIDs {
			srcIdx := comMap[commitID.ID]
			newTrace.Digests[idx] = gTrace.Digests[srcIdx]
		}
		ret.Traces[traceID] = newTrace
	}

	// The second return value is not used in testing so we don't calculate it.
	return ret, nil, nil
}

// mockVCS is a mock vcsinfo.VCS that implements just LastNIndex.
type mockVCS struct {
	idxCommits []*vcsinfo.IndexCommit
	commits    []*tiling.Commit
}

func newMockVCS(commits []*tiling.Commit) vcsinfo.VCS {
	ret := make([]*vcsinfo.IndexCommit, len(commits))
	startIdx := 20
	for idx, commit := range commits {
		ret[idx] = &vcsinfo.IndexCommit{
			Index:     startIdx + idx,
			Hash:      commit.Hash,
			Timestamp: time.Unix(commit.CommitTime, 0),
		}
	}
	return &mockVCS{
		idxCommits: ret,
		commits:    commits,
	}
}

func (m *mockVCS) LastNIndex(N int) []*vcsinfo.IndexCommit {
	if N > len(m.idxCommits)-1 {
		return m.idxCommits
	}
	return m.idxCommits[len(m.idxCommits)-N:]
}

func (m *mockVCS) Range(begin time.Time, end time.Time) []*vcsinfo.IndexCommit     { return nil }
func (m *mockVCS) ByIndex(ctx context.Context, N int) (*vcsinfo.LongCommit, error) { return nil, nil }
func (m *mockVCS) Update(ctx context.Context, pull bool, allBranches bool) error   { return nil }
func (m *mockVCS) From(start time.Time) []string                                   { return nil }
func (m *mockVCS) Details(ctx context.Context, hash string, includeBranchInfo bool) (*vcsinfo.LongCommit, error) {
	for _, commit := range m.commits {
		if commit.Hash == hash {
			return &vcsinfo.LongCommit{
				ShortCommit: &vcsinfo.ShortCommit{Hash: commit.Hash, Author: commit.Author},
				Timestamp:   time.Unix(commit.CommitTime, 0),
			}, nil
		}
	}
	return nil, nil
}
func (m *mockVCS) DetailsMulti(ctx context.Context, hashes []string, includeBranchInfo bool) ([]*vcsinfo.LongCommit, error) {
	ret := make([]*vcsinfo.LongCommit, len(hashes))
	var err error
	for idx, hash := range hashes {
		if ret[idx], err = m.Details(ctx, hash, includeBranchInfo); err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func (m *mockVCS) GetBranch() string {
	return "master"
}

func (m *mockVCS) IndexOf(ctx context.Context, hash string) (int, error) { return 0, nil }
func (m *mockVCS) GetFile(ctx context.Context, fileName string, commitHash string) (string, error) {
	return "", nil
}
func (m *mockVCS) ResolveCommit(ctx context.Context, commitHash string) (string, error) {
	return "", nil
}

func loadSample(t assert.TestingT, fileName string) *serialize.Sample {
	file, err := os.Open(fileName)
	assert.NoError(t, err)

	sample, err := serialize.DeserializeSample(file)
	assert.NoError(t, err)
	return sample
}
