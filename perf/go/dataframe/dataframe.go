// Package dataframe provides DataFrame which is a TraceSet with a calculated
// ParamSet and associated commit info.
package dataframe

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vec32"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/progress"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/types"
)

const (
	// DEFAULT_NUM_COMMITS is the number of commits in the DataFrame returned
	// from New().
	DEFAULT_NUM_COMMITS = 50

	MAX_SAMPLE_SIZE = 5000
)

type contextKey string

const (
	querySemaphoreKey contextKey = "querySemaphore"
	loaderMetricsKey  contextKey = "loaderMetrics"
)

// LoaderMetrics is an interface for tracking database query metrics.
type LoaderMetrics interface {
	RecordDbQuery(duration time.Duration)
}

// WithQuerySemaphore returns a new context with the query semaphore attached.
func WithQuerySemaphore(ctx context.Context, sem chan struct{}) context.Context {
	return context.WithValue(ctx, querySemaphoreKey, sem)
}

// QuerySemaphoreFromContext retrieves the query semaphore channel from context if present.
func QuerySemaphoreFromContext(ctx context.Context) (chan struct{}, bool) {
	if semVal := ctx.Value(querySemaphoreKey); semVal != nil {
		if sem, ok := semVal.(chan struct{}); ok {
			return sem, true
		}
	}
	return nil, false
}

// WithLoaderMetrics returns a new context with loader metrics tracking attached.
func WithLoaderMetrics(ctx context.Context, metrics LoaderMetrics) context.Context {
	return context.WithValue(ctx, loaderMetricsKey, metrics)
}

// LoaderMetricsFromContext retrieves loader metrics from context if present.
func LoaderMetricsFromContext(ctx context.Context) (LoaderMetrics, bool) {
	if mVal := ctx.Value(loaderMetricsKey); mVal != nil {
		if m, ok := mVal.(LoaderMetrics); ok {
			return m, true
		}
	}
	return nil, false
}

// NewNFromKeysOptions holds optional configuration parameters for NewNFromKeys.
type NewNFromKeysOptions struct {
	// TileSize overrides default tile size if > 0.
	TileSize int32

	// MaxEmptyTiles limits the maximum consecutive empty steps scanned before giving up if > 0.
	MaxEmptyTiles int

	// LimitOutputToN caps the returned DataFrame columns to N data points.
	LimitOutputToN bool

	// SkipMetadata suppresses querying git commit details and trace source file info.
	SkipMetadata bool
}

