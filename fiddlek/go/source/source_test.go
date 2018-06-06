package source

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
)

func TestNew(t *testing.T) {
	testutils.SmallTest(t)
	s, err := New("../../source")
	assert.NoError(t, err)
	assert.True(t, len(s.thumbnails) > 5)
}
