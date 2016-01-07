// valueweight calculates the data needed for a word cloud from a slice of params.
package valueweight

import (
	"sort"

	"go.skia.org/infra/perf/go/types"
)

// FromParams takes a slice of params and returns a [][]types.ValueWeight,
// which is the expected input for word-cloud-sk's.
func FromParams(traceparams []map[string]string) [][]types.ValueWeight {
	n := len(traceparams)

	// map[param key] -> map[param value] -> count
	counts := map[string]map[string]int{}
	// Count up the number of times each param value appears.
	for _, params := range traceparams {
		for key, value := range params {
			if count, ok := counts[key]; !ok {
				counts[key] = map[string]int{
					value: 1,
				}
			} else {
				count[value] += 1
			}
		}
	}

	ret := [][]types.ValueWeight{}
	for _, count := range counts {
		vw := []types.ValueWeight{}
		for value, weight := range count {
			vw = append(vw, types.ValueWeight{
				Value:  value,
				Weight: int(14*float64(weight)/float64(n)) + 12,
			})
		}
		sort.Sort(ValueWeightSlice(vw))
		if len(vw) > 10 {
			vw = vw[:10]
			vw = append(vw, types.ValueWeight{
				Value:  "...",
				Weight: 12,
			})
		}
		ret = append(ret, vw)
	}
	sort.Sort(ValueWeightSliceSlice(ret))
	return ret
}

// ValueWeightSliceSlice is a utility type for sorting [][]types.ValueWeight.
type ValueWeightSliceSlice [][]types.ValueWeight

func (p ValueWeightSliceSlice) Len() int { return len(p) }
func (p ValueWeightSliceSlice) Less(i, j int) bool {
	// Sort by Weight of the 0th ValueWeight,
	// then by the length of the ValueWeight slice,
	// then by Value.
	if p[i][0].Weight == p[j][0].Weight {
		if len(p[i]) == len(p[j]) {
			return p[i][0].Value < p[j][0].Value
		} else {
			return len(p[i]) < len(p[j])
		}
	} else {
		return p[i][0].Weight > p[j][0].Weight
	}

}
func (p ValueWeightSliceSlice) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

// ValueWeightSlice is a utility type for sorting []types.ValueWeight by
// Weight.
type ValueWeightSlice []types.ValueWeight

func (p ValueWeightSlice) Len() int           { return len(p) }
func (p ValueWeightSlice) Less(i, j int) bool { return p[i].Weight > p[j].Weight }
func (p ValueWeightSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
