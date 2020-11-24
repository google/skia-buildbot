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
	GetLatestTile(context.Context) (types.TileNumber, error)
	GetParamSet(ctx context.Context, tileNumber types.TileNumber) (paramtools.ReadOnlyParamSet, error)
}

// ParamSetRefresher keeps a fresh paramtools.ParamSet that represents all the
// traces stored in the two most recent tiles in the trace store.
type ParamSetRefresher struct {
	traceStore OPSProvider
	period     time.Duration

	mutex sync.Mutex // protects ps.
	ps    paramtools.ReadOnlyParamSet
}

// NewParamSetRefresher builds a new *ParamSetRefresher.
func NewParamSetRefresher(traceStore OPSProvider) *ParamSetRefresher {
	return &ParamSetRefresher{
		traceStore: traceStore,
		ps:         paramtools.ReadOnlyParamSet{},
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

	tileKey, err := pf.traceStore.GetLatestTile(ctx)
	if err != nil {
		return skerr.Wrapf(err, "Failed to get starting tile.")
	}
	ps := paramtools.NewParamSet()
	ps1, err := pf.traceStore.GetParamSet(ctx, tileKey)
	if err != nil {
		return skerr.Wrapf(err, "Failed to paramset from latest tile.")
	}
	ps.AddParamSet(ps1)

	tileKey = tileKey.Prev()
	ps2, err := pf.traceStore.GetParamSet(ctx, tileKey)
	if err != nil {
		return skerr.Wrapf(err, "Failed to paramset from second to latest tile.")
	}
	ps.AddParamSet(ps2)
	ps.Normalize()

	pf.mutex.Lock()
	defer pf.mutex.Unlock()
	pf.ps = ps.Freeze()
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
func (pf *ParamSetRefresher) Get() paramtools.ReadOnlyParamSet {
	pf.mutex.Lock()
	defer pf.mutex.Unlock()
	return pf.ps
}
