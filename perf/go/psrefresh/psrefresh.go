package psrefresh

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/btts"
)

// OPSProvider allows access to OrdererParamSets. btts.BigTableTraceStore implements this interface.
type OPSProvider interface {
	GetLatestTile() (btts.TileKey, error)
	GetOrderedParamSet(ctx context.Context, tileKey btts.TileKey) (*paramtools.OrderedParamSet, error)
}

// ParamSetRefresher keeps a fresh paramtools.ParamSet that represents all
// the traces stored in the two most recent tile in the trace store.
type ParamSetRefresher struct {
	traceStore OPSProvider
	period     time.Duration

	mutex sync.Mutex // protects ps.
	ps    paramtools.ParamSet
}

// NewParamSetRefresher builds a new *ParamSetRefresher.
//
// The 'period' is how often the paramset should be refreshed.
func NewParamSetRefresher(traceStore OPSProvider, period time.Duration) (*ParamSetRefresher, error) {
	ret := &ParamSetRefresher{
		traceStore: traceStore,
		period:     period,
		ps:         paramtools.ParamSet{},
	}
	if err := ret.oneStep(); err != nil {
		return nil, fmt.Errorf("Failed to build the initial ParamSet: %s", err)
	}
	go ret.refresh()
	return ret, nil
}

func (pf *ParamSetRefresher) oneStep() error {
	ctx := context.Background()

	tileKey, err := pf.traceStore.GetLatestTile()
	if err != nil {
		return nil
	}
	ops, err := pf.traceStore.GetOrderedParamSet(ctx, tileKey)
	if err != nil {
		return err
	}
	ps := ops.ParamSet
	tileKey = tileKey.PrevTile()
	ops2, err := pf.traceStore.GetOrderedParamSet(ctx, tileKey)
	if err != nil {
		return err
	}
	ps.AddParamSet(ops2.ParamSet)

	pf.mutex.Lock()
	defer pf.mutex.Unlock()
	pf.ps = ps
	return nil
}

func (pf *ParamSetRefresher) refresh() {
	stepFailures := metrics2.GetCounter("paramset_refresh_failures", nil)
	for range time.Tick(pf.period) {
		if err := pf.oneStep(); err != nil {
			sklog.Errorf("Failed to refresh the ParamSet: %s", err)
			stepFailures.Inc(1)
		}
	}
}

// Get returns the fresh paramset.
//
// It is not a copy, do not modify it.
func (pf *ParamSetRefresher) Get() paramtools.ParamSet {
	pf.mutex.Lock()
	defer pf.mutex.Unlock()
	return pf.ps
}
