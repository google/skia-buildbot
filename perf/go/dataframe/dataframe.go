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
	"go.skia.org/infra/perf/go/ptracestore"
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
	NewFromQueryAndRange(begin, end time.Time, q *query.Query, progress types.Progress) (*DataFrame, error)

	// NewFromKeysAndRange returns a populated DataFrame of the traces that match
	// the given set of 'keys' over the range of [begin, end). The 'progress'
	// callback is called periodically as the query is processed.
	NewFromKeysAndRange(keys []string, begin, end time.Time, progress types.Progress) (*DataFrame, error)

	// NewFromCommitIDsAndQuery returns a populated DataFrame of the traces that
	// match the given time set of commits 'cids' and the query 'q'. The 'progress'
	// callback is called periodically as the query is processed.
	NewFromCommitIDsAndQuery(ctx context.Context, cids []*cid.CommitID, cidl *cid.CommitIDLookup, q *query.Query, progress types.Progress) (*DataFrame, error)

	// TODO Add func to count matches.
	// TODO Add func to get merged paramset for a date range.
	// TODO Add func to get dataframe of condensed traces of N data points ending at a given index.
}

// ptracestoreDataFrameBuilder implements DataFrameBuilder using ptracestore.
type ptracestoreDataFrameBuilder struct {
	vcs   vcsinfo.VCS
	store ptracestore.PTraceStore
}

func NewDataFrameBuilderFromPTraceStore(vcs vcsinfo.VCS, store ptracestore.PTraceStore) DataFrameBuilder {
	return &ptracestoreDataFrameBuilder{
		vcs:   vcs,
		store: store,
	}
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

// rangeImpl returns the slices of ColumnHeader and cid.CommitID that
// are needed by DataFrame and ptracestore.PTraceStore, respectively. The
// slices are populated from the given vcsinfo.IndexCommits.
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
// needed by DataFrame and ptracestore.PTraceStore, respectively. The slices
// are for the last N commits in the repo.
//
// Returns 0 for 'skip', the number of commits skipped.
func lastN(vcs vcsinfo.VCS, n int) ([]*ColumnHeader, []*cid.CommitID, int) {
	return rangeImpl(vcs.LastNIndex(n), 0)
}

// getRange returns the slices of ColumnHeader and cid.CommitID that are
// needed by DataFrame and ptracestore.PTraceStore, respectively. The slices
// are for the commits that fall in the given time range [begin, end).
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

func _new(colHeaders []*ColumnHeader, commitIDs []*cid.CommitID, matches ptracestore.KeyMatches, store ptracestore.PTraceStore, progress types.Progress, skip int) (*DataFrame, error) {
	defer timer.New("_new time").Stop()
	traceSet, err := store.Match(commitIDs, matches, progress)
	if err != nil {
		return nil, fmt.Errorf("DataFrame failed to query for all traces: %s", err)
	}
	d := &DataFrame{
		TraceSet: traceSet,
		Header:   colHeaders,
		ParamSet: paramtools.ParamSet{},
		Skip:     skip,
	}

	d.BuildParamSet()
	return d, nil
}

// See DataFrameBuilder.
func (p *ptracestoreDataFrameBuilder) New(progress types.Progress) (*DataFrame, error) {
	return p.NewN(progress, DEFAULT_NUM_COMMITS)
}

// See DataFrameBuilder.
func (p *ptracestoreDataFrameBuilder) NewN(progress types.Progress, n int) (*DataFrame, error) {
	colHeaders, commitIDs, skip := lastN(p.vcs, n)
	matches := func(key string) bool {
		return true
	}
	return _new(colHeaders, commitIDs, matches, p.store, progress, skip)
}

// See DataFrameBuilder.
func (p *ptracestoreDataFrameBuilder) NewFromQueryAndRange(begin, end time.Time, q *query.Query, progress types.Progress) (*DataFrame, error) {
	defer timer.New("NewFromQueryAndRange time").Stop()
	colHeaders, commitIDs, skip := getRange(p.vcs, begin, end, true)
	return _new(colHeaders, commitIDs, q.Matches, p.store, progress, skip)
}

// See DataFrameBuilder.
func (p *ptracestoreDataFrameBuilder) NewFromKeysAndRange(keys []string, begin, end time.Time, progress types.Progress) (*DataFrame, error) {
	defer timer.New("NewFromKeysAndRange time").Stop()
	colHeaders, commitIDs, skip := getRange(p.vcs, begin, end, true)
	sort.Strings(keys)
	matches := func(key string) bool {
		i := sort.SearchStrings(keys, key)
		if i > len(keys)-1 {
			return false
		}
		return keys[i] == key
	}
	return _new(colHeaders, commitIDs, matches, p.store, progress, skip)
}

// See DataFrameBuilder.
func (p *ptracestoreDataFrameBuilder) NewFromCommitIDsAndQuery(ctx context.Context, cids []*cid.CommitID, cidl *cid.CommitIDLookup, q *query.Query, progress types.Progress) (*DataFrame, error) {
	details, err := cidl.Lookup(ctx, cids)
	if err != nil {
		return nil, fmt.Errorf("Failed to look up CommitIDs: %s", err)
	}
	colHeaders := []*ColumnHeader{}
	for _, d := range details {
		colHeaders = append(colHeaders, &ColumnHeader{
			Source:    d.Source,
			Offset:    int64(d.Offset),
			Timestamp: d.Timestamp,
		})
	}
	return _new(colHeaders, cids, q.Matches, p.store, progress, 0)
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

// Validate that the concrete ptracestoreDataFrameBuilder faithfully implements the DataFrameBuidler interface.
var _ DataFrameBuilder = (*ptracestoreDataFrameBuilder)(nil)
