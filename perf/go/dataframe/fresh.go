package dataframe

import (
	"fmt"
	"sync"
	"time"

	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/ptracestore"
)

type Refresher struct {
	vcs    vcsinfo.VCS
	store  ptracestore.PTraceStore
	df     *DataFrame
	period time.Duration
	mutex  sync.Mutex
}

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
		f.oneStep()
	}
}

func (f *Refresher) Get() *DataFrame {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.df
}