// DataFrameBuilder is an interface for things that construct DataFrames.
type DataFrameBuilder interface {
	// GetTraceStore returns the underlying TraceStore.
	GetTraceStore() tracestore.TraceStore

	// NewFromQueryAndRange returns a populated DataFrame of the traces that match
	// the given time range [begin, end) and the passed in query, or a non-nil
	// error if the traces can't be retrieved. The 'progress' callback is called
	// periodically as the query is processed.
	NewFromQueryAndRange(ctx context.Context, begin, end time.Time, q *query.Query, progress progress.Progress) (*DataFrame, error)

	// NewFromQueryAndRangeKeepParents returns a populated DataFrame of the traces that match
	// the given time range [begin, end) and the passed in query, or a non-nil
	// error if the traces can't be retrieved. The 'progress' callback is called
	// periodically as the query is processed. This method does not filter out parent traces.
	NewFromQueryAndRangeKeepParents(ctx context.Context, begin, end time.Time, q *query.Query, progress progress.Progress) (*DataFrame, error)

	// NewFromKeysAndRange returns a populated DataFrame of the traces that match
	// the given set of 'keys' over the range of [begin, end). The 'progress'
	// callback is called periodically as the query is processed.
	NewFromKeysAndRange(ctx context.Context, keys []string, begin, end time.Time, progress progress.Progress) (*DataFrame, error)

	// NewNFromQuery returns a populated DataFrame of condensed traces of N data
	// points ending at the given 'end' time that match the given query.
	NewNFromQuery(ctx context.Context, end time.Time, q *query.Query, n int32, progress progress.Progress) (*DataFrame, error)

	// NewNFromQueryKeepParents returns a populated DataFrame of condensed traces of N data
	// points ending at the given 'end' time that match the given query. This method does not
	// filter out parent traces.
	NewNFromQueryKeepParents(ctx context.Context, end time.Time, q *query.Query, n int32, progress progress.Progress) (*DataFrame, error)

	// NewNFromKeys returns a populated DataFrame of condensed traces of N data
	// points ending at the given 'end' time for the given keys.
	NewNFromKeys(ctx context.Context, end time.Time, keys []string, n int32, progress progress.Progress, opts ...NewNFromKeysOptions) (*DataFrame, error)

	// NewNFromKeysRecursive loads data recursively backwards in history
	// using dynamic clustering and a query-then-split tree structure
	// until at least N non-sentinel data points are retrieved for all keys.
	NewNFromKeysRecursive(ctx context.Context, end time.Time, keys []string, n int32, progress progress.Progress, skipMetadata bool) (*DataFrame, error)

	// NumMatches returns the number of traces that will match the query.
	NumMatches(ctx context.Context, q *query.Query) (int64, error)

	// PreflightQuery returns the number of traces that will match the query and
	// a refined ParamSet to use for further queries. The referenceParamSet
	// should be a ParamSet that includes all the Params that could appear in a
	// query. For example, the ParamSet managed by ParamSetRefresher.
	PreflightQuery(ctx context.Context, q *query.Query, referenceParamSet paramtools.ReadOnlyParamSet) (int64, paramtools.ParamSet, error)
}

// TimestampSeconds represents a timestamp in seconds from the Unix epoch.
type TimestampSeconds int64

// ColumnHeader describes each column in a DataFrame.
type ColumnHeader struct {
	Offset    types.CommitNumber `json:"offset"`
	Timestamp TimestampSeconds   `json:"timestamp"`
	Hash      string             `json:"hash"`
	Author    string             `json:"author"`
	Message   string             `json:"message"`
	Url       string             `json:"url"`
}

// DataFrame stores Perf measurements in a table where each row is a Trace
// indexed by a structured key (see go/query), and each column is described by
// a ColumnHeader, which could be a commit or a trybot patch level.
//
// Skip is the number of commits skipped to bring the DataFrame down
// to less than MAX_SAMPLE_SIZE commits. If Skip is zero then no
// commits were skipped.
//
// The name DataFrame was gratuitously borrowed from R.
type DataFrame struct {
	TraceSet      types.TraceSet                    `json:"traceset"`
	Header        []*ColumnHeader                   `json:"header"`
	ParamSet      paramtools.ReadOnlyParamSet       `json:"paramset"`
	Skip          int                               `json:"skip"`
	TraceMetadata []types.TraceMetadata             `json:"traceMetadata"`
	SourceInfo    map[string]*types.TraceSourceInfo `json:"-"`
}

// BuildParamSet rebuilds d.ParamSet from the keys of d.TraceSet.
func (d *DataFrame) BuildParamSet() {
	paramSet := paramtools.ParamSet{}
	for key := range d.TraceSet {
		paramSet.AddParamsFromKey(key)
	}
	for _, values := range paramSet {
		sort.Strings(values)
	}
	paramSet.Normalize()
	d.ParamSet = paramSet.Freeze()
}

func simpleMap(n int) map[int]int {
	ret := map[int]int{}
	for i := 0; i < n; i += 1 {
		ret[i] = i
	}
	return ret
}

