// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package prpc

import (
	"testing"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestFormat(t *testing.T) {
	Convey("requestFormat", t, func() {
		test := func(contentType string, expectedFormat Format, expectedErr interface{}) {
			Convey("Content-Type: "+contentType, func() {
				actualFormat, err := FormatFromContentType(contentType)
				So(err, ShouldErrLike, expectedErr)
				if err == nil {
					So(actualFormat, ShouldEqual, expectedFormat)
				}
			})
		}

		test("", FormatBinary, nil)
		test(ContentTypePRPC, FormatBinary, nil)
		test(mtPRPCBinary, FormatBinary, nil)
		test(mtPRPCJSONPB, FormatJSONPB, nil)
		test(mtPRPCText, FormatText, nil)
		test(
			ContentTypePRPC+"; encoding=blah",
			0,
			`invalid encoding parameter: "blah". Valid values: "binary", "json", "text"`)
		test(ContentTypePRPC+"; boo=true", 0, `unexpected parameter "boo"`)

		test(ContentTypeJSON, FormatJSONPB, nil)
		test(ContentTypeJSON+"; whatever=true", FormatJSONPB, nil)

		test("x", 0, `unknown content type: "x"`)
		test("x,y", 0, "mime: expected slash after first token")
	})
}
