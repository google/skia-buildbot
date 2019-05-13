package recent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestRecent(t *testing.T) {
	unittest.SmallTest(t)
	r := New()
	list := r.List()
	assert.Len(t, list, 0)

	r.Add([]byte("{}"))
	list = r.List()
	assert.Len(t, list, 1)

	json := "{\"foo\": 2}"
	r.Add([]byte(json))
	list = r.List()
	assert.Len(t, list, 2)
	// Confirm that new additions show up at the beginning
	// of the list.
	assert.Equal(t, json, list[0].JSON)

	// Confirm that we never store more than MAX_RECENT entries.
	for i := 0; i < MAX_RECENT+5; i++ {
		r.Add([]byte("{}"))
	}
	assert.Len(t, r.recent, MAX_RECENT)
}