// MergeColumnHeaders creates a merged header from the two given headers.
//
// I.e. {1,4,5} + {3,4} => {1,3,4,5}
func MergeColumnHeaders(a, b []*ColumnHeader) ([]*ColumnHeader, map[int]int, map[int]int) {
	if len(a) == 0 {
		return b, simpleMap(0), simpleMap(len(b))
	} else if len(b) == 0 {
		return a, simpleMap(len(a)), simpleMap(0)
	}
	aMap := map[int]int{}
	bMap := map[int]int{}
	numA := len(a)
	numB := len(b)
	pA := 0
	pB := 0
	ret := []*ColumnHeader{}
	for {
		if pA == numA && pB == numB {
			break
		}
		if pA == numA {
			// Copy in the rest of b.
			for i := pB; i < numB; i++ {
				bMap[i] = len(ret)
				ret = append(ret, b[i])
			}
			break
		}
		if pB == numB {
			// Copy in the rest of a.
			for i := pA; i < numA; i++ {
				aMap[i] = len(ret)
				ret = append(ret, a[i])
			}
			break
		}
		if a[pA].Offset < b[pB].Offset {
			aMap[pA] = len(ret)
			ret = append(ret, a[pA])
			pA += 1
		} else if a[pA].Offset > b[pB].Offset {
			bMap[pB] = len(ret)
			ret = append(ret, b[pB])
			pB += 1
		} else {
			aMap[pA] = len(ret)
			bMap[pB] = len(ret)
			ret = append(ret, a[pA])
			pA += 1
			pB += 1
		}
	}
	return ret, aMap, bMap
}

// Join create a new DataFrame that is the union of 'a' and 'b'.
//
// Will handle the case of a and b having data for different sets of commits,
// i.e. a.Header doesn't have to equal b.Header.
func Join(a, b *DataFrame) *DataFrame {
	ret := NewEmpty()
	// Build a merged set of headers.
	header, aMap, bMap := MergeColumnHeaders(a.Header, b.Header)
	ret.Header = header
	if len(a.Header) == 0 {
		a.Header = b.Header
	}
	ret.Skip = b.Skip
	ps := paramtools.NewParamSet()
	ps.AddParamSet(a.ParamSet)
	ps.AddParamSet(b.ParamSet)
	ps.Normalize()
	ret.ParamSet = ps.Freeze()
	traceLen := len(ret.Header)
	for key, sourceTrace := range a.TraceSet {
		if _, ok := ret.TraceSet[key]; !ok {
			ret.TraceSet[key] = types.NewTrace(traceLen)
		}
		destTrace := ret.TraceSet[key]
		for sourceOffset, sourceValue := range sourceTrace {
			destTrace[aMap[sourceOffset]] = sourceValue
		}
	}
	for key, sourceTrace := range b.TraceSet {
		if _, ok := ret.TraceSet[key]; !ok {
			ret.TraceSet[key] = types.NewTrace(traceLen)
		}
		destTrace := ret.TraceSet[key]
		for sourceOffset, sourceValue := range sourceTrace {
			destTrace[bMap[sourceOffset]] = sourceValue
		}
	}
	for traceId := range a.SourceInfo {
		if _, ok := ret.SourceInfo[traceId]; !ok {
			ret.SourceInfo[traceId] = types.NewTraceSourceInfo()
		}
		ret.SourceInfo[traceId].CopyFrom(a.SourceInfo[traceId])
	}
	for traceId := range b.SourceInfo {
		if _, ok := ret.SourceInfo[traceId]; !ok {
			ret.SourceInfo[traceId] = types.NewTraceSourceInfo()
		}
		ret.SourceInfo[traceId].CopyFrom(b.SourceInfo[traceId])
	}
	return ret
}

// TraceFilter is a function type that should return true if trace 'tr' should
// be removed from a DataFrame. It is used in FilterOut.
type TraceFilter func(tr types.Trace) bool

// FilterOut removes traces from d.TraceSet if the filter function 'f' returns
// true for a trace.
//
// FilterOut rebuilds the ParamSet to match the new set of traces once
// filtering is complete.
func (d *DataFrame) FilterOut(f TraceFilter) {
	for key, tr := range d.TraceSet {
		if f(tr) {
			delete(d.TraceSet, key)
		}
	}
	d.BuildParamSet()
}

