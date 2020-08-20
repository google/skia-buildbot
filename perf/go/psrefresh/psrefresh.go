package psrefresh

import (
	"context"
	"sync"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/types"
)

// OPSProvider allows access to OrdererParamSets. TraceStore implements this interface.
type OPSProvider interface {
	GetLatestTile() (types.TileNumber, error)
	GetOrderedParamSet(ctx context.Context, tileNumber types.TileNumber) (*paramtools.OrderedParamSet, error)
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
func NewParamSetRefresher(traceStore OPSProvider) *ParamSetRefresher {
	return &ParamSetRefresher{
		traceStore: traceStore,
		ps:         paramtools.ParamSet{},
	}
}

// Start actually starts the refreshing process.
//
// The 'period' is how often the paramset should be refreshed.
func (pf *ParamSetRefresher) Start(period time.Duration) error {
	pf.period = period

	if err := pf.oneStep(); err != nil {
		return skerr.Wrapf(err, "Failed to build the initial ParamSet")
	}
	go pf.refresh()
	return nil

}

func (pf *ParamSetRefresher) oneStep() error {
	ctx := context.Background()

	tileKey, err := pf.traceStore.GetLatestTile()
	if err != nil {
		return skerr.Wrapf(err, "Failed to get starting tile.")
	}
	ops, err := pf.traceStore.GetOrderedParamSet(ctx, tileKey)
	if err != nil {
		return skerr.Wrapf(err, "Failed to paramset from latest tile.")
	}
	ps := ops.ParamSet
	tileKey = tileKey.Prev()
	ops2, err := pf.traceStore.GetOrderedParamSet(ctx, tileKey)
	if err != nil {
		return skerr.Wrapf(err, "Failed to paramset from second to latest tile.")
	}
	ps.AddParamSet(ops2.ParamSet)
	ps.Normalize()

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
