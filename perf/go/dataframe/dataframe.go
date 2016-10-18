// Package dataframe provides DataFrame which is a TraceSet with a calculated
// ParamSet and associated commit info.
package dataframe

import (
	"fmt"
	"sort"
	"time"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/ptracestore"
)

const (
	// DEFAULT_NUM_COMMITS is the number of commits in the DataFrame returned
	// from New().
	DEFAULT_NUM_COMMITS = 50

	MAX_SAMPLE_SIZE = 256
)

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
	TraceSet ptracestore.TraceSet `json:"traceset"`
	Header   []*ColumnHeader      `json:"header"`
	ParamSet paramtools.ParamSet  `json:"paramset"`
	Skip     int                  `json:"skip"`
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
// are for the last 50 commits in the repo.
//
// Returns 0 for 'skip', the number of commits skipped.
func lastN(vcs vcsinfo.VCS) ([]*ColumnHeader, []*cid.CommitID, int) {
	return rangeImpl(vcs.LastNIndex(DEFAULT_NUM_COMMITS), 0)
}

// getRange returns the slices of ColumnHeader and cid.CommitID that are
// needed by DataFrame and ptracestore.PTraceStore, respectively. The slices
// are for the commits that fall in the given time range [begin, end).
//
// The value for 'skip', the number of commits skipped, is also returned.
func getRange(vcs vcsinfo.VCS, begin, end time.Time) ([]*ColumnHeader, []*cid.CommitID, int) {
	commits, skip := DownSample(vcs.Range(begin, end), MAX_SAMPLE_SIZE)
	return rangeImpl(commits, skip)
}

func _new(colHeaders []*ColumnHeader, commitIDs []*cid.CommitID, q *query.Query, store ptracestore.PTraceStore, progress ptracestore.Progress, skip int) (*DataFrame, error) {
	defer timer.New("_new time").Stop()
	traceSet, err := store.Match(commitIDs, q, progress)
	if err != nil {
		return nil, fmt.Errorf("DataFrame failed to query for all traces: %s", err)
	}
	paramSet := paramtools.ParamSet{}
	for key, _ := range traceSet {
		paramSet.AddParamsFromKey(key)
	}
	for _, values := range paramSet {
		sort.Strings(values)
	}
	return &DataFrame{
		TraceSet: traceSet,
		Header:   colHeaders,
		ParamSet: paramSet,
		Skip:     skip,
	}, nil
}

// New returns a populated DataFrame of the last 50 commits given the 'vcs' and
// 'store', or a non-nil error if there was a failure retrieving the traces.
func New(vcs vcsinfo.VCS, store ptracestore.PTraceStore, progress ptracestore.Progress) (*DataFrame, error) {
	colHeaders, commitIDs, skip := lastN(vcs)
	return _new(colHeaders, commitIDs, &query.Query{}, store, progress, skip)
}

// NewFromQueryAndRange returns a populated DataFrame of the traces that match
// the given time range [begin, end) and the passed in query, or a non-nil
// error if the traces can't be retrieved. The 'progress' callback is called
// periodically as the query is processed.
func NewFromQueryAndRange(vcs vcsinfo.VCS, store ptracestore.PTraceStore, begin, end time.Time, q *query.Query, progress ptracestore.Progress) (*DataFrame, error) {
	defer timer.New("NewFromQueryAndRange time").Stop()
	colHeaders, commitIDs, skip := getRange(vcs, begin, end)
	return _new(colHeaders, commitIDs, q, store, progress, skip)
}

// NewEmpty returns a new empty DataFrame.
func NewEmpty() *DataFrame {
	return &DataFrame{
		TraceSet: ptracestore.TraceSet{},
		Header:   []*ColumnHeader{},
		ParamSet: paramtools.ParamSet{},
	}
}

// NewHeaderOnly returns a DataFrame with a populated Header, with no traces.
// The 'progress' callback is called periodically as the query is processed.
func NewHeaderOnly(vcs vcsinfo.VCS, begin, end time.Time) *DataFrame {
	colHeaders, _, skip := getRange(vcs, begin, end)
	return &DataFrame{
		TraceSet: ptracestore.TraceSet{},
		Header:   colHeaders,
		ParamSet: paramtools.ParamSet{},
		Skip:     skip,
	}
}
