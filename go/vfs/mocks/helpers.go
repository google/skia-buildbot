package mocks

import (
	"io"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vfs"
)

// SetupMockFS takes a map of file paths to their contents and sets up all
// required VFS mocks (for Open, Stat, Close, ReadDir, and Read).
// Paths ending in "/" are treated as empty directories and their contents are
// ignored.
func SetupMockFS(t *testing.T, fs *FS, files map[string]string) {
	dirPathToChildren := map[string][]string{}
	isDirectory := map[string]bool{}

	for filePath := range files {
		current := filePath
		for {
			parent, base := path.Split(current)
			parent = path.Clean(parent)
			if parent == base {
				break
			}
			isDirectory[parent] = true

			if base == "" {
				// This is an empty directory.
				if _, ok := dirPathToChildren[parent]; !ok {
					dirPathToChildren[parent] = []string{}
				}
			} else {
				exists := false
				for _, child := range dirPathToChildren[parent] {
					if child == base {
						exists = true
						break
					}
				}
				if !exists {
					dirPathToChildren[parent] = append(dirPathToChildren[parent], base)
				}
			}
			current = parent
		}
	}

	for dirPath, children := range dirPathToChildren {
		dirFile := NewFile(t)
		fiDir := vfs.FileInfo{
			Name:    path.Base(dirPath),
			IsDir:   true,
			Size:    128,
			Mode:    os.ModeDir | os.ModePerm,
			ModTime: time.Now(),
		}.Get()

		fs.On("Open", testutils.AnyContext, dirPath).Return(dirFile, nil).Maybe()
		dirFile.On("Stat", testutils.AnyContext).Return(fiDir, nil).Maybe()
		dirFile.On("Close", testutils.AnyContext).Return(nil).Maybe()

		var childInfos []os.FileInfo
		for _, childBase := range children {
			childPath := path.Join(dirPath, childBase)
			childIsDir := isDirectory[childPath]
			var mode os.FileMode = os.ModePerm
			if childIsDir {
				mode = mode | os.ModeDir
			}
			childSize := int64(128)
			if !childIsDir {
				childSize = int64(len(files[childPath]))
			}
			fiChild := vfs.FileInfo{
				Name:    childBase,
				IsDir:   childIsDir,
				Size:    childSize,
				Mode:    mode,
				ModTime: time.Now(),
			}.Get()
			childInfos = append(childInfos, fiChild)
		}
		dirFile.On("ReadDir", testutils.AnyContext, -1).Return(childInfos, nil).Maybe()
	}

	for filePath, content := range files {
		fileMock := NewFile(t)
		fiFile := vfs.FileInfo{
			Name:    path.Base(filePath),
			IsDir:   false,
			Size:    int64(len(content)),
			Mode:    os.ModePerm,
			ModTime: time.Now(),
		}.Get()

		fs.On("Open", testutils.AnyContext, filePath).Return(fileMock, nil).Maybe()
		fileMock.On("Stat", testutils.AnyContext).Return(fiFile, nil).Maybe()
		fileMock.On("Close", testutils.AnyContext).Return(nil).Maybe()

		data := []byte(content)
		fileMock.On("Read", testutils.AnyContext, mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
			buf := args.Get(1).([]uint8)
			copy(buf, data)
		}).Return(len(data), io.EOF).Maybe()
	}

	// Catch-all: anything not specified above should return os.ErrNotExist.
	fs.On("Open", testutils.AnyContext, mock.Anything).Return(nil, os.ErrNotExist).Maybe()
}
