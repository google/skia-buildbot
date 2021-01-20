package human

import (
	"encoding/json"
	"testing"
	"time"

	"go.skia.org/infra/go/testutils/unittest"
)

func testTickMarks(t *testing.T, ts []int64, expected []*Tick) {
	ticks := TickMarks(ts, time.UTC)
	if got, want := len(ticks), len(expected); got != want {
		t.Fatalf("Wrong length: Got %v Want %v", got, want)
	}
	for i, e := range expected {
		if got, want := ticks[i].X, e.X; got != want {
			t.Errorf("X is wrong: Got %v Want %v", got, want)
		}
		if got, want := ticks[i].Value, e.Value; got != want {
			t.Errorf("Value is wrong: Got %v Want %v", got, want)
		}
	}
}

func TestTickMarks(t *testing.T) {
	unittest.SmallTest(t)
	// Test Months.
	ts := []int64{
		time.Date(2014, 6, 1, 0, 0, 0, 0, time.UTC).Unix(),
		time.Date(2014, 7, 1, 0, 0, 0, 0, time.UTC).Unix(),
		time.Date(2014, 7, 2, 0, 0, 0, 0, time.UTC).Unix(),
		time.Date(2014, 9, 1, 0, 0, 0, 0, time.UTC).Unix(),
	}
	expected := []*Tick{
		{
			X:     0,
			Value: "Jun",
		},
		{
			X:     0.5,
			Value: "Jul",
		},
		{
			X:     2.5,
			Value: "Sep",
		},
	}
	testTickMarks(t, ts, expected)

	// Test Days.
	ts = []int64{
		time.Date(2014, 8, 1, 0, 0, 0, 0, time.UTC).Unix(),
		time.Date(2014, 8, 3, 0, 0, 0, 0, time.UTC).Unix(),
		time.Date(2014, 8, 5, 0, 0, 0, 0, time.UTC).Unix(),
	}
	expected = []*Tick{
		{
			X:     0,
			Value: "1st",
		},
		{
			X:     0.5,
			Value: "3rd",
		},
		{
			X:     1.5,
			Value: "5th",
		},
	}
	testTickMarks(t, ts, expected)

	// Test Hours.
	ts = []int64{
		time.Date(2014, 8, 1, 1, 0, 0, 0, time.UTC).Unix(),
		time.Date(2014, 8, 1, 2, 0, 0, 0, time.UTC).Unix(),
		time.Date(2014, 8, 1, 3, 30, 0, 0, time.UTC).Unix(),
		time.Date(2014, 8, 1, 13, 30, 0, 0, time.UTC).Unix(),
	}
	expected = []*Tick{
		{
			X:     0,
			Value: "1am",
		},
		{
			X:     0.5,
			Value: "2am",
		},
		{
			X:     1.5,
			Value: "3am",
		},
		{
			X:     2.5,
			Value: "1pm",
		},
	}
	testTickMarks(t, ts, expected)

	// Test Minues.
	ts = []int64{
		time.Date(2014, 8, 1, 1, 10, 0, 0, time.UTC).Unix(),
		time.Date(2014, 8, 1, 1, 11, 0, 0, time.UTC).Unix(),
		time.Date(2014, 8, 1, 1, 20, 0, 0, time.UTC).Unix(),
		time.Date(2014, 8, 1, 1, 23, 0, 0, time.UTC).Unix(),
	}
	expected = []*Tick{
		{
			X:     0,
			Value: "01:10am",
		},
		{
			X:     0.5,
			Value: "01:11am",
		},
		{
			X:     1.5,
			Value: "01:20am",
		},
		{
			X:     2.5,
			Value: "01:23am",
		},
	}
	testTickMarks(t, ts, expected)

	// Test Seconds.
	ts = []int64{
		time.Date(2014, 8, 1, 1, 10, 5, 0, time.UTC).Unix(),
		time.Date(2014, 8, 1, 1, 10, 6, 0, time.UTC).Unix(),
		time.Date(2014, 8, 1, 1, 10, 10, 0, time.UTC).Unix(),
		time.Date(2014, 8, 1, 1, 10, 20, 0, time.UTC).Unix(),
	}
	expected = []*Tick{
		{
			X:     0,
			Value: "01:10:05am",
		},
		{
			X:     0.5,
			Value: "01:10:06am",
		},
		{
			X:     1.5,
			Value: "01:10:10am",
		},
		{
			X:     2.5,
			Value: "01:10:20am",
		},
	}
	testTickMarks(t, ts, expected)

	// Test Weekdays.
	ts = []int64{
		time.Date(2014, 8, 1, 1, 0, 0, 0, time.UTC).Unix(),
		time.Date(2014, 8, 2, 1, 0, 0, 0, time.UTC).Unix(),
		time.Date(2014, 8, 3, 1, 0, 0, 0, time.UTC).Unix(),
	}
	expected = []*Tick{
		{
			X:     0,
			Value: "Fri",
		},
		{
			X:     0.5,
			Value: "Sat",
		},
		{
			X:     1.5,
			Value: "Sun",
		},
	}
	testTickMarks(t, ts, expected)

	// Test ToFlot.
	b, err := json.MarshalIndent(ToFlot(TickMarks(ts, time.UTC)), "", "  ")
	if err != nil {
		t.Fatalf("Failed to encode: %s", err)
	}
	got := string(b)
	want := `[
  [
    0,
    "Fri"
  ],
  [
    0.5,
    "Sat"
  ],
  [
    1.5,
    "Sun"
  ]
]`

	if got != want {
		t.Errorf("ToFlot failed: Got %v Want %v", got, want)
	}
}

