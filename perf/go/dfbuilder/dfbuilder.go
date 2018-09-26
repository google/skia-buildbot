package dfbuilder

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/btts"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/dataframe"
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
		commits, skip = dataframe.DownSample(commits, dataframe.MAX_SAMPLE_SIZE)
	}
	return fromIndexCommit(commits, skip)
}

// tileMapOffsetToIndex maps the offset of each point in a tile to the index it
// should appear in the resulting Trace.
type tileMapOffsetToIndex map[btts.TileKey]map[int32]int32

// buildTileMapOffsetToIndex returns a tileMapOffsetToIndex for the given indices and the given BigTableTraceStore.
//
// The returned map is used when loading traces out of tiles.
func buildTileMapOffsetToIndex(indices []int32, store *btts.BigTableTraceStore) tileMapOffsetToIndex {
	ret := tileMapOffsetToIndex{}
	for targetIndex, sourceIndex := range indices {
		tileKey := store.TileKey(sourceIndex)
		if traceMap, ok := ret[tileKey]; !ok {
			ret[tileKey] = map[int32]int32{
				store.OffsetFromIndex(sourceIndex): int32(targetIndex),
			}
		} else {
			traceMap[store.OffsetFromIndex(sourceIndex)] = int32(targetIndex)
		}
	}
	return ret
}

// new builds a DataFrame for the given columns and populates it with traces that match the given query.
//
// The progress callback is triggered once for every tile.
func (b *builder) new(colHeaders []*dataframe.ColumnHeader, indices []int32, q *query.Query, progress types.Progress, skip int) (*dataframe.DataFrame, error) {
	// TODO tickle progress as each Go routine completes.
	defer timer.New("btts_new").Stop()
	// Determine which tiles we are querying over, and how each tile maps into our results.
	mapper := buildTileMapOffsetToIndex(indices, b.store)

	var mutex sync.Mutex // mutex protects traceSet, paramSet, and stepsCompleted.
	traceSet := types.TraceSet{}
	paramSet := paramtools.ParamSet{}
	stepsCompleted := 0
	// triggerProgress must only be called when the caller has mutex locked.
	triggerProgress := func() {
		stepsCompleted += 1
		if progress != nil {
			progress(stepsCompleted, len(mapper))
		}
	}

	var g errgroup.Group
	// For each tile.
	for tileKey, traceMap := range mapper {
		tileKey := tileKey
		traceMap := traceMap
		// TODO(jcgregorio) If we query across a large number of tiles N then this will spawn N*8 Go routines
		// all hitting the backend at the same time. Maybe we need a worker pool if this becomes a problem.
		g.Go(func() error {
			// Get the OPS, which we need to encode the query, and decode the traceids of the results.
			ops, err := b.store.GetOrderedParamSet(tileKey)
			if err != nil {
				return err
			}
			// Convert query to regex.
			r, err := q.Regexp(ops)
			if err != nil {
				sklog.Infof("Failed to compile query regex: %s", err)
				// Not an error, we just won't match anything in this tile.
				return nil
			}
			// Query for matching traces in the given tile.
			traces, err := b.store.QueryTraces(tileKey, r)
			if err != nil {
				return err
			}
			mutex.Lock()
			defer mutex.Unlock()
			// For each trace, convert the encodedKey to a structured key
			// and copy the trace values into their final destination.
			for encodedKey, tileTrace := range traces {
				p, err := ops.DecodeParamsFromString(encodedKey)
				if err != nil {
					// It is possible we matched a trace that appeared after we grabbed the OPS,
					// so just ignore it.
					continue
				}
				paramSet.AddParams(p)
				key, err := query.MakeKey(p)
				if err != nil {
					return err
				}
				trace, ok := traceSet[key]
				if !ok {
					trace = types.NewTrace(len(indices))
				}
				for offset, value := range tileTrace {
					trace[traceMap[int32(offset)]] = value
				}
				traceSet[key] = trace
			}
			triggerProgress()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("Failed while querying: %s", err)
	}
	d := &dataframe.DataFrame{
		TraceSet: traceSet,
		Header:   colHeaders,
		ParamSet: paramSet,
		Skip:     skip,
	}
	return d, nil
}

// See DataFrameBuilder.
func (b *builder) New(progress types.Progress) (*dataframe.DataFrame, error) {
	return b.NewN(progress, dataframe.DEFAULT_NUM_COMMITS)
}

// See DataFrameBuilder.
func (b *builder) NewN(progress types.Progress, n int) (*dataframe.DataFrame, error) {
	colHeaders, indices, skip := lastNCommits(b.vcs, n)
	q, err := query.New(url.Values{})
	if err != nil {
		return nil, err
	}
	return b.new(colHeaders, indices, q, progress, skip)
}

// See DataFrameBuilder.
func (b *builder) NewFromQueryAndRange(begin, end time.Time, q *query.Query, progress types.Progress) (*dataframe.DataFrame, error) {
	colHeaders, indices, skip := fromTimeRange(b.vcs, begin, end, true)
	return b.new(colHeaders, indices, q, progress, skip)
}

// See DataFrameBuilder.
func (b *builder) NewFromKeysAndRange(keys []string, begin, end time.Time, progress types.Progress) (*dataframe.DataFrame, error) {
	// TODO tickle progress as each Go routine completes.
	defer timer.New("NewFromKeysAndRange").Stop()
	colHeaders, indices, skip := fromTimeRange(b.vcs, begin, end, true)

	// Determine which tiles we are querying over, and how each tile maps into our results.
	mapper := buildTileMapOffsetToIndex(indices, b.store)

	var mutex sync.Mutex // mutex protects traceSet and paramSet.
	traceSet := types.TraceSet{}
	paramSet := paramtools.ParamSet{}
	stepsCompleted := 0
	// triggerProgress must only be called when the caller has mutex locked.
	triggerProgress := func() {
		stepsCompleted += 1
		if progress != nil {
			progress(stepsCompleted, len(mapper))
		}
	}

	var g errgroup.Group
	// For each tile.
	for tileKey, traceMap := range mapper {
		tileKey := tileKey
		traceMap := traceMap
		g.Go(func() error {
			// Read the traces for the given keys.
			traces, err := b.store.ReadTraces(tileKey, keys)
			if err != nil {
				return err
			}
			mutex.Lock()
			defer mutex.Unlock()
			// For each trace, convert the encodedKey to a structured key
			// and copy the trace values into their final destination.
			for key, tileTrace := range traces {
				trace, ok := traceSet[key]
				if !ok {
					trace = types.NewTrace(len(indices))
				}
				for offset, value := range tileTrace {
					trace[traceMap[int32(offset)]] = value
				}
				traceSet[key] = trace
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
		ParamSet: paramSet,
		Skip:     skip,
	}
	triggerProgress()
	return d, nil
}

// See DataFrameBuilder.
func (b *builder) NewFromCommitIDsAndQuery(ctx context.Context, cids []*cid.CommitID, cidl *cid.CommitIDLookup, q *query.Query, progress types.Progress) (*dataframe.DataFrame, error) {
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
	return b.new(colHeaders, indices, q, progress, 0)
}

// Validate that the concrete bttsDataFrameBuilder faithfully implements the DataFrameBuidler interface.
var _ dataframe.DataFrameBuilder = (*builder)(nil)
