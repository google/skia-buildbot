package mocks

import (
	fs "io/fs"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vfs"
)

func TestMockFS(t *testing.T) {
	myFS := NewFS(t)
	SetupMockFS(t, myFS, map[string]string{
		"root-file.txt":        "contents",
		"subdir/":              "",
		"subdir2/sub-file.txt": "contents2",
	})
	actualFiles := []string{}
	actualDirs := []string{}
	err := vfs.Walk(t.Context(), myFS, ".", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return skerr.Wrapf(err, "failed walk %q", path)
		}
		if info.IsDir() {
			actualDirs = append(actualDirs, path)
		} else {
			actualFiles = append(actualFiles, path)
		}
		return nil
	})
	require.NoError(t, err)

	require.Equal(t, []string{"root-file.txt", "subdir2/sub-file.txt"}, actualFiles)
	require.Equal(t, []string{".", "subdir", "subdir2"}, actualDirs)
}
