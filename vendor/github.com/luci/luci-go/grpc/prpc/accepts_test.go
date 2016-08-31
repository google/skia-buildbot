// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package prpc

import (
	"testing"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestAccept(t *testing.T) {
	t.Parallel()

	Convey("parseAccept", t, func() {
		type params map[string]string

		test := func(value string, expectedErr interface{}, expectedTypes ...acceptType) {
			Convey("mediaType="+value, func() {
				actual, err := parseAccept(value)
				So(err, ShouldErrLike, expectedErr)
				So(actual, ShouldResemble, accept(expectedTypes))
			})
		}
		test("", nil)
		qf1 := func(mediaType string, p params) acceptType {
			return acceptType{
				MediaType:       mediaType,
				MediaTypeParams: p,
				QualityFactor:   1.0,
			}
		}

		test("text/html", nil,
			qf1("text/html", params{}),
		)

		test("text/html; level=1", nil,
			qf1("text/html", params{"level": "1"}),
		)
		test("TEXT/HTML; LEVEL=1", nil,
			qf1("text/html", params{"level": "1"}),
		)

		test("text/html, application/json", nil,
			qf1("text/html", params{}),
			qf1("application/json", params{}),
		)

		test("text/html; level=1, application/json", nil,
			qf1("text/html", params{"level": "1"}),
			qf1("application/json", params{}),
		)

		test("text/html; level=1, application/json; foo=bar", nil,
			qf1("text/html", params{"level": "1"}),
			qf1("application/json", params{"foo": "bar"}),
		)

		test("text/html; level=1; q=0.5", nil,
			acceptType{
				MediaType:       "text/html",
				MediaTypeParams: params{"level": "1"},
				QualityFactor:   0.5,
			},
		)
		test("text/html; level=1; q=0.5; a=1", nil,
			acceptType{
				MediaType:       "text/html",
				MediaTypeParams: params{"level": "1"},
				QualityFactor:   0.5,
				// AcceptParams is not parsed.
			},
		)
		test("TEXT/HTML; LEVEL=1; Q=0.5", nil,
			acceptType{
				MediaType:       "text/html",
				MediaTypeParams: params{"level": "1"},
				QualityFactor:   0.5,
			},
		)
		test("text/html; level=1; q=0.5, application/json", nil,
			acceptType{
				MediaType:       "text/html",
				MediaTypeParams: params{"level": "1"},
				QualityFactor:   0.5,
			},
			qf1("application/json", params{}),
		)
		test("text/html; level=1; q=0.5, application/json; a=b", nil,
			acceptType{
				MediaType:       "text/html",
				MediaTypeParams: params{"level": "1"},
				QualityFactor:   0.5,
			},
			qf1("application/json", params{"a": "b"}),
		)
		test("text/html; level=1; q=0.5, */*; q=0.1", nil,
			acceptType{
				MediaType:       "text/html",
				MediaTypeParams: params{"level": "1"},
				QualityFactor:   0.5,
			},
			acceptType{
				MediaType:       "*/*",
				MediaTypeParams: params{},
				QualityFactor:   0.1,
			},
		)
		test("text/html; level=1; q=0.5, */*; x=3;q=0.1; y=5", nil,
			acceptType{
				MediaType:       "text/html",
				MediaTypeParams: params{"level": "1"},
				QualityFactor:   0.5,
			},
			acceptType{
				MediaType:       "*/*",
				MediaTypeParams: params{"x": "3"},
				QualityFactor:   0.1,
				// AcceptType is not parsed.
			},
		)

		test("text/ html", "expected token after slash")
		test("text / html", "expected slash after first token")
		test("text/html, ", "no media type")
		test("text/html;;;;", "invalid media parameter")
		test("text/html;q=q", "q parameter: expected a floating-point number")
		test("text/html,a=q", "expected slash after first token")
	})

	Convey("qParamSplit", t, func() {
		type params map[string]string

		test := func(value, mediaType, qValue, acceptParams string) {
			Convey("value="+value, func() {
				actualMediaType, actualQValue, actualAcceptParams := qParamSplit(value)
				So(actualMediaType, ShouldEqual, mediaType)
				So(actualQValue, ShouldEqual, qValue)
				So(actualAcceptParams, ShouldEqual, acceptParams)
			})
		}

		test("", "", "", "")
		test("a/b", "a/b", "", "")

		test("a/b;mtp1=1", "a/b;mtp1=1", "", "")
		test("a/b;q=1", "a/b", "1", "")
		test("a/b; q=1", "a/b", "1", "")
		test("a/b;mtp1=1;q=1", "a/b;mtp1=1", "1", "")
		test("a/b;mtp1=1;q=1;", "a/b;mtp1=1", "1", "")
		test("a/b;mtp1=1; q=1", "a/b;mtp1=1", "1", "")
		test("a/b;mtp1=1;q =1", "a/b;mtp1=1", "1", "")
		test("a/b;mtp1=1;q  =1", "a/b;mtp1=1", "1", "")
		test("a/b;mtp1=1;q= 1", "a/b;mtp1=1", "1", "")
		test("a/b;mtp1=1;q=1 ", "a/b;mtp1=1", "1", "")

		test("a/b;mtp1=1;Q=1", "a/b;mtp1=1", "1", "")

		test("a/b;mtp1=1;q=1.0", "a/b;mtp1=1", "1.0", "")
		test("a/b;mtp1=1;q=1.0;", "a/b;mtp1=1", "1.0", "")

		test("a/b;mtp1=1;q=1;at1=1", "a/b;mtp1=1", "1", "at1=1")
		test("a/b;mtp1=1;q=1; at1=1", "a/b;mtp1=1", "1", "at1=1")
		test("a/b;mtp1=1;q=1;at1=1;", "a/b;mtp1=1", "1", "at1=1;")
		test("a/b;mtp1=1;  q  =  1.0  ;at1=1;", "a/b;mtp1=1", "1.0", "at1=1;")

		test("a/b;mtp1=1;  q  =  foo", "a/b;mtp1=1", "foo", "")

		test("a/b;c=q", "a/b;c=q", "", "")
		test("a/b;mtp1=1;  q", "a/b;mtp1=1;  q", "", "")
		test("a/b;mtp1=1;  q=", "a/b;mtp1=1;  q=", "", "")
	})
}
