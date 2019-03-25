package time_window

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

func TestTimeWindow(t *testing.T) {
	testutils.SmallTest(t)

	// Assert that the given string parses with no error.
	P := func(s string) *TimeWindow {
		w, err := Parse(s)
		assert.NoError(t, err)
		assert.NotNil(t, w)
		return w
	}
	// Assert that the given string fails to parse with the given error.
	F := func(s, expect string) {
		w, err := Parse(s)
		assert.EqualError(t, err, expect)
		assert.Nil(t, w)
	}
	// Assert that the given string parses with no error, and assert that
	// the given result is returned by Test for the given Time.
	PT := func(s string, expect bool, ts time.Time) {
		assert.Equal(t, expect, P(s).Test(ts))
	}

	// Test parsing.
	P("* 00:00:00-23:59:59")
	P("M-F 00:00:01-16:22:00")
	P("M-W,Sa-Su 16:00:00-17:00:00")
	P("M,W,F 05:23:00-08:45:19,09:37:26-12:12:10")
	P("M,W,F 05:23:00-08:45:19, 09:37:26-12:12:10")
	P("M,W,F 05:23:00-08:45:19; Tu,Th 09:37:26-12:12:10")
	F("blahblah", "Expected format \"D hh:mm:ss\", not \"blahblah\"")
	F("M,W,D 00:00:00-00:00:01", "Unknown day \"D\"")
	F("* 00:00:00", "Expected window format \"hh:mm:ss-hh:mm:ss\", not \"00:00:00\"")
	F("* 00:00:00-24:00:00", "Hours must be between 0-23, not 24")
	F("* 00:00:00-00:60:00", "Minutes must be between 0-59, not 60")
	F("* 00:00:00-00:00:60", "Seconds must be between 0-59, not 60")
	F("* 00:00:01-00:00:00", "Window end time must be after start time.")

	// Verify that we include/exclude the correct times.
	baseDate := time.Date(2019, 3, 24, 0, 0, 0, 0, time.UTC) // This is a Sunday, which has index 0.
	getTs := func(d time.Weekday, h, m, s int) time.Time {
		return baseDate.Add(time.Duration(24*int(d)+h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(s)*time.Second)
	}

	expr := "* 00:00:00-12:00:00"
	for _, day := range dayMap {
		PT(expr, true, getTs(day, 0, 0, 0))
		PT(expr, true, getTs(day, 11, 59, 59))
		PT(expr, false, getTs(day, 12, 0, 0))
	}

	expr = "M-F 00:00:00-23:59:59"
	for _, day := range dayMap {
		expect := day >= 1 && day <= 5
		PT(expr, expect, getTs(day, 0, 0, 0))
	}

	expr = "F 22:00:00-23:00:00"
	for _, day := range dayMap {
		expect := day == time.Friday
		PT(expr, expect, getTs(day, 22, 30, 0))
	}

	// A nil TimeWindow always returns true from Test.
	assert.Equal(t, true, (*TimeWindow)(nil).Test(time.Now()))
}
