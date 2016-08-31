// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package utils

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestZip(t *testing.T) {
	Convey("ZlibCompress/ZlibDecompress roundtrip", t, func() {
		data := "blah-blah"
		for i := 0; i < 10; i++ {
			data += data
		}

		blob, err := ZlibCompress([]byte(data))
		So(err, ShouldBeNil)

		back, err := ZlibDecompress(blob)
		So(err, ShouldBeNil)

		So(back, ShouldResemble, []byte(data))
	})

	Convey("ZlibDecompress garbage", t, func() {
		_, err := ZlibDecompress([]byte("garbage"))
		So(err, ShouldNotBeNil)
	})
}
