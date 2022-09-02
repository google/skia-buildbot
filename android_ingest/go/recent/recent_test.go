package recent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRecent(t *testing.T) {
	r := New()
	good, bad := r.List()
	assert.Len(t, good, 0)
	assert.Len(t, bad, 0)

	r.AddGood([]byte("{}"))
	good, bad = r.List()
	assert.Len(t, good, 1)
	assert.Len(t, bad, 0)

	r.AddBad([]byte("{"), "malformed")
	good, bad = r.List()
	assert.Len(t, good, 1)
	assert.Len(t, bad, 1)
	assert.Equal(t, "{", bad[0].JSON)

	r.AddBad([]byte("{\"foo\": 2}"), "missing data")
	good, bad = r.List()
	assert.Len(t, good, 1)
	assert.Len(t, bad, 2)
	assert.Equal(t, "{\n  \"foo\": 2\n}", bad[0].JSON)
	assert.Equal(t, "missing data", bad[0].Reason)

	json := "{\"foo\": 2}"
	r.AddGood([]byte(json))
	good, bad = r.List()
	assert.Len(t, good, 2)
	assert.Len(t, bad, 2)
	// Confirm that new additions show up at the beginning
	// of the list.
	assert.Equal(t, json, good[0].JSON)

	// Confirm that we never store more than MAX_RECENT entries.
	for i := 0; i < maxRecent+5; i++ {
		r.AddGood([]byte("{}"))
	}
	assert.Len(t, r.recentGood, maxRecent)
}
