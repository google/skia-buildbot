// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package iam

import (
	"encoding/json"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestPolicy(t *testing.T) {
	Convey("Serializing empty structs", t, func() {
		p := Policy{}

		So(json.Unmarshal([]byte("{}"), &p), ShouldBeNil)
		So(p, ShouldResemble, Policy{
			Bindings:           make(PolicyBindings),
			UnrecognizedFields: make(map[string]*json.RawMessage),
		})

		blob, err := json.Marshal(p)
		So(err, ShouldBeNil)
		So(string(blob), ShouldEqual, `{"bindings":[],"etag":""}`)

		blob, err = json.Marshal(Policy{}) // with nils
		So(err, ShouldBeNil)
		So(string(blob), ShouldEqual, `{"bindings":[],"etag":""}`)
	})

	Convey("Serializing back and forth", t, func() {
		p := Policy{}

		data := `{
			"bindings": [
				{"role": "role1", "members": ["a", "b", "c"]}
			],
			"etag": "abcdef",
			"unknown1": "blah",
			"unknown2": ["blah"],
			"unknown3": {"blah": ["blah"]}
		}`
		So(json.Unmarshal([]byte(data), &p), ShouldBeNil)

		blob, err := json.Marshal(p)
		So(err, ShouldBeNil)
		So(string(blob), ShouldEqual,
			`{"bindings":[{"role":"role1","members":["a","b","c"]}],"etag":"abcdef",`+
				`"unknown1":"blah","unknown2":["blah"],"unknown3":{"blah":["blah"]}}`)
	})

	Convey("GrantRole and RevokeRole work", t, func() {
		p := Policy{}

		p.GrantRole("role") // edge case, empty list of principals
		So(p, ShouldResemble, Policy{})

		p.RevokeRole("role") // same
		So(p, ShouldResemble, Policy{})

		p.GrantRole("role", "a", "a", "b", "c")
		So(p.Bindings, ShouldResemble, PolicyBindings{
			"role": membersSet{"a": struct{}{}, "b": struct{}{}, "c": struct{}{}},
		})

		p.RevokeRole("unknown", "a")
		p.RevokeRole("role", "b", "b", "b")
		So(p.Bindings, ShouldResemble, PolicyBindings{
			"role": membersSet{"a": struct{}{}, "c": struct{}{}},
		})
	})

	Convey("Equals work", t, func() {
		p1 := Policy{}
		p2 := Policy{}

		p1.GrantRole("role", "a", "b", "c")
		p2.GrantRole("role", "a", "d", "e")
		So(p1.Equals(p2), ShouldBeFalse)
		So(p2.Equals(p1), ShouldBeFalse)

		p2.RevokeRole("role", "d", "e")
		p2.GrantRole("role", "b", "c")
		So(p1.Equals(p2), ShouldBeTrue)
		So(p2.Equals(p1), ShouldBeTrue)

		p2.Etag = "blah"
		So(p1.Equals(p2), ShouldBeFalse)

		blob := json.RawMessage(nil)

		p2.Etag = ""
		p2.UnrecognizedFields = map[string]*json.RawMessage{"blah": &blob}
		So(p1.Equals(p2), ShouldBeFalse)

		p1.UnrecognizedFields = map[string]*json.RawMessage{"blah": &blob}
		So(p1.Equals(p2), ShouldBeTrue)
	})

	Convey("Clone works", t, func() {
		p1 := Policy{
			Etag:               "blah",
			UnrecognizedFields: map[string]*json.RawMessage{"blah": nil},
		}

		p2 := p1.Clone()
		So(p1, ShouldNotEqual, p2)
		So(p1.Equals(p2), ShouldBeTrue)

		p1.GrantRole("role", "a", "b", "c")

		p2 = p1.Clone()
		So(p1, ShouldNotEqual, p2)
		So(p1.Equals(p2), ShouldBeTrue)
	})
}
