package dataframe

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/cid"
)

// Refresher keeps a fresh DataFrame of the last DEFAULT_NUM_COMMITS commits.
type Refresher struct {
	n         int32 // Tile size.
	vcs       vcsinfo.VCS
	df        *DataFrame
	period    time.Duration
	mutex     sync.Mutex
	dfBuilder DataFrameBuilder
	q         *query.Query
	cidl      *cid.CommitIDLookup
}

// NewRefresher creates a new Refresher that updates the dataframe every
// 'period'.
//
// A non-nil error will be returned if the initial DataFrame cannot be
// populated. I.e. if NewRefresher returns w/o error than the caller
// can be assured that Get() will return a non-nil DataFrame.
func NewRefresher(ctx context.Context, vcs vcsinfo.VCS, dfBuilder DataFrameBuilder, period time.Duration, n int32, cidl *cid.CommitIDLookup) (*Refresher, error) {
	q, err := query.New(url.Values{})
	if err != nil {
		return nil, err
	}
	ret := &Refresher{
		vcs:       vcs,
		dfBuilder: dfBuilder,
		period:    period,
		n:         n,
		q:         q,
		cidl:      cidl,
	}
	if err := ret.oneStep(ctx); err != nil {
		return nil, fmt.Errorf("Failed to build the initial DataFrame: %s", err)
	}
	go ret.refresh(ctx)
	return ret, nil
}

func (f *Refresher) oneStep(ctx context.Context) error {
	if err := f.vcs.Update(ctx, true, false); err != nil {
		sklog.Errorf("Failed to update repo: %s", err)
	}
	// Pick two commits far enough apart that they hit two tiles.
	lastNCommits := f.vcs.LastNIndex(int(f.n + 1))
	twoCommits := []*cid.CommitID{
		&cid.CommitID{
			Offset: lastNCommits[0].Index,
			Source: "master",
		},
		&cid.CommitID{
			Offset: lastNCommits[f.n].Index,
			Source: "master",
		},
	}
	newDf, err := f.dfBuilder.NewFromCommitIDsAndQuery(context.Background(), twoCommits, f.cidl, f.q, nil)
	if err != nil {
		return err
	}
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.df = newDf
	return nil
}

func (f *Refresher) refresh(ctx context.Context) {
	for range time.Tick(f.period) {
		if err := f.oneStep(ctx); err != nil {
			sklog.Errorf("Failed to refresh the DataFrame: %s", err)
		}
	}
}

// Get returns a DataFrame. It is not safe for modification, only for reading.
func (f *Refresher) Get() *DataFrame {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.df
}
