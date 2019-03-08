package storage

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/tiling"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/serialize"
	"go.skia.org/infra/golden/go/types"
)

const (
	// TEST_HASHES_GS_PATH is the bucket/path combination where the test file will be written.
	TEST_HASHES_GS_PATH = "skia-infra-testdata/hash_files/testing-known-hashes.txt"

	// TEST_BASELINE_GS_PATH is the root path of all baseline file in GCS.
	TEST_BASELINE_GS_PATH = "skia-infra-testdata/hash_files/testing-baselines"

	// Directory with testdata.
	TEST_DATA_DIR = "./testdata"

	// Local file location of the test data.
	TEST_DATA_PATH = TEST_DATA_DIR + "/10-test-sample-4bytes.tile"

	// Folder in the testdata bucket. See go/testutils for details.
	TEST_DATA_STORAGE_PATH = "gold-testdata/10-test-sample-4bytes.tile"
)

var (
	issueID = int64(5678)

	startCommit = &tiling.Commit{
		CommitTime: time.Now().Add(-time.Hour * 10).Unix(),
		Hash:       "abb84b151a49eca5a6e107c51a1f1b7da73454bf",
		Author:     "Jon Doe",
	}
	endCommit = &tiling.Commit{
		CommitTime: time.Now().Unix(),
		Hash:       "51465f0ed60ce2cacff3653c7d1d70317679fc06",
		Author:     "Jane Doe",
	}

	masterBaseline = &baseline.CommitableBaseLine{
		StartCommit: startCommit,
		EndCommit:   endCommit,
		Baseline: types.TestExp{
			"test-1": map[string]types.Label{"d1": types.POSITIVE},
		},
		Issue: 0,
	}

	issueBaseline = &baseline.CommitableBaseLine{
		StartCommit: endCommit,
		EndCommit:   endCommit,
		Baseline: types.TestExp{
			"test-3": map[string]types.Label{"d2": types.POSITIVE},
		},
		Issue: issueID,
	}
)

func TestWritingHashes(t *testing.T) {
	testutils.LargeTest(t)
	gsClient, opt := initGSClient(t)

	knownDigests := []string{
		"c003788f8d306ff1226e2a460835dae4",
		"885b31941c25efc313b0fd66d55b86d9",
		"264d0d87b12ba337f796fc592cd5357d",
		"69c2fbf8e89a48058b2f45ad4ea46a35",
		"2c4d605c16e7d5b23294c0433fa3ed17",
		"782717cf6ed9329fc43cb5a6c830cbce",
		"e143ca619f2172d06bb0dcc4d72af414",
		"26aff0619c829bc149f7c0171fcca442",
		"72d61ae8e232c3a279cc3cdbf6ef73e5",
		"f1eb049dac1cfa3c70aac8fc6ad5496f",
	}
	assert.NoError(t, gsClient.WriteKnownDigests(knownDigests))
	removePaths := []string{opt.HashesGSPath}
	defer func() {
		for _, path := range removePaths {
			_ = gsClient.removeGSPath(path)
		}
	}()

	found, err := gsClient.loadKnownDigests()
	assert.NoError(t, err)
	assert.Equal(t, knownDigests, found)
}

