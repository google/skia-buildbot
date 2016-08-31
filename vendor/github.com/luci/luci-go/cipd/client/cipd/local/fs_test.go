// Copyright 2014 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package local

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCwdRelToAbs(t *testing.T) {
	Convey("CwdRelToAbs works", t, func() {
		fs := tempFileSystem()
		p, err := fs.CwdRelToAbs(fs.Root())
		So(err, ShouldBeNil)
		So(p, ShouldEqual, fs.Root())

		p, err = fs.CwdRelToAbs(fs.join("already_abs/def"))
		So(err, ShouldBeNil)
		So(p, ShouldEqual, fs.join("already_abs/def"))

		_, err = fs.CwdRelToAbs(fs.join(".."))
		So(err, ShouldNotBeNil)

		_, err = fs.CwdRelToAbs(fs.join("../.."))
		So(err, ShouldNotBeNil)

		_, err = fs.CwdRelToAbs(fs.join("../abc"))
		So(err, ShouldNotBeNil)
	})
}

func TestRootRelToAbs(t *testing.T) {
	Convey("RootRelToAbs works", t, func() {
		fs := tempFileSystem()

		p, err := fs.RootRelToAbs(".")
		So(err, ShouldBeNil)
		So(p, ShouldEqual, fs.Root())

		p, err = fs.RootRelToAbs(fs.join("already_abs/def"))
		So(err, ShouldBeNil)
		So(p, ShouldEqual, fs.join("already_abs/def"))

		_, err = fs.RootRelToAbs(filepath.FromSlash("abc/../../def"))
		So(err, ShouldNotBeNil)
	})
}

func TestEnsureDirectory(t *testing.T) {
	ctx := context.Background()

	Convey("EnsureDirectory checks root", t, func() {
		fs := tempFileSystem()
		_, err := fs.EnsureDirectory(ctx, fs.join(".."))
		So(err, ShouldNotBeNil)
	})

	Convey("EnsureDirectory works", t, func() {
		fs := tempFileSystem()
		p, err := fs.EnsureDirectory(ctx, fs.join("x/../y/z"))
		So(err, ShouldBeNil)
		So(p, ShouldEqual, fs.join("y/z"))
		So(fs.isDir("y/z"), ShouldBeTrue)
		// Same one.
		_, err = fs.EnsureDirectory(ctx, fs.join("x/../y/z"))
		So(err, ShouldBeNil)
	})
}

func TestEnsureSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows: no symlinks")
	}

	ctx := context.Background()

	Convey("EnsureSymlink checks root", t, func() {
		fs := tempFileSystem()
		So(fs.EnsureSymlink(ctx, fs.join(".."), fs.Root()), ShouldNotBeNil)
	})

	Convey("EnsureSymlink creates new symlink", t, func() {
		fs := tempFileSystem()
		So(fs.EnsureSymlink(ctx, fs.join("symlink"), "target"), ShouldBeNil)
		So(fs.readLink("symlink"), ShouldEqual, "target")
	})

	Convey("EnsureSymlink builds full path", t, func() {
		fs := tempFileSystem()
		So(fs.EnsureSymlink(ctx, fs.join("a/b/c"), "target"), ShouldBeNil)
		So(fs.readLink("a/b/c"), ShouldEqual, "target")
	})

	Convey("EnsureSymlink replaces existing symlink", t, func() {
		fs := tempFileSystem()
		// Replace with same one, then with another one.
		So(fs.EnsureSymlink(ctx, fs.join("symlink"), "target"), ShouldBeNil)
		So(fs.EnsureSymlink(ctx, fs.join("symlink"), "target"), ShouldBeNil)
		So(fs.EnsureSymlink(ctx, fs.join("symlink"), "another"), ShouldBeNil)
		So(fs.readLink("symlink"), ShouldEqual, "another")
	})

	Convey("EnsureSymlink replaces existing file", t, func() {
		fs := tempFileSystem()
		fs.write("path", "blah")
		So(fs.EnsureSymlink(ctx, fs.join("path"), "target"), ShouldBeNil)
		So(fs.readLink("path"), ShouldEqual, "target")
	})

	Convey("EnsureSymlink replaces existing directory", t, func() {
		fs := tempFileSystem()
		fs.write("a/b/c", "something")
		So(fs.EnsureSymlink(ctx, fs.join("a"), "target"), ShouldBeNil)
		So(fs.readLink("a"), ShouldEqual, "target")
	})
}

