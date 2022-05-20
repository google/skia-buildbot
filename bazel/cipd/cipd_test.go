package cipd_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/bazel/cipd/git"
	"go.skia.org/infra/bazel/cipd/vpython"
)

func TestFindGit(t *testing.T) {
	path, err := git.FindGit()
	require.NoError(t, err)
	assertFileExists(t, path)
}

func TestFindVPython3(t *testing.T) {
	path, err := vpython.FindVPython3()
	require.NoError(t, err)
	assertFileExists(t, path)
}

func assertFileExists(t *testing.T, path string) {
	fileInfo, err := os.Stat(path)
	require.NoError(t, err)
	assert.False(t, fileInfo.IsDir())
	assert.NotZero(t, fileInfo.Size())
}
