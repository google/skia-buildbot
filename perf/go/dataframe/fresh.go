package dataframe

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/cid"
)

const NUM_TILES = 10

// Refresher keeps a fresh DataFrame.
//
// N.B. that the Paramset and keys of TraceSet are valid.  The Header and the
// values of the traces in the TraceSet are not representative of a full tile.
type Refresher struct {
	tileSize  int32
	vcs       vcsinfo.VCS
	period    time.Duration
	dfBuilder DataFrameBuilder
	q         *query.Query
	cidl      *cid.CommitIDLookup

	mutex sync.Mutex // protects df.
	df    *DataFrame
}

// NewRefresher creates a new Refresher that updates the dataframe every
// 'period'.
//
// A non-nil error will be returned if the initial DataFrame cannot be
// populated. I.e. if NewRefresher returns w/o error than the caller
// can be assured that Get() will return a non-nil DataFrame.
func NewRefresher(ctx context.Context, vcs vcsinfo.VCS, dfBuilder DataFrameBuilder, period time.Duration, tileSize int32, cidl *cid.CommitIDLookup) (*Refresher, error) {
	q, err := query.New(url.Values{})
	if err != nil {
		return nil, err
	}
	ret := &Refresher{
		vcs:       vcs,
		dfBuilder: dfBuilder,
		period:    period,
		tileSize:  tileSize,
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
	// Pick enough commits far enough apart that they hit NUM_TILES tiles.
	//
	// Note that this won't work for Android because sparse data, ugh!!!!!
	lastNCommits := f.vcs.LastNIndex(int(NUM_TILES*f.tileSize + 1))
	commits := []*cid.CommitID{}
	for i := 0; i < NUM_TILES; i++ {
		commits = append(commits, &cid.CommitID{
			Offset: lastNCommits[i*int(f.tileSize)].Index,
			Source: "master",
		})
	}
	newDf, err := f.dfBuilder.NewFromCommitIDsAndQuery(context.Background(), commits, f.cidl, f.q, nil)
	if err != nil {
		return err
	}
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.df = newDf
	return nil
}

func (f *Refresher) refresh(ctx context.Context) {
	stepFailures := metrics2.GetCounter("dataframe_refresh_failures", nil)
	for range time.Tick(f.period) {
		if err := f.oneStep(ctx); err != nil {
			sklog.Errorf("Failed to refresh the DataFrame: %s", err)
			stepFailures.Inc(1)
		}
	}
}

// Get returns a DataFrame. It is not safe for modification, only for reading.
//
// N.B. that the Paramset and keys of TraceSet are valid.  The Header and the
// values of the traces in the TraceSet are not representative of a full tile.
func (f *Refresher) Get() *DataFrame {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.df
}
