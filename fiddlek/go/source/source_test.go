package source

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
)

func TestNew(t *testing.T) {

	testdataDir := testutils.TestDataDir(t)
	s, err := New(testdataDir)
	assert.NoError(t, err)
	assert.True(t, len(s.thumbnails) > 5)
}
