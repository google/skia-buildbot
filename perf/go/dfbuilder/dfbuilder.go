package dfbuilder

import (
	"context"
	"time"

	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/btts"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/ptracestore"
)

// bttsDataFrameBuilder implements DataFrameBuilder using btts.
type bttsDataFrameBuilder struct {
	vcs   vcsinfo.VCS
	store btts.BigTableTraceStore
}

func NewDataFrameBuilderFromBTTS(vcs vcsinfo.VCS, store btts.BigTableTraceStore) dataframe.DataFrameBuilder {
	return &bttsDataFrameBuilder{
		vcs:   vcs,
		store: store,
	}
}

// fromIndexCommit returns the slices of ColumnHeader and index.
// The slices are populated from the given vcsinfo.IndexCommits.
//
// The value for 'skip', the number of commits skipped, is passed through to
// the return values.
func fromIndexCommit(resp []*vcsinfo.IndexCommit, skip int) ([]*dataframe.ColumnHeader, []int32, int) {
	headers := []*dataframe.ColumnHeader{}
	indices := []int32{}
	for _, r := range resp {
		headers = append(headers, &dataframe.ColumnHeader{
			Source:    "master",
			Offset:    int64(r.Index),
			Timestamp: r.Timestamp.Unix(),
		})
		indices = append(indices, int32(r.Index))
	}
	return headers, indices, skip
}

// lastNCommits returns the slices of ColumnHeader and cid.CommitID that are
// needed by DataFrame and ptracestore.PTraceStore, respectively. The slices
// are for the last N commits in the repo.
//
// Returns 0 for 'skip', the number of commits skipped.
func lastNCommits(vcs vcsinfo.VCS, n int) ([]*dataframe.ColumnHeader, []int32, int) {
	return fromIndexCommit(vcs.LastNIndex(n), 0)
}

// fromTimeRange returns the slices of ColumnHeader and int32. The slices
// are for the commits that fall in the given time range [begin, end).
//
// If 'downsample' is true then the number of commits returned is limited
// to MAX_SAMPLE_SIZE.
//
// The value for 'skip', the number of commits skipped, is also returned.
func fromTimeRange(vcs vcsinfo.VCS, begin, end time.Time, downsample bool) ([]*dataframe.ColumnHeader, []int32, int) {
	commits := vcs.Range(begin, end)
	skip := 0
	if downsample {
		commits, skip = dataframe.DownSample(vcs.Range(begin, end), dataframe.MAX_SAMPLE_SIZE)
	}
	return fromIndexCommit(commits, skip)
}

type TileMap map[btts.TileKey]map[int32]int32

// For each tileKey a map is returned that maps from tile offset to the location in the resulting trace.
//
// The returned map is used when loading traces out of tiles.
func buildTraceMapper(indices []int32, store *btts.BigTableTraceStore) TileMap {
	ret := TileMap{}
	for targetIndex, sourceIndex := range indices {
		tileKey := store.TileKey(sourceIndex)
		if tm, ok := ret[tileKey]; !ok {
			ret[tileKey] = map[int32]int32{
				store.OffsetFromIndex(sourceIndex): int32(targetIndex),
			}
		} else {
			tm[store.OffsetFromIndex(sourceIndex)] = int32(targetIndex)
		}
	}
	return ret
}

// See DataFrameBuilder.
func (b *bttsDataFrameBuilder) New(progress ptracestore.Progress) (*dataframe.DataFrame, error) {
	return nil, nil
}

// See DataFrameBuilder.
func (b *bttsDataFrameBuilder) NewN(progress ptracestore.Progress, n int) (*dataframe.DataFrame, error) {
	return nil, nil
}

// See DataFrameBuilder.
func (b *bttsDataFrameBuilder) NewFromQueryAndRange(begin, end time.Time, q *query.Query, progress ptracestore.Progress) (*dataframe.DataFrame, error) {
	return nil, nil
}

// See DataFrameBuilder.
func (b *bttsDataFrameBuilder) NewFromKeysAndRange(keys []string, begin, end time.Time, progress ptracestore.Progress) (*dataframe.DataFrame, error) {
	return nil, nil
}

// See DataFrameBuilder.
func (b *bttsDataFrameBuilder) NewFromCommitIDsAndQuery(ctx context.Context, cids []*cid.CommitID, cidl *cid.CommitIDLookup, q *query.Query, progress ptracestore.Progress) (*dataframe.DataFrame, error) {
	return nil, nil
}

// Validate that the concrete bttsDataFrameBuilder faithfully implements the DataFrameBuidler interface.
var _ dataframe.DataFrameBuilder = (*bttsDataFrameBuilder)(nil)
