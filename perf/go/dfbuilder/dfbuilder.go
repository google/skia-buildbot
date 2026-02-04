package dfbuilder

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"go.opencensus.io/trace"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/dataframe"
	perfgit "go.skia.org/infra/perf/go/git"
	pqp "go.skia.org/infra/perf/go/preflightqueryprocessor"
	"go.skia.org/infra/perf/go/progress"
	"go.skia.org/infra/perf/go/tracecache"
	"go.skia.org/infra/perf/go/tracefilter"
	"go.skia.org/infra/perf/go/tracesetbuilder"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/types"
	"golang.org/x/sync/errgroup"
)

// Filtering is a custom type used to define
// the modes for filtering parent traces
type Filtering bool

const (
	// newNMaxSearch is the minimum number of queries to perform that returned
	// no data before giving up.
	newNMaxSearch = 40

	// It is possible for some ParamSet tiles to have "bad" data, for example, a
	// tremendous amount of garbage data from a bad ingestion process. This
	// timeout guards against that eventuality by limiting how long Queries get
	// to run on each tile.
	//
	// Note that this value is much shorter than the default timeout for all SQL
	// requests because during regression detection we can issue many Queries and
	// as each one hits that bad ParamSet tile we can clog the backend with these
	// long running queries.
	singleTileQueryTimeout = time.Minute

	// Filter parent traces
	doFilterParentTraces Filtering = true

	// Do not filter parent traces
	doNotFilterParentTraces Filtering = false

	// Size of the channel to return when getting traceids from the cache.
	traceIdCacheChannelSize = 10000
)

// builder implements DataFrameBuilder using TraceStore.
type builder struct {
	git                                perfgit.Git
	store                              tracestore.TraceStore
	tileSize                           int32
	numPreflightTiles                  int
	filterParentTraces                 Filtering
	QueryCommitChunkSize               int
	maxEmptyTiles                      int
	tracecache                         *tracecache.TraceCache
	preflightSubqueriesForExistingKeys bool
	includedParams                     []string

	newTimer                      metrics2.Float64SummaryMetric
	newByTileTimer                metrics2.Float64SummaryMetric
	newFromQueryAndRangeTimer     metrics2.Float64SummaryMetric
	newFromKeysAndRangeTimer      metrics2.Float64SummaryMetric
	newFromCommitIDsAndQueryTimer metrics2.Float64SummaryMetric
	newNFromQueryTimer            metrics2.Float64SummaryMetric
	newNFromKeysTimer             metrics2.Float64SummaryMetric
	preflightQueryTimer           metrics2.Float64SummaryMetric
}

// NewDataFrameBuilderFromTraceStore builds a DataFrameBuilder.
func NewDataFrameBuilderFromTraceStore(git perfgit.Git, store tracestore.TraceStore, traceCache *tracecache.TraceCache, numPreflightTiles int, filterParentTraces Filtering, queryCommitChunkSize int, maxEmptyTiles int, preflightSubqueriesForExistingKeys bool, includedParams []string) dataframe.DataFrameBuilder {
	if maxEmptyTiles <= 0 {
		maxEmptyTiles = newNMaxSearch
	}
	return &builder{
		git:                                git,
		store:                              store,
		tracecache:                         traceCache,
		numPreflightTiles:                  numPreflightTiles,
		tileSize:                           store.TileSize(),
		filterParentTraces:                 filterParentTraces,
		QueryCommitChunkSize:               queryCommitChunkSize,
		maxEmptyTiles:                      maxEmptyTiles,
		preflightSubqueriesForExistingKeys: preflightSubqueriesForExistingKeys,
		includedParams:                     includedParams,
		newTimer:                           metrics2.GetFloat64SummaryMetric("perfserver_dfbuilder_new"),
		newByTileTimer:                     metrics2.GetFloat64SummaryMetric("perfserver_dfbuilder_newByTile"),
		newFromQueryAndRangeTimer:          metrics2.GetFloat64SummaryMetric("perfserver_dfbuilder_newFromQueryAndRange"),
		newFromKeysAndRangeTimer:           metrics2.GetFloat64SummaryMetric("perfserver_dfbuilder_newFromKeysAndRange"),
		newFromCommitIDsAndQueryTimer:      metrics2.GetFloat64SummaryMetric("perfserver_dfbuilder_newFromCommitIDsAndQuery"),
		newNFromQueryTimer:                 metrics2.GetFloat64SummaryMetric("perfserver_dfbuilder_newNFromQuery"),
		newNFromKeysTimer:                  metrics2.GetFloat64SummaryMetric("perfserver_dfbuilder_newNFromKeys"),
		preflightQueryTimer:                metrics2.GetFloat64SummaryMetric("perfserver_dfbuilder_preflightQuery"),
	}
}

