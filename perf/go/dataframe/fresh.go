package dataframe

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/go/vcsinfo"
)

// Refresher keeps a fresh DataFrame of the last DEFAULT_NUM_COMMITS commits.
type Refresher struct {
	n         int // Number of commits in the DataFrame.
	vcs       vcsinfo.VCS
	df        *DataFrame
	period    time.Duration
	mutex     sync.Mutex
	dfBuilder DataFrameBuilder
}

// NewRefresher creates a new Refresher that updates the dataframe every
// 'period'.
//
// A non-nil error will be returned if the initial DataFrame cannot be
// populated. I.e. if NewRefresher returns w/o error than the caller
// can be assured that Get() will return a non-nil DataFrame.
func NewRefresher(ctx context.Context, vcs vcsinfo.VCS, dfBuilder DataFrameBuilder, period time.Duration, n int) (*Refresher, error) {
	ret := &Refresher{
		vcs:       vcs,
		dfBuilder: dfBuilder,
		period:    period,
		n:         n,
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
	newDf, err := f.dfBuilder.NewN(nil, f.n)
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
