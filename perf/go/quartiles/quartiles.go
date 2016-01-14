// quartiles calculates the distribution of trace values in quartiles.
package quartiles

import (
	"net/url"

	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/perf/go/kmlabel"
	"go.skia.org/infra/perf/go/tilestats"
	"go.skia.org/infra/perf/go/types"
)

// Quartiles describes the tiles values at a particular commit in terms of
// quartiles.
//
// Each trace will fall into one of 6 buckets, Q1-Q4 which are the four
// standard quartiles, and then Improvements and Regressions, which are the
// outliers below Q1 or above Q4 respectively. Note that we use Q1-1.5*IQR and
// Q3 + 1.5*IQR as the bounds to detect outliers, where IQR is Q3 - Q1, the
// interquartile range.  See https://en.wikipedia.org/wiki/Quartile
type Quartiles struct {
	TotalTraces       int                 `json:"total_traces"`
	Missing           int                 `json:"missing"`
	Matches           int                 `json:"matches"`
	TraceStatsMissing int                 `json:"trace_stats_missing"`
	Improvements      kmlabel.Description `json:"improvements"`
	Q1                kmlabel.Description `json:"q1"`
	Q2                kmlabel.Description `json:"q2"`
	Q3                kmlabel.Description `json:"q3"`
	Q4                kmlabel.Description `json:"q4"`
	Regressions       kmlabel.Description `json:"regressions"`
}

// FromTile calculates a Quartiles using the values of the first commit in the given tile,
// using only traces that match the given query 'q'.
func FromTile(tile *tiling.Tile, tilestats *tilestats.TileStats, q url.Values) *Quartiles {
	ret := &Quartiles{
		TotalTraces: len(tile.Traces),
	}
	imp := map[string]map[string]string{}
	q1 := map[string]map[string]string{}
	q2 := map[string]map[string]string{}
	q3 := map[string]map[string]string{}
	q4 := map[string]map[string]string{}
	reg := map[string]map[string]string{}
	tilestats.RLock()
	defer tilestats.RUnLock()
	for traceid, tr := range tile.Traces {
		if tiling.Matches(tr, q) {
			st, ok := tilestats.TraceStats(traceid)
			if !ok {
				continue
			}
			ret.Matches += 1
			value := tr.(*types.PerfTrace).Values[0]
			iqr := st.Q3 - st.Q1
			if value < (st.Q1 - 1.5*iqr) {
				imp[traceid] = tr.Params()
			} else if value < st.Q1 {
				q1[traceid] = tr.Params()
			} else if value < st.Q2 {
				q2[traceid] = tr.Params()
			} else if value < st.Q3 {
				q3[traceid] = tr.Params()
			} else if value < (st.Q3 + 1.5*iqr) {
				q4[traceid] = tr.Params()
			} else {
				reg[traceid] = tr.Params()
			}
		} else {
			ret.Missing += 1
		}
	}

	ret.Improvements = kmlabel.ClusterAndDescribe(tile.ParamSet, imp, ret.Matches)
	ret.Q1 = kmlabel.ClusterAndDescribe(tile.ParamSet, q1, ret.Matches)
	ret.Q2 = kmlabel.ClusterAndDescribe(tile.ParamSet, q2, ret.Matches)
	ret.Q3 = kmlabel.ClusterAndDescribe(tile.ParamSet, q3, ret.Matches)
	ret.Q4 = kmlabel.ClusterAndDescribe(tile.ParamSet, q4, ret.Matches)
	ret.Regressions = kmlabel.ClusterAndDescribe(tile.ParamSet, reg, ret.Matches)
	return ret
}
