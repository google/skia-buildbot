package source

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestNew(t *testing.T) {
	unittest.SmallTest(t)
	s, err := New(filepath.Join(testutils.GetRepoRoot(t), "fiddlek", "source"))
	assert.NoError(t, err)
	assert.True(t, len(s.thumbnails) > 5)
}
