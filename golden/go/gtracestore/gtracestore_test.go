package gtracestore

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gcs/gcs_testutils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
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

func TestGTraceStore(t *testing.T) {
	testutils.LargeTest(t)

	if !fileutil.FileExists(TEST_DATA_PATH) {
		err := gcs_testutils.DownloadTestDataFile(t, gcs_testutils.TEST_DATA_BUCKET, TEST_DATA_STORAGE_PATH, TEST_DATA_PATH)
		assert.NoError(t, err, "Unable to download testdata.")
	}

	sample := loadSample(t, TEST_DATA_PATH)
	tile := sample.Tile

	btConf := &BTConfig{
		ProjectID:  "test-project",
		InstanceID: "testinstance",
		TableID:    "testtable",
		VCS:        newMockVCS(tile.Commits),
	}

	ctx := context.Background()
	assert.NoError(t, bt.DeleteTables(btConf.ProjectID, btConf.InstanceID, btConf.TableID))
	assert.NoError(t, InitBT(btConf))

	traceStore, err := NewBTTraceStore(ctx, btConf, false)
	assert.NoError(t, err)
	btts := traceStore.(*btTraceStore)

	// For each value in tile get the traceIDs that are not empty.
	tileLen := tile.LastCommitIndex() + 1
	traceIDsPerCommit := make([][]tiling.TraceId, tileLen)

	for traceID, trace := range tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		for i := 0; i < tileLen; i++ {
			if gTrace.Digests[i] != types.MISSING_DIGEST {
				traceIDsPerCommit[i] = append(traceIDsPerCommit[i], traceID)
			}
		}
	}
	rand.Seed(time.Now().UnixNano())
	for _, tids := range traceIDsPerCommit {
		rand.Shuffle(len(tids), func(i, j int) { tids[i], tids[j] = tids[j], tids[i] })
	}

	indices := make([]int, tileLen)
	maxIndex := 0
	maxLen := len(traceIDsPerCommit[0])
	for idx := range indices {
		if len(traceIDsPerCommit[idx]) > maxLen {
			maxLen = len(traceIDsPerCommit[idx])
			maxIndex = idx
		}
		indices[idx] = idx
	}

	// Ingest the biggest tile.
	entries := []*Entry{}
	allDigests := map[types.Digest]bool{"": true}
	for _, traceID := range traceIDsPerCommit[maxIndex] {
		t := tile.Traces[traceID].(*types.GoldenTrace)
		digest := t.Digests[maxIndex]
		allDigests[digest] = true
		entries = append(entries, &Entry{Value: digest, Params: t.Params()})
	}
	assert.NoError(t, traceStore.Put(ctx, tile.Commits[maxIndex].Hash, entries, time.Now()))

	maxTileKey, _, err := btts.getTileKey(ctx, tile.Commits[maxIndex].Hash)
	assert.NoError(t, err)

	foundDigestMap, err := traceStore.(*btTraceStore).getDigestMap(ctx, maxTileKey)
	assert.NoError(t, err)
	assert.Equal(t, len(allDigests), foundDigestMap.Len())

	for digest := range allDigests {
		id, err := foundDigestMap.ID(digest)
		assert.NoError(t, err)
		if digest == "" {
			assert.Equal(t, int32(0), id)
		} else {
			assert.NotEqual(t, int32(0), id)
		}
	}

	traceIDsPerCommit[maxIndex] = []tiling.TraceId{}
	rand.Shuffle(len(indices), func(i, j int) { indices[i], indices[j] = indices[j], indices[i] })

	// Randomly add samples from the tile to that
	for len(indices) > 0 {
		rand.Shuffle(len(indices), func(i, j int) { indices[i], indices[j] = indices[j], indices[i] })
		idx := indices[0]
		indices = indices[1:]
		if len(traceIDsPerCommit[idx]) == 0 {
			continue
		}

		entries := []*Entry{}
		for _, traceID := range traceIDsPerCommit[idx] {
			t := tile.Traces[traceID].(*types.GoldenTrace)
			digest := t.Digests[idx]
			allDigests[digest] = true
			entries = append(entries, &Entry{Value: digest, Params: t.Params()})
		}
		assert.NoError(t, traceStore.Put(ctx, tile.Commits[idx].Hash, entries, time.Now()))
	}

	// Load the tile and verify it's identical.
	fetchTileLen := tileLen
	foundTile, commits, cardinalities, err := traceStore.GetTile(ctx, fetchTileLen, false)
	assert.NoError(t, err)
	assert.NotNil(t, commits)
	assert.Equal(t, tile.Commits[len(tile.Commits)-fetchTileLen:], commits)
	assert.NotNil(t, cardinalities)

	// assert.Equal(t, tile, foundTile)
	assert.Equal(t, len(tile.Traces), len(foundTile.Traces))
	for traceID, trace := range tile.Traces {
		gt := trace.(*types.GoldenTrace)
		params := gt.Params()
		found := false

		foundCount := 0
		for _, foundTrace := range foundTile.Traces {
			if deepequal.DeepEqual(params, foundTrace.Params()) {
				foundCount++
			}
		}
		assert.Equal(t, 1, foundCount)

		for foundID, foundTrace := range foundTile.Traces {
			if deepequal.DeepEqual(params, foundTrace.Params()) {
				expDigests := gt.Digests[len(gt.Digests)-fetchTileLen:]
				found = true
				fgt := foundTrace.(*types.GoldenTrace)
				assert.Equal(t, len(expDigests), len(fgt.Digests))

				diff := []string{}
				diffStr := ""
				for idx, digest := range expDigests {
					isDiff := digest != fgt.Digests[idx]
					if isDiff {
						diff = append(diff, fmt.Sprintf("%d", idx))
						diffStr += fmt.Sprintf("    %q  !=  %q   \n", digest, fgt.Digests[idx])
					}
				}

				delete(foundTile.Traces, foundID)
				break
			}
		}
		assert.True(t, found)
		delete(tile.Traces, traceID)
	}
	assert.Equal(t, 0, len(foundTile.Traces))
	assert.Equal(t, 0, len(tile.Traces))
}

