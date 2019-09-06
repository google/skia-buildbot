package dfbuilder

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"go.opencensus.io/trace"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/btts"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/tracesetbuilder"
	"go.skia.org/infra/perf/go/types"
	"golang.org/x/sync/errgroup"
)

const (
	// NEW_N_FROM_KEY_STEP is the length of time to do a search for each step
	// when constructing a NewN* query.
	NEW_N_FROM_KEY_STEP = 12 * time.Hour

	// NEW_N_MAX_SEARCH is the minimum number of queries to perform that returned
	// no data before giving up.
	NEW_N_MAX_SEARCH = 4
)

// builder implements DataFrameBuilder using btts.
type builder struct {
	vcs      vcsinfo.VCS
	store    *btts.BigTableTraceStore
	tileSize int32
}

func NewDataFrameBuilderFromBTTS(vcs vcsinfo.VCS, store *btts.BigTableTraceStore) dataframe.DataFrameBuilder {
	return &builder{
		vcs:      vcs,
		store:    store,
		tileSize: store.TileSize(),
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
	ctx, span := trace.StartSpan(ctx, "dfbuilder fromIndexRange")
	defer span.End()

	g, ok := vcs.(*gitinfo.GitInfo)
	headers := []*dataframe.ColumnHeader{}
	indices := []int32{}
	for i := beginIndex; i <= endIndex; i++ {
		if ok {
			// This is a temporary performance enhancement for Perf.
			// It will be removed once Perf moves to gitstore.
			ts, err := g.TimestampAtIndex(int(i))
			if err != nil {
				return nil, nil, 0, fmt.Errorf("Range of commits invalid: %s", err)
			}
			headers = append(headers, &dataframe.ColumnHeader{
				Source:    "master",
				Offset:    int64(i),
				Timestamp: ts.Unix(),
			})
			indices = append(indices, i)
		} else {
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
func (b *builder) new(ctx context.Context, colHeaders []*dataframe.ColumnHeader, indices []int32, q *query.Query, progress types.Progress, skip int) (*dataframe.DataFrame, error) {
	ctx, span := trace.StartSpan(ctx, "dfbuilder.new")
	defer span.End()

	// TODO tickle progress as each Go routine completes.
	defer timer.New("dfbuilder_new").Stop()
	// Determine which tiles we are querying over, and how each tile maps into our results.
	mapper := buildTileMapOffsetToIndex(indices, b.store)

	traceSetBuilder := tracesetbuilder.New(len(indices))
	defer traceSetBuilder.Close()

	var mutex sync.Mutex // mutex protects stepsCompleted.
	stepsCompleted := 0
	triggerProgress := func() {
		mutex.Lock()
		defer mutex.Unlock()
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

			// Query for matching traces in the given tile.
			traces, err := b.store.QueryTracesByIndex(ctx, tileKey, q)
			if err != nil {
				return err
			}
			sklog.Debugf("found %d traces for %s", len(traces), tileKey.OpsRowName())

			traceSetBuilder.Add(traceMap, traces)
			triggerProgress()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("Failed while querying: %s", err)
	}
	traceSet, paramSet := traceSetBuilder.Build(ctx)
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
	return b.new(context.TODO(), colHeaders, indices, q, progress, skip)
}

// See DataFrameBuilder.
func (b *builder) NewFromQueryAndRange(begin, end time.Time, q *query.Query, downsample bool, progress types.Progress) (*dataframe.DataFrame, error) {
	colHeaders, indices, skip := fromTimeRange(b.vcs, begin, end, downsample)
	return b.new(context.TODO(), colHeaders, indices, q, progress, skip)
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
				p, err := query.ParseKey(key)
				if err != nil {
					continue
				}
				paramSet.AddParams(p)
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
	return b.new(ctx, colHeaders, indices, q, progress, 0)
}

// findIndexForTime finds the index of the closest commit <= 'end'.
//
// Pass in zero time, i.e. time.Time{} to indicate to just get the most recent commit.
func (b *builder) findIndexForTime(ctx context.Context, end time.Time) (int32, error) {
	ctx, span := trace.StartSpan(ctx, "dfbuilder.findIndexForTime")
	defer span.End()

	var err error
	endIndex := 0

	if end.IsZero() {
		commits := b.vcs.LastNIndex(1)
		if len(commits) == 0 {
			return 0, fmt.Errorf("Failed to find an end commit.")
		}
		return int32(commits[0].Index), nil
	}

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
	ctx, span := trace.StartSpan(ctx, "dfbuilder.NewNFromQuery")
	defer span.End()

	sklog.Infof("Querying to: %v", end)

	ret := dataframe.NewEmpty()
	var total int32 // total number of commits we've added to ret so far.
	steps := 1      // Total number of times we've gone through the loop below, used in the progress() callback.
	numStepsNoData := 0

	endIndex, err := b.findIndexForTime(ctx, end)
	if err != nil {
		return nil, fmt.Errorf("Failed to find end index: %s", err)
	}
	beginIndex := endIndex - (b.tileSize - 1)
	if beginIndex < 0 {
		beginIndex = 0
	}

	sklog.Infof("BeginIndex: %d  EndIndex: %d", beginIndex, endIndex)
	for total < n {

		// Query for traces.
		headers, indices, skip, err := fromIndexRange(ctx, b.vcs, beginIndex, endIndex)
		if err != nil {
			return nil, fmt.Errorf("Failed building index range: %s", err)
		}
		df, err := b.new(ctx, headers, indices, q, nil, skip)
		if err != nil {
			return nil, fmt.Errorf("Failed while querying: %s", err)
		}

		nonMissing := 0
		// Total up the number of data points we have for each commit.
		counts := make([]int, len(df.Header))
		for _, tr := range df.TraceSet {
			for i, x := range tr {
				if x != vec32.MISSING_DATA_SENTINEL {
					counts[i] += 1
					nonMissing += 1
				}
			}
		}
		// If there are no matches then we might be done.
		if nonMissing == 0 {
			numStepsNoData += 1
		}
		if numStepsNoData > NEW_N_MAX_SEARCH {
			sklog.Infof("Failed querying: %s", q)
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
		sklog.Infof("Total: %d Steps: %d NumStepsNoData: %d", total, steps, numStepsNoData)

		if total == n {
			break
		}

		if progress != nil {
			progress(steps, steps+1)
		}
		steps += 1

		endIndex -= b.tileSize
		beginIndex -= b.tileSize
		if endIndex < 0 {
			break
		}
		if beginIndex < 0 {
			beginIndex = 0
		}
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

	endIndex, err := b.findIndexForTime(ctx, end)
	if err != nil {
		return nil, fmt.Errorf("Failed to find end index: %s", err)
	}
	beginIndex := endIndex - (b.tileSize - 1)
	if beginIndex < 0 {
		beginIndex = 0
	}

	ret := dataframe.NewEmpty()
	var total int32 // total number of commits we've added to ret so far.
	steps := 1      // Total number of times we've gone through the loop below, used in the progress() callback.
	numStepsNoData := 0

	for total < n {
		headers, indices, skip, err := fromIndexRange(ctx, b.vcs, beginIndex, endIndex)
		if err != nil {
			return nil, fmt.Errorf("Failed building index range: %s", err)
		}

		// Determine which tiles we are querying over, and how each tile maps into our results.
		mapper := buildTileMapOffsetToIndex(indices, b.store)

		traceSet := types.TraceSet{}
		for tileKey, traceMap := range mapper {
			// Read the traces for the given keys.
			traces, err := b.store.ReadTraces(tileKey, keys)
			if err != nil {
				return nil, err
			}
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
		}
		df := &dataframe.DataFrame{
			TraceSet: traceSet,
			Header:   headers,
			ParamSet: paramtools.ParamSet{},
			Skip:     skip,
		}
		df.BuildParamSet()

		nonMissing := 0
		// Total up the number of data points we have for each commit.
		counts := make([]int, len(df.Header))
		for _, tr := range df.TraceSet {
			for i, x := range tr {
				if x != vec32.MISSING_DATA_SENTINEL {
					counts[i] += 1
					nonMissing += 1
				}
			}
		}
		// If there are no matches then we might be done.
		if nonMissing == 0 {
			numStepsNoData += 1
		}
		if numStepsNoData > NEW_N_MAX_SEARCH {
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

		sklog.Infof("Total: %d Steps: %d NumStepsNoData: %d", total, steps, numStepsNoData)

		if total == n {
			break
		}

		if progress != nil {
			progress(steps, steps+1)
		}
		steps += 1

		endIndex -= b.tileSize
		beginIndex -= b.tileSize
		if endIndex < 0 {
			break
		}
		if beginIndex < 0 {
			beginIndex = 0
		}

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
func (b *builder) PreflightQuery(ctx context.Context, end time.Time, q *query.Query) (int64, paramtools.ParamSet, error) {
	var count int64
	ps := paramtools.ParamSet{}

	tileKey, err := b.store.GetLatestTile()
	if err != nil {
		return -1, nil, err
	}

	if q.Empty() {
		// If the query is empty then we have a shortcut for building the
		// ParamSet by just using the OPS. In that case we only need to count
		// encodedKeys to get the count.
		for i := 0; i < 2; i++ {
			ops, err := b.store.GetOrderedParamSet(ctx, tileKey)
			if err != nil {
				return -1, nil, err
			}
			ps.AddParamSet(ops.ParamSet)
			tileKey = tileKey.PrevTile()
		}

		count, err = b.store.TraceCount(ctx, tileKey)
		if err != nil {
			return -1, nil, err
		}

	} else {
		// Since the query isn't empty we'll have to run a partial query
		// to build the ParamSet.
		out, err := b.store.QueryTracesIDOnlyByIndex(ctx, tileKey, q)
		if err != nil {
			return -1, nil, fmt.Errorf("Failed to query traces: %s", err)
		}
		for p := range out {
			ps.AddParams(p)
		}
		// We only start counting when we get to the second to last tile, since
		// the latest tile might not be fully populated.
		tileKey = tileKey.PrevTile()
		out, err = b.store.QueryTracesIDOnlyByIndex(ctx, tileKey, q)
		if err != nil {
			return -1, nil, fmt.Errorf("Failed to query traces: %s", err)
		}
		for p := range out {
			count++
			ps.AddParams(p)
		}

		// Now we have the ParamSet that corresponds to the query, but for each
		// key in the query we need to go back and put in all the values that
		// appear for that key since the user can make more selections in that
		// key.
		//
		// TODO(jcgregorio) We could skip this step if we fill in the values on
		// the client which already has a copy of the full ParamSet.
		ops, err := b.store.GetOrderedParamSet(ctx, tileKey)
		if err != nil {
			return -1, nil, err
		}
		queryPlan, err := q.QueryPlan(ops)
		if err != nil {
			return -1, nil, err
		}
		for key := range queryPlan {
			ps[key] = ops.ParamSet[key]
		}
	}

	ps.Normalize()
	return count, ps, nil
}

// Validate that the concrete bttsDataFrameBuilder faithfully implements the DataFrameBuidler interface.
var _ dataframe.DataFrameBuilder = (*builder)(nil)
