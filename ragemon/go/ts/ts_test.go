package ts

import (
	"math"
	"testing"
	"time"

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

	for i := 0; i < 20; i++ {
		ts.Add(Point{
			Timestamp: now.Add(time.Duration(i*60) * time.Second).Unix(),
			Value:     int64(24 * i),
		})
	}
	b, err = ts.Bytes()
	assert.NoError(t, err)
	assert.Equal(t, 48, len(b)) // Better than the 22*16=352 for a naive encoding.
}

func TestRoundTrip(t *testing.T) {
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
	_, _, err := decode([]byte{})
	assert.Error(t, err)

	_, err = NewFromData([]byte{})
	assert.Error(t, err)

	_, err = NewFromData([]byte("a"))
	assert.Error(t, err)
}
