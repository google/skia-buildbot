package samplestats

import (
	"math"
	"sort"
)

// An Order defines a sort order for a slice of Rows.
type Order func(rows []Row, i, j int) bool

// ByName sorts tables by the trace id name column
func ByName(rows []Row, i, j int) bool {
	return rows[i].Name < rows[j].Name
}

// ByDelta sorts tables by the Delta column.
func ByDelta(rows []Row, i, j int) bool {
	// Always sort the NaN results (insignificant changes) to the top.
	if math.IsNaN(rows[i].Delta) {
		return true
	}
	return rows[i].Delta < rows[j].Delta
}

// Reverse returns the reverse of the given order.
func Reverse(order Order) Order {
	return func(rows []Row, i, j int) bool { return order(rows, j, i) }
}

// Sort sorts a Table t (in place) by the given order.
func Sort(rows []Row, order Order) {
	sort.SliceStable(rows, func(i, j int) bool { return order(rows, i, j) })
}
