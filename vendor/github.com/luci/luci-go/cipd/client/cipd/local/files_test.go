// Copyright 2014 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package local

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"
)

func TestScanFileSystem(t *testing.T) {
	Convey("Given a temp directory", t, func() {
		tempDir, err := ioutil.TempDir("", "cipd_test")
		So(err, ShouldBeNil)
		Reset(func() { os.RemoveAll(tempDir) })

		Convey("Scan empty dir works", func() {
			files, err := ScanFileSystem(tempDir, tempDir, nil)
			So(files, ShouldBeEmpty)
			So(err, ShouldBeNil)
		})

		Convey("Discovering single file works", func() {
			writeFile(tempDir, "single_file", "12345", 0666)
			files, err := ScanFileSystem(tempDir, tempDir, nil)
			So(len(files), ShouldEqual, 1)
			So(err, ShouldBeNil)

			file := files[0]
			So(file.Name(), ShouldEqual, "single_file")
			So(file.Size(), ShouldEqual, uint64(5))
			So(file.Executable(), ShouldBeFalse)

			r, err := file.Open()
			if r != nil {
				defer r.Close()
			}
			So(err, ShouldBeNil)
			buf, err := ioutil.ReadAll(r)
			So(buf, ShouldResemble, []byte("12345"))
			So(err, ShouldBeNil)
		})

		Convey("Enumerating subdirectories", func() {
			writeFile(tempDir, "a", "", 0666)
			writeFile(tempDir, "b", "", 0666)
			writeFile(tempDir, "1/a", "", 0666)
			writeFile(tempDir, "1/b", "", 0666)
			writeFile(tempDir, "1/2/a", "", 0666)
			files, err := ScanFileSystem(tempDir, tempDir, nil)
			So(err, ShouldBeNil)
			names := []string{}
			for _, f := range files {
				names = append(names, f.Name())
			}
			// Order matters. Slashes matters.
			So(names, ShouldResemble, []string{
				"1/2/a",
				"1/a",
				"1/b",
				"a",
				"b",
			})
		})

		Convey("Empty subdirectories are skipped", func() {
			mkDir(tempDir, "a")
			mkDir(tempDir, "1/2/3")
			mkDir(tempDir, "1/c")
			writeFile(tempDir, "1/d/file", "1234", 0666)
			files, err := ScanFileSystem(tempDir, tempDir, nil)
			So(len(files), ShouldEqual, 1)
			So(err, ShouldBeNil)
			So(files[0].Name(), ShouldEqual, "1/d/file")
		})

		Convey("Non root start path works", func() {
			writeFile(tempDir, "a", "", 0666)
			writeFile(tempDir, "b", "", 0666)
			writeFile(tempDir, "1/a", "", 0666)
			writeFile(tempDir, "1/b", "", 0666)
			writeFile(tempDir, "1/2/a", "", 0666)
			files, err := ScanFileSystem(filepath.Join(tempDir, "1"), tempDir, nil)
			So(err, ShouldBeNil)
			names := []string{}
			for _, f := range files {
				names = append(names, f.Name())
			}
			// Order matters. Slashes matters.
			So(names, ShouldResemble, []string{
				"1/2/a",
				"1/a",
				"1/b",
			})
		})

		Convey("Start path must be under root", func() {
			_, err := ScanFileSystem(filepath.Dir(tempDir), tempDir, nil)
			So(err, ShouldNotBeNil)
		})

		Convey("Exclude filter works", func() {
			writeFile(tempDir, "a", "", 0666)
			writeFile(tempDir, "b", "", 0666)
			writeFile(tempDir, "1/a", "", 0666)
			writeFile(tempDir, "1/b", "", 0666)
			writeFile(tempDir, "1/2/a", "", 0666)

			// Exclude "a" and entire "1/" directory.
			excluderCalls := []string{}
			excluder := func(abs string) bool {
				excluderCalls = append(excluderCalls, abs)
				if abs == filepath.Join(tempDir, "a") {
					return true
				}
				if abs == filepath.Join(tempDir, "1") {
					return true
				}
				return false
			}

			files, err := ScanFileSystem(tempDir, tempDir, excluder)
			So(err, ShouldBeNil)
			So(len(files), ShouldEqual, 1)
			So(files[0].Name(), ShouldEqual, "b")

			// "1/*" subdir should have been skipped completely.
			So(excluderCalls, ShouldResemble, []string{
				filepath.Join(tempDir, "1"),
				filepath.Join(tempDir, "a"),
				filepath.Join(tempDir, "b"),
			})
		})

		if runtime.GOOS != "windows" {
			Convey("Discovering single executable file works", func() {
				writeFile(tempDir, "single_file", "12345", 0766)
				files, err := ScanFileSystem(tempDir, tempDir, nil)
				So(len(files), ShouldEqual, 1)
				So(err, ShouldBeNil)
				file := files[0]
				So(file.Executable(), ShouldBeTrue)
			})

			Convey("Relative symlink to outside of package cause error", func() {
				writeSymlink(tempDir, "a/b1/rel_symlink", filepath.FromSlash("../../.."))
				_, err := ScanFileSystem(tempDir, tempDir, nil)
				So(err, ShouldNotBeNil)
			})
		}
	})
}

