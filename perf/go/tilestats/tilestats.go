// tilestats calculates basic statistics for each trace in the current tile.
package tilestats

import (
	"sort"
	"sync"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/vec"
)

// TraceStats contains statistics for a trace.
type TraceStats struct {
	Mean   float64
	StdDev float64

	// Qn are the first, second, and third quartiles.
	Q1 float64
	Q2 float64
	Q3 float64
}

// TileStats calculates TraceStats for each trace in the current tile.
type TileStats struct {
	evt   *eventbus.EventBus
	stats map[string]*TraceStats
	mutex sync.Mutex
}

// New create a new TileStats. The eventbus is monitored for new tiles
// and the stats are recalculated every time the tile is updated.
func New(evt *eventbus.EventBus) *TileStats {
	ret := &TileStats{
		evt:   evt,
		stats: map[string]*TraceStats{},
	}
	evt.SubscribeAsync(db.NEW_TILE_AVAILABLE_EVENT, func(it interface{}) {
		tile := it.(*tiling.Tile)
		glog.Info("TileStats: Beginning.")
		ret.calcStats(tile)
		glog.Info("TileStats: Finished.")
	})

	return ret
}

// cleanCopy creates a copy of a perf trace with all the missing values elided.
func cleanCopy(vec []float64) []float64 {
	ret := make([]float64, 0, len(vec))
	for _, f := range vec {
		if f != config.MISSING_DATA_SENTINEL {
			ret = append(ret, f)
		}
	}
	return ret
}

// median finds the median of a sorted slice of float64.
//
// It alsu return true if the length of the slice is odd.
func median(vec []float64) (float64, bool) {
	n := len(vec)
	n_2 := n / 2
	if n%2 == 0 {
		return (vec[n_2-1] + vec[n_2]) / 2, false
	} else {
		return vec[n_2], true
	}

}

// calcTraceStats returns the mean, stddev, q1, q2, and q3 of the given trace.
func calcTraceStats(trace []float64) (float64, float64, float64, float64, float64) {
	v := cleanCopy(trace)
	sort.Float64s(v)
	mean, stddev, _ := vec.MeanAndStdDev(v)
	n := len(v)
	q1 := 0.0
	q2 := 0.0
	q3 := 0.0
	// Calculate the quartiles, using Method 1: https://en.wikipedia.org/wiki/Quartile#Method_1
	if n > 1 {
		odd := false
		q2, odd = median(v)
		q1 = q2
		q3 = q2
		if n > 2 {
			q1, _ = median(v[0 : n/2])
			if odd {
				q3, _ = median(v[n/2+1:])
			} else {
				q3, _ = median(v[n/2:])
			}
		}
	}
	return mean, stddev, q1, q2, q3
}

// calcStats recalculates the stats for each trace in the given tile.
func (t *TileStats) calcStats(tile *tiling.Tile) {
	for id, tr := range tile.Traces {
		ptr := tr.(*types.PerfTrace)
		mean, stddev, q1, q2, q3 := calcTraceStats(ptr.Values)
		t.mutex.Lock()
		if _, ok := t.stats[id]; !ok {
			t.stats[id] = &TraceStats{}
		}
		st := t.stats[id]
		st.Mean = mean
		st.StdDev = stddev
		st.Q1 = q1
		st.Q2 = q2
		st.Q3 = q3
		t.mutex.Unlock()
	}
}