func TestEnsureFile(t *testing.T) {
	ctx := context.Background()

	Convey("EnsureFile checks root", t, func() {
		fs := tempFileSystem()
		So(EnsureFile(ctx, fs, fs.join(".."), strings.NewReader("blah")), ShouldNotBeNil)
	})

	Convey("EnsureFile creates new file", t, func() {
		fs := tempFileSystem()
		So(EnsureFile(ctx, fs, fs.join("name"), strings.NewReader("blah")), ShouldBeNil)
		So(fs.read("name"), ShouldEqual, "blah")
	})

	Convey("EnsureFile builds full path", t, func() {
		fs := tempFileSystem()
		So(EnsureFile(ctx, fs, fs.join("a/b/c"), strings.NewReader("blah")), ShouldBeNil)
		So(fs.read("a/b/c"), ShouldEqual, "blah")
	})

	if runtime.GOOS != "windows" {
		Convey("EnsureFile replaces existing symlink", t, func() {
			fs := tempFileSystem()
			So(fs.EnsureSymlink(ctx, fs.join("path"), "target"), ShouldBeNil)
			So(EnsureFile(ctx, fs, fs.join("path"), strings.NewReader("blah")), ShouldBeNil)
			So(fs.read("path"), ShouldEqual, "blah")
		})
	}

	Convey("EnsureFile replaces existing file", t, func() {
		fs := tempFileSystem()
		So(EnsureFile(ctx, fs, fs.join("path"), strings.NewReader("huh")), ShouldBeNil)
		So(EnsureFile(ctx, fs, fs.join("path"), strings.NewReader("blah")), ShouldBeNil)
		So(fs.read("path"), ShouldEqual, "blah")
	})

	Convey("EnsureFile replaces existing directory", t, func() {
		fs := tempFileSystem()
		fs.write("a/b/c", "something")
		So(EnsureFile(ctx, fs, fs.join("a"), strings.NewReader("blah")), ShouldBeNil)
		So(fs.read("a"), ShouldEqual, "blah")
	})
}

func TestEnsureFileGone(t *testing.T) {
	ctx := context.Background()

	Convey("EnsureFileGone checks root", t, func() {
		fs := tempFileSystem()
		So(fs.EnsureFileGone(ctx, fs.join("../abc")), ShouldNotBeNil)
	})

	Convey("EnsureFileGone works", t, func() {
		fs := tempFileSystem()
		fs.write("abc", "")
		So(fs.EnsureFileGone(ctx, fs.join("abc")), ShouldBeNil)
		So(fs.isMissing("abc"), ShouldBeTrue)
	})

	Convey("EnsureFileGone works with missing file", t, func() {
		fs := tempFileSystem()
		So(fs.EnsureFileGone(ctx, fs.join("abc")), ShouldBeNil)
	})

	if runtime.GOOS != "windows" {
		Convey("EnsureFileGone works with symlink", t, func() {
			fs := tempFileSystem()
			So(fs.EnsureSymlink(ctx, fs.join("abc"), "target"), ShouldBeNil)
			So(fs.EnsureFileGone(ctx, fs.join("abc")), ShouldBeNil)
			So(fs.isMissing("abc"), ShouldBeTrue)
		})
	}
}

func TestEnsureDirectoryGone(t *testing.T) {
	ctx := context.Background()

	Convey("EnsureDirectoryGone checks root", t, func() {
		fs := tempFileSystem()
		So(fs.EnsureDirectoryGone(ctx, fs.join("../abc")), ShouldNotBeNil)
	})

	Convey("EnsureDirectoryGone works", t, func() {
		fs := tempFileSystem()
		fs.write("dir/a/1", "")
		fs.write("dir/a/2", "")
		fs.write("dir/b/1", "")
		So(fs.EnsureDirectoryGone(ctx, fs.join("dir")), ShouldBeNil)
		So(fs.isMissing("dir"), ShouldBeTrue)
	})

	Convey("EnsureDirectoryGone works with missing dir", t, func() {
		fs := tempFileSystem()
		So(fs.EnsureDirectoryGone(ctx, fs.join("missing")), ShouldBeNil)
	})

}

