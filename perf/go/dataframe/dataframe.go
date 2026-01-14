// Package dataframe provides DataFrame which is a TraceSet with a calculated
// ParamSet and associated commit info.
package dataframe

import (
	"context"
	"fmt"
	"sort"
	"time"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/vec32"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/progress"
	"go.skia.org/infra/perf/go/types"
)

const (
	// DEFAULT_NUM_COMMITS is the number of commits in the DataFrame returned
	// from New().
	DEFAULT_NUM_COMMITS = 50

	MAX_SAMPLE_SIZE = 5000
)

// DataFrameBuilder is an interface for things that construct DataFrames.
type DataFrameBuilder interface {
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

	// NewNFromQuery returns a populated DataFrame of condensed traces of N data
	// points ending at the given 'end' time for the given keys.
	NewNFromKeys(ctx context.Context, end time.Time, keys []string, n int32, progress progress.Progress) (*DataFrame, error)

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
		return nil, fmt.Errorf("Slize exceeds current dataframe bounds.")
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
