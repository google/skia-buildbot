// Package dfloader implements results.Loader using a DataFrameBuilder.
package dfloader

import (
	"context"
	"time"

	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
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

	df, err := l.dfb.NewNFromQuery(ctx, time.Unix(commit.Timestamp, 0), q, TraceHistorySize+1, progress)
	if err != nil {
		return results.TryBotResponse{}, skerr.Wrap(err)
	}
	return results.TryBotResponse{}, nil
}

// Assert that we fulfill the interface.
var _ results.Loader = (*Loader)(nil)