// Slice returns a dataframe that contains a subset of the current dataframe,
// starting from 'offset', the next 'size' num points will be returned as a new
// dataframe. Note that the data is composed of slices of the original data,
// not copies, so the returned dataframe must not be altered.
func (d *DataFrame) Slice(offset, size int) (*DataFrame, error) {
	if offset+size > len(d.Header) {
		return nil, fmt.Errorf("Slice exceeds current dataframe bounds.")
	}
	ret := NewEmpty()
	ret.Header = d.Header[offset : offset+size]
	for key, tr := range d.TraceSet {
		ret.TraceSet[key] = tr[offset : offset+size]
	}
	ret.BuildParamSet()
	return ret, nil
}

// Compress returns a DataFrame with all columns that don't contain any data
// removed. If the DataFrame is already fully compressed then the original
// DataFrame is returned.
func (d *DataFrame) Compress() *DataFrame {
	// Total up the number of data points we have for each commit.
	counts := make([]int, len(d.Header))
	for _, tr := range d.TraceSet {
		for i, x := range tr {
			if x != vec32.MissingDataSentinel {
				counts[i]++
			}
		}
	}

	// Find all the colums that contain at least one non-missing data point and
	// store the indexes of those columns into sourceIndexes.
	sourceIndexes := []int{}
	for i, count := range counts {
		if count > 0 {
			sourceIndexes = append(sourceIndexes, i)
		}
	}
	n := len(sourceIndexes)

	// If every column has data then there's nothing to do, this DataFrame is
	// already fully compressed.
	if n == len(d.Header) {
		return d
	}

	ret := NewEmpty()

	// Copy over the headers.
	for _, sourceIndex := range sourceIndexes {
		ret.Header = append(ret.Header, d.Header[sourceIndex])
	}

	// Create the new shorter traces.
	for key, sourceTrace := range d.TraceSet {
		trace := vec32.New(n)
		for i, sourceIndex := range sourceIndexes {
			trace[i] = sourceTrace[sourceIndex]
		}
		ret.TraceSet[key] = trace
	}
	// The ParamSet remains unchanged.
	ret.ParamSet = d.ParamSet
	ret.SourceInfo = d.SourceInfo

	return ret
}

// FromTimeRange returns the slices of ColumnHeader and int32. The slices
// are for the commits that fall in the given time range [begin, end).
//
// The value for 'skip', the number of commits skipped, is also returned.
func FromTimeRange(ctx context.Context, git perfgit.Git, begin, end time.Time) ([]*ColumnHeader, []types.CommitNumber, int, error) {
	commits, err := git.CommitSliceFromTimeRange(ctx, begin, end)
	if err != nil {
		return nil, nil, 0, skerr.Wrapf(err, "Failed to get headers and commit numbers from time range.")
	}
	colHeader := make([]*ColumnHeader, len(commits), len(commits))
	commitNumbers := make([]types.CommitNumber, len(commits), len(commits))
	for i, commit := range commits {
		colHeader[i] = &ColumnHeader{
			Offset:    commit.CommitNumber,
			Timestamp: TimestampSeconds(commit.Timestamp),
			Hash:      commit.GitHash,
			Author:    commit.Author,
			Message:   commit.Subject,
			Url:       commit.URL,
		}
		commitNumbers[i] = commit.CommitNumber
	}
	return colHeader, commitNumbers, 0, nil
}

// NewEmpty returns a new empty DataFrame.
func NewEmpty() *DataFrame {
	return &DataFrame{
		TraceSet:   types.TraceSet{},
		Header:     []*ColumnHeader{},
		ParamSet:   paramtools.NewReadOnlyParamSet(),
		SourceInfo: map[string]*types.TraceSourceInfo{},
	}
}

