package gs

import (
	"testing"
	"time"
)

// compareStringSlices compares two string slices, and returns true iff the
// contents and sequence of the two slices are identical.
func compareStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestGetLatestGSDirs(t *testing.T) {
	startTS := time.Date(1970, time.November, 29, 13, 45, 20, 67, time.UTC).Unix()
	endTS := time.Date(1972, time.February, 2, 3, 45, 20, 67, time.UTC).Unix()
	results := GetLatestGSDirs(startTS, endTS, "prefix")
	expected := []string{
		"prefix/1970/11/29",
		"prefix/1970/11/30",
		"prefix/1970/12",
		"prefix/1971",
		"prefix/1972/01",
		"prefix/1972/02/01",
		"prefix/1972/02/02/00",
		"prefix/1972/02/02/01",
		"prefix/1972/02/02/02",
		"prefix/1972/02/02/03",
	}
	if !compareStringSlices(results, expected) {
		t.Errorf("GetLatestGSDirs unexpected results! Got:\n%s\nWant:\n%s", results, expected)
	}
}
