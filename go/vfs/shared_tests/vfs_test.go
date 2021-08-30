package shared_tests

// This file contains tests which would go in go/vfs except that they use the
// mocks in go/vfs/mocks, which depends on go/vfs and thus would create a cycle.

import (
	"context"
	"io"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vfs"
	"go.skia.org/infra/go/vfs/mocks"
)

func TestReuseContextFile(t *testing.T) {
	unittest.SmallTest(t)

	// Setup.
	origFile := &mocks.File{}
	key := &struct{}{}
	val := &struct{}{}
	ctx := context.WithValue(context.Background(), key, val)
	wrapFile := vfs.WithContext(ctx, origFile)
	var buf []byte

	// Mock calls to the underlying File which match the Context passed to
	// WithContext.
	origFile.On("Read", ctx, buf).Return(0, nil)
	origFile.On("Stat", ctx).Return(nil, nil)
	origFile.On("Close", ctx).Return(nil)

	// Call the associated methods on wrapFile.
	_, err := wrapFile.Read(buf)
	require.NoError(t, err)
	_, err = wrapFile.Stat()
	require.NoError(t, err)
	err = wrapFile.Close()
	require.NoError(t, err)
	origFile.AssertExpectations(t)
}

func TestReadFile(t *testing.T) {
	unittest.SmallTest(t)

	ctx := context.Background()
	name := "myfile.txt"
	fs := &mocks.FS{}
	f := &mocks.File{}
	fs.On("Open", ctx, name).Return(f, nil)
	contents := []uint8("hello world")
	// Note: this only works because our input is smaller than the buffer size
	// used by ioutil.ReadAll.
	f.On("Read", ctx, mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(1).([]uint8)
		copy(arg, contents)
	}).Return(len(contents), io.EOF)
	f.On("Close", ctx).Return(nil)

	result, err := vfs.ReadFile(ctx, fs, name)
	require.NoError(t, err)
	require.Equal(t, contents, result)
	fs.AssertExpectations(t)
	f.AssertExpectations(t)
}

func TestReadDir(t *testing.T) {
	unittest.SmallTest(t)

	ctx := context.Background()
	name := "mydir"
	fs := &mocks.FS{}
	dir := &mocks.File{}
	contents := []os.FileInfo{
		vfs.FileInfo{
			Name:    "myfile.txt",
			Size:    128,
			Mode:    os.ModePerm,
			ModTime: time.Now(),
			IsDir:   false,
			Sys:     nil,
		}.Get(),
		vfs.FileInfo{
			Name:    "subdir",
			Size:    0,
			Mode:    os.ModePerm | os.ModeDir,
			ModTime: time.Now(),
			IsDir:   true,
			Sys:     nil,
		}.Get(),
	}
	fs.On("Open", ctx, name).Return(dir, nil)
	dir.On("ReadDir", ctx, -1).Return(contents, nil)
	dir.On("Close", ctx).Return(nil)

	result, err := vfs.ReadDir(ctx, fs, name)
	require.NoError(t, err)
	assertdeep.Equal(t, contents, result)
	fs.AssertExpectations(t)
	dir.AssertExpectations(t)
}

func TestStat(t *testing.T) {
	unittest.SmallTest(t)

	ctx := context.Background()
	fs := &mocks.FS{}
	dir, dirFi := mockStat(ctx, fs, "myDir", true)

	result, err := vfs.Stat(ctx, fs, dirFi.Name())
	require.NoError(t, err)
	assertdeep.Equal(t, dirFi, result)
	fs.AssertExpectations(t)
	dir.AssertExpectations(t)
}

func TestWalk(t *testing.T) {
	unittest.SmallTest(t)

	ctx := context.Background()
	fs := &mocks.FS{}

	rootDir, rootDirFi := mockStat(ctx, fs, "root", true)
	_, rootFileFi := mockStat(ctx, fs, path.Join(rootDirFi.Name(), "rootFile"), false)
	subDir, subDirFi := mockStat(ctx, fs, path.Join(rootDirFi.Name(), "subdir"), true)
	rootDir.On("ReadDir", ctx, -1).Return([]os.FileInfo{subDirFi, rootFileFi}, nil)
	_, subDirFileFi := mockStat(ctx, fs, path.Join(rootDirFi.Name(), subDirFi.Name(), "subdirFile"), false)
	subDir.On("ReadDir", ctx, -1).Return([]os.FileInfo{subDirFileFi}, nil)

	expect := map[string]os.FileInfo{
		rootDirFi.Name(): rootDirFi,
		path.Join(rootDirFi.Name(), rootFileFi.Name()):                    rootFileFi,
		path.Join(rootDirFi.Name(), subDirFi.Name()):                      subDirFi,
		path.Join(rootDirFi.Name(), subDirFi.Name(), subDirFileFi.Name()): subDirFileFi,
	}
	visited := map[string]bool{}
	walkFn := func(name string, fi os.FileInfo, err error) error {
		require.NoError(t, err)
		assertdeep.Equal(t, expect[name], fi)
		visited[name] = true
		return nil
	}
	require.NoError(t, vfs.Walk(ctx, fs, rootDirFi.Name(), walkFn))
	require.Equal(t, len(expect), len(visited))

	// TODO(borenet): Test filepath.SkipDir.
}

func mockStat(ctx context.Context, fs *mocks.FS, fullPath string, isDir bool) (*mock.Mock, *vfs.FileInfoImpl) {
	fi := vfs.FileInfo{
		Name:    filepath.Base(fullPath),
		Size:    128,
		Mode:    os.ModePerm,
		ModTime: time.Now(),
		IsDir:   isDir,
		Sys:     nil,
	}.Get()
	var file vfs.File
	var m *mock.Mock
	if isDir {
		fi.FileInfo.Mode = fi.FileInfo.Mode | os.ModeDir
		dir := &mocks.File{}
		m = &dir.Mock
		file = dir
	} else {
		f := &mocks.File{}
		m = &f.Mock
		file = f
	}
	fs.On("Open", ctx, fullPath).Return(file, nil)
	m.On("Stat", ctx).Return(fi, nil)
	m.On("Close", ctx).Return(nil)
	return m, fi
}
