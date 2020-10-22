// Package dfloader implements results.Loader using a DataFrameBuilder.
package dfloader

import (
	"context"
	"fmt"
	"time"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/dataframe"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/trybot/results"
	"go.skia.org/infra/perf/go/trybot/store"
	"go.skia.org/infra/perf/go/types"
)

// TraceHistorySize is the number of points we load for each trace.
const TraceHistorySize = 20

// ErrQueryMustNotBeEmpty is returned if an empty query is passed in the TryBotRequest.
var ErrQueryMustNotBeEmpty = fmt.Errorf("Query must not be empty.")

// Loader implements results.Loader.
type Loader struct {
	dfb   dataframe.DataFrameBuilder
	store store.TryBotStore
	git   *perfgit.Git
}

// New returns a new Loader instance.
func New(dfb dataframe.DataFrameBuilder, store store.TryBotStore, git *perfgit.Git) Loader {
	return Loader{
		dfb:   dfb,
		store: store,
		git:   git,
	}
}

// Load implements the results.Loader interface.
func (l *Loader) Load(ctx context.Context, request results.TryBotRequest, progress types.Progress) (results.TryBotResponse, error) {
	timestamp := time.Now()
	if request.Kind == results.Commit {
		commit, err := l.git.CommitFromCommitNumber(ctx, request.CommitNumber)
		if err != nil {
			return results.TryBotResponse{}, skerr.Wrap(err)
		}
		timestamp = time.Unix(commit.Timestamp, 0)
	}

	q, err := query.NewFromString(request.Query)
	if err != nil {
		return results.TryBotResponse{}, skerr.Wrap(err)
	}
	if request.Kind == results.Commit && q.Empty() {
		return results.TryBotResponse{}, ErrQueryMustNotBeEmpty
	}

	var df *dataframe.DataFrame
	rebuildParamSet := false
	if request.Kind == results.Commit {
		// Always pull in TraceHistorySize+1 trace values. The TraceHistorySize
		// represents the history of the trace, and the TraceHistorySize+1 point
		// represents either the commit under inspection or a placeholder for the
		// trybot value, which lets us avoid a second memory allocation, which we'd
		// get if we had only queried for TraceHistorySize values.
		df, err = l.dfb.NewNFromQuery(ctx, timestamp, q, TraceHistorySize+1, progress)
		if err != nil {
			return results.TryBotResponse{}, skerr.Wrap(err)
		}
	} else {
		// Load the trybot results.
		storeResults, err := l.store.Get(ctx, request.CL, request.PatchNumber)
		if err != nil {
			return results.TryBotResponse{}, skerr.Wrap(err)
		}
		traceNames := make([]string, 0, len(storeResults))
		for _, results := range storeResults {
			traceNames = append(traceNames, results.TraceName)
		}
		// Query for all traces that match up with the trybot results.
		df, err = l.dfb.NewNFromKeys(ctx, timestamp, traceNames, TraceHistorySize+1, progress)
		if err != nil {
			return results.TryBotResponse{}, skerr.Wrap(err)
		}
		// Replace the last value in each trace with the trybot result.

		for _, results := range storeResults {
			values, ok := df.TraceSet[results.TraceName]
			if !ok {
				delete(df.TraceSet, results.TraceName)
				// At this point the df.ParamSet is no longer valid and we should rebuild it.
				rebuildParamSet = true
				continue
			}
			values[len(values)-1] = results.Value
		}
	}

	ret := results.TryBotResponse{}
	ret.Header = df.Header
	if request.Kind == results.TryBot && len(ret.Header) > 0 {
		ret.Header[len(ret.Header)-1].Offset = types.BadCommitNumber
	}
	ret.ParamSet = df.ParamSet

	res := make([]results.TryBotResult, 0, len(df.TraceSet))
	// Loop over all the traces and parse the key into params and pass the
	// values to vec32.StdDevRatio.
	for traceName, values := range df.TraceSet {
		params, err := query.ParseKey(traceName)
		if err != nil {
			sklog.Errorf("Failed to parse %q: %s", traceName, err)
			rebuildParamSet = true
			continue
		}
		stddevRatio, median, lower, upper, err := vec32.StdDevRatio(values)
		if err != nil {
			rebuildParamSet = true
			continue
		}
		res = append(res, results.TryBotResult{
			Params:      params,
			Median:      median,
			Lower:       lower,
			Upper:       upper,
			StdDevRatio: stddevRatio,
			Values:      values,
		})
	}
	ret.Results = res
	if rebuildParamSet {
		ps := paramtools.NewParamSet()
		for _, res := range ret.Results {
			ps.AddParams(res.Params)
		}
		ret.ParamSet = ps
	}

	return ret, nil
}

// Assert that we fulfill the interface.
var _ results.Loader = (*Loader)(nil)
