package dataframe

import (
	"fmt"
	"sync"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
)

// Refresher keeps a fresh DataFrame.
//
// N.B. that the Paramset and keys of TraceSet are valid.  The Header and the
// values of the traces in the TraceSet are not representative of a full tile.
type Refresher struct {
	numTiles  int
	period    time.Duration
	dfBuilder DataFrameBuilder

	mutex sync.Mutex // protects df.
	df    *DataFrame
}

// NewRefresher creates a new Refresher that updates the dataframe every
// 'period'.
//
// A non-nil error will be returned if the initial DataFrame cannot be
// populated. I.e. if NewRefresher returns w/o error than the caller
// can be assured that Get() will return a non-nil DataFrame.
func NewRefresher(dfBuilder DataFrameBuilder, period time.Duration, numTiles int) (*Refresher, error) {
	ret := &Refresher{
		dfBuilder: dfBuilder,
		period:    period,
		numTiles:  numTiles,
	}
	if err := ret.oneStep(); err != nil {
		return nil, fmt.Errorf("Failed to build the initial DataFrame: %s", err)
	}
	go ret.refresh()
	return ret, nil
}

func (f *Refresher) oneStep() error {
	newDf, err := f.dfBuilder.NewKeysOnly(f.numTiles)
	if err != nil {
		return err
	}
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.df = newDf
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
func (f *Refresher) Get() *DataFrame {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.df
}