func TestWritingBaselines(t *testing.T) {
	testutils.LargeTest(t)

	gsClient, _ := initGSClient(t)
	removePaths := []string{}
	defer func() {
		for _, path := range removePaths {
			_ = gsClient.removeGSPath(path)
		}
	}()

	path, err := gsClient.WriteBaseLine(masterBaseline)
	assert.NoError(t, err)
	removePaths = append(removePaths, strings.TrimPrefix(path, "gs://"))

	foundBaseline, err := gsClient.ReadBaseline(endCommit.Hash, 0)
	assert.NoError(t, err)
	assert.Equal(t, masterBaseline, foundBaseline)

	// Add a baseline for an issue
	path, err = gsClient.WriteBaseLine(issueBaseline)
	assert.NoError(t, err)
	removePaths = append(removePaths, strings.TrimPrefix(path, "gs://"))

	foundBaseline, err = gsClient.ReadBaseline("", issueID)
	assert.NoError(t, err)
	assert.Equal(t, issueBaseline, foundBaseline)
	baseLiner, err := NewBaseliner(gsClient, nil, nil, nil, nil)
	assert.NoError(t, err)

	// Fetch the combined baselines
	storages := &Storage{
		GStorageClient: gsClient,
		Baseliner:      baseLiner,
	}
	combined := &baseline.CommitableBaseLine{}
	*combined = *masterBaseline
	combined.Baseline = masterBaseline.Baseline.DeepCopy()
	combined.Baseline.Update(issueBaseline.Baseline)

	foundBaseline, err = storages.Baseliner.FetchBaseline(endCommit.Hash, issueID, 0)
	assert.NoError(t, err)
	assert.Equal(t, combined, foundBaseline)
}

func TestBaselineRobustness(t *testing.T) {
	testutils.LargeTest(t)

	gsClient, _ := initGSClient(t)

	removePaths := []string{}
	defer func() {
		for _, path := range removePaths {
			_ = gsClient.removeGSPath(path)
		}
	}()

	// Read the master baseline that has not been written
	foundBaseline, err := gsClient.ReadBaseline("", 5344)
	assert.NoError(t, err)
	assert.Nil(t, foundBaseline)

	// Test reading a non-existing baseline for an issue
	foundBaseline, err = gsClient.ReadBaseline("", 5344)
	assert.NoError(t, err)
	assert.Nil(t, foundBaseline)

	path, err := gsClient.WriteBaseLine(masterBaseline)
	assert.NoError(t, err)
	removePaths = append(removePaths, strings.TrimPrefix(path, "gs://"))

	baseLiner, err := NewBaseliner(gsClient, nil, nil, nil, nil)
	assert.NoError(t, err)

	// Fetch the combined baselines when there are no baselines for the issue
	storages := &Storage{
		GStorageClient: gsClient,
		Baseliner:      baseLiner,
	}
	foundBaseline, err = storages.Baseliner.FetchBaseline(endCommit.Hash, 5344, 0)
	assert.NoError(t, err)
	assert.Equal(t, masterBaseline, foundBaseline)
}

func initGSClient(t *testing.T) (*GStorageClient, *GSClientOptions) {
	timeStamp := fmt.Sprintf("%032d", time.Now().UnixNano())
	opt := &GSClientOptions{
		HashesGSPath:   TEST_HASHES_GS_PATH + "-" + timeStamp,
		BaselineGSPath: TEST_BASELINE_GS_PATH + "-" + timeStamp,
	}
	gsClient, err := NewGStorageClient(nil, opt)
	assert.NoError(t, err)
	return gsClient, opt
}

func TestCondenseTile(t *testing.T) {
	testutils.LargeTest(t)

	bucket, storagePath, outputPath := gcs.TEST_DATA_BUCKET, TEST_DATA_STORAGE_PATH, TEST_DATA_PATH

	err := gcs.DownloadTestDataFile(t, bucket, storagePath, outputPath)
	assert.NoError(t, err, "Unable to download testdata.")
	sample := loadSample(t, outputPath)
	testTile := sample.Tile

	testCondenseForSize(t, testTile, len(testTile.Commits), len(testTile.Commits)/2)
	testCondenseForSize(t, testTile, len(testTile.Commits), int(float64(len(testTile.Commits))*0.75))
	testCondenseForSize(t, testTile, len(testTile.Commits), len(testTile.Commits)/5)
	testCondenseForSize(t, testTile, len(testTile.Commits), 0)
}

