// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package utils

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestPem(t *testing.T) {
	Convey("DumpPEM/ParsePEM roundtrip", t, func() {
		data := []byte("blah-blah")
		back, err := ParsePEM(DumpPEM(data, "DATA"), "DATA")
		So(err, ShouldBeNil)
		So(back, ShouldResemble, data)
	})

	Convey("ParsePEM wrong header", t, func() {
		data := []byte("blah-blah")
		_, err := ParsePEM(DumpPEM(data, "DATA"), "NOT DATA")
		So(err, ShouldNotBeNil)
	})

	Convey("ParsePEM not a PEM", t, func() {
		_, err := ParsePEM("blah-blah", "NOT DATA")
		So(err, ShouldNotBeNil)
	})
}
