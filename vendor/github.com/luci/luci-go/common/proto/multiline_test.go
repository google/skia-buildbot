// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package proto

import (
	"testing"

	. "github.com/luci/luci-go/common/testing/assertions"

	. "github.com/smartystreets/goconvey/convey"
)

func TestMultiline(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		name, expect, data string
	}{
		{"basic", `something: "this\000 \t is\n\na \"basic\" test\\example"`, `something: << EOF
		  this` + "\x00" + ` 	 is

		  a "basic" test\example
		EOF`},
		{"contained", `something: "A << B"`, `something: "A << B"`},
		{"indent", `something: "this\n  is indented\n\nwith empty line"`, `something: << EOF
			this
			  is indented
	` + /* this prevents editors from eating whitespace */ `
			with empty line
		EOF`},
		{"col 0 align", `something: "this\n  is indented\n\nwith empty line"`, `something: << EOF
this
  is indented

with empty line
		EOF`},
		{"nested", `something: "<< nerp\nOther\nnerp\nfoo"`, `something: << EOF
		<< nerp
		Other
		nerp
		foo
		EOF`},
		{"multi", `something: "this is something"
			else: "this is else"`, `something: << EOF
		this is something
		EOF
			else: << ELSE
			this is else
			ELSE`},
		{"indented first line", `something: "  this line\nis indented\n  this too"`, `something: <<DERP
		  this line
		is indented
		  this too
		DERP`},
		{"mixed indents are not indents", `something: "\ttab\n  spaces"`, `something: <<DERP
			tab
		  spaces
		DERP`},
	}

	Convey("Test ParseMultilineStrings", t, func() {
		for _, tc := range tcs {
			Convey(tc.name, func() {
				data, err := ParseMultilineStrings(tc.data)
				So(err, ShouldBeNil)
				So(data, ShouldEqual, tc.expect)
			})
		}

		Convey("missing terminator", func() {
			_, err := ParseMultilineStrings(`<<DERP
			Some stuff
			`)
			So(err, ShouldErrLike, `failed to find matching terminator "DERP"`)
		})
	})
}
