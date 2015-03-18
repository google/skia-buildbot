package human

import (
	"encoding/json"
	"testing"
	"time"
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
		t.Errorf(": Got %v Want %v", got, want)
	}
}

func TestParseDuration(t *testing.T) {
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
	}

	for _, tc := range testCases {
		d, err := ParseDuration(tc.s)
		if got, want := d, tc.d; got != want {
			t.Errorf("Wrong duration: Got %v Want %v", got, want)
		}
		if got, want := err != nil, tc.hasErr; got != want {
			t.Errorf("Wrong err status: Got %v Want %v", got, want)
		}
	}
}
