// Copyright 2014 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package local

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"io/ioutil"
	"os"
	"testing"

	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBuildInstance(t *testing.T) {
	ctx := context.Background()

	Convey("Building empty package", t, func() {
		out := bytes.Buffer{}
		err := BuildInstance(ctx, BuildInstanceOptions{
			Input:       []File{},
			Output:      &out,
			PackageName: "testing",
		})
		So(err, ShouldBeNil)

		// BuildInstance builds deterministic zip. It MUST NOT depend on
		// the platform, or a time of day, or anything else, only on the input data.
		So(getSHA1(&out), ShouldEqual, "23f2c4900785ac8faa2f38e473925b840e574ccc")

		// There should be a single file: the manifest.
		goodManifest := `{
  "format_version": "1",
  "package_name": "testing"
}`
		files := readZip(out.Bytes())
		So(files, ShouldResemble, []zippedFile{
			{
				// See structs.go, manifestName.
				name: ".cipdpkg/manifest.json",
				size: uint64(len(goodManifest)),
				mode: 0600,
				body: []byte(goodManifest),
			},
		})
	})

	Convey("Building package with a bunch of files", t, func() {
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
			VersionFile: "version.json",
			InstallMode: "copy",
		})
		So(err, ShouldBeNil)

		goodManifest := `{
  "format_version": "1",
  "package_name": "testing",
  "version_file": "version.json",
  "install_mode": "copy"
}`

		// The manifest and all added files.
		files := readZip(out.Bytes())
		So(files, ShouldResemble, []zippedFile{
			{
				name: "testing/qwerty",
				size: 5,
				mode: 0600,
				body: []byte("12345"),
			},
			{
				name: "abc",
				size: 3,
				mode: 0700,
				body: []byte("duh"),
			},
			{
				name: "rel_symlink",
				size: 3,
				mode: 0600 | os.ModeSymlink,
				body: []byte("abc"),
			},
			{
				name: "abs_symlink",
				size: 8,
				mode: 0600 | os.ModeSymlink,
				body: []byte("/abc/def"),
			},
			{
				// See structs.go, manifestName.
				name: ".cipdpkg/manifest.json",
				size: uint64(len(goodManifest)),
				mode: 0600,
				body: []byte(goodManifest),
			},
		})
	})

	Convey("Duplicate files fail", t, func() {
		err := BuildInstance(ctx, BuildInstanceOptions{
			Input: []File{
				NewTestFile("a", "12345", false),
				NewTestFile("a", "12345", false),
			},
			Output:      &bytes.Buffer{},
			PackageName: "testing",
		})
		So(err, ShouldNotBeNil)
	})

	Convey("Writing to service dir fails", t, func() {
		err := BuildInstance(ctx, BuildInstanceOptions{
			Input: []File{
				NewTestFile(".cipdpkg/stuff", "12345", false),
			},
			Output:      &bytes.Buffer{},
			PackageName: "testing",
		})
		So(err, ShouldNotBeNil)
	})

	Convey("Bad name fails", t, func() {
		err := BuildInstance(ctx, BuildInstanceOptions{
			Output:      &bytes.Buffer{},
			PackageName: "../../asdad",
		})
		So(err, ShouldNotBeNil)
	})

	Convey("Bad version file fails", t, func() {
		err := BuildInstance(ctx, BuildInstanceOptions{
			Output:      &bytes.Buffer{},
			PackageName: "abc",
			VersionFile: "../bad/path",
		})
		So(err, ShouldNotBeNil)
	})
}

////////////////////////////////////////////////////////////////////////////////

// getSHA1 returns SHA1 hex digest of a byte buffer.
func getSHA1(buf *bytes.Buffer) string {
	h := sha1.New()
	h.Write(buf.Bytes())
	return hex.EncodeToString(h.Sum(nil))
}

////////////////////////////////////////////////////////////////////////////////

type zippedFile struct {
	name string
	size uint64
	mode os.FileMode
	body []byte
}

// readZip scans zip directory and returns files it finds.
func readZip(data []byte) []zippedFile {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		panic("Failed to open zip file")
	}
	files := make([]zippedFile, len(z.File))
	for i, zf := range z.File {
		reader, err := zf.Open()
		if err != nil {
			panic("Failed to open file inside zip")
		}
		body, err := ioutil.ReadAll(reader)
		if err != nil {
			panic("Failed to read zipped file")
		}
		files[i] = zippedFile{
			name: zf.Name,
			size: zf.FileHeader.UncompressedSize64,
			mode: zf.Mode(),
			body: body,
		}
	}
	return files
}
