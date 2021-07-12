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
	traceStore   OPSProvider
	period       time.Duration
	numParamSets int

	mutex sync.Mutex // protects ps.
	ps    paramtools.ReadOnlyParamSet
}

// NewParamSetRefresher builds a new *ParamSetRefresher.
func NewParamSetRefresher(traceStore OPSProvider, numParamSets int) *ParamSetRefresher {
	return &ParamSetRefresher{
		traceStore:   traceStore,
		numParamSets: numParamSets,
		ps:           paramtools.ReadOnlyParamSet{},
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
	first := true
	for i := 0; i < pf.numParamSets; i++ {
		ps1, err := pf.traceStore.GetParamSet(ctx, tileKey)
		if err != nil {
			if first {
				// Only the failing on the first tile should be an error,
				// previous tiles may be empty, or invalid.
				return skerr.Wrapf(err, "Failed to get paramset from first tile.")
			}
			sklog.Warningf("Failed to get paramset from %d most recent tile: %s", i, err)
		}
		first = false
		ps.AddParamSet(ps1)
		tileKey = tileKey.Prev()
	}

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
