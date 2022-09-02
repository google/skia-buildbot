package time_window

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTimeWindow(t *testing.T) {

	// Assert that the given string parses with no error.
	P := func(s string) *TimeWindow {
		w, err := Parse(s)
		require.NoError(t, err)
		require.NotNil(t, w)
		return w
	}
	// Assert that the given string fails to parse with the given error.
	F := func(s, expect string) {
		w, err := Parse(s)
		require.EqualError(t, err, expect)
		require.Nil(t, w)
	}
	// Assert that the given string parses with no error, and assert that
	// the given result is returned by Test for the given Time.
	PT := func(s string, expect bool, ts time.Time) {
		require.Equal(t, expect, P(s).Test(ts))
	}

	// Test parsing.
	P("* 00:00-23:59")
	P("M-F 00:01-16:22")
	P("M-W,Sa-Su 16:00-17:00")
	P("M,W,F 05:23-08:45")
	P("M,W,F 05:23- 08:45 ")
	P("M,W,F 05:23-08:45; Tu,Th 09:37-12:12")
	P("* 00:01-00:00") // End time may be before start time; that pushes the end time to the next day.
	F("blahblah", "Expected format \"D hh:mm\", not \"blahblah\"")
	F("M,W,D 00:00-00:01", "Unknown day \"D\"")
	F("* 00:00", "Expected window format \"hh:mm-hh:mm\", not \"00:00\"")
	F("* 00:00-24:00", "Hours must be between 0-23, not 24")
	F("* 00:00-00:60", "Minutes must be between 0-59, not 60")

	// Verify that we include/exclude the correct times.
	baseDate := time.Date(2019, 3, 24, 0, 0, 0, 0, time.UTC) // This is a Sunday, which has index 0.
	getTs := func(d time.Weekday, h, m int) time.Time {
		return baseDate.Add(time.Duration(24*int(d)+h)*time.Hour + time.Duration(m)*time.Minute)
	}

	expr := "* 00:00-12:00"
	for _, day := range dayMap {
		PT(expr, true, getTs(day, 0, 0))
		PT(expr, true, getTs(day, 11, 59))
		PT(expr, false, getTs(day, 12, 0))
	}

	expr = "M-F 00:00-23:59"
	for _, day := range dayMap {
		expect := day >= 1 && day <= 5
		PT(expr, expect, getTs(day, 0, 0))
	}

	expr = "F 22:00-23:00"
	for _, day := range dayMap {
		expect := day == time.Friday
		PT(expr, expect, getTs(day, 22, 30))
	}

	expr = "F-W 00:00-23:59" // Make sure we get the week wraparound right.
	for _, day := range dayMap {
		expect := day != time.Thursday
		PT(expr, expect, getTs(day, 12, 0))
	}

	expr = "M 22:00-02:00" // This one wraps around to Tuesday morning.
	for _, day := range dayMap {
		PT(expr, day == time.Monday, getTs(day, 23, 0))
		PT(expr, day == time.Tuesday, getTs(day, 1, 0))
	}

	// A nil TimeWindow always returns true from Test.
	require.Equal(t, true, (*TimeWindow)(nil).Test(time.Now()))
}
