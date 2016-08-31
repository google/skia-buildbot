// Copyright 2015 The LUCI Authors. All rights reserved.
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

	. "github.com/smartystreets/goconvey/convey"
)

func TestLoadPackageDef(t *testing.T) {
	Convey("LoadPackageDef empty works", t, func() {
		body := strings.NewReader(`{"package": "package/name"}`)
		def, err := LoadPackageDef(body, nil)
		So(err, ShouldBeNil)
		So(def, ShouldResemble, PackageDef{
			Package: "package/name",
			Root:    ".",
		})
	})

	Convey("LoadPackageDef works", t, func() {
		body := strings.NewReader(`{
			"package": "package/${var1}",
			"root": "../..",
			"install_mode": "copy",
			"data": [
				{
					"file": "some_file_${var1}"
				},
				{
					"file": "another_file_${var2}"
				},
				{
					"dir": "some/directory"
				},
				{
					"version_file": "some/path/version_${var1}.json"
				},
				{
					"dir": "another/${var2}",
					"exclude": [
						".*\\.pyc",
						"abc_${var2}_def"
					]
				}
			]
		}`)
		def, err := LoadPackageDef(body, map[string]string{
			"var1": "value1",
			"var2": "value2",
		})
		So(err, ShouldBeNil)
		So(def, ShouldResemble, PackageDef{
			Package:     "package/value1",
			Root:        "../..",
			InstallMode: "copy",
			Data: []PackageChunkDef{
				{
					File: "some_file_value1",
				},
				{
					File: "another_file_value2",
				},
				{
					Dir: "some/directory",
				},
				{
					VersionFile: "some/path/version_value1.json",
				},
				{
					Dir: "another/value2",
					Exclude: []string{
						".*\\.pyc",
						"abc_value2_def",
					},
				},
			},
		})
		So(def.VersionFile(), ShouldEqual, "some/path/version_value1.json")
	})

	Convey("LoadPackageDef not yaml", t, func() {
		body := strings.NewReader(`{ not yaml)`)
		_, err := LoadPackageDef(body, nil)
		So(err, ShouldNotBeNil)
	})

	Convey("LoadPackageDef bad type", t, func() {
		body := strings.NewReader(`{"package": []}`)
		_, err := LoadPackageDef(body, nil)
		So(err, ShouldNotBeNil)
	})

	Convey("LoadPackageDef missing variable", t, func() {
		body := strings.NewReader(`{
			"package": "abd",
			"data": [{"file": "${missing_var}"}]
		}`)
		_, err := LoadPackageDef(body, nil)
		So(err, ShouldNotBeNil)
	})

	Convey("LoadPackageDef space in missing variable", t, func() {
		body := strings.NewReader(`{
			"package": "abd",
			"data": [{"file": "${missing var}"}]
		}`)
		_, err := LoadPackageDef(body, nil)
		So(err, ShouldNotBeNil)
	})

	Convey("LoadPackageDef bad package name", t, func() {
		body := strings.NewReader(`{"package": "not a valid name"}`)
		_, err := LoadPackageDef(body, nil)
		So(err, ShouldNotBeNil)
	})

	Convey("LoadPackageDef bad file section (no dir or file)", t, func() {
		body := strings.NewReader(`{
			"package": "package/name",
			"data": [
				{"exclude": []}
			]
		}`)
		_, err := LoadPackageDef(body, nil)
		So(err, ShouldNotBeNil)
	})

	Convey("LoadPackageDef bad file section (both dir and file)", t, func() {
		body := strings.NewReader(`{
			"package": "package/name",
			"data": [
				{"file": "abc", "dir": "def"}
			]
		}`)
		_, err := LoadPackageDef(body, nil)
		So(err, ShouldNotBeNil)
	})

	Convey("LoadPackageDef bad version_file", t, func() {
		body := strings.NewReader(`{
			"package": "package/name",
			"data": [
				{"version_file": "../some/path.json"}
			]
		}`)
		_, err := LoadPackageDef(body, nil)
		So(err, ShouldNotBeNil)
	})

	Convey("LoadPackageDef two version_file entries", t, func() {
		body := strings.NewReader(`{
			"package": "package/name",
			"data": [
				{"version_file": "some/path.json"},
				{"version_file": "some/path.json"}
			]
		}`)
		_, err := LoadPackageDef(body, nil)
		So(err, ShouldNotBeNil)
	})
}