func TestWrapFile(t *testing.T) {
	Convey("Given a temp directory", t, func() {
		tempDir, err := ioutil.TempDir("", "cipd_test")
		So(err, ShouldBeNil)
		Reset(func() { os.RemoveAll(tempDir) })

		Convey("WrapFile simple file works", func() {
			writeFile(tempDir, "dir/a/b", "12345", 0666)
			out, err := WrapFile(filepath.Join(tempDir, "dir", "a", "b"), tempDir, nil)
			So(err, ShouldBeNil)
			So(out.Name(), ShouldEqual, "dir/a/b")
		})

		Convey("WrapFile directory fails", func() {
			mkDir(tempDir, "dir")
			_, err := WrapFile(filepath.Join(tempDir, "dir"), tempDir, nil)
			So(err, ShouldNotBeNil)
		})

		Convey("WrapFile outside of root fails", func() {
			mkDir(tempDir, "a")
			writeFile(tempDir, "b", "body", 0666)
			_, err := WrapFile(filepath.Join(tempDir, "b"), filepath.Join(tempDir, "a"), nil)
			So(err, ShouldNotBeNil)
		})

		Convey("WrapFile outside of root fails (tricky path)", func() {
			mkDir(tempDir, "a")
			// "abc" starts with "a", it tricks naive string.HasPrefix subpath check.
			writeFile(tempDir, "abc", "body", 0666)
			_, err := WrapFile(filepath.Join(tempDir, "abc"), filepath.Join(tempDir, "a"), nil)
			So(err, ShouldNotBeNil)
		})

		if runtime.GOOS != "windows" {
			Convey("WrapFile executable file works", func() {
				writeFile(tempDir, "single_file", "12345", 0766)
				out, err := WrapFile(filepath.Join(tempDir, "single_file"), tempDir, nil)
				So(err, ShouldBeNil)
				So(out.Executable(), ShouldBeTrue)
			})

			Convey("WrapFile rel symlink in root", func() {
				writeSymlink(tempDir, "a/b/c", filepath.FromSlash("../../d"))
				mkDir(tempDir, "d")
				out, err := WrapFile(filepath.Join(tempDir, "a", "b", "c"), tempDir, nil)
				So(err, ShouldBeNil)
				ensureSymlinkTarget(out, "../../d")
			})

			Convey("WrapFile rel symlink outside root", func() {
				writeSymlink(tempDir, "a/b/c", filepath.FromSlash("../../../d"))
				_, err := WrapFile(filepath.Join(tempDir, "a", "b", "c"), tempDir, nil)
				So(err, ShouldNotBeNil)
			})

			Convey("WrapFile abs symlink in root", func() {
				writeSymlink(tempDir, "a/b/c", filepath.Join(tempDir, "a", "d"))
				out, err := WrapFile(filepath.Join(tempDir, "a", "b", "c"), tempDir, nil)
				So(err, ShouldBeNil)
				ensureSymlinkTarget(out, "../d")
			})

			Convey("WrapFile abs symlink outside root", func() {
				writeSymlink(tempDir, "a/b/c", filepath.Dir(tempDir))
				out, err := WrapFile(filepath.Join(tempDir, "a", "b", "c"), tempDir, nil)
				So(err, ShouldBeNil)
				ensureSymlinkTarget(out, filepath.ToSlash(filepath.Dir(tempDir)))
			})
		}
	})
}

func mkDir(root string, path string) {
	abs := filepath.Join(root, filepath.FromSlash(path))
	err := os.MkdirAll(abs, 0777)
	if err != nil {
		panic("Failed to create a directory under temp directory")
	}
}

func writeFile(root string, path string, data string, mode os.FileMode) {
	abs := filepath.Join(root, filepath.FromSlash(path))
	os.MkdirAll(filepath.Dir(abs), 0777)
	err := ioutil.WriteFile(abs, []byte(data), mode)
	if err != nil {
		panic("Failed to write a temp file")
	}
}