func loadSample(t assert.TestingT, fileName string) *serialize.Sample {
	file, err := os.Open(fileName)
	assert.NoError(t, err)

	sample, err := serialize.DeserializeSample(file)
	assert.NoError(t, err)

	return sample
}

type mockVcs struct {
	commits   []*vcsinfo.IndexCommit
	commitMap map[string]*vcsinfo.LongCommit
}

func newMockVCS(tileCommits []*tiling.Commit) vcsinfo.VCS {
	commits := make([]*vcsinfo.IndexCommit, 0, len(tileCommits))
	commitMap := make(map[string]*vcsinfo.LongCommit, len(tileCommits))
	for idx, c := range tileCommits {
		commits = append(commits, &vcsinfo.IndexCommit{
			Hash:      c.Hash,
			Index:     idx,
			Timestamp: time.Unix(c.CommitTime, 0),
		})
		commitMap[c.Hash] = &vcsinfo.LongCommit{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:   c.Hash,
				Author: c.Author,
			},
			Timestamp: time.Unix(c.CommitTime, 0),
		}
	}
	return &mockVcs{
		commits:   commits,
		commitMap: commitMap,
	}
}

func (m *mockVcs) IndexOf(ctx context.Context, hash string) (int, error) {
	for i, c := range m.commits {
		if c.Hash == hash {
			return i, nil
		}
	}
	return 0, fmt.Errorf("Not found: %s", hash)
}

func (m *mockVcs) LastNIndex(N int) []*vcsinfo.IndexCommit {
	return m.commits[len(m.commits)-util.MinInt(len(m.commits), N):]
}

func (m *mockVcs) DetailsMulti(ctx context.Context, hashes []string, includeBranchInfo bool) ([]*vcsinfo.LongCommit, error) {
	ret := make([]*vcsinfo.LongCommit, len(hashes))
	for idx, hash := range hashes {
		ret[idx] = m.commitMap[hash]
	}
	return ret, nil
}

func (m *mockVcs) From(start time.Time) []string                                   { return nil }
func (m *mockVcs) Range(begin, end time.Time) []*vcsinfo.IndexCommit               { return nil }
func (m *mockVcs) Update(ctx context.Context, pull, allBranches bool) error        { return nil }
func (m *mockVcs) ByIndex(ctx context.Context, N int) (*vcsinfo.LongCommit, error) { return nil, nil }
func (m *mockVcs) GetBranch() string                                               { return "master" }
func (m *mockVcs) Details(ctx context.Context, hash string, includeBranchInfo bool) (*vcsinfo.LongCommit, error) {
	return nil, nil
}

func (m *mockVcs) GetFile(ctx context.Context, fileName, commitHash string) (string, error) {
	return "", nil
}

func (m *mockVcs) ResolveCommit(ctx context.Context, commitHash string) (string, error) {
	return "", nil
}
