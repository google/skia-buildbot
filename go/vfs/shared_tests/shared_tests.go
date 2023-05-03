package shared_tests

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/vfs"
)

const (
	FakeFileName = "rootFile"
)

var (
	FakeContents = []byte(`hello world`)
	FakeFileInfo = vfs.FileInfo{
		Name:    FakeFileName,
		Size:    int64(len(FakeContents)),
		Mode:    0666,
		ModTime: time.Time{},
		IsDir:   false,
		Sys:     nil,
	}.Get()
)

// TestFS tests a vfs.FS implementation. It expects the passed-in FS to have the
// following structure:
//
//	./
//		subDir/
//			subDirFile => "subDirFile contents"
//		rootFile => FakeContents
func TestFS(ctx context.Context, t sktest.TestingT, fs vfs.FS) {
	defer func() {
		require.NoError(t, fs.Close(ctx))
	}()
	rootDir, err := fs.Open(ctx, ".")
	require.NoError(t, err)
	require.NotNil(t, rootDir)
	defer func() {
		require.NoError(t, rootDir.Close(ctx))
	}()

	// Stat.
	fi, err := rootDir.Stat(ctx)
	require.NoError(t, err)
	require.NotNil(t, fi)
	require.Equal(t, true, fi.IsDir())
	require.Equal(t, os.ModeDir, fi.Mode()&os.ModeDir)

	// ReadDir.
	contents, err := rootDir.ReadDir(ctx, -1)
	require.NoError(t, err)
	require.Equal(t, 2, len(contents))
	// Fix ordering if necessary.
	if contents[0].Name() == FakeFileName {
		contents[0], contents[1] = contents[1], contents[0]
	}
	require.Equal(t, "subdir", contents[0].Name())
	require.True(t, contents[0].IsDir())
	require.Equal(t, os.ModeDir, contents[0].Mode()&os.ModeDir)
	require.Equal(t, FakeFileName, contents[1].Name())
	require.False(t, contents[1].IsDir())
	require.Equal(t, os.FileMode(0), contents[1].Mode()&os.ModeDir)

	// Read.
	rootFile, err := fs.Open(ctx, FakeFileName)
	require.NoError(t, err)
	rootFileContents, err := ioutil.ReadAll(vfs.WithContext(ctx, rootFile))
	require.NoError(t, err)
	require.Equal(t, []byte(FakeContents), rootFileContents)
	st, err := rootFile.Stat(ctx)
	require.NoError(t, err)
	// Check that the mode is the same. We already know the content and name are the same, so
	// everything else should be ok too. Asserting other things, like access/creation time can be
	// a little flaky, so we do not do that.
	require.Equal(t, contents[1].Mode(), st.Mode())
	require.NoError(t, rootFile.Close(ctx))

	// Walk.
	visited := map[string]bool{}
	require.NoError(t, vfs.Walk(ctx, fs, ".", func(fp string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		visited[fp] = true
		return nil
	}))
	require.True(t, visited["."])
	require.True(t, visited[FakeFileName])
	require.True(t, visited["subdir"])
	require.True(t, visited[path.Join("subdir", "subDirFile")])
}

// MakeTestFiles creates a temporary directory containing the files and
// directories expected by TestFS.
func MakeTestFiles(t sktest.TestingT) string {
	tmp := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "subdir"), os.ModePerm))
	require.NoError(t, ioutil.WriteFile(filepath.Join(tmp, FakeFileName), FakeContents, os.ModePerm))
	require.NoError(t, ioutil.WriteFile(filepath.Join(tmp, "subdir", "subDirFile"), []byte("subDirFile contents"), os.ModePerm))
	return tmp
}

func TestVFS_ReadOnly(t sktest.TestingT, fs vfs.FS) {
	ctx := context.Background()
	contents, err := vfs.ReadFile(ctx, fs, FakeFileName)
	require.NoError(t, err)
	require.Equal(t, FakeContents, contents)
}

func TestVFS_ReadWrite(t sktest.TestingT, fs vfs.FS) {
	ctx := context.Background()
	contents, err := vfs.ReadFile(ctx, fs, FakeFileName)
	require.NoError(t, err)
	require.Equal(t, FakeContents, contents)
	newContent := []byte("new contents")
	require.NoError(t, vfs.WriteFile(ctx, fs, FakeFileName, newContent))
	contents, err = vfs.ReadFile(ctx, fs, FakeFileName)
	require.NoError(t, err)
	require.Equal(t, newContent, contents)
}

func TestVFS_MultiWrite_ChangedToOriginal(t sktest.TestingT, fs vfs.FS) {
	ctx := context.Background()
	newContent := []byte("new contents")
	require.NoError(t, vfs.WriteFile(ctx, fs, FakeFileName, newContent))
	contents, err := vfs.ReadFile(ctx, fs, FakeFileName)
	require.NoError(t, err)
	require.Equal(t, newContent, contents)
	require.NoError(t, vfs.WriteFile(ctx, fs, FakeFileName, FakeContents))
	contents, err = vfs.ReadFile(ctx, fs, FakeFileName)
	require.NoError(t, err)
	require.Equal(t, FakeContents, contents)
}