func testCondenseForSize(t *testing.T, testTile *tiling.Tile, nCommits, nEmpty int) {
	sparseTile := testTile.Copy()

	// Make sure half the commits are empty. Some commits might empty from the get go.
	empty := map[int]bool{0: true, 1: true}
	for idx := range sparseTile.Commits {
		found := false
		for _, trace := range sparseTile.Traces {
			if (trace.(*types.GoldenTrace)).Values[idx] != types.MISSING_DIGEST {
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
			gTrace.Values[idx] = types.MISSING_DIGEST
		}
	}

	storages := Storage{
		TraceDB:  &mockTDB{tile: sparseTile.Copy()},
		Git:      newMockVCS(sparseTile.Commits),
		NCommits: len(sparseTile.Commits) * 2,
	}

	denseTile, commitSum, err := storages.getNewCondensedTile(nil)
	assert.NoError(t, err)

	// verify the right commits were selected
	assert.Equal(t, len(empty), len(sparseTile.Commits)-len(denseTile.Commits))
	denseIdx := 0
	for idx, commit := range sparseTile.Commits {
		sklog.Infof("Sparse: %s - %s", commit.Hash, commit.Author)

		if !empty[idx] {
			assert.Equal(t, commit, denseTile.Commits[denseIdx])
			denseIdx++
		}
	}

	// Remove all leading empty commits
	startIdx := 0
	for empty[startIdx] {
		startIdx++
	}
	expAllCommits := sparseTile.Commits[startIdx:]
	sklog.Infof("Length: %d  %d", len(expAllCommits), len(commitSum.AllCommits()))
	assert.Equal(t, expAllCommits, commitSum.AllCommits())
}

type mockTDB struct {
	tile *tiling.Tile
}

func (m *mockTDB) List(begin, end time.Time) ([]*tracedb.CommitID, error)                 { return nil, nil }
func (m *mockTDB) Close() error                                                           { return nil }
func (m *mockTDB) Add(commitID *tracedb.CommitID, values map[string]*tracedb.Entry) error { return nil }

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
		for k, v := range gTrace.Params_ {
			newTrace.Params_[k] = v
		}
		paramSet.AddParams(gTrace.Params_)

		// Set the values in the order of the commit IDs.
		for idx, commitID := range commitIDs {
			srcIdx := comMap[commitID.ID]
			newTrace.Values[idx] = gTrace.Values[srcIdx]
		}
		ret.Traces[traceID] = newTrace
	}

	// The second return value is not used in testing so we don't calculate it.
	return ret, nil, nil
}

// mockVCS is a mock vcsinfo.VCS that implements just LastNIndex.
type mockVCS struct {
	ret []*vcsinfo.IndexCommit
}

func newMockVCS(commits []*tiling.Commit) vcsinfo.VCS {
	ret := make([]*vcsinfo.IndexCommit, len(commits))
	startIdx := 20
	startTime := time.Now().Add(-time.Hour * 24 * 7).UTC()
	for idx, commit := range commits {
		ret[idx] = &vcsinfo.IndexCommit{
			Index:     startIdx + idx,
			Hash:      commit.Hash,
			Timestamp: startTime.Add(2 * time.Second * time.Duration(idx+1)),
		}
	}
	return &mockVCS{
		ret: ret,
	}
}

func (m *mockVCS) LastNIndex(N int) []*vcsinfo.IndexCommit {
	if N > len(m.ret)-1 {
		return m.ret
	}
	return m.ret[len(m.ret)-N:]
}

func (m *mockVCS) Range(begin time.Time, end time.Time) []*vcsinfo.IndexCommit     { return nil }
func (m *mockVCS) ByIndex(ctx context.Context, N int) (*vcsinfo.LongCommit, error) { return nil, nil }
func (m *mockVCS) Update(ctx context.Context, pull bool, allBranches bool) error   { return nil }
func (m *mockVCS) From(start time.Time) []string                                   { return nil }
func (m *mockVCS) Details(ctx context.Context, hash string, includeBranchInfo bool) (*vcsinfo.LongCommit, error) {
	return nil, nil
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
