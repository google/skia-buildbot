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
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/btts"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/types"
	"golang.org/x/sync/errgroup"
)

const (
	NEW_N_FROM_KEY_STEP = 4 * 24 * time.Hour
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

// fromIndexRange returns the headers and indices for all the commits
// between beginIndex and endIndex inclusive.
func fromIndexRange(ctx context.Context, vcs vcsinfo.VCS, beginIndex, endIndex int32) ([]*dataframe.ColumnHeader, []int32, int, error) {
	headers := []*dataframe.ColumnHeader{}
	indices := []int32{}
	for i := beginIndex; i <= endIndex; i++ {
		commit, err := vcs.ByIndex(ctx, int(i))
		if err != nil {
			return nil, nil, 0, fmt.Errorf("Range of commits invalid: %s", err)
		}
		headers = append(headers, &dataframe.ColumnHeader{
			Source:    "master",
			Offset:    int64(i),
			Timestamp: commit.Timestamp.Unix(),
		})
		indices = append(indices, i)
	}
	return headers, indices, 0, nil
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
	defer timer.New("dfbuilder_new").Stop()
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
			defer timer.New("dfbuilder_by_tile").Stop()
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
			if !q.Empty() && r.String() == "" {
				// Not an error, we just won't match anything in this tile. This
				// condition occurs if a new key appears from one tile to the next, in
				// which case Regexp(ops) returns "" for the Tile that's never seen the
				// key.
				sklog.Info("Query matches all traces, which we'll ignore.")
				return nil
			}
			// Query for matching traces in the given tile.
			traces, err := b.store.QueryTraces(tileKey, r)
			if err != nil {
				return err
			}
			sklog.Debugf("found %d traces for %s", len(traces), tileKey.OpsRowName())
			mutex.Lock()
			defer mutex.Unlock()
			sklog.Debugf("before processing traces for %s, len(traceSet) is %d, len(paramSet) is %d", tileKey.OpsRowName(), len(traceSet), len(paramSet))
			defer timer.New("dfbuilder_by_tile_mutex_held").Stop()
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
				for srcIndex, dstIndex := range traceMap {
					trace[dstIndex] = tileTrace[srcIndex]
				}
				traceSet[key] = trace
			}
			sklog.Debugf("after processing traces for %s, len(traceSet) is %d, len(paramSet) is %d", tileKey.OpsRowName(), len(traceSet), len(paramSet))
			triggerProgress()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("Failed while querying: %s", err)
	}
	paramSet.Normalize()
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
func (b *builder) NewFromQueryAndRange(begin, end time.Time, q *query.Query, downsample bool, progress types.Progress) (*dataframe.DataFrame, error) {
	colHeaders, indices, skip := fromTimeRange(b.vcs, begin, end, downsample)
	return b.new(colHeaders, indices, q, progress, skip)
}