// fromIndexRange returns the headers and indices for all the commits
// between beginIndex and endIndex inclusive.
func fromIndexRange(ctx context.Context, git perfgit.Git, beginIndex, endIndex types.CommitNumber) ([]*dataframe.ColumnHeader, []types.CommitNumber, int, error) {
	ctx, span := trace.StartSpan(ctx, "dfbuilder.fromIndexRange")
	defer span.End()

	commits, err := git.CommitSliceFromCommitNumberRange(ctx, beginIndex, endIndex)
	if err != nil {
		return nil, nil, 0, skerr.Wrapf(err, "Failed to get headers and commit numbers from time range.")
	}
	colHeader := make([]*dataframe.ColumnHeader, len(commits), len(commits))
	commitNumbers := make([]types.CommitNumber, len(commits), len(commits))
	for i, commit := range commits {
		colHeader[i] = &dataframe.ColumnHeader{
			Offset:    commit.CommitNumber,
			Timestamp: dataframe.TimestampSeconds(commit.Timestamp),
			Hash:      commit.GitHash,
			Author:    commit.Author,
			Message:   commit.Subject,
			Url:       commit.URL,
		}
		commitNumbers[i] = commit.CommitNumber
	}
	return colHeader, commitNumbers, 0, nil
}

// tileMapOffsetToIndex maps the offset of each point in a tile to the index it
// should appear in the resulting Trace.
type tileMapOffsetToIndex map[types.TileNumber]map[int32]int32

// sliceOfTileNumbersFromCommits returns a slice of types.TileNumber that contains every Tile that needs
// to be queried based on the all the commits in indices.
func sliceOfTileNumbersFromCommits(indices []types.CommitNumber, store tracestore.TraceStore) []types.TileNumber {
	ret := []types.TileNumber{}
	if len(indices) == 0 {
		return ret
	}
	begin := store.TileNumber(indices[0])
	end := store.TileNumber(indices[len(indices)-1])
	for i := begin; i <= end; i++ {
		ret = append(ret, i)
	}
	return ret
}

