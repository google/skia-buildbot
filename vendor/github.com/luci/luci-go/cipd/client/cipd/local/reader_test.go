// Copyright 2014 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package local

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"golang.org/x/net/context"

	. "github.com/luci/luci-go/cipd/client/cipd/common"
	. "github.com/smartystreets/goconvey/convey"
)

func normalizeJSON(s string) (string, error) {
	// Round trip through default json marshaller to normalize indentation.
	var x map[string]interface{}
	if err := json.Unmarshal([]byte(s), &x); err != nil {
		return "", err
	}
	blob, err := json.Marshal(x)
	if err != nil {
		return "", err
	}
	return string(blob), nil
}

func shouldBeSameJSONDict(actual interface{}, expected ...interface{}) string {
	if len(expected) != 1 {
		return "Too many argument for shouldBeSameJSONDict"
	}
	actualNorm, err := normalizeJSON(actual.(string))
	if err != nil {
		return err.Error()
	}
	expectedNorm, err := normalizeJSON(expected[0].(string))
	if err != nil {
		return err.Error()
	}
	return ShouldEqual(actualNorm, expectedNorm)
}

func TestPackageReading(t *testing.T) {
	ctx := context.Background()

	Convey("Open empty package works", t, func() {
		// Build an empty package.
		out := bytes.Buffer{}
		err := BuildInstance(ctx, BuildInstanceOptions{
			Output:      &out,
			PackageName: "testing",
		})
		So(err, ShouldBeNil)

		// Open it.
		inst, err := OpenInstance(ctx, bytes.NewReader(out.Bytes()), "")
		if inst != nil {
			defer inst.Close()
		}
		So(inst, ShouldNotBeNil)
		So(err, ShouldBeNil)
		So(inst.Pin(), ShouldResemble, Pin{"testing", "23f2c4900785ac8faa2f38e473925b840e574ccc"})
		So(len(inst.Files()), ShouldEqual, 1)

		// Contains single manifest file.
		f := inst.Files()[0]
		So(f.Name(), ShouldEqual, ".cipdpkg/manifest.json")
		So(f.Executable(), ShouldBeFalse)
		r, err := f.Open()
		if r != nil {
			defer r.Close()
		}
		So(err, ShouldBeNil)
		manifest, err := ioutil.ReadAll(r)
		So(err, ShouldBeNil)

		goodManifest := `{
			"format_version": "1",
			"package_name": "testing"
		}`
		So(string(manifest), shouldBeSameJSONDict, goodManifest)
	})

	Convey("Open empty package with unexpected instance ID", t, func() {
		// Build an empty package.
		out := bytes.Buffer{}
		err := BuildInstance(ctx, BuildInstanceOptions{
			Output:      &out,
			PackageName: "testing",
		})
		So(err, ShouldBeNil)

		// Attempt to open it, providing correct instance ID, should work.
		source := bytes.NewReader(out.Bytes())
		inst, err := OpenInstance(ctx, source, "23f2c4900785ac8faa2f38e473925b840e574ccc")
		So(err, ShouldBeNil)
		So(inst, ShouldNotBeNil)
		inst.Close()

		// Attempt to open it, providing incorrect instance ID.
		inst, err = OpenInstance(ctx, source, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
		So(err, ShouldNotBeNil)
		So(inst, ShouldBeNil)
	})

	Convey("OpenInstanceFile works", t, func() {
		// Open temp file.
		tempFile, err := ioutil.TempFile("", "cipdtest")
		So(err, ShouldBeNil)
		tempFilePath := tempFile.Name()
		defer os.Remove(tempFilePath)

		// Write empty package to it.
		err = BuildInstance(ctx, BuildInstanceOptions{
			Output:      tempFile,
			PackageName: "testing",
		})
		So(err, ShouldBeNil)
		tempFile.Close()

		// Read the package.
		inst, err := OpenInstanceFile(ctx, tempFilePath, "")
		if inst != nil {
			defer inst.Close()
		}
		So(inst, ShouldNotBeNil)
		So(err, ShouldBeNil)
	})

	Convey("ExtractInstance works", t, func() {
		// Add a bunch of files to a package.
		out := bytes.Buffer{}
		err := BuildInstance(ctx, BuildInstanceOptions{
			Input: []File{
				NewTestFile("testing/qwerty", "12345", false),
				NewTestFile("abc", "duh", true),
				NewTestSymlink("rel_symlink", "abc"),
				NewTestSymlink("abs_symlink", "/abc/def"),
			},
			Output:      &out,
			PackageName: "testing",
			VersionFile: "subpath/version.json",
		})
		So(err, ShouldBeNil)

		// Extract files.
		inst, err := OpenInstance(ctx, bytes.NewReader(out.Bytes()), "")
		if inst != nil {
			defer inst.Close()
		}
		So(err, ShouldBeNil)
		dest := &testDestination{}
		err = ExtractInstance(ctx, inst, dest)
		So(err, ShouldBeNil)
		So(dest.beginCalls, ShouldEqual, 1)
		So(dest.endCalls, ShouldEqual, 1)

		// Verify file list, file data and flags are correct.
		names := []string{}
		for _, f := range dest.files {
			names = append(names, f.name)
		}
		So(names, ShouldResemble, []string{
			"testing/qwerty",
			"abc",
			"rel_symlink",
			"abs_symlink",
			".cipdpkg/manifest.json",
			"subpath/version.json",
		})
		So(string(dest.files[0].Bytes()), ShouldEqual, "12345")
		So(dest.files[1].executable, ShouldBeTrue)
		So(dest.files[2].symlinkTarget, ShouldEqual, "abc")
		So(dest.files[3].symlinkTarget, ShouldEqual, "/abc/def")

		// Verify manifest file is correct.
		goodManifest := `{
			"format_version": "1",
			"package_name": "testing",
			"version_file": "subpath/version.json",
			"files": [
				{
					"name": "testing/qwerty",
					"size": 5
				},
				{
					"name": "abc",
					"size": 3,
					"executable": true
				},
				{
					"name": "rel_symlink",
					"size": 0,
					"symlink": "abc"
				},
				{
					"name": "abs_symlink",
					"size": 0,
					"symlink": "/abc/def"
				},
				{
					"name": "subpath/version.json",
					"size": 92
				}
			]
		}`
		So(dest.files[4].name, ShouldEqual, ".cipdpkg/manifest.json")
		So(string(dest.files[4].Bytes()), shouldBeSameJSONDict, goodManifest)

		// Verify version file is correct.
		goodVersionFile := `{
			"instance_id": "e01c6837705d21418a61bc17c794746daa32379f",
			"package_name": "testing"
		}`
		So(dest.files[5].name, ShouldEqual, "subpath/version.json")
		So(string(dest.files[5].Bytes()), shouldBeSameJSONDict, goodVersionFile)
	})
}

////////////////////////////////////////////////////////////////////////////////

type testDestination struct {
	beginCalls int
	endCalls   int
	files      []*testDestinationFile
}

type testDestinationFile struct {
	bytes.Buffer
	name          string
	executable    bool
	symlinkTarget string
}

func (d *testDestinationFile) Close() error { return nil }

func (d *testDestination) Begin(context.Context) error {
	d.beginCalls++
	return nil
}

func (d *testDestination) CreateFile(ctx context.Context, name string, executable bool) (io.WriteCloser, error) {
	f := &testDestinationFile{
		name:       name,
		executable: executable,
	}
	d.files = append(d.files, f)
	return f, nil
}

func (d *testDestination) CreateSymlink(ctx context.Context, name string, target string) error {
	f := &testDestinationFile{
		name:          name,
		symlinkTarget: target,
	}
	d.files = append(d.files, f)
	return nil
}

func (d *testDestination) End(ctx context.Context, success bool) error {
	d.endCalls++
	return nil
}
