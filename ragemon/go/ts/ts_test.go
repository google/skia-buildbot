package ts

import (
	"math"
	"reflect"
	"testing"
	"time"

	"go.skia.org/infra/go/testutils"

	"github.com/stretchr/testify/assert"
)

func roundTrip(t *testing.T, pts []Point) {
	ts := New(pts[0])
	for _, p := range pts[1:] {
		ts.Add(p)
	}
	b, err := ts.Bytes()
	assert.NoError(t, err)
	ts_out, err := NewFromData(b)
	assert.NoError(t, err)
	assert.Equal(t, pts, ts_out.data)
}

func TestTS(t *testing.T) {
	testutils.SmallTest(t)
	now := time.Unix(1471877350, 0)
	ts := New(Point{
		Timestamp: now.Unix(),
		Value:     10,
	})
	ts.Add(Point{
		Timestamp: now.Add(15 * time.Second).Unix(),
		Value:     -12,
	})
	b, err := ts.Bytes()
	assert.NoError(t, err)
	assert.Equal(t, 8, len(b)) // Better than the 32 for a naive encoding.

	for i := 1; i < 21; i++ {
		ts.Add(Point{
			Timestamp: now.Add(time.Duration(i*60) * time.Second).Unix(),
			Value:     int64(24 * i),
		})
	}
	b, err = ts.Bytes()
	assert.NoError(t, err)
	assert.Equal(t, 48, len(b)) // Better than the 22*16=352 for a naive encoding.
}

func TestAddWrongOrder(t *testing.T) {
	testutils.SmallTest(t)
	ts := New(Point{
		Timestamp: 140,
		Value:     10,
	})
	ts.Add(Point{
		Timestamp: 130,
		Value:     10,
	})
	assert.Equal(t, 1, len(ts.data))
}

func TestRoundTrip(t *testing.T) {
	testutils.SmallTest(t)
	now := time.Unix(1471877350, 0)
	roundTrip(t, []Point{
		Point{
			Timestamp: now.Unix(),
			Value:     10,
		},
		Point{
			Timestamp: now.Add(15 * time.Second).Unix(),
			Value:     -12,
		},
		Point{
			Timestamp: now.Add(30 * time.Second).Unix(),
			Value:     -13,
		},
	})

	roundTrip(t, []Point{
		Point{
			Timestamp: now.Unix(),
			Value:     -12,
		},
		Point{
			Timestamp: now.Add(15 * time.Second).Unix(),
			Value:     314e10,
		},
		Point{
			Timestamp: now.Add(30 * time.Second).Unix(),
			Value:     -12,
		},
	})

	roundTrip(t, []Point{
		Point{
			Timestamp: now.Unix(),
			Value:     0,
		},
		Point{
			Timestamp: now.Add(15 * time.Second).Unix(),
			Value:     math.MaxInt64 - 1,
		},
		Point{
			Timestamp: now.Add(30 * time.Second).Unix(),
			Value:     math.MinInt64 + 1,
		},
		Point{
			Timestamp: now.Add(45 * time.Second).Unix(),
			Value:     math.MaxInt64 - 1,
		},
	})

}

func TestErrors(t *testing.T) {
	testutils.SmallTest(t)
	_, _, err := decode([]byte{})
	assert.Error(t, err)

	_, err = NewFromData([]byte{})
	assert.Error(t, err)

	_, err = NewFromData([]byte("a"))
	assert.Error(t, err)
}

func TestRange(t *testing.T) {
	testutils.SmallTest(t)
	now := int64(1471877350)
	pt := Point{
		Timestamp: now,
		Value:     10,
	}

	assert.True(t, inRange(now, now+1, pt), "[a,a+delta) should match.")
	assert.True(t, inRange(now-1, now+1, pt), "[a-s,a+s) should match.")
	assert.False(t, inRange(now, now, pt), "[a,a) shouldn't match anything.")
	assert.False(t, inRange(now-1, now, pt), "[a-delta,a) shouldn't match a pt at 'a'.")
	assert.False(t, inRange(now+1, now-1, pt), "[a,b) where b<a shouldn't match anything.")
}

func TestInRange(t *testing.T) {
	testutils.SmallTest(t)
	now := int64(1471877350)
	series1 := &TimeSeries{
		data: []Point{
			Point{
				Timestamp: now,
				Value:     10,
			},
			Point{
				Timestamp: now + 15,
				Value:     20,
			},
			Point{
				Timestamp: now + 30,
				Value:     31,
			},
			Point{
				Timestamp: now + 45,
				Value:     42,
			},
		},
	}

	series2 := &TimeSeries{
		data: []Point{
			Point{
				Timestamp: now,
				Value:     10,
			},
		},
	}

	testCases := []struct {
		series   *TimeSeries
		begin    int64
		end      int64
		expected []Point
	}{
		// series1
		{
			series:   series1,
			begin:    now,
			end:      now,
			expected: []Point{},
		},
		{
			series:   series1,
			begin:    now,
			end:      now + 1,
			expected: series1.data[:1],
		},
		{
			series:   series1,
			begin:    now + 1,
			end:      now + 16,
			expected: series1.data[1:2],
		},
		{
			series:   series1,
			begin:    now + 1,
			end:      now + 30,
			expected: series1.data[1:2],
		},
		{
			series:   series1,
			begin:    now + 1,
			end:      now + 31,
			expected: series1.data[1:3],
		},
		{
			series:   series1,
			begin:    now,
			end:      now + 16,
			expected: series1.data[:2],
		},
		{
			series:   series1,
			begin:    now + 60,
			end:      now + 61,
			expected: []Point{},
		},
		{
			series:   series1,
			begin:    now - 2,
			end:      now - 1,
			expected: []Point{},
		},
		{
			series:   series1,
			begin:    now,
			end:      now + 46,
			expected: series1.data,
		},

		// series2
		{
			series:   series2,
			begin:    now,
			end:      now,
			expected: []Point{},
		},
		{
			series:   series2,
			begin:    now,
			end:      now + 1,
			expected: series2.data[:1],
		},
		{
			series:   series2,
			begin:    now,
			end:      now + 16,
			expected: series2.data[:1],
		},
		{
			series:   series2,
			begin:    now + 60,
			end:      now + 61,
			expected: []Point{},
		},
		{
			series:   series2,
			begin:    now - 2,
			end:      now - 1,
			expected: []Point{},
		},
	}

	for i, tc := range testCases {
		if got, want := tc.series.PointsInRange(tc.begin, tc.end), tc.expected; !reflect.DeepEqual(got, want) {
			t.Errorf("PointsInRange method - Failed case %d series %v for range [%d, %d) Got %v Want %v", i, tc.series.data, tc.begin, tc.end, got, want)
		}
		b, err := tc.series.Bytes()
		if err != nil {
			t.Errorf("Failed case %d to convert series to []byte: %s", i, err)
			continue
		}
		got, err := PointsInRange(tc.begin, tc.end, b)
		if err != nil {
			t.Errorf("Failed case %d to deserialize series from []byte: %s", i, err)
			continue
		}
		if want := tc.expected; !reflect.DeepEqual(got, want) {
			t.Errorf("PointsInRange function - Failed case %d series %v for range [%d, %d) Got %v Want %v", i, tc.series.data, tc.begin, tc.end, got, want)
		}
	}
}
