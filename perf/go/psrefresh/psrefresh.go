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

type ParamSetRefresher struct {
	ts     *btts.BigTableTraceStore
	period time.Duration

	mutex sync.Mutex // protects ps.
	ps    paramtools.ParamSet
}

func NewParamSetRefresher(ts *btts.BigTableTraceStore, period time.Duration) (*ParamSetRefresher, error) {
	ret := &ParamSetRefresher{
		ts:     ts,
		period: period,
		ps:     paramtools.ParamSet{},
	}
	if err := ret.oneStep(); err != nil {
		return nil, fmt.Errorf("Failed to build the initial ParamSet: %s", err)
	}
	go ret.refresh()
	return ret, nil
}

func (pf *ParamSetRefresher) oneStep() error {
	ctx := context.Background()
	tileKey, err := pf.ts.GetLatestTile()
	if err != nil {
		return nil
	}
	ops, err := pf.ts.GetOrderedParamSet(ctx, tileKey)
	if err != nil {
		return err
	}
	ps := ops.ParamSet
	tileKey = tileKey.PrevTile()
	ops2, err := pf.ts.GetOrderedParamSet(ctx, tileKey)
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

func (pf *ParamSetRefresher) Get() paramtools.ParamSet {
	defer pf.mutex.Unlock()
	return pf.ps
}
