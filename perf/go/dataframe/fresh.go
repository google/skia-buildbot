package dataframe

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

// Refresher keeps a fresh paramset and total trace count.
//
type Refresher struct {
	period    time.Duration
	dfBuilder DataFrameBuilder
	vcs       vcsinfo.VCS

	mutex sync.Mutex // protects count and ps.
	count int64
	ps    paramtools.ParamSet
}

// NewRefresher creates a new Refresher that updates every 'period'.
//
// A non-nil error will be returned if the initial data cannot be
// populated. I.e. if NewRefresher returns w/o error than the caller can be
// assured that Get() will return valid data.
//
// It also periodically refreshes vcs.
// TODO(jcgregorio) Move to another process, or drop once we move to gitsync.
func NewRefresher(vcs vcsinfo.VCS, dfBuilder DataFrameBuilder, period time.Duration) (*Refresher, error) {
	ret := &Refresher{
		dfBuilder: dfBuilder,
		period:    period,
		vcs:       vcs,
	}
	if err := ret.oneStep(); err != nil {
		return nil, fmt.Errorf("Failed to build the initial DataFrame: %s", err)
	}
	go ret.refresh()
	return ret, nil
}

func (f *Refresher) oneStep() error {
	if err := f.vcs.Update(context.Background(), true, false); err != nil {
		return skerr.Wrap(err)
	}
	emptyQuery, err := query.New(url.Values{})
	if err != nil {
		return skerr.Wrap(err)
	}
	count, ps, err := f.dfBuilder.PreflightQuery(context.Background(), time.Now(), emptyQuery)
	if err != nil {
		return skerr.Wrap(err)
	}
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.count = count
	f.ps = ps
	return nil
}

func (f *Refresher) refresh() {
	stepFailures := metrics2.GetCounter("dataframe_refresh_failures", nil)
	for range time.Tick(f.period) {
		if err := f.oneStep(); err != nil {
			sklog.Errorf("Failed to refresh the DataFrame: %s", err)
			stepFailures.Inc(1)
		}
	}
}

// Get returns a DataFrame. It is not safe for modification, only for reading.
//
// N.B. that the Paramset and keys of TraceSet are valid.  The Header and the
// values of the traces in the TraceSet are not representative of a full tile.
func (f *Refresher) Get() (int64, paramtools.ParamSet) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.count, f.ps
}