// NewHeaderOnly returns a DataFrame with a populated Header, with no traces.
// The 'progress' callback is called periodically as the query is processed.
func NewHeaderOnly(ctx context.Context, git perfgit.Git, begin, end time.Time) (*DataFrame, error) {
	defer timer.New("NewHeaderOnly time").Stop()
	colHeaders, _, skip, err := FromTimeRange(ctx, git, begin, end)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed creating header only dataframe.")
	}
	return &DataFrame{
		TraceSet:   types.TraceSet{},
		Header:     colHeaders,
		ParamSet:   paramtools.NewReadOnlyParamSet(),
		Skip:       skip,
		SourceInfo: map[string]*types.TraceSourceInfo{},
	}, nil
}

// MergeMultipleColumnHeaders creates a merged header from multiple headers.
// Returns the unified header, and a slice of maps (one map per input header)
// mapping each input header index to its corresponding unified header index.
func MergeMultipleColumnHeaders(headers [][]*ColumnHeader) ([]*ColumnHeader, [][]int) {
	// Find all unique offsets
	uniqueOffsets := map[types.CommitNumber]*ColumnHeader{}
	for _, h := range headers {
		for _, col := range h {
			if col == nil {
				continue
			}
			if _, ok := uniqueOffsets[col.Offset]; !ok {
				colCopy := *col
				uniqueOffsets[col.Offset] = &colCopy
			}
		}
	}

	// Sort unique offsets ascending
	offsets := make([]types.CommitNumber, 0, len(uniqueOffsets))
	for off := range uniqueOffsets {
		offsets = append(offsets, off)
	}
	sort.Slice(offsets, func(i, j int) bool {
		return offsets[i] < offsets[j]
	})

	// Build unified header
	unifiedHeader := make([]*ColumnHeader, len(offsets))
	unifiedOffsetToIndex := map[types.CommitNumber]int{}
	for i, off := range offsets {
		unifiedHeader[i] = uniqueOffsets[off]
		unifiedOffsetToIndex[off] = i
	}

	// Map each input header's offsets to their index in the unified header
	maps := make([][]int, len(headers))
	for idx, h := range headers {
		m := make([]int, len(h))
		for i := range m {
			m[i] = -1
		}
		for i, col := range h {
			if col != nil {
				m[i] = unifiedOffsetToIndex[col.Offset]
			}
		}
		maps[idx] = m
	}

	return unifiedHeader, maps
}

// Copy creates a deep copy of the DataFrame.
// Note: TraceMetadata is intentionally excluded here to maintain parity and simplicity
// with the original copy/join logic. TraceMetadata is not populated during dataframe loading;
// it is populated separately later during UI/frontend requests.
func (d *DataFrame) Copy() *DataFrame {
	if d == nil {
		return nil
	}
	ret := NewEmpty()
	ret.Skip = d.Skip
	ret.ParamSet = d.ParamSet

	ret.Header = make([]*ColumnHeader, len(d.Header))
	for i, h := range d.Header {
		if h != nil {
			colCopy := *h
			ret.Header[i] = &colCopy
		}
	}

	ret.TraceSet = make(types.TraceSet, len(d.TraceSet))
	for k, tr := range d.TraceSet {
		trCopy := make(types.Trace, len(tr))
		copy(trCopy, tr)
		ret.TraceSet[k] = trCopy
	}

	for traceId, srcInfo := range d.SourceInfo {
		if srcInfo != nil {
			ret.SourceInfo[traceId] = types.NewTraceSourceInfo()
			ret.SourceInfo[traceId].CopyFrom(srcInfo)
		}
	}

	return ret
}

