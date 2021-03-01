package fileutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestGetHourlyDirs(t *testing.T) {
	unittest.SmallTest(t)
	startTS := time.Date(1972, time.November, 29, 13, 45, 20, 67, time.UTC)
	endTS := time.Date(1982, time.February, 2, 3, 45, 20, 67, time.UTC)
	testFirstLastGetHourly(t, startTS, endTS, "prefix/1972/11/29/13", "prefix/1982/02/02/03")

	// Test when the boundary is exactly on the hour.
	startTS = time.Date(1985, time.November, 20, 13, 00, 00, 00, time.UTC)
	endTS = time.Date(1985, time.November, 20, 15, 00, 00, 00, time.UTC)
	testFirstLastGetHourly(t, startTS, endTS, "prefix/1985/11/20/13", "prefix/1985/11/20/15")

	// Test boundaries within an hour.
	startTS = time.Date(1985, time.November, 20, 13, 00, 00, 00, time.UTC)
	endTS = time.Date(1985, time.November, 20, 13, 01, 00, 00, time.UTC)
	testFirstLastGetHourly(t, startTS, endTS, "prefix/1985/11/20/13")

	// Make sure we get nothing when the endTime is before the start time.
	require.Equal(t, []string{}, GetHourlyDirs("prefix", startTS, startTS.Add(-10*time.Second)))
}

func testFirstLastGetHourly(t *testing.T, startTS, endTS time.Time, firstLast ...string) {
	ret := GetHourlyDirs("prefix", startTS, endTS)
	require.Equal(t, firstLast[0], ret[0])
	if len(firstLast) > 1 {
		require.Equal(t, firstLast[1], ret[len(ret)-1])
	}
}
