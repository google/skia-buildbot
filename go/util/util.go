package util

import (
	"crypto/rand"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/skia-dev/glog"
)

const (
	_          = iota // ignore first value by assigning to blank identifier
	KB float64 = 1 << (10 * iota)
	MB
	GB
	TB
	PB
)

// GetFormattedByteSize returns a formatted pretty string representation of the
// provided byte size. Eg: Input of 1024 would return "1.00KB".
func GetFormattedByteSize(b float64) string {
	switch {
	case b >= PB:
		return fmt.Sprintf("%.2fPB", b/PB)
	case b >= TB:
		return fmt.Sprintf("%.2fTB", b/TB)
	case b >= GB:
		return fmt.Sprintf("%.2fGB", b/GB)
	case b >= MB:
		return fmt.Sprintf("%.2fMB", b/MB)
	case b >= KB:
		return fmt.Sprintf("%.2fKB", b/KB)
	}
	return fmt.Sprintf("%.2fB", b)
}

// In returns true if |s| is *in* |a| slice.
func In(s string, a []string) bool {
	for _, x := range a {
		if x == s {
			return true
		}
	}
	return false
}

// AtMost returns a subslice of at most the first n members of a.
func AtMost(a []string, n int) []string {
	if n > len(a) {
		n = len(a)
	}
	return a[:n]
}

func SSliceEqual(a, b []string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	sort.Strings(a)
	sort.Strings(b)
	for i, aa := range a {
		if aa != b[i] {
			return false
		}
	}
	return true
}

type int64Slice []int64

func (p int64Slice) Len() int           { return len(p) }
func (p int64Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p int64Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Int64Equal returns true if the int64 slices are equal.
func Int64Equal(a, b []int64) bool {
	sort.Sort(int64Slice(a))
	sort.Sort(int64Slice(b))
	if len(a) != len(b) {
		return false
	}
	for i, x := range a {
		if x != b[i] {
			return false
		}
	}
	return true
}

// MapsEqual checks if the two maps are equal.
func MapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	// Since they are the same size we only need to check from one side, i.e.
	// compare a's values to b's values.
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

// MaxInt returns largest integer of a and b.
func MaxInt(a, b int) int {
	if a < b {
		return b
	}
	return a
}

// MinInt returns the smaller integer of a and b.
func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// AbsInt returns the absolute value of v.
func AbsInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// SignInt returns -1, 1 or 0 depending on the sign of v.
func SignInt(v int) int {
	if v < 0 {
		return -1
	}
	if v > 0 {
		return 1
	}
	return 0
}

// Returns the current time in milliseconds since the epoch.
func TimeStampMs() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

// Generate a 16-byte random ID.
func GenerateID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%X", b), nil
}

// IntersectIntSets calculates the intersection of a list
// of integer sets.
func IntersectIntSets(sets []map[int]bool, minIdx int) map[int]bool {
	resultSet := make(map[int]bool, len(sets[minIdx]))
	for val := range sets[minIdx] {
		resultSet[val] = true
	}

	for _, oneSet := range sets {
		for k := range resultSet {
			if !oneSet[k] {
				delete(resultSet, k)
			}
		}
	}

	return resultSet
}

// KeysOfStringSet returns the keys of a set of strings represented by the keys
// of a map.
func KeysOfStringSet(set map[string]bool) []string {
	ret := make([]string, 0, len(set))
	for v := range set {
		ret = append(ret, v)
	}

	return ret
}

// KeysOfIntSet returns the keys of a set of strings represented by the keys
// of a map.
func KeysOfIntSet(set map[int]bool) []int {
	ret := make([]int, 0, len(set))
	for v := range set {
		ret = append(ret, v)
	}
	return ret
}

// UnionStrings returns a union of all unique strings in the input slices.
func UnionStrings(lists ...[]string) []string {
	result := map[string]bool{}
	for _, set := range lists {
		for _, val := range set {
			result[val] = true
		}
	}
	return KeysOfStringSet(result)
}

// RepeatJoin repeats a given string N times with the given separator between
// each instance.
func RepeatJoin(str, sep string, n int) string {
	if n <= 0 {
		return ""
	}
	return str + strings.Repeat(sep+str, n-1)
}

func AddParamsToParamSet(a map[string][]string, b map[string]string) map[string][]string {
	for k, v := range b {
		// You might be tempted to replace this with
		// sort.SearchStrings(), but that's actually slower for short
		// slices. The breakpoint seems to around 50, and since most
		// of our ParamSet lists are short that ends up being slower.
		if _, ok := a[k]; !ok {
			a[k] = []string{v}
		} else if !In(v, a[k]) {
			a[k] = append(a[k], v)
		}
	}
	return a
}

// KeysOfParamSet returns the keys of a param set.
func KeysOfParamSet(set map[string][]string) []string {
	ret := make([]string, 0, len(set))
	for v := range set {
		ret = append(ret, v)
	}

	return ret
}

// Close wraps an io.Closer and logs an error if one is returned.
func Close(c io.Closer) {
	if err := c.Close(); err != nil {
		glog.Errorf("Failed to Close(): %v", err)
	}
}