func TestParseDuration(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		s      string
		d      time.Duration
		hasErr bool
	}{
		{
			s:      "",
			d:      time.Duration(0),
			hasErr: true,
		},
		{
			s:      "1minute",
			d:      time.Duration(0),
			hasErr: true,
		},
		{
			s:      "3",
			d:      time.Duration(0),
			hasErr: true,
		},
		{
			s:      "100s",
			d:      100 * time.Second,
			hasErr: false,
		},
		{
			s:      "9m",
			d:      9 * time.Minute,
			hasErr: false,
		},
		{
			s:      "10h",
			d:      10 * time.Hour,
			hasErr: false,
		},
		{
			s:      "2d",
			d:      2 * 24 * time.Hour,
			hasErr: false,
		},
		{
			s:      "52w",
			d:      52 * 7 * 24 * time.Hour,
			hasErr: false,
		},
		{
			s:      "1m5s",
			d:      1*time.Minute + 5*time.Second,
			hasErr: false,
		},
		{
			s:      "2w3d",
			d:      2*7*24*time.Hour + 3*24*time.Hour,
			hasErr: false,
		},
		{
			s:      " 2w 3d ",
			d:      2*7*24*time.Hour + 3*24*time.Hour,
			hasErr: false,
		},
	}

	for idx, tc := range testCases {
		d, err := ParseDuration(tc.s)
		if got, want := d, tc.d; got != want {
			t.Errorf("Wrong duration for %d: Got %v Want %v", idx, got, want)
		}
		if got, want := err != nil, tc.hasErr; got != want {
			t.Errorf("Wrong err status for %d: Got %v Want %v", idx, got, want)
		}
	}
}

func TestDuration(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		expected string
		value    time.Duration
	}{
		{"     0s", 0},
		{"     1s", time.Second},
		{"     1s", -time.Second},
		{"     1m", 60 * time.Second},
		{" 1m  1s", 61 * time.Second},
		{" 1m 59s", 119 * time.Second},
		{"59m 59s", 3599 * time.Second},
		{" 1h  1s", 3601 * time.Second},
		{"23h 59m", 24*60*60*time.Second - time.Second},
		{"     1d", 24 * 60 * 60 * time.Second},
		{" 1d  1s", 24*60*60*time.Second + time.Second},
		{" 1w  1s", 7*24*60*60*time.Second + time.Second},
		{" 1y  1d", 365*24*60*60*time.Second + 24*60*60*time.Second},
	}
	for _, tc := range testCases {
		if got, want := Duration(tc.value), tc.expected; got != want {
			t.Errorf("Failed case Got %q Want %q", got, want)
		}
	}
}
