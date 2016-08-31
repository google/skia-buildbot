// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package coordinator

import (
	"fmt"
	"testing"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestLogStreamEncoding(t *testing.T) {
	t.Parallel()

	Convey(`Testing log stream key encode/decode`, t, func() {

		Convey(`Will encode "quux" into "Key_cXV1eA~~".`, func() {
			// Note that we test a key whose length is not a multiple of 3 so that we
			// can assert that the padding is correct, too.
			enc := encodeKey("quux")
			So(enc, ShouldEqual, "Key_cXV1eA~~")
		})

		for _, s := range []string{
			"",
			"hello",
			"from the outside",
			"+-#$!? \t\n",
		} {
			Convey(fmt.Sprintf(`Can encode: %q`, s), func() {
				enc := encodeKey(s)
				So(enc, ShouldStartWith, encodedKeyPrefix)

				Convey(`And then decode back into the original string.`, func() {
					dec, err := decodeKey(enc)
					So(err, ShouldBeNil)
					So(dec, ShouldEqual, s)
				})
			})
		}

		Convey(`Will fail to decode a string that doesn't begin with the encoded prefix.`, func() {
			_, err := decodeKey("foo")
			So(err, ShouldErrLike, "encoded key missing prefix")
		})

		Convey(`Will fail to decode a string that's not properly encoded.`, func() {
			_, err := decodeKey(encodedKeyPrefix + "!!!")
			So(err, ShouldErrLike, "failed to decode key")
		})
	})
}
