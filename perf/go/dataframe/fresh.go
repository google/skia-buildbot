package dataframe

import (
	"fmt"
	"sync"
	"time"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/ptracestore"
)

// Refresher keeps a fresh DataFrame of the last DEFAULT_NUM_COMMITS commits.
type Refresher struct {
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
func NewRefresher(vcs vcsinfo.VCS, store ptracestore.PTraceStore, period time.Duration) (*Refresher, error) {
	ret := &Refresher{
		vcs:    vcs,
		store:  store,
		period: period,
	}
	if err := ret.oneStep(); err != nil {
		return nil, fmt.Errorf("Failed to build the initial DataFrame: %s", err)
	}
	go ret.refresh()
	return ret, nil
}

func (f *Refresher) oneStep() error {
	newDf, err := New(f.vcs, f.store)
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
			glog.Errorf("Failed to refresh the DataFrame: %s", err)
		}
	}
}

// Get returns a DataFrame. It is not safe for modification, only for reading.
func (f *Refresher) Get() *DataFrame {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.df
}