// new builds a DataFrame for the given columns and populates it with traces that match the given query.
//
// The progress callback is triggered once for every tile.
func (b *builder) new(ctx context.Context, colHeaders []*dataframe.ColumnHeader, indices []types.CommitNumber, q *query.Query, progress progress.Progress, skip int) (*dataframe.DataFrame, error) {
	ctx, span := trace.StartSpan(ctx, "dfbuilder.new")
	defer span.End()

	defer timer.NewWithSummary("perfserver_dfbuilder_new", b.newTimer).Stop()
	// Determine which tiles we are querying over, and how each tile maps into our results.
	tilesToQuery := sliceOfTileNumbersFromCommits(indices, b.store)

	commitNumberToOutputIndex := map[types.CommitNumber]int32{}
	for i, c := range indices {
		commitNumberToOutputIndex[c] = int32(i)
	}

	traceSetBuilder := tracesetbuilder.New(len(indices))
	defer traceSetBuilder.Close()

	var mutex sync.Mutex // mutex protects tilesCompleted.
	tilesCompleted := 0
	triggerProgress := func() {
		mutex.Lock()
		defer mutex.Unlock()
		tilesCompleted++
		progress.Message("Tiles", fmt.Sprintf("%d/%d", tilesCompleted, len(tilesToQuery)))
	}

	var g errgroup.Group
	sourceInfo := map[string]*types.TraceSourceInfo{}
	sourceinfoMutex := sync.Mutex{}
	// For each tile.
	for _, tileNumber := range tilesToQuery {
		tileNumber := tileNumber
		g.Go(func() error {
			defer timer.NewWithSummary("perfserver_dfbuilder_new_by_tile", b.newByTileTimer).Stop()

			// Query for matching traces in the given tile.
			queryContext, cancel := context.WithTimeout(ctx, singleTileQueryTimeout)
			defer cancel()
			traces, commits, sourceFileInfo, err := b.store.QueryTraces(queryContext, tileNumber, q, b.tracecache)
			if err != nil {
				return err
			}

			traceSetBuilder.Add(commitNumberToOutputIndex, commits, traces)
			sourceinfoMutex.Lock()
			defer sourceinfoMutex.Unlock()
			for traceid := range sourceFileInfo {
				if _, ok := sourceInfo[traceid]; !ok {
					sourceInfo[traceid] = types.NewTraceSourceInfo()
				}
				sourceInfo[traceid].CopyFrom(sourceFileInfo[traceid])
			}
			triggerProgress()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		span.SetStatus(trace.Status{
			Code:    trace.StatusCodeInternal,
			Message: err.Error(),
		})

		return nil, fmt.Errorf("Failed while querying: %s", err)
	}
	traceSet, paramSet := traceSetBuilder.Build(ctx)
	d := &dataframe.DataFrame{
		TraceSet:   traceSet,
		Header:     colHeaders,
		ParamSet:   paramSet,
		Skip:       skip,
		SourceInfo: sourceInfo,
	}
	return d.Compress(), nil
}

// See DataFrameBuilder.
func (b *builder) NewFromQueryAndRange(ctx context.Context, begin, end time.Time, q *query.Query, progress progress.Progress) (*dataframe.DataFrame, error) {
	return b.newFromQueryAndRange(ctx, begin, end, q, false, progress)
}

// See DataFrameBuilder.
func (b *builder) NewFromQueryAndRangeKeepParents(ctx context.Context, begin, end time.Time, q *query.Query, progress progress.Progress) (*dataframe.DataFrame, error) {
	return b.newFromQueryAndRange(ctx, begin, end, q, true, progress)
}

func (b *builder) newFromQueryAndRange(ctx context.Context, begin, end time.Time, q *query.Query, disableFilterParentTraces bool, progress progress.Progress) (*dataframe.DataFrame, error) {
	ctx, span := trace.StartSpan(ctx, "dfbuilder.NewFromQueryAndRange")
	defer span.End()

	defer timer.NewWithSummary("perfserver_dfbuilder_NewFromQueryAndRange", b.newFromQueryAndRangeTimer).Stop()

	colHeaders, indices, skip, err := dataframe.FromTimeRange(ctx, b.git, begin, end)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	df, err := b.new(ctx, colHeaders, indices, q, progress, skip)
	if err != nil {
		return nil, err
	}

	if b.filterParentTraces == doFilterParentTraces && !disableFilterParentTraces {
		df.TraceSet = filterParentTraces(df.TraceSet)
	}
	return df, nil
}

// See DataFrameBuilder.
func (b *builder) NewFromKeysAndRange(ctx context.Context, keys []string, begin, end time.Time, progress progress.Progress) (*dataframe.DataFrame, error) {
	ctx, span := trace.StartSpan(ctx, "dfbuilder.NewFromKeysAndRange")
	defer span.End()

	defer timer.NewWithSummary("perfserver_dfbuilder_NewFromKeysAndRange", b.newFromKeysAndRangeTimer).Stop()
	colHeaders, indices, skip, err := dataframe.FromTimeRange(ctx, b.git, begin, end)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Determine which tiles we are querying over, and how each tile maps into our results.
	// In this case we don't need this, instead a mapping from CommitNumber to Index would be easier.
	mapper := sliceOfTileNumbersFromCommits(indices, b.store)

	commitNumberToOutputIndex := map[types.CommitNumber]int32{}
	for i, c := range indices {
		commitNumberToOutputIndex[c] = int32(i)
	}

	var mutex sync.Mutex // mutex protects traceSet and paramSet.
	traceSet := types.TraceSet{}
	paramSet := paramtools.ParamSet{}
	sourceInfo := map[string]*types.TraceSourceInfo{}
	stepsCompleted := 0
	// triggerProgress must only be called when the caller has mutex locked.
	triggerProgress := func() {
		stepsCompleted += 1
		progress.Message("Tiles", fmt.Sprintf("%d/%d", stepsCompleted, len(mapper)))
	}

	var g errgroup.Group
	// For each tile.
	for _, tileNumber := range mapper {
		tileKey := tileNumber
		// traceMap := traceMap
		g.Go(func() error {
			// Read the traces for the given keys.
			traces, commits, sourceFileInfo, err := b.store.ReadTraces(ctx, tileKey, keys)
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
				for i, c := range commits {
					dstIndex, ok := commitNumberToOutputIndex[c.CommitNumber]
					if !ok {
						continue
					}
					trace[dstIndex] = tileTrace[i]
				}
				traceSet[key] = trace
				p, err := query.ParseKey(key)
				if err != nil {
					continue
				}
				paramSet.AddParams(p)
			}
			for traceid := range sourceFileInfo {
				if _, ok := sourceInfo[traceid]; !ok {
					sourceInfo[traceid] = types.NewTraceSourceInfo()
				}
				sourceInfo[traceid].CopyFrom(sourceFileInfo[traceid])
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("Failed while querying: %s", err)
	}
	d := &dataframe.DataFrame{
		TraceSet:   traceSet,
		Header:     colHeaders,
		ParamSet:   paramSet.Freeze(),
		Skip:       skip,
		SourceInfo: sourceInfo,
	}
	triggerProgress()
	return d.Compress(), nil
}

// findIndexForTime finds the index of the closest commit <= 'end'.
//
// Pass in zero time, i.e. time.Time{} to indicate to just get the most recent commit.
func (b *builder) findIndexForTime(ctx context.Context, end time.Time) (types.CommitNumber, error) {
	ctx, span := trace.StartSpan(ctx, "dfbuilder.findIndexForTime")
	defer span.End()

	return b.git.CommitNumberFromTime(ctx, end)
}

// See DataFrameBuilder.
func (b *builder) NewNFromQuery(ctx context.Context, end time.Time, q *query.Query, n int32, progress progress.Progress) (*dataframe.DataFrame, error) {
	return b.newNFromQuery(ctx, end, q, n, false, progress)
}

// See DataFrameBuilder.
func (b *builder) NewNFromQueryKeepParents(ctx context.Context, end time.Time, q *query.Query, n int32, progress progress.Progress) (*dataframe.DataFrame, error) {
	return b.newNFromQuery(ctx, end, q, n, true, progress)
}

func (b *builder) newNFromQuery(ctx context.Context, end time.Time, q *query.Query, n int32, disableFilterParentTraces bool, progress progress.Progress) (*dataframe.DataFrame, error) {
	ctx, span := trace.StartSpan(ctx, "dfbuilder.NewNFromQuery")
	defer span.End()

	defer timer.NewWithSummary("perfserver_dfbuilder_NewNFromQuery", b.newNFromQueryTimer).Stop()

	sklog.Infof("Querying to: %v", end)

	ret := dataframe.NewEmpty()
	ps := paramtools.NewParamSet()
	var total int32 // total number of commits we've added to ret so far.
	steps := 1      // Total number of times we've gone through the loop below, used in the progress() callback.
	numStepsNoData := 0

	endIndex, err := b.findIndexForTime(ctx, end)
	if err != nil {
		return nil, fmt.Errorf("Failed to find end index: %s", err)
	}
	if endIndex == types.BadCommitNumber {
		return dataframe.NewEmpty(), nil
	}

	// beginIndex is the index of the first commit in the tile that endIndex is
	// in. We are OK if beginIndex == endIndex because fromIndexRange returns
	// headers from begin to end *inclusive*.
	beginIndex := b.store.CommitNumberOfTileStart(endIndex)

	if b.QueryCommitChunkSize > 0 {
		beginIndex = endIndex - types.CommitNumber(b.QueryCommitChunkSize)
	}

	// Note on the significance of the beginIndex and endIndex values.
	// The b.new() call below looks at this range of indices, figures out what
	// tiles are present in that range and then runs queries on those tiles in
	// parallel. Having this range be greater than the tile size increases the
	// parallelism on that call and can lead to faster response times. You can
	// consider adjusting the QueryCommitChunkSize value in case instances are
	// showing slower response times and have a smaller tile size.
	sklog.Infof("BeginIndex: %d  EndIndex: %d", beginIndex, endIndex)
	for total < n {
		// Query for traces.
		headers, indices, skip, err := fromIndexRange(ctx, b.git, beginIndex, endIndex)
		if err != nil {
			return nil, fmt.Errorf("Failed building index range: %s", err)
		}

		df, err := b.new(ctx, headers, indices, q, progress, skip)
		if err != nil {
			return nil, fmt.Errorf("Failed while querying: %s", err)
		}

		nonMissing := 0
		// Total up the number of data points we have for each commit.
		counts := make([]int, len(df.Header))
		for _, tr := range df.TraceSet {
			for i, x := range tr {
				if x != vec32.MissingDataSentinel {
					counts[i] += 1
					nonMissing += 1
				}
			}
		}
		// If there are no matches then we might be done.
		if nonMissing == 0 {
			numStepsNoData += 1
		}
		if numStepsNoData > b.maxEmptyTiles {
			sklog.Infof("Failed querying: %s", q)
			break
		}

		ps.AddParamSet(df.ParamSet)

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
		for traceId := range df.SourceInfo {
			if _, ok := ret.SourceInfo[traceId]; !ok {
				ret.SourceInfo[traceId] = types.NewTraceSourceInfo()
			}
			ret.SourceInfo[traceId].CopyFrom(df.SourceInfo[traceId])
		}
		sklog.Infof("Total: %d Steps: %d NumStepsNoData: %d Query: %v", total, steps, numStepsNoData, q.String())

		if total == n {
			break
		}

		progress.Message("Tiles", fmt.Sprintf("Tiles searched: %d. Found %d/%d points.", steps, total, n))
		steps += 1

		// Now step back a full tile.

		// At this point we know beginIndex points to the 0th column in a tile,
		// so endIndex is easy to calculate.
		endIndex = beginIndex - 1
		if endIndex < 0 {
			break
		}
		beginIndex = b.store.CommitNumberOfTileStart(endIndex)
		if b.QueryCommitChunkSize > 0 {
			beginIndex = endIndex - types.CommitNumber(b.QueryCommitChunkSize)
		}
		if beginIndex < 0 {
			beginIndex = 0
		}
	}
	ps.Normalize()
	ret.ParamSet = ps.Freeze()
	if b.filterParentTraces == doFilterParentTraces && !disableFilterParentTraces {
		ret.TraceSet = filterParentTraces(ret.TraceSet)
	}

	if trimIndex := n - total; trimIndex > 0 {
		// Trim down the traces so they are the same length as ret.Header.
		for key, tr := range ret.TraceSet {
			if len(tr) > int(trimIndex) {
				ret.TraceSet[key] = tr[trimIndex:]
			}
		}
	}

	return ret, nil
}

// See DataFrameBuilder.
func (b *builder) NewNFromKeys(ctx context.Context, end time.Time, keys []string, n int32, progress progress.Progress) (*dataframe.DataFrame, error) {
	ctx, span := trace.StartSpan(ctx, "dfbuilder.NewNFromKeys")
	defer span.End()

	defer timer.NewWithSummary("perfserver_dfbuilder_NewNFromKeys", b.newNFromKeysTimer).Stop()

	endIndex, err := b.findIndexForTime(ctx, end)
	if err != nil {
		return nil, fmt.Errorf("Failed to find end index: %s", err)
	}
	if endIndex == types.BadCommitNumber {
		return dataframe.NewEmpty(), nil
	}
	beginIndex := endIndex.Add(-(b.tileSize - 1))
	if beginIndex < 0 {
		beginIndex = 0
	}

	ret := dataframe.NewEmpty()
	ps := paramtools.NewParamSet()
	var total int32 // total number of commits we've added to ret so far.
	steps := 1      // Total number of times we've gone through the loop below, used in the progress() callback.
	numStepsNoData := 0

	for total < n {
		headers, indices, skip, err := fromIndexRange(ctx, b.git, beginIndex, endIndex)
		if err != nil {
			return nil, fmt.Errorf("Failed building index range: %s", err)
		}

		// Determine which tiles we are querying over, and how each tile maps into our results.
		mapper := sliceOfTileNumbersFromCommits(indices, b.store)

		commitNumberToOutputIndex := map[types.CommitNumber]int32{}
		for i, c := range indices {
			commitNumberToOutputIndex[c] = int32(i)
		}

		traceSet := types.TraceSet{}
		sourceInfo := map[string]*types.TraceSourceInfo{}
		for _, tileNumber := range mapper {
			// Read the traces for the given keys.
			traces, commits, sourceFileInfo, err := b.store.ReadTraces(ctx, tileNumber, keys)
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
				for i, c := range commits {
					// dstIndex := traceMap[b.store.OffsetFromCommitNumber(c.CommitNumber)]
					dstIndex, ok := commitNumberToOutputIndex[c.CommitNumber]
					if !ok {
						continue
					}
					trace[dstIndex] = tileTrace[i]
				}
				/*
					for srcIndex, dstIndex := range traceMap {
						// What to we do with commits here?
						trace[dstIndex] = tileTrace[srcIndex]
					}
				*/
				traceSet[key] = trace
			}
			for traceid := range sourceFileInfo {
				if _, ok := sourceInfo[traceid]; !ok {
					sourceInfo[traceid] = types.NewTraceSourceInfo()
				}
				sourceInfo[traceid].CopyFrom(sourceFileInfo[traceid])
			}
		}
		df := &dataframe.DataFrame{
			TraceSet:   traceSet,
			Header:     headers,
			ParamSet:   paramtools.NewReadOnlyParamSet(),
			Skip:       skip,
			SourceInfo: sourceInfo,
		}
		df.BuildParamSet()

		nonMissing := 0
		// Total up the number of data points we have for each commit.
		counts := make([]int, len(df.Header))
		for _, tr := range df.TraceSet {
			for i, x := range tr {
				if x != vec32.MissingDataSentinel {
					counts[i] += 1
					nonMissing += 1
				}
			}
		}
		// If there are no matches then we might be done.
		if nonMissing == 0 {
			numStepsNoData += 1
		}
		if numStepsNoData > newNMaxSearch {
			break
		}

		ps.AddParamSet(df.ParamSet)

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

		progress.Message("Tiles", fmt.Sprintf("Tiles searched: %d. Found %d/%d points.", steps, total, n))
		steps += 1

		endIndex = endIndex.Add(-b.tileSize)
		beginIndex = endIndex.Add(-b.tileSize)
		if endIndex < 0 {
			break
		}
		if beginIndex < 0 {
			beginIndex = 0
		}
	}
	ps.Normalize()
	ret.ParamSet = ps.Freeze()
	if b.filterParentTraces {
		ret.TraceSet = filterParentTraces(ret.TraceSet)
	}
	if total < n {
		// Trim down the traces so they are the same length as ret.Header.
		for key, tr := range ret.TraceSet {
			ret.TraceSet[key] = tr[n-total:]
		}
	}

	return ret, nil
}

// PreflightQuery implements dataframe.DataFrameBuilder.
func (b *builder) PreflightQuery(ctx context.Context, mainQuery *query.Query, referenceParamSet paramtools.ReadOnlyParamSet) (int64, paramtools.ParamSet, error) {
	ctx, span := trace.StartSpan(ctx, "dfbuilder.PreflightQuery")
	defer span.End()

	defer timer.NewWithSummary("perfserver_dfbuilder_PreflightQuery", b.preflightQueryTimer).Stop()

	timeBeforeGetLatestTile := time.Now()
	tileNumber, err := b.store.GetLatestTile(ctx)
	if err != nil {
		return -1, nil, err
	}

	if mainQuery.Empty() {
		return -1, nil, skerr.Fmt("Can not pre-flight an empty query")
	}
	duration := time.Since(timeBeforeGetLatestTile)
	sklog.Debugf("Time spent to get latest tile is %d ms", int64(duration/time.Millisecond))

	// Since the query isn't empty we'll have to run a partial query to build the ParamSet.
	// We do so over the configured number of most recent preflight tiles.

	queryContext, cancel := context.WithTimeout(ctx, time.Duration(b.numPreflightTiles)*singleTileQueryTimeout)
	defer cancel()
	errg, ectx := errgroup.WithContext(queryContext)
	timeBeforeQueryTraces := time.Now()

	// Query traces corresponding to the main query. Count their number.
	// This will get us the ParamSet corresponding to the query, but we should also
	// find the values appearing for keys present in the main query.
	// Those are determined by subqueries created by removing keys one at a time.
	// For example, if the main query is
	// "benchmark=A&bot=mac&subtest=c1",
	// then the resulting paramset should contains benchmark values that are determined by
	// "bot=mac&subtest=c1" query.
	// In particular, we don't want to put all values for benchmark without any filtering
	// if there are other keys in the main query that can limit the possible options.
	mainQueryObj := pqp.NewPreflightMainQueryProcessor(mainQuery)
	mainQueryObj.SetKeysToDetectMissing(b.includedParams)
	tileNumberClone := tileNumber
	errg.Go(func() error {
		return b.preflightProcessRecentTiles(ectx, mainQueryObj, tileNumberClone)
	})

	// If the flag is enabled, we execute subqueries that will filter values
	// for keys present in the query.
	// Otherwise, we just all possible values from the reference ParamSet.
	if b.preflightSubqueriesForExistingKeys {
		doCreateSubqueryWithoutKey := func(key string) (*query.Query, error) {
			newParams := url.Values{}
			for _, p := range mainQuery.Params {
				if p.Key() != key {
					newParams[p.Key()] = p.Values
				}
			}
			return query.New(newParams)
		}

		// We process all subqueries in parallel.
		for _, param := range mainQuery.Params {
			key := param.Key()
			errg.Go(func() error {
				qWithoutKey, err := doCreateSubqueryWithoutKey(key)

				if err != nil {
					sklog.Errorf("failed to create sub-query: %s", err)
					return err
				}

				subQueryObj := pqp.NewPreflightSubQueryProcessor(mainQueryObj, qWithoutKey, key)

				// Nothing restricts this key, set all possible values.
				if qWithoutKey.Empty() {
					subQueryObj.SetReferenceParamKey(key, referenceParamSet)
					return nil
				}

				// Query the database for traces matching the subquery.
				// Again, we query $(builder.numPreflightTiles) recent tiles.
				currentTileNumber := tileNumber
				return b.preflightProcessRecentTiles(ectx, subQueryObj, currentTileNumber)
			})
		}
	} else {
		for _, p := range mainQuery.Params {
			key := p.Key()
			mainQueryObj.SetReferenceParamKey(key, referenceParamSet)
		}
	}

	if err = errg.Wait(); err != nil {
		return -1, nil, err
	}

	duration = time.Since(timeBeforeQueryTraces)
	sklog.Debugf("Time spent to query traces is %d ms", int64(duration/time.Millisecond))

	timeBeforeNormalize := time.Now()

	ps := mainQueryObj.GetParamSet()
	count := mainQueryObj.GetCount()

	ps.Normalize()
	duration = time.Since(timeBeforeNormalize)
	sklog.Debugf("Time spent to normalize param set is %d ms", int64(duration/time.Millisecond))

	return int64(count), *ps, nil
}

// NumMatches implements dataframe.DataFrameBuilder.
func (b *builder) NumMatches(ctx context.Context, q *query.Query) (int64, error) {
	ctx, span := trace.StartSpan(ctx, "dfbuilder.NumMatches")
	defer span.End()
	defer timer.NewWithSummary("perfserver_dfbuilder_NumMatches", b.preflightQueryTimer).Stop()

	var count int64

	tileNumber, err := b.store.GetLatestTile(ctx)
	if err != nil {
		return -1, skerr.Wrap(err)
	}

	queryContext, cancel := context.WithTimeout(ctx, 2*singleTileQueryTimeout)
	defer cancel()

	// Count the matches in the first tile.
	out, err := b.store.QueryTracesIDOnly(queryContext, tileNumber, q)
	if err != nil {
		return -1, skerr.Wrapf(err, "Failed to query traces.")
	}
	var tileOneCount int64
	for range out {
		tileOneCount++
	}
	count = tileOneCount

	// Now move to the previous tile.
	tileNumber = tileNumber.Prev()
	if tileNumber != types.BadTileNumber {
		// Count the matches in the second tile.
		out, err = b.store.QueryTracesIDOnly(queryContext, tileNumber, q)
		if err != nil {
			return -1, fmt.Errorf("Failed to query traces: %s", err)
		}
		var tileTwoCount int64
		for range out {
			tileTwoCount++
		}
		// Use the larger of the two counts as our result.
		if tileTwoCount > count {
			count = tileTwoCount
		}
	}

	return count, nil
}

// Filters out parent traces if there are child traces present.
// Eg: if there are two traces
// T1 = master=m1,bot=b1,test=t1,subtest_1=s1
// T2 = master=m1,bot=b1,test=t1
// we will filter out T2 and only keep T1 since T2 is a parent
// of T1
func filterParentTraces(traceSet types.TraceSet) types.TraceSet {
	traceFilter := tracefilter.NewTraceFilter()
	paramSetKeys := []string{"master", "bot", "benchmark", "test", "subtest_1", "subtest_2", "subtest_3", "subtest_4", "subtest_5"}
	for key := range traceSet {
		params, err := query.ParseKey(key)
		if err != nil {
			sklog.Errorf("Error parsing key: %s", err)
		}

		// Now lets get the path in the order of the keys
		// specified in paramSetKeys
		path := []string{}
		for _, paramKey := range paramSetKeys {
			paramValue, ok := params[paramKey]
			if ok {
				path = append(path, paramValue)
			}
		}

		traceFilter.AddPath(path, key)
	}

	filteredKeys := traceFilter.GetLeafNodeTraceKeys()
	filteredTraceSet := types.TraceSet{}
	for _, filteredKey := range filteredKeys {
		filteredTraceSet[filteredKey] = traceSet[filteredKey]
	}

	sklog.Infof("Filtered trace set length: %d, original trace set length: %d", len(filteredTraceSet), len(traceSet))
	return filteredTraceSet
}

// cacheTraceIdsIfNeeded adds the given trace ids to the cache if it is configured.
func (b *builder) cacheTraceIdsIfNeeded(ctx context.Context, tileNumber types.TileNumber, q *query.Query, traceIds []paramtools.Params) {
	if b.tracecache != nil {
		err := b.tracecache.CacheTraceIds(ctx, tileNumber, q, traceIds)
		if err != nil {
			sklog.Warningf("Error caching traceIds: %v", err)
		}
	}
}

// getTraceIds returns the traceIds matching the query in the given tile number.
// This function will first attempt to get this information from the cache. If the
// cache does not return any data or if cache is not configured for the instance,
// it will perform a database search for the trace ids.
// The first return param is true if there was a cache miss and/or a db query was done.
func (b *builder) getTraceIds(ctx context.Context, tileNumber types.TileNumber, q *query.Query) (bool, <-chan paramtools.Params, error) {
	if b.tracecache != nil {
		traceIdsFromCache, err := b.tracecache.GetTraceIds(ctx, tileNumber, q)
		if err != nil {
			sklog.Errorf("Error retrieving trace ids from cache: %v", err)
			return true, nil, err
		}
		// Check if any trace data was found in cache. If not, fall back to the database search.
		if traceIdsFromCache == nil {
			sklog.Infof("No trace ids were found in cache for tile: %d, query: %v", tileNumber, q)
			paramsChannel, err := b.store.QueryTracesIDOnly(ctx, tileNumber, q)
			return true, paramsChannel, err
		} else {
			traceIdsChannel := make(chan paramtools.Params, traceIdCacheChannelSize)
			sklog.Infof("Retrieved %d trace ids from cache for tile %d and query %v", len(traceIdsFromCache), tileNumber, q)
			go func() {
				for _, traceId := range traceIdsFromCache {
					traceIdsChannel <- traceId
				}
				close(traceIdsChannel)
			}()
			return false, traceIdsChannel, nil
		}
	}

	// Cache is not configured, so let's do a database query.
	sklog.Infof("Cache is not enabled, performing database query.")
	paramsChannel, err := b.store.QueryTracesIDOnly(ctx, tileNumber, q)
	return true, paramsChannel, err
}

// Query matching traceIDs for a single tile and process them using given aggregator.
func (b *builder) preflightProcessTile(ctx context.Context, aggregator pqp.ParamSetAggregator, iterateTileNumber types.TileNumber) error {
	// Count the matches and sum the params in the tile.
	q := aggregator.GetQuery()
	cacheMiss, out, err := b.getTraceIds(ctx, iterateTileNumber, q)
	if err != nil {
		sklog.Errorf("failed to query traces at tile %d with error: %s", iterateTileNumber, err)
		return err
	}

	traceIdsForTile := aggregator.ProcessTraceIds(out)

	// At this point we have all the traces gathered. Let's add them
	// to the cache if there is a cache configured for the instance.
	if cacheMiss {
		b.cacheTraceIdsIfNeeded(ctx, iterateTileNumber, q, traceIdsForTile)
	}

	return nil
}

func (b *builder) preflightProcessRecentTiles(ctx context.Context, aggregator pqp.ParamSetAggregator, tileNumber types.TileNumber) error {
	errgroupTile, ectx := errgroup.WithContext(ctx)

	// Query all tiles in parallel to speed things up.
	for i := 0; i < b.numPreflightTiles; i++ {
		currentTileNumber := tileNumber
		errgroupTile.Go(func() error {
			return b.preflightProcessTile(ectx, aggregator, currentTileNumber)
		})

		// Now move to the previous tile.
		tileNumber = tileNumber.Prev()
		if tileNumber == types.BadTileNumber {
			break
		}
	}

	err := errgroupTile.Wait()
	if err != nil {
		sklog.Errorf("Failed to query recent tiles with error %s", err)
		return err
	}

	aggregator.Finalize()
	return nil
}

// Validate that *builder faithfully implements the DataFrameBuidler interface.
var _ dataframe.DataFrameBuilder = (*builder)(nil)
