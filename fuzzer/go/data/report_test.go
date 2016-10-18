package data

import (
	"reflect"
	"testing"
)

func TestSortedFuzzReports(t *testing.T) {
	a := make(SortedFuzzReports, 0, 5)
	addingOrder := []string{"gggg", "aaaa", "cccc", "eeee", "dddd", "bbbb",
		"ffff"}

	for _, key := range addingOrder {
		a = a.Append(MockReport("skpicture", key))
	}

	b := make(SortedFuzzReports, 0, 5)
	sortedOrder := []string{"aaaa", "bbbb", "cccc", "dddd", "eeee",
		"ffff", "gggg"}

	for _, key := range sortedOrder {
		// just add them in already sorted order
		b = append(b, MockReport("skpicture", key))
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("Expected: %#v\n, but was: %#v", a, b)
	}
}