func TestExclusion(t *testing.T) {
	Convey("makeExclusionFilter works", t, func() {
		filter, err := makeExclusionFilter("a/b/c", []string{
			".*\\.pyc",
			".*/pip-.*-build/.*",
			"bin/activate",
			"lib/.*/site-packages/.*\\.dist-info/RECORD",
		})
		So(err, ShouldBeNil)
		So(filter, ShouldNotBeNil)

		// Not inside "a/b/c".
		So(filter(filepath.FromSlash("a/b/test.pyc")), ShouldBeFalse)

		// "a/b/c" itself.
		So(filter(filepath.FromSlash("a/b/c")), ShouldBeFalse)

		// *.pyc filtering.
		So(filter(filepath.FromSlash("a/b/c/test.pyc")), ShouldBeTrue)
		So(filter(filepath.FromSlash("a/b/c/test.py")), ShouldBeFalse)
		So(filter(filepath.FromSlash("a/b/c/d/e/f/test.pyc")), ShouldBeTrue)
		So(filter(filepath.FromSlash("a/b/c/d/e/f/test.py")), ShouldBeFalse)

		// Subdir filtering.
		So(filter(filepath.FromSlash("a/b/c/x/pip-blah-build/d/e/f")), ShouldBeTrue)

		// Single file exclusion.
		So(filter(filepath.FromSlash("a/b/c/bin/activate")), ShouldBeTrue)
		So(filter(filepath.FromSlash("a/b/c/bin/activate2")), ShouldBeFalse)
		So(filter(filepath.FromSlash("a/b/c/d/bin/activate")), ShouldBeFalse)

		// More complicated regexp.
		p := "a/b/c/lib/python2.7/site-packages/coverage-3.7.1.dist-info/RECORD"
		So(filter(filepath.FromSlash(p)), ShouldBeTrue)
	})

	Convey("makeExclusionFilter bad regexp", t, func() {
		_, err := makeExclusionFilter("a/b/c", []string{"****"})
		So(err, ShouldNotBeNil)
	})
}

func TestFindFiles(t *testing.T) {
	Convey("Given a temp directory", t, func() {
		tempDir, err := ioutil.TempDir("", "cipd_test")
		So(err, ShouldBeNil)
		Reset(func() { os.RemoveAll(tempDir) })

		mkF := func(path string) { writeFile(tempDir, path, "", 0666) }
		mkD := func(path string) { mkDir(tempDir, path) }
		mkL := func(path, target string) { writeSymlink(tempDir, path, target) }

		Convey("FindFiles works", func() {
			mkF("ENV/abc.py")
			mkF("ENV/abc.pyc") // excluded via "exclude: '.*\.pyc'"
			mkF("ENV/abc.pyo")
			mkF("ENV/dir/def.py")
			mkD("ENV/empty")      // will be skipped
			mkF("ENV/exclude_me") // excluded via "exclude: 'exclude_me'"

			// Symlinks do not work on Windows.
			if runtime.GOOS != "windows" {
				mkL("ENV/abs_link", filepath.Dir(tempDir))
				mkL("ENV/rel_link", "abc.py")
				mkL("ENV/abs_in_root", filepath.Join(tempDir, "ENV", "dir", "def.py"))
			}

			mkF("infra/xyz.py")
			mkF("infra/zzz.pyo")
			mkF("infra/excluded.py")
			mkF("infra/excluded_dir/a")
			mkF("infra/excluded_dir/b")

			mkF("file1.py")
			mkF("dir/file2.py")

			mkF("garbage/a")
			mkF("garbage/b")

			pkgDef := PackageDef{
				Package: "test",
				Root:    "../../",
				Data: []PackageChunkDef{
					{
						Dir:     "ENV",
						Exclude: []string{".*\\.pyc", "exclude_me"},
					},
					{
						Dir: "infra",
						Exclude: []string{
							".*\\.pyo",
							"excluded.py",
							"excluded_dir",
						},
					},
					{File: "file1.py"},
					{File: "dir/file2.py"},
					// Will be "deduplicated", because already matched by first entry.
					{File: "ENV/abc.py"},
				},
			}

			files, err := pkgDef.FindFiles(filepath.Join(tempDir, "a", "b"))
			So(err, ShouldBeNil)
			names := []string{}
			byName := make(map[string]File, len(files))
			for _, f := range files {
				names = append(names, f.Name())
				byName[f.Name()] = f
			}

			if runtime.GOOS == "windows" {
				So(names, ShouldResemble, []string{
					"ENV/abc.py",
					"ENV/abc.pyo",
					"ENV/dir/def.py",
					"dir/file2.py",
					"file1.py",
					"infra/xyz.py",
				})
			} else {
				So(names, ShouldResemble, []string{
					"ENV/abc.py",
					"ENV/abc.pyo",
					"ENV/abs_in_root",
					"ENV/abs_link",
					"ENV/dir/def.py",
					"ENV/rel_link",
					"dir/file2.py",
					"file1.py",
					"infra/xyz.py",
				})
				// Separately check symlinks.
				ensureSymlinkTarget(byName["ENV/abs_in_root"], "dir/def.py")
				ensureSymlinkTarget(byName["ENV/abs_link"], filepath.ToSlash(filepath.Dir(tempDir)))
				ensureSymlinkTarget(byName["ENV/rel_link"], "abc.py")
			}
		})
	})
}
