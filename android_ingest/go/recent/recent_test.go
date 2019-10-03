package recent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestRecent(t *testing.T) {
	unittest.SmallTest(t)
	r := New()
	good, bad := r.List()
	assert.Len(t, good, 0)
	assert.Len(t, bad, 0)

	r.AddGood([]byte("{}"))
	good, bad = r.List()
	assert.Len(t, good, 1)
	assert.Len(t, bad, 0)

	r.AddBad([]byte("{"))
	good, bad = r.List()
	assert.Len(t, good, 1)
	assert.Len(t, bad, 1)

	json := "{\"foo\": 2}"
	r.AddGood([]byte(json))
	good, bad = r.List()
	assert.Len(t, good, 2)
	assert.Len(t, bad, 1)
	// Confirm that new additions show up at the beginning
	// of the list.
	assert.Equal(t, json, good[0].JSON)

	// Confirm that we never store more than MAX_RECENT entries.
	for i := 0; i < MAX_RECENT+5; i++ {
		r.AddGood([]byte("{}"))
	}
	assert.Len(t, r.recentGood, MAX_RECENT)
}
