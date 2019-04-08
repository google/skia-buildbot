package events

import (
	"encoding/binary"
	"math"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
)

func encodeEvent(v float64) []byte {
	rv := make([]byte, 8)
	binary.BigEndian.PutUint64(rv, math.Float64bits(v))
	return rv
}

func decodeEvent(b []byte) float64 {
	return math.Float64frombits(binary.BigEndian.Uint64(b))
}

func testInsertRetrieve(t *testing.T, d EventDB) {
	now := time.Now()
	k1 := now.Add(-3 * time.Second)
	v1 := 0.05
	s := "my-stream"
	assert.NoError(t, d.Insert(&Event{
		Stream:    s,
		Timestamp: k1,
		Data:      encodeEvent(v1),
	}))

	end := now.Add(time.Second)
	start := end.Add(-100 * time.Second)
	vs, err := d.Range(s, start, end)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(vs))
	assert.Equal(t, v1, decodeEvent(vs[0].Data))
}
