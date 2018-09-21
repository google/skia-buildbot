package dfbuilder

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/btts"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/ptracestore"
	"go.skia.org/infra/perf/go/types"
	"golang.org/x/sync/errgroup"
)

// builder implements DataFrameBuilder using btts.
type builder struct {
	vcs   vcsinfo.VCS
	store *btts.BigTableTraceStore
}

func NewDataFrameBuilderFromBTTS(vcs vcsinfo.VCS, store *btts.BigTableTraceStore) dataframe.DataFrameBuilder {
	return &builder{
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

func (b *builder) _new(colHeaders []*dataframe.ColumnHeader, indices []int32, q *query.Query, progress ptracestore.Progress, skip int) (*dataframe.DataFrame, error) {
	defer timer.New("btts_new").Stop()
	mapper := buildTraceMapper(indices, b.store)

	var mutex sync.Mutex // mutex protects traceSet.
	traceSet := types.TraceSet{}
	var g errgroup.Group
	for tileKey, traceMap := range mapper {
		tileKey := tileKey
		traceMap := traceMap
		g.Go(func() error {
			ops, err := b.store.GetOrderedParamSet(tileKey)
			if err != nil {
				return err
			}
			r, err := q.Regexp(ops)
			if err != nil {
				return err
			}
			traces, err := b.store.QueryTraces(tileKey, r)
			if err != nil {
				return err
			}
			mutex.Lock()
			defer mutex.Unlock()

			for encodedKey, trace := range traces {
				p, err := ops.DecodeParamsFromString(encodedKey)
				if err != nil {
					return err
				}
				key, err := query.MakeKey(p)
				if err != nil {
					return err
				}
				if _, ok := traceSet[key]; !ok {
					traceSet[key] = types.NewTrace(len(indices))
				}
				for offset, value := range trace {
					traceSet[key][traceMap[int32(offset)]] = value
				}
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("Failed while querying: %s", err)
	}
	d := &dataframe.DataFrame{
		TraceSet: traceSet,
		Header:   colHeaders,
		ParamSet: paramtools.ParamSet{},
		Skip:     skip,
	}
	d.BuildParamSet()
	return d, nil
}

// See DataFrameBuilder.
func (b *builder) New(progress ptracestore.Progress) (*dataframe.DataFrame, error) {
	return nil, nil
}

// See DataFrameBuilder.
func (b *builder) NewN(progress ptracestore.Progress, n int) (*dataframe.DataFrame, error) {
	return nil, nil
}

// See DataFrameBuilder.
func (b *builder) NewFromQueryAndRange(begin, end time.Time, q *query.Query, progress ptracestore.Progress) (*dataframe.DataFrame, error) {
	return nil, nil
}

// See DataFrameBuilder.
func (b *builder) NewFromKeysAndRange(keys []string, begin, end time.Time, progress ptracestore.Progress) (*dataframe.DataFrame, error) {
	return nil, nil
}

// See DataFrameBuilder.
func (b *builder) NewFromCommitIDsAndQuery(ctx context.Context, cids []*cid.CommitID, cidl *cid.CommitIDLookup, q *query.Query, progress ptracestore.Progress) (*dataframe.DataFrame, error) {
	details, err := cidl.Lookup(ctx, cids)
	if err != nil {
		return nil, fmt.Errorf("Failed to look up CommitIDs: %s", err)
	}
	colHeaders := []*dataframe.ColumnHeader{}
	indices := []int32{}
	for _, d := range details {
		colHeaders = append(colHeaders, &dataframe.ColumnHeader{
			Source:    d.Source,
			Offset:    int64(d.Offset),
			Timestamp: d.Timestamp,
		})
		indices = append(indices, int32(d.Offset))
	}
	b._new(colHeaders, indices, q, progress, 0)
	return nil, nil
}

// Validate that the concrete bttsDataFrameBuilder faithfully implements the DataFrameBuidler interface.
var _ dataframe.DataFrameBuilder = (*builder)(nil)
