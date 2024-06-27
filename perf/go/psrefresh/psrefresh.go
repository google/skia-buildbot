package psrefresh

import (
	"context"
	"net/url"
	"sync"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/types"
)

// OPSProvider allows access to OrdererParamSets. TraceStore implements this interface.
type OPSProvider interface {
	GetLatestTile(context.Context) (types.TileNumber, error)
	GetParamSet(ctx context.Context, tileNumber types.TileNumber) (paramtools.ReadOnlyParamSet, error)
}

// ParamSetRefresher provides an interface for accessing instance param sets.
type ParamSetRefresher interface {
	// GetAll returns all the paramsets in the instance.
	GetAll() paramtools.ReadOnlyParamSet

	// GetParamSetForQuery returns the paramsets filtered for the provided query.
	GetParamSetForQuery(ctx context.Context, query *query.Query, q url.Values) (int64, paramtools.ParamSet, error)

	// Start kicks off the param set refresh routine.
	Start(period time.Duration) error
}

// defaultParamSetRefresher keeps a fresh paramtools.ParamSet that represents all the
// traces stored in the two most recent tiles in the trace store.
type defaultParamSetRefresher struct {
	traceStore   OPSProvider
	period       time.Duration
	numParamSets int
	dfBuilder    dataframe.DataFrameBuilder
	qConfig      config.QueryConfig

	mutex sync.Mutex // protects ps.
	ps    paramtools.ReadOnlyParamSet
}

// NewDefaultParamSetRefresher builds a new *ParamSetRefresher.
func NewDefaultParamSetRefresher(traceStore OPSProvider, numParamSets int, dfBuilder dataframe.DataFrameBuilder, qconfig config.QueryConfig) *defaultParamSetRefresher {
	return &defaultParamSetRefresher{
		traceStore:   traceStore,
		numParamSets: numParamSets,
		dfBuilder:    dfBuilder,
		qConfig:      qconfig,
		ps:           paramtools.ReadOnlyParamSet{},
	}
}

// Start actually starts the refreshing process.
//
// The 'period' is how often the paramset should be refreshed.
func (pf *defaultParamSetRefresher) Start(period time.Duration) error {
	pf.period = period
	sklog.Info("Refresher refreshing")

	if err := pf.oneStep(); err != nil {
		return skerr.Wrapf(err, "Failed to build the initial ParamSet")
	}
	go pf.refresh()
	return nil

}

func (pf *defaultParamSetRefresher) oneStep() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)
	defer cancel()

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
	pf.ps = ps.Freeze()
	return nil
}

func (pf *defaultParamSetRefresher) refresh() {
	stepFailures := metrics2.GetCounter("paramset_refresh_failures", nil)
	for range time.Tick(pf.period) {
		if err := pf.oneStep(); err != nil {
			sklog.Errorf("Failed to refresh the ParamSet: %s", err)
			stepFailures.Inc(1)
		}
	}
}

// GetAll returns the fresh paramset.
func (pf *defaultParamSetRefresher) GetAll() paramtools.ReadOnlyParamSet {
	pf.mutex.Lock()
	defer pf.mutex.Unlock()
	return pf.ps
}

// GetParamSetForQuery returns the trace count, paramset for the given query.
func (pf *defaultParamSetRefresher) GetParamSetForQuery(ctx context.Context, query *query.Query, q url.Values) (int64, paramtools.ParamSet, error) {
	return pf.dfBuilder.PreflightQuery(ctx, query, pf.GetAll())
}

// append the default values for parameters
func (pf *defaultParamSetRefresher) UpdateQueryValueWithDefaults(v url.Values) {
	if len(pf.qConfig.DefaultParamSelections) > 0 {
		for key, values := range pf.qConfig.DefaultParamSelections {
			v[key] = values
		}
	}
}

// check whether value is part of the list validValues
func ShouldCacheValue(value string, validValues []string) bool {
	for _, validValue := range validValues {
		if value == validValue {
			return true
		}
	}
	return false
}

// Confirm we implement the interface.
var _ ParamSetRefresher = (*defaultParamSetRefresher)(nil)
