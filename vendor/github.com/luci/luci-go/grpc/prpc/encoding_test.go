// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package prpc

import (
	"net/http"
	"strings"
	"testing"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestEncoding(t *testing.T) {
	t.Parallel()

	Convey("responseFormat", t, func() {
		test := func(acceptHeader string, expectedFormat Format, expectedErr interface{}) {
			acceptHeader = strings.Replace(acceptHeader, "{json}", mtPRPCJSONPB, -1)
			acceptHeader = strings.Replace(acceptHeader, "{binary}", mtPRPCBinary, -1)
			acceptHeader = strings.Replace(acceptHeader, "{text}", mtPRPCText, -1)

			Convey("Accept: "+acceptHeader, func() {
				actualFormat, err := responseFormat(acceptHeader)
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
		test(ContentTypeJSON, FormatJSONPB, nil)

		test("application/*", FormatBinary, nil)
		test("*/*", FormatBinary, nil)

		// test cases with multiple types
		test("{json},{binary}", FormatBinary, nil)
		test("{json},{binary};q=0.9", FormatJSONPB, nil)
		test("{json};q=1,{binary};q=0.9", FormatJSONPB, nil)
		test("{json},{text}", FormatJSONPB, nil)
		test("{json};q=0.9,{text}", FormatText, nil)
		test("{binary},{json},{text}", FormatBinary, nil)

		test("{json},{binary},*/*", FormatBinary, nil)
		test("{json},{binary},*/*;q=0.9", FormatBinary, nil)
		test("{json},{binary},*/*;x=y", FormatBinary, nil)
		test("{json},{binary};q=0.9,*/*", FormatBinary, nil)
		test("{json},{binary};q=0.9,*/*;q=0.8", FormatJSONPB, nil)

		// supported and unsupported mix
		test("{json},foo/bar", FormatJSONPB, nil)
		test("{json};q=0.1,foo/bar", FormatJSONPB, nil)
		test("foo/bar;q=0.1,{json}", FormatJSONPB, nil)

		// only unsupported types
		const err406 = "pRPC: Accept header: specified media types are not not supported"
		test(ContentTypePRPC+"; boo=true", 0, err406)
		test(ContentTypePRPC+"; encoding=blah", 0, err406)
		test("x", 0, err406)
		test("x,y", 0, err406)

		test("x//y", 0, "pRPC: Accept header: expected token after slash")
	})

	Convey("respondMessage", t, func() {
		msg := &HelloReply{Message: "Hi"}

		test := func(f Format, body []byte, contentType string) {
			Convey(contentType, func() {
				res := respondMessage(msg, f)
				So(res.code, ShouldEqual, codes.OK)
				So(res.header, ShouldResemble, http.Header{
					headerContentType: []string{contentType},
				})
				So(res.body, ShouldResemble, body)
			})
		}

		msgBytes, err := proto.Marshal(msg)
		So(err, ShouldBeNil)

		test(FormatBinary, msgBytes, mtPRPCBinary)
		test(FormatJSONPB, []byte(JSONPBPrefix+"{\"message\":\"Hi\"}\n"), mtPRPCJSONPB)
		test(FormatText, []byte("message: \"Hi\"\n"), mtPRPCText)
	})
}
