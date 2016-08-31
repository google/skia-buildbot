// Copyright 2014 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package local

import (
	"bytes"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestReadManifest(t *testing.T) {
	var goodManifest = `{
  "format_version": "1",
  "package_name": "package/name"
}`

	Convey("readManifest can read valid manifest", t, func() {
		manifest, err := readManifest(strings.NewReader(goodManifest))
		So(manifest, ShouldResemble, Manifest{
			FormatVersion: "1",
			PackageName:   "package/name",
		})
		So(err, ShouldBeNil)
	})

	Convey("readManifest rejects invalid manifest", t, func() {
		manifest, err := readManifest(strings.NewReader("I'm not a manifest"))
		So(manifest, ShouldResemble, Manifest{})
		So(err, ShouldNotBeNil)
	})

	Convey("writeManifest works", t, func() {
		buf := &bytes.Buffer{}
		m := Manifest{
			FormatVersion: "1",
			PackageName:   "package/name",
		}
		So(writeManifest(&m, buf), ShouldBeNil)
		So(string(buf.Bytes()), ShouldEqual, goodManifest)
	})
}