// MultiJoin merges multiple DataFrames into a single unified DataFrame.
// This is significantly more efficient than calling Join repeatedly in a loop.
func MultiJoin(ctx context.Context, dfs ...*DataFrame) (*DataFrame, error) {
	activeDfs := filterActiveDataFrames(dfs)
	if len(activeDfs) == 0 {
		return NewEmpty(), nil
	}
	if len(activeDfs) == 1 {
		return activeDfs[0].Copy(), nil
	}

	headers := make([][]*ColumnHeader, len(activeDfs))
	for i, df := range activeDfs {
		headers[i] = df.Header
	}
	unifiedHeader, indexMaps := MergeMultipleColumnHeaders(headers)

	ret := NewEmpty()
	ret.Header = unifiedHeader
	// Skip of the merged frame is the skip of the last dataframe (same as Join)
	ret.Skip = activeDfs[len(activeDfs)-1].Skip
	ret.ParamSet = mergeParamSets(activeDfs)

	keyList := collectUniqueTraceKeys(activeDfs)
	traceSet, err := mergeTraceSetsParallel(ctx, activeDfs, keyList, indexMaps, len(unifiedHeader))
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to merge trace sets")
	}
	ret.TraceSet = traceSet
	mergeSourceInfo(ret, activeDfs)

	return ret, nil
}

func filterActiveDataFrames(dfs []*DataFrame) []*DataFrame {
	activeDfs := make([]*DataFrame, 0, len(dfs))
	for _, df := range dfs {
		if df != nil && len(df.Header) > 0 {
			activeDfs = append(activeDfs, df)
		}
	}
	return activeDfs
}

func mergeParamSets(dfs []*DataFrame) paramtools.ReadOnlyParamSet {
	ps := paramtools.NewParamSet()
	for _, df := range dfs {
		ps.AddParamSet(df.ParamSet)
	}
	ps.Normalize()
	return ps.Freeze()
}

func collectUniqueTraceKeys(dfs []*DataFrame) []string {
	allKeysMap := map[string]bool{}
	for _, df := range dfs {
		for key := range df.TraceSet {
			allKeysMap[key] = true
		}
	}
	keyList := make([]string, 0, len(allKeysMap))
	for key := range allKeysMap {
		keyList = append(keyList, key)
	}
	return keyList
}

func mergeTraceSetsParallel(ctx context.Context, activeDfs []*DataFrame, keyList []string, indexMaps [][]int, traceLen int) (types.TraceSet, error) {
	traceSet := make(types.TraceSet, len(keyList))
	if len(keyList) == 0 {
		return traceSet, nil
	}

	var mu sync.Mutex
	numWorkers := runtime.NumCPU()
	chunkSize := 500
	err := util.ChunkIterParallelPool(ctx, len(keyList), chunkSize, numWorkers, func(ctx context.Context, startIdx, endIdx int) error {
		localTraces := make(map[string]types.Trace, endIdx-startIdx)
		for _, key := range keyList[startIdx:endIdx] {
			destTrace := types.NewTrace(traceLen)
			for dfIdx, df := range activeDfs {
				sourceTrace, ok := df.TraceSet[key]
				if !ok {
					continue
				}
				m := indexMaps[dfIdx]
				for srcIdx, val := range sourceTrace {
					if srcIdx >= len(m) {
						break
					}
					if destIdx := m[srcIdx]; destIdx != -1 {
						destTrace[destIdx] = val
					}
				}
			}
			localTraces[key] = destTrace
		}

		mu.Lock()
		for k, v := range localTraces {
			traceSet[k] = v
		}
		mu.Unlock()
		return nil
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "parallel chunk iteration failed")
	}

	return traceSet, nil
}

func mergeSourceInfo(ret *DataFrame, activeDfs []*DataFrame) {
	for _, df := range activeDfs {
		for traceId, srcInfo := range df.SourceInfo {
			if srcInfo == nil {
				continue
			}
			if _, ok := ret.SourceInfo[traceId]; !ok {
				ret.SourceInfo[traceId] = types.NewTraceSourceInfo()
			}
			ret.SourceInfo[traceId].CopyFrom(srcInfo)
		}
	}
}
