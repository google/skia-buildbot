// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"
)

const (
	testDataDir = "testdata"
)

var train = flag.Bool("test.train", false, "retrain golden files")

func TestMain(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOARCH == "386" {
		t.Skipf("we don't have protoc these build machines.")
	}

	Convey("Main", t, func() {
		testDataDirF, err := os.Open(testDataDir)
		So(err, ShouldBeNil)
		defer testDataDirF.Close()

		testCaseDirs, err := testDataDirF.Readdirnames(0)
		So(err, ShouldBeNil)

		runOn := func(sourceDir string, test func(tmpDir string)) {
			tmpGoPath, err := ioutil.TempDir("", "cproto-test")
			So(err, ShouldBeNil)
			defer os.RemoveAll(tmpGoPath)

			pkgPath := filepath.Join(tmpGoPath, "src", "tmp")
			err = os.MkdirAll(pkgPath, 0777)
			So(err, ShouldBeNil)

			names, err := findProtoFiles(sourceDir)
			So(err, ShouldBeNil)
			So(names, ShouldNotBeEmpty)
			for _, name := range names {
				name = filepath.Base(name)
				err = copyFile(
					filepath.Join(sourceDir, name),
					filepath.Join(pkgPath, name))
				So(err, ShouldBeNil)
			}

			// Run cproto.
			goPaths := []string{tmpGoPath}
			goPaths = append(goPaths, strings.Split(os.Getenv("GOPATH"), string(filepath.ListSeparator))...)
			err = run(context.Background(), []string{tmpGoPath}, pkgPath)
			So(err, ShouldBeNil)

			test(pkgPath)
		}

		for _, testDir := range testCaseDirs {
			testDir := filepath.Join(testDataDir, testDir)
			info, err := os.Stat(testDir)
			So(err, ShouldBeNil)
			if !info.IsDir() {
				continue
			}

			Convey("Check generated .go files for "+testDir, func() {
				runOn(testDir, func(tmpDir string) {
					goldenFiles, err := filepath.Glob(filepath.Join(testDir, "*.golden"))
					So(err, ShouldBeNil)
					So(goldenFiles, ShouldNotBeEmpty)
					for _, golden := range goldenFiles {
						gotFile := strings.TrimSuffix(filepath.Base(golden), ".golden") + ".go"
						gotFile = filepath.Join(tmpDir, gotFile)
						got, err := ioutil.ReadFile(gotFile)
						So(err, ShouldBeNil)

						if *train {
							err := ioutil.WriteFile(golden, got, 0777)
							So(err, ShouldBeNil)
						}

						want, err := ioutil.ReadFile(golden)
						So(err, ShouldBeNil)

						So(string(got), ShouldEqual, string(want))
					}
				})
			})
		}
	})
}

func copyFile(src, dest string) error {
	data, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(dest, data, 0666)
}
