package source

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestNew(t *testing.T) {
	unittest.SmallTest(t)
	s, err := New("../../source")
	assert.NoError(t, err)
	assert.True(t, len(s.thumbnails) > 5)
}