// See DataFrameBuilder.
func (b *builder) NewFromKeysAndRange(keys []string, begin, end time.Time, downsample bool, progress types.Progress) (*dataframe.DataFrame, error) {
	// TODO tickle progress as each Go routine completes.
	defer timer.New("NewFromKeysAndRange").Stop()
	colHeaders, indices, skip := fromTimeRange(b.vcs, begin, end, downsample)

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
				for srcIndex, dstIndex := range traceMap {
					trace[dstIndex] = tileTrace[srcIndex]
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

// findIndexForTime finds the index of the closest commit <= 'end'.
func (b *builder) findIndexForTime(ctx context.Context, end time.Time) (int32, error) {
	var err error
	endIndex := 0

	hashes := b.vcs.From(end)
	if len(hashes) > 0 {
		endIndex, err = b.vcs.IndexOf(ctx, hashes[0])
		if err != nil {
			return 0, fmt.Errorf("Failed loading end commit: %s", err)
		}
	} else {
		commits := b.vcs.LastNIndex(1)
		if len(commits) == 0 {
			return 0, fmt.Errorf("Failed to find an end commit.")
		}
		endIndex = commits[0].Index
	}
	return int32(endIndex), nil
}

// See DataFrameBuilder.
func (b *builder) NewNFromQuery(ctx context.Context, end time.Time, q *query.Query, n int32, progress types.Progress) (*dataframe.DataFrame, error) {
	begin := end.Add(-NEW_N_FROM_KEY_STEP)

	ret := dataframe.NewEmpty()
	var total int32 // total number of commits we've added to ret so far.
	steps := 1      // Total number of times we've gone through the loop below, used in the progress() callback.

	for total < n {
		endIndex, err := b.findIndexForTime(ctx, end)
		if err != nil {
			return nil, fmt.Errorf("Failed to find end index: %s", err)
		}
		beginIndex, err := b.findIndexForTime(ctx, begin)
		if err != nil {
			return nil, fmt.Errorf("Failed to find begin index: %s", err)
		}
		if endIndex == beginIndex {
			break
		}

		// Query for traces.
		headers, indices, skip, err := fromIndexRange(ctx, b.vcs, beginIndex, endIndex)
		if err != nil {
			return nil, fmt.Errorf("Failed building index range: %s", err)
		}
		df, err := b.new(headers, indices, q, nil, skip)
		if err != nil {
			return nil, fmt.Errorf("Failed while querying: %s", err)
		}

		// If there are no matches then we're done.
		if len(df.TraceSet) == 0 {
			break
		}

		// Total up the number of data points we have for each commit.
		counts := make([]int, len(df.Header))
		for _, tr := range df.TraceSet {
			for i, x := range tr {
				if x != vec32.MISSING_DATA_SENTINEL {
					counts[i] += 1
				}
			}
		}

		// If we have no data for any commit then we are done.
		done := true
		for _, count := range counts {
			if count > 0 {
				done = false
				break
			}
		}
		if done {
			break
		}

		ret.ParamSet.AddParamSet(df.ParamSet)

		// For each commit that has data, copy the data from df into ret.
		// Move backwards down the trace since we are building the result from 'end' backwards.
		for i := len(counts) - 1; i >= 0; i-- {
			if counts[i] > 0 {
				ret.Header = append([]*dataframe.ColumnHeader{df.Header[i]}, ret.Header...)
				for key, sourceTrace := range df.TraceSet {
					if _, ok := ret.TraceSet[key]; !ok {
						ret.TraceSet[key] = vec32.New(int(n))
					}
					ret.TraceSet[key][n-1-total] = sourceTrace[i]
				}
				total += 1

				// If we've added enough commits to ret then we are done.
				if total == n {
					break
				}
			}
		}

		if progress != nil {
			progress(steps, steps+1)
		}
		steps += 1

		end = begin.Add(-time.Millisecond) // Since our ranges are half open, i.e. they always include 'begin'.
		begin = end.Add(-NEW_N_FROM_KEY_STEP)
	}

	if total < n {
		// Trim down the traces so they are the same length as ret.Header.
		for key, tr := range ret.TraceSet {
			ret.TraceSet[key] = tr[n-total:]
		}
	}

	return ret, nil
}

// See DataFrameBuilder.
func (b *builder) NewNFromKeys(ctx context.Context, end time.Time, keys []string, n int32, progress types.Progress) (*dataframe.DataFrame, error) {
	defer timer.New("NewNFromKeys").Stop()

	// For the lack of a better heuristic, we'll load one day of data at a time.
	begin := end.Add(-NEW_N_FROM_KEY_STEP)

	ret := dataframe.NewEmpty()
	var total int32 // total number of commits we've added to ret so far.
	steps := 1      // Total number of times we've gone through the loop below, used in the progress() callback.

	for total < n {
		df, err := b.NewFromKeysAndRange(keys, begin, end, false, nil)
		if err != nil {
			return nil, fmt.Errorf("Failed while querying: %s", err)
		}

		// If there are no matches then we're done.
		if len(df.TraceSet) == 0 {
			break
		}

		// Total up the number of data points we have for each commit.
		counts := make([]int, len(df.Header))
		for _, tr := range df.TraceSet {
			for i, x := range tr {
				if x != vec32.MISSING_DATA_SENTINEL {
					counts[i] += 1
				}
			}
		}

		// If we have no data for any commit then we are done.
		done := true
		for _, count := range counts {
			if count > 0 {
				done = false
				break
			}
		}
		if done {
			break
		}

		ret.ParamSet.AddParamSet(df.ParamSet)

		// For each commit that has data, copy the data from df into ret.
		// Move backwards down the trace since we are building the result from 'end' backwards.
		for i := len(counts) - 1; i >= 0; i-- {
			if counts[i] > 0 {
				ret.Header = append([]*dataframe.ColumnHeader{df.Header[i]}, ret.Header...)
				for key, sourceTrace := range df.TraceSet {
					if _, ok := ret.TraceSet[key]; !ok {
						ret.TraceSet[key] = vec32.New(int(n))
					}
					ret.TraceSet[key][n-1-total] = sourceTrace[i]
				}
				total += 1

				// If we've added enough commits to ret then we are done.
				if total == n {
					break
				}
			}
		}

		if progress != nil {
			progress(steps, steps+1)
		}
		steps += 1

		// Step back another day.
		end = begin.Add(-time.Millisecond) // Since our ranges are half open, i.e. they always include 'begin'.
		begin = end.Add(-NEW_N_FROM_KEY_STEP)
	}

	if total < n {
		// Trim down the traces so they are the same length as ret.Header.
		for key, tr := range ret.TraceSet {
			ret.TraceSet[key] = tr[n-total:]
		}
	}

	return ret, nil
}

// Validate that the concrete bttsDataFrameBuilder faithfully implements the DataFrameBuidler interface.
var _ dataframe.DataFrameBuilder = (*builder)(nil)
