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
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/types"
)

const (
	// DEFAULT_NUM_COMMITS is the number of commits in the DataFrame returned
	// from New().
	DEFAULT_NUM_COMMITS = 50

	MAX_SAMPLE_SIZE = 256
)

// DataFrameBuilder is an interface for things that construct DataFrames.
type DataFrameBuilder interface {
	// New returns a populated DataFrame of the last 50 commits or a non-nil
	// error if there was a failure retrieving the traces.
	New(progress types.Progress) (*DataFrame, error)

	// NewN returns a populated DataFrame of the last N commits or a non-nil
	// error if there was a failure retrieving the traces.
	NewN(progress types.Progress, n int) (*DataFrame, error)

	// NewFromQueryAndRange returns a populated DataFrame of the traces that match
	// the given time range [begin, end) and the passed in query, or a non-nil
	// error if the traces can't be retrieved. The 'progress' callback is called
	// periodically as the query is processed.
	NewFromQueryAndRange(begin, end time.Time, q *query.Query, downsample bool, progress types.Progress) (*DataFrame, error)

	// NewFromKeysAndRange returns a populated DataFrame of the traces that match
	// the given set of 'keys' over the range of [begin, end). The 'progress'
	// callback is called periodically as the query is processed.
	NewFromKeysAndRange(keys []string, begin, end time.Time, downsample bool, progress types.Progress) (*DataFrame, error)

	// NewFromCommitIDsAndQuery returns a populated DataFrame of the traces that
	// match the given time set of commits 'cids' and the query 'q'. The 'progress'
	// callback is called periodically as the query is processed.
	NewFromCommitIDsAndQuery(ctx context.Context, cids []*cid.CommitID, cidl *cid.CommitIDLookup, q *query.Query, progress types.Progress) (*DataFrame, error)

	// NewNFromQuery returns a populated DataFrame of condensed traces of N data
	// points ending at the given 'end' time that match the given query.
	NewNFromQuery(ctx context.Context, end time.Time, q *query.Query, n int32, progress types.Progress) (*DataFrame, error)

	// NewNFromQuery returns a populated DataFrame of condensed traces of N data
	// points ending at the given 'end' time for the given keys.
	NewNFromKeys(ctx context.Context, end time.Time, keys []string, n int32, progress types.Progress) (*DataFrame, error)

	// NewKeysOnly is a dataframe built for the most recent numTiles Tiles
	// that has no data in it. Used for the dataframe Refresher.
	NewKeysOnly(numTiles int) (*DataFrame, error)

	// TODO Add func to count matches.
	// TODO Add func to get merged paramset for a date range.
}

// ColumnHeader describes each column in a DataFrame.
type ColumnHeader struct {
	Source    string `json:"source"`
	Offset    int64  `json:"offset"`
	Timestamp int64  `json:"timestamp"` // In seconds from the Unix epoch.
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
	TraceSet types.TraceSet      `json:"traceset"`
	Header   []*ColumnHeader     `json:"header"`
	ParamSet paramtools.ParamSet `json:"paramset"`
	Skip     int                 `json:"skip"`
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
	d.ParamSet = paramSet
}

func simpleMap(n int) map[int]int {
	ret := map[int]int{}
	for i := 0; i < n; i += 1 {
		ret[i] = i
	}
	return ret
}

// merge creates a merged header from the two given headers.
//
// I.e. {1,4,5} + {3,4} => {1,3,4,5}
func merge(a, b []*ColumnHeader) ([]*ColumnHeader, map[int]int, map[int]int) {
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
	header, aMap, bMap := merge(a.Header, b.Header)
	ret.Header = header
	if len(a.Header) == 0 {
		a.Header = b.Header
	}
	ret.Skip = b.Skip
	ret.ParamSet.AddParamSet(a.ParamSet)
	ret.ParamSet.AddParamSet(b.ParamSet)
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

// rangeImpl returns the slices of ColumnHeader and cid.CommitID that are
// needed by DataFrame. The slices are populated from the given
// vcsinfo.IndexCommits.
//
// The value for 'skip', the number of commits skipped, is passed through to
// the return values.
func rangeImpl(resp []*vcsinfo.IndexCommit, skip int) ([]*ColumnHeader, []*cid.CommitID, int) {
	headers := []*ColumnHeader{}
	commits := []*cid.CommitID{}
	for _, r := range resp {
		commits = append(commits, &cid.CommitID{
			Offset: r.Index,
			Source: "master",
		})
		headers = append(headers, &ColumnHeader{
			Source:    "master",
			Offset:    int64(r.Index),
			Timestamp: r.Timestamp.Unix(),
		})
	}
	return headers, commits, skip
}

// lastN returns the slices of ColumnHeader and cid.CommitID that are
// needed by DataFrame. The slices are for the last N commits in the repo.
//
// Returns 0 for 'skip', the number of commits skipped.
func lastN(vcs vcsinfo.VCS, n int) ([]*ColumnHeader, []*cid.CommitID, int) {
	return rangeImpl(vcs.LastNIndex(n), 0)
}

// getRange returns the slices of ColumnHeader and cid.CommitID that are needed
// by DataFrame. The slices are for the commits that fall in the given time
// range [begin, end).
//
// If 'downsample' is true then the number of commits returned is limited
// to MAX_SAMPLE_SIZE.
//
// The value for 'skip', the number of commits skipped, is also returned.
func getRange(vcs vcsinfo.VCS, begin, end time.Time, downsample bool) ([]*ColumnHeader, []*cid.CommitID, int) {
	commits := vcs.Range(begin, end)
	skip := 0
	if downsample {
		commits, skip = DownSample(vcs.Range(begin, end), MAX_SAMPLE_SIZE)
	}
	return rangeImpl(commits, skip)
}

// NewEmpty returns a new empty DataFrame.
func NewEmpty() *DataFrame {
	return &DataFrame{
		TraceSet: types.TraceSet{},
		Header:   []*ColumnHeader{},
		ParamSet: paramtools.ParamSet{},
	}
}

// NewHeaderOnly returns a DataFrame with a populated Header, with no traces.
// The 'progress' callback is called periodically as the query is processed.
//
// If 'downsample' is true then the number of commits returned is limited
// to MAX_SAMPLE_SIZE.
func NewHeaderOnly(vcs vcsinfo.VCS, begin, end time.Time, downsample bool) *DataFrame {
	defer timer.New("NewHeaderOnly time").Stop()
	colHeaders, _, skip := getRange(vcs, begin, end, downsample)
	return &DataFrame{
		TraceSet: types.TraceSet{},
		Header:   colHeaders,
		ParamSet: paramtools.ParamSet{},
		Skip:     skip,
	}
}
