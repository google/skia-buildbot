package clustering2

import (
	"sort"
	"strings"
)

// ValuePercent is a weight proportional to the number of times the key=value
// appears in a cluster. Used in ClusterSummary.
type ValuePercent struct {
	// Value is the key value pair, e.g. "config=8888".
	Value string `json:"value"`

	// Percent is a percentage as an int, i.e. 80% is represented as 80.
	Percent int `json:"percent"`
}

// valuePercentSortable is a utility class for sorting the ValuePercent's by Weight.
type valuePercentSortable []ValuePercent

func (p valuePercentSortable) Len() int           { return len(p) }
func (p valuePercentSortable) Less(i, j int) bool { return p[i].Percent > p[j].Percent }
func (p valuePercentSortable) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// valuePercentSliceSortable is a utility class for sorting []ValuePercent's by
// the Percent of the first entry in the []ValuePercent. If they are equal then
// sort by Value.
type valuePercentSliceSortable [][]ValuePercent

func (p valuePercentSliceSortable) Len() int { return len(p) }
func (p valuePercentSliceSortable) Less(i, j int) bool {
	if p[i][0].Percent == p[j][0].Percent {
		return p[i][0].Value < p[j][0].Value
	}
	return p[i][0].Percent > p[j][0].Percent
}
func (p valuePercentSliceSortable) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

// SortValuePercentSlice the slice of ValuePercent in a way that's useful to humans.
func SortValuePercentSlice(arr []ValuePercent) {
	// We want the keys that have the highest Percents at the top, but we also want
	// to group all results by key.
	//
	// E.g. we want a final sort that looks like this.
	//   config=8888 90
	//   config=565  10
	//   arch=x86    80
	//   arch=arm    20

	// First break the slice into different slices, one for each unique key.
	subSlicesByKey := map[string][]ValuePercent{}
	for _, vp := range arr {
		key := strings.Split(vp.Value, "=")[0]
		subSlicesByKey[key] = append(subSlicesByKey[key], vp)
	}

	// Now sort each of those slices.
	subSlices := [][]ValuePercent{}
	for _, subSlice := range subSlicesByKey {
		sort.Sort(valuePercentSortable(subSlice))
		subSlices = append(subSlices, subSlice)
	}

	// Now sort the slice of slices by the first entry in each slice.
	sort.Sort(valuePercentSliceSortable(subSlices))

	// Now reassemble into a single slice.
	i := 0
	for _, subSlice := range subSlices {
		for _, vp := range subSlice {
			arr[i] = vp
			i++
		}
	}
}
