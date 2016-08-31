// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package hierarchy

import (
	"fmt"
	"testing"

	"github.com/luci/luci-go/logdog/common/types"
	. "github.com/smartystreets/goconvey/convey"
)

func TestComponents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path types.StreamPath
		full bool
		comp []*Component
	}{
		{path: "", full: false, comp: nil},
		{path: "foo/bar/+/baz", full: true, comp: []*Component{
			{"foo/bar/+", "baz", true},
			{"foo/bar", "+", false},
			{"foo", "bar", false},
			{"", "foo", false},
		}},
		{path: "foo/bar/+", full: false, comp: []*Component{
			{"foo/bar", "+", false},
			{"foo", "bar", false},
			{"", "foo", false},
		}},
	}

	Convey(`Testing MakeComponents`, t, func() {
		for _, tc := range tests {
			Convey(fmt.Sprintf(`Path %q yields %d component(s)`, tc.path, len(tc.comp)), func() {
				So(Components(tc.path, tc.full), ShouldResemble, tc.comp)
			})
		}
	})
}

func TestComponentID(t *testing.T) {
	t.Parallel()

	encodingTestCases := []struct {
		cid     componentID
		encoded string
	}{
		{componentID{"foo", "0", false}, "y~8000~foo"},
		{componentID{"foo", "0", true}, "n~8000~foo"},
		{componentID{"foo", "00", true}, "n~8000:8080~foo"},
		{componentID{"foo", "003", true}, "n~81c0:8180~foo"},
		{componentID{"foo", "bar", true}, "s~bar~foo"},
	}
	Convey(`Component ID Encoding`, t, func() {
		for _, tc := range encodingTestCases {
			Convey(fmt.Sprintf(`%+v encodes to %q`, tc.cid, tc.encoded), func() {
				So(tc.cid.key(), ShouldResemble, tc.encoded)

				var cid componentID
				cid.setID(tc.encoded)
				So(cid, ShouldResemble, tc.cid)
			})
		}
	})

	decodingTestCases := []struct {
		encoded string
		cid     componentID
	}{
		{"y~8000:8000~foo", componentID{"foo", "0", false}},
		{"y~8000~foo", componentID{"foo", "0", false}},
		{"n~8000:8080~foo", componentID{"foo", "00", true}},
	}
	Convey(`Component ID Decoding`, t, func() {
		for _, tc := range decodingTestCases {
			Convey(fmt.Sprintf(`%q decodes to %+v`, tc.encoded, tc.cid), func() {
				var cid componentID
				cid.setID(tc.encoded)
				So(cid, ShouldResemble, tc.cid)
			})
		}
	})
}
