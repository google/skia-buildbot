package source

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	s, err := New("../../source")
	assert.NoError(t, err)
	assert.True(t, len(s.thumbnails) > 5)
}
