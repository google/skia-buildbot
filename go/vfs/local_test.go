package vfs

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestLocal(t *testing.T) {
	unittest.MediumTest(t)

	ctx := context.Background()
	tmp, cleanup := testutils.TempDir(t)
	defer cleanup()
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "subdir"), os.ModePerm))
	require.NoError(t, ioutil.WriteFile(filepath.Join(tmp, "rootFile"), []byte("rootFile contents"), os.ModePerm))
	require.NoError(t, ioutil.WriteFile(filepath.Join(tmp, "subdir", "subDirFile"), []byte("subDirFile contents"), os.ModePerm))

	fs := Local(tmp)
	file, err := fs.Open(ctx, ".")
	require.NoError(t, err)
	require.NotNil(t, file)

	fi, err := file.Stat(ctx)
	require.NoError(t, err)
	require.NotNil(t, fi)
	require.Equal(t, true, fi.IsDir())
	require.Equal(t, os.ModeDir, fi.Mode()&os.ModeDir)

	_, err = ioutil.ReadAll(WithContext(ctx, file))
	require.Error(t, err)

	dir, ok := fi.(ReadDirFile)
	require.True(t, ok)
	contents, err := dir.ReadDir(ctx, -1)
	require.NoError(t, err)
	require.Equal(t, 2, len(contents))
	// Fix ordering if necessary.
	if contents[0].Name() == "rootFile" {
		contents[0], contents[1] = contents[1], contents[0]
	}
	require.Equal(t, "subdir", contents[0].Name)
	require.True(t, contents[0].IsDir())

	// TODO: finish ReadDir, Read, Close, and Walk.
}
