package cockroachdb

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindCockroach(t *testing.T) {
	path, err := FindCockroach()
	require.NoError(t, err)
	assertFileExists(t, path)
}

func assertFileExists(t *testing.T, path string) {
	fileInfo, err := os.Stat(path)
	require.NoError(t, err)
	assert.False(t, fileInfo.IsDir())
	assert.NotZero(t, fileInfo.Size())
}
