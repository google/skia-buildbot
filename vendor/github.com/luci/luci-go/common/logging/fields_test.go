// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package logging

import (
	"errors"
	"fmt"
	"sort"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

type stringStruct struct {
	Value string
}

var _ fmt.Stringer = (*stringStruct)(nil)

func (s *stringStruct) String() string {
	return s.Value
}

// TestFieldEntry tests methods associated with the FieldEntry and
// fieldEntrySlice types.
func TestFieldEntry(t *testing.T) {
	Convey(`A FieldEntry instance: "value" => "\"Hello, World!\""`, t, func() {
		fe := FieldEntry{"value", `"Hello, World!"`}

		Convey(`Has a String() value, "value":"\"Hello, World!\"".`, func() {
			So(fe.String(), ShouldEqual, `"value":"\"Hello, World!\""`)
		})
	})

	Convey(`A FieldEntry instance: "value" => 42`, t, func() {
		fe := FieldEntry{"value", 42}

		Convey(`Has a String() value, "value":"42".`, func() {
			So(fe.String(), ShouldEqual, `"value":42`)
		})
	})

	Convey(`A FieldEntry instance: "value" => stringStruct{"My \"value\""}`, t, func() {
		fe := FieldEntry{"value", &stringStruct{`My "value"`}}

		Convey(`Has a String() value, "value":"My \"value\"".`, func() {
			So(fe.String(), ShouldEqual, `"value":"My \"value\""`)
		})
	})

	Convey(`A FieldEntry instance: "value" => error{"There was a \"failure\"}`, t, func() {
		fe := FieldEntry{"value", errors.New(`There was a "failure"`)}

		Convey(`Has a String() value, "value":"There was a \"failure\"".`, func() {
			So(fe.String(), ShouldEqual, `"value":"There was a \"failure\""`)
		})
	})

	Convey(`A FieldEntry instance: "value" => struct{a: "Hello!", b: 42}`, t, func() {
		type myStruct struct {
			a string
			b int
		}
		fe := FieldEntry{"value", &myStruct{"Hello!", 42}}

		Convey(`Has a String() value, "value":myStruct { a: "Hello!", b: 42 }".`, func() {
			So(fe.String(), ShouldEqual, `"value":&logging.myStruct{a:"Hello!", b:42}`)
		})
	})

	Convey(`A fieldEntrySlice: {foo/bar, error/z, asdf/baz}`, t, func() {
		fes := fieldEntrySlice{
			&FieldEntry{"foo", "bar"},
			&FieldEntry{ErrorKey, errors.New("z")},
			&FieldEntry{"asdf", "baz"},
		}

		Convey(`Should be sorted: [error, asdf, foo].`, func() {
			sorted := make(fieldEntrySlice, len(fes))
			copy(sorted, fes)
			sort.Sort(sorted)

			So(sorted, ShouldResemble, fieldEntrySlice{fes[1], fes[2], fes[0]})
		})
	})
}

func TestFields(t *testing.T) {
	Convey(`A nil Fields`, t, func() {
		fm := Fields(nil)

		Convey(`Returns nil when Copied with an empty Fields.`, func() {
			So(fm.Copy(Fields{}), ShouldBeNil)
		})

		Convey(`Returns a populated Fields when Copied with a populated Fields.`, func() {
			other := Fields{
				"foo": "bar",
				"baz": "qux",
			}
			So(fm.Copy(other), ShouldResemble, Fields{"foo": "bar", "baz": "qux"})
		})

		Convey(`Returns the populated Fields when Copied with a populated Fields.`, func() {
			other := Fields{
				"foo": "bar",
				"baz": "qux",
			}
			So(fm.Copy(other), ShouldResemble, other)
		})
	})

	Convey(`A populated Fields`, t, func() {
		fm := NewFields(map[string]interface{}{
			"foo": "bar",
			"baz": "qux",
		})
		So(fm, ShouldHaveSameTypeAs, Fields(nil))

		Convey(`Returns an augmented Fields when Copied with a populated Fields.`, func() {
			other := Fields{
				ErrorKey: errors.New("err"),
			}
			So(fm.Copy(other), ShouldResemble, Fields{"foo": "bar", "baz": "qux", ErrorKey: errors.New("err")})
		})

		Convey(`Has a String representation: {"baz":"qux", "foo":"bar"}`, func() {
			So(fm.String(), ShouldEqual, `{"baz":"qux", "foo":"bar"}`)
		})
	})
}

func TestContextFields(t *testing.T) {
	Convey(`An empty Context`, t, func() {
		c := context.Background()

		Convey(`Has no Fields.`, func() {
			So(GetFields(c), ShouldBeNil)
		})

		Convey(`Sets {"foo": "bar", "baz": "qux"}`, func() {
			c = SetFields(c, Fields{
				"foo": "bar",
				"baz": "qux",
			})
			So(GetFields(c), ShouldResemble, Fields{
				"foo": "bar",
				"baz": "qux",
			})

			Convey(`Is overridden by: {"foo": "override", "error": "failure"}`, func() {
				c = SetFields(c, Fields{
					"foo":   "override",
					"error": errors.New("failure"),
				})

				So(GetFields(c), ShouldResemble, Fields{
					"foo":   "override",
					"baz":   "qux",
					"error": errors.New("failure"),
				})
			})
		})
	})
}