func writeSymlink(root string, path string, target string) {
	abs := filepath.Join(root, filepath.FromSlash(path))
	os.MkdirAll(filepath.Dir(abs), 0777)
	err := os.Symlink(target, abs)
	if err != nil {
		panic("Failed to create symlink")
	}
}

func ensureSymlinkTarget(file File, target string) {
	So(file.Symlink(), ShouldBeTrue)
	discoveredTarget, err := file.SymlinkTarget()
	So(err, ShouldBeNil)
	So(discoveredTarget, ShouldEqual, target)
}

func TestFileSystemDestination(t *testing.T) {
	ctx := context.Background()

	Convey("Given a temp directory", t, func() {
		tempDir, err := ioutil.TempDir("", "cipd_test")
		destDir := filepath.Join(tempDir, "dest")
		So(err, ShouldBeNil)
		dest := NewFileSystemDestination(destDir, nil)
		Reset(func() { os.RemoveAll(tempDir) })

		writeFileToDest := func(name string, executable bool, data string) {
			writer, err := dest.CreateFile(ctx, name, executable)
			if writer != nil {
				defer writer.Close()
			}
			So(err, ShouldBeNil)
			_, err = writer.Write([]byte(data))
			So(err, ShouldBeNil)
		}

		writeSymlinkToDest := func(name string, target string) {
			err := dest.CreateSymlink(ctx, name, target)
			So(err, ShouldBeNil)
		}

		Convey("Empty success write works", func() {
			So(dest.Begin(ctx), ShouldBeNil)
			So(dest.End(ctx, true), ShouldBeNil)

			// Should create a new directory.
			stat, err := os.Stat(destDir)
			So(err, ShouldBeNil)
			So(stat.IsDir(), ShouldBeTrue)

			// And it should be empty.
			files, err := ScanFileSystem(destDir, destDir, nil)
			So(err, ShouldBeNil)
			So(len(files), ShouldEqual, 0)
		})

		Convey("Empty failed write works", func() {
			So(dest.Begin(ctx), ShouldBeNil)
			So(dest.End(ctx, false), ShouldBeNil)

			// Doesn't create a directory.
			_, err := os.Stat(destDir)
			So(os.IsNotExist(err), ShouldBeTrue)
		})

		Convey("Double begin or double end fails", func() {
			So(dest.Begin(ctx), ShouldBeNil)
			So(dest.Begin(ctx), ShouldNotBeNil)
			So(dest.End(ctx, true), ShouldBeNil)
			So(dest.End(ctx, true), ShouldNotBeNil)
		})

		Convey("CreateFile works only when destination is open", func() {
			wr, err := dest.CreateFile(ctx, "testing", true)
			So(wr, ShouldBeNil)
			So(err, ShouldNotBeNil)
		})

		Convey("CreateFile rejects invalid relative paths", func() {
			So(dest.Begin(ctx), ShouldBeNil)
			defer dest.End(ctx, true)

			// Rel path that is still inside the package is ok.
			wr, err := dest.CreateFile(ctx, "a/b/c/../../../d", false)
			So(err, ShouldBeNil)
			wr.Close()

			// Rel path pointing outside is forbidden.
			_, err = dest.CreateFile(ctx, "a/b/c/../../../../d", false)
			So(err, ShouldNotBeNil)
		})

		if runtime.GOOS != "windows" {
			Convey("CreateSymlink rejects invalid relative paths", func() {
				So(dest.Begin(ctx), ShouldBeNil)
				defer dest.End(ctx, true)

				// Rel symlink to a file inside the destination is OK.
				So(dest.CreateSymlink(ctx, "a/b/c", "../.."), ShouldBeNil)
				// Rel symlink to a file outside -> error.
				So(dest.CreateSymlink(ctx, "a/b/c", "../../.."), ShouldNotBeNil)
			})
		}

		Convey("Committing bunch of files works", func() {
			So(dest.Begin(ctx), ShouldBeNil)
			writeFileToDest("a", false, "a data")
			writeFileToDest("exe", true, "exe data")
			writeFileToDest("dir/c", false, "dir/c data")
			writeFileToDest("dir/dir/d", false, "dir/dir/c data")
			if runtime.GOOS != "windows" {
				writeSymlinkToDest("abs_symlink", filepath.FromSlash(tempDir))
				writeSymlinkToDest("dir/dir/rel_symlink", "../../a")
			}
			So(dest.End(ctx, true), ShouldBeNil)

			// Ensure everything is there.
			files, err := ScanFileSystem(destDir, destDir, nil)
			So(err, ShouldBeNil)
			names := []string{}
			mapping := map[string]File{}
			for _, f := range files {
				names = append(names, f.Name())
				mapping[f.Name()] = f
			}

			if runtime.GOOS == "windows" {
				So(names, ShouldResemble, []string{
					"a",
					"dir/c",
					"dir/dir/d",
					"exe",
				})
			} else {
				So(names, ShouldResemble, []string{
					"a",
					"abs_symlink",
					"dir/c",
					"dir/dir/d",
					"dir/dir/rel_symlink",
					"exe",
				})
			}

			// Ensure data is valid (check first file only).
			r, err := mapping["a"].Open()
			if r != nil {
				defer r.Close()
			}
			So(err, ShouldBeNil)
			data, err := ioutil.ReadAll(r)
			So(err, ShouldBeNil)
			So(data, ShouldResemble, []byte("a data"))

			// File mode and symlinks are valid.
			if runtime.GOOS != "windows" {
				So(mapping["exe"].Executable(), ShouldBeTrue)
				ensureSymlinkTarget(mapping["abs_symlink"], filepath.FromSlash(tempDir))
				ensureSymlinkTarget(mapping["dir/dir/rel_symlink"], "../../a")
			}

			// Ensure no temp files left.
			allFiles, err := ScanFileSystem(tempDir, tempDir, nil)
			So(len(allFiles), ShouldEqual, len(files))
		})

		Convey("Rolling back bunch of files works", func() {
			So(dest.Begin(ctx), ShouldBeNil)
			writeFileToDest("a", false, "a data")
			writeFileToDest("dir/c", false, "dir/c data")
			if runtime.GOOS != "windows" {
				writeSymlinkToDest("dir/d", "c")
			}
			So(dest.End(ctx, false), ShouldBeNil)

			// No dest directory.
			_, err := os.Stat(destDir)
			So(os.IsNotExist(err), ShouldBeTrue)

			// Ensure no temp files left.
			allFiles, err := ScanFileSystem(tempDir, tempDir, nil)
			So(len(allFiles), ShouldEqual, 0)
		})

		Convey("Overwriting a directory works", func() {
			// Create dest directory manually with some stuff.
			err := os.Mkdir(destDir, 0777)
			So(err, ShouldBeNil)
			err = ioutil.WriteFile(filepath.Join(destDir, "data"), []byte("data"), 0666)
			So(err, ShouldBeNil)

			// Now deploy something to it.
			So(dest.Begin(ctx), ShouldBeNil)
			writeFileToDest("a", false, "a data")
			if runtime.GOOS != "windows" {
				writeSymlinkToDest("b", "a")
			}
			So(dest.End(ctx, true), ShouldBeNil)

			// Overwritten.
			files, err := ScanFileSystem(destDir, destDir, nil)
			So(err, ShouldBeNil)
			if runtime.GOOS == "windows" {
				So(len(files), ShouldEqual, 1)
				So(files[0].Name(), ShouldEqual, "a")
			} else {
				So(len(files), ShouldEqual, 2)
				So(files[0].Name(), ShouldEqual, "a")
				So(files[1].Name(), ShouldEqual, "b")
			}
		})

		Convey("Not overwriting a directory works", func() {
			// Create dest directory manually with some stuff.
			err := os.Mkdir(destDir, 0777)
			So(err, ShouldBeNil)
			err = ioutil.WriteFile(filepath.Join(destDir, "data"), []byte("data"), 0666)
			So(err, ShouldBeNil)

			// Now attempt deploy something to it, but roll back.
			So(dest.Begin(ctx), ShouldBeNil)
			writeFileToDest("a", false, "a data")
			if runtime.GOOS != "windows" {
				writeSymlinkToDest("b", "a")
			}
			So(dest.End(ctx, false), ShouldBeNil)

			// Kept as is.
			files, err := ScanFileSystem(destDir, destDir, nil)
			So(err, ShouldBeNil)
			So(len(files), ShouldEqual, 1)
			So(files[0].Name(), ShouldEqual, "data")
		})

		Convey("Opening file twice fails", func() {
			So(dest.Begin(ctx), ShouldBeNil)
			writeFileToDest("a", false, "a data")
			w, err := dest.CreateFile(ctx, "a", false)
			So(w, ShouldBeNil)
			So(err, ShouldNotBeNil)
			So(dest.End(ctx, true), ShouldBeNil)
		})

		Convey("End with opened files fail", func() {
			So(dest.Begin(ctx), ShouldBeNil)
			w, err := dest.CreateFile(ctx, "a", false)
			So(w, ShouldNotBeNil)
			So(err, ShouldBeNil)
			So(dest.End(ctx, true), ShouldNotBeNil)
			w.Close()
			So(dest.End(ctx, true), ShouldBeNil)
		})
	})
}
