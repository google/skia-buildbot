package dataframe

import (
	"fmt"
	"sync"
	"time"

	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/ptracestore"
)

// Refresher keeps a fresh DataFrame of the last DEFAULT_NUM_COMMITS commits.
type Refresher struct {
	n      int // Number of commits in the DataFrame.
	vcs    vcsinfo.VCS
	store  ptracestore.PTraceStore
	df     *DataFrame
	period time.Duration
	mutex  sync.Mutex
}

// NewRefresher creates a new Refresher that updates the dataframe every
// 'period'.
//
// A non-nil error will be returned if the initial DataFrame cannot be
// populated. I.e. if NewRefresher returns w/o error than the caller
// can be assured that Get() will return a non-nil DataFrame.
func NewRefresher(vcs vcsinfo.VCS, store ptracestore.PTraceStore, period time.Duration, n int) (*Refresher, error) {
	ret := &Refresher{
		vcs:    vcs,
		store:  store,
		period: period,
		n:      n,
	}
	if err := ret.oneStep(); err != nil {
		return nil, fmt.Errorf("Failed to build the initial DataFrame: %s", err)
	}
	go ret.refresh()
	return ret, nil
}

func (f *Refresher) oneStep() error {
	if err := f.vcs.Update(true, false); err != nil {
		sklog.Errorf("Failed to update repo: %s", err)
	}
	newDf, err := NewN(f.vcs, f.store, nil, f.n)
	if err != nil {
		return err
	}
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.df = newDf
	return nil
}

func (f *Refresher) refresh() {
	for _ = range time.Tick(f.period) {
		if err := f.oneStep(); err != nil {
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