func TestReplace(t *testing.T) {
	ctx := context.Background()

	Convey("Replace checks root", t, func() {
		fs := tempFileSystem()
		So(fs.Replace(ctx, fs.join("../abc"), fs.join("def")), ShouldNotBeNil)
		fs.write("def", "")
		So(fs.Replace(ctx, fs.join("def"), fs.join("../abc")), ShouldNotBeNil)
	})

	Convey("Replace does nothing if oldpath == newpath", t, func() {
		fs := tempFileSystem()
		So(fs.Replace(ctx, fs.join("abc"), fs.join("abc/d/..")), ShouldBeNil)
	})

	Convey("Replace recognizes missing oldpath", t, func() {
		fs := tempFileSystem()
		So(fs.Replace(ctx, fs.join("missing"), fs.join("abc")), ShouldNotBeNil)
	})

	Convey("Replace creates file and dir if missing", t, func() {
		fs := tempFileSystem()
		fs.write("a/123", "")
		So(fs.Replace(ctx, fs.join("a/123"), fs.join("b/c/d")), ShouldBeNil)
		So(fs.isMissing("a/123"), ShouldBeTrue)
		So(fs.isFile("b/c/d/"), ShouldBeTrue)
	})

	Convey("Replace replaces regular file with a file", t, func() {
		fs := tempFileSystem()
		fs.write("a/123", "xxx")
		fs.write("b/c/d", "yyy")
		So(fs.Replace(ctx, fs.join("a/123"), fs.join("b/c/d")), ShouldBeNil)
		So(fs.isMissing("a/123"), ShouldBeTrue)
		So(fs.read("b/c/d"), ShouldEqual, "xxx")
	})

	Convey("Replace replaces regular file with a dir", t, func() {
		fs := tempFileSystem()
		fs.write("a/123/456", "xxx")
		fs.write("b/c/d", "yyy")
		So(fs.Replace(ctx, fs.join("a/123"), fs.join("b/c/d")), ShouldBeNil)
		So(fs.isMissing("a/123"), ShouldBeTrue)
		So(fs.read("b/c/d/456"), ShouldEqual, "xxx")
	})

	Convey("Replace replaces empty dir with a file", t, func() {
		fs := tempFileSystem()
		fs.write("a/123", "xxx")
		_, err := fs.EnsureDirectory(ctx, fs.join("b/c/d"))
		So(err, ShouldBeNil)
		So(fs.Replace(ctx, fs.join("a/123"), fs.join("b/c/d")), ShouldBeNil)
		So(fs.isMissing("a/123"), ShouldBeTrue)
		So(fs.read("b/c/d"), ShouldEqual, "xxx")
	})

	Convey("Replace replaces empty dir with a dir", t, func() {
		fs := tempFileSystem()
		fs.write("a/123/456", "xxx")
		_, err := fs.EnsureDirectory(ctx, fs.join("b/c/d"))
		So(err, ShouldBeNil)
		So(fs.Replace(ctx, fs.join("a/123"), fs.join("b/c/d")), ShouldBeNil)
		So(fs.isMissing("a/123"), ShouldBeTrue)
		So(fs.read("b/c/d/456"), ShouldEqual, "xxx")
	})

	Convey("Replace replaces dir with a file", t, func() {
		fs := tempFileSystem()
		fs.write("a/123", "xxx")
		fs.write("b/c/d/456", "yyy")
		So(fs.Replace(ctx, fs.join("a/123"), fs.join("b/c/d")), ShouldBeNil)
		So(fs.isMissing("a/123"), ShouldBeTrue)
		So(fs.read("b/c/d"), ShouldEqual, "xxx")
	})

	Convey("Replace replaces dir with a dir", t, func() {
		fs := tempFileSystem()
		fs.write("a/123/456", "xxx")
		fs.write("b/c/d/456", "yyy")
		So(fs.Replace(ctx, fs.join("a/123"), fs.join("b/c/d")), ShouldBeNil)
		So(fs.isMissing("a/123"), ShouldBeTrue)
		So(fs.read("b/c/d/456"), ShouldEqual, "xxx")
	})
}

///////

// tempFileSystem returns FileSystem for tests built over a temp directory.
func tempFileSystem() *tempFileSystemImpl {
	tempDir, err := ioutil.TempDir("", "cipd_test")
	So(err, ShouldBeNil)
	Reset(func() { os.RemoveAll(tempDir) })
	return &tempFileSystemImpl{NewFileSystem(tempDir)}
}

type tempFileSystemImpl struct {
	FileSystem
}

// join returns absolute path given a slash separated path relative to Root().
func (f *tempFileSystemImpl) join(path string) string {
	return filepath.Join(f.Root(), filepath.FromSlash(path))
}

// write creates a file at a given slash separated path relative to Root().
func (f *tempFileSystemImpl) write(rel string, data string) {
	abs := f.join(rel)
	err := os.MkdirAll(filepath.Dir(abs), 0777)
	So(err, ShouldBeNil)
	file, err := os.Create(abs)
	So(err, ShouldBeNil)
	file.WriteString(data)
	file.Close()
}

// read reads an existing file at a given slash separated path relative to Root().
func (f *tempFileSystemImpl) read(rel string) string {
	data, err := ioutil.ReadFile(f.join(rel))
	So(err, ShouldBeNil)
	return string(data)
}

// readLink reads a symlink at a given slash separated path relative to Root().
func (f *tempFileSystemImpl) readLink(rel string) string {
	val, err := os.Readlink(f.join(rel))
	So(err, ShouldBeNil)
	return val
}

// isMissing returns true if there's no file at a given slash separated path
// relative to Root().
func (f *tempFileSystemImpl) isMissing(rel string) bool {
	_, err := os.Stat(f.join(rel))
	return os.IsNotExist(err)
}

// isDir returns true if a file at a given slash separated path relative to
// Root() is a directory.
func (f *tempFileSystemImpl) isDir(rel string) bool {
	stat, err := os.Stat(f.join(rel))
	if err != nil {
		return false
	}
	return stat.IsDir()
}

// isFile returns true if a file at a given slash separated path relative to
// Root() is a file (not a directory).
func (f *tempFileSystemImpl) isFile(rel string) bool {
	stat, err := os.Stat(f.join(rel))
	if err != nil {
		return false
	}
	return !stat.IsDir()
}
