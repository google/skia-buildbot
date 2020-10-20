// Package dfloader implements results.Loader using a DataFrameBuilder.
package dfloader

import (
	"context"
	"time"

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
	if request.Kind == results.TryBot {
		return results.TryBotResponse{}, skerr.Fmt("Unimplmented")
	}
	commit, err := l.git.CommitFromCommitNumber(ctx, request.CommitNumber)
	if err != nil {
		return results.TryBotResponse{}, skerr.Wrap(err)
	}

	q, err := query.NewFromString(request.Query)
	if err != nil {
		return results.TryBotResponse{}, skerr.Wrap(err)
	}

	// Always pull in TraceHistorySize+1 trace values. The TraceHistorySize
	// represents the history of the trace, and the TraceHistorySize+1 point
	// represents either the commit under inspection or a placeholder for the
	// trybot value, which lets us avoid a second memory allocation, which we'd
	// get if we had only queried for TraceHistorySize values.
	df, err := l.dfb.NewNFromQuery(ctx, time.Unix(commit.Timestamp, 0), q, TraceHistorySize+1, progress)
	if err != nil {
		return results.TryBotResponse{}, skerr.Wrap(err)
	}
	ret := results.TryBotResponse{}
	ret.Header = df.Header
	ret.ParamSet = df.ParamSet

	res := make([]results.TryBotResult, 0, len(df.TraceSet))
	// Loop over all the traces and parse the key into params
	// and pass the values to vec32.StdDevRatio.
	for traceName, values := range df.TraceSet {
		params, err := query.ParseKey(traceName)
		if err != nil {
			sklog.Errorf("Failed to parse %q: %s", traceName, err)
			continue
		}
		median, lower, upper, stddevRatio, err := vec32.StdDevRatio(values)
		if err != nil {
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

	return ret, nil
}

// Assert that we fulfill the interface.
var _ results.Loader = (*Loader)(nil)
