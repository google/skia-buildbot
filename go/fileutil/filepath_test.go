package fileutil

import (
	"fmt"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
)

func TestGetHourlyDirs(t *testing.T) {
	testutils.SmallTest(t)
	startTS := time.Date(1972, time.November, 29, 13, 45, 20, 67, time.UTC).Unix()
	endTS := time.Date(1982, time.February, 2, 3, 45, 20, 67, time.UTC).Unix()
	results := GetHourlyDirs("prefix", startTS, endTS)
	assert.True(t, len(results) > 0)

	// Only check the first and the last expected date.
	assert.Equal(t, results[0], "prefix/1972/11/29/13")
	assert.Equal(t, results[len(results)-1], "prefix/1982/02/02/03")

	// Test when the boundary is a full hour.
	startTS = time.Date(1985, time.November, 20, 13, 00, 00, 00, time.UTC).Unix()
	endTS = time.Date(1985, time.November, 20, 15, 00, 00, 00, time.UTC).Unix()
	testFirstLastGetHourly(t, startTS, endTS, "prefix/1985/11/20/13", "prefix/1985/11/20/16")

}

func testFirstLastGetHourly(t *testing.T, startTS, endTS int64, first, last string) {
	ret := GetHourlyDirs("prefix", startTS, endTS)
	fmt.Printf("%v\n", ret)
	assert.Equal(t, first, ret[0])
	assert.Equal(t, last, ret[len(ret)-1])
}

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
