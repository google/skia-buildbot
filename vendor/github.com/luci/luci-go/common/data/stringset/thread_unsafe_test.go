// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package stringset

import (
	"sort"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestThreadUnsafeSet(t *testing.T) {
	t.Parallel()

	Convey("Test Thread Unsafe Set", t, func() {
		s := New(10)

		Convey("Can add elements", func() {
			So(s.Add("hello"), ShouldBeTrue)
			So(s.Add("hello"), ShouldBeFalse)
			So(s.Add("world"), ShouldBeTrue)
			So(s.Len(), ShouldEqual, 2)
			sl := s.ToSlice()
			sort.Strings(sl)
			So(sl, ShouldResemble, []string{"hello", "world"})

			Convey("Can remove stuff", func() {
				So(s.Del("foo"), ShouldBeFalse)
				So(s.Del("world"), ShouldBeTrue)
				So(s.Del("world"), ShouldBeFalse)
				So(s.ToSlice(), ShouldResemble, []string{"hello"})
			})

			Convey("Can peek them", func() {
				str, found := s.Peek()
				So(found, ShouldBeTrue)
				So(sl, ShouldContain, str)

				_, found = New(10).Peek()
				So(found, ShouldBeFalse)
			})

			Convey("Can pop them", func() {
				newList := []string{}
				str, found := s.Pop()
				So(found, ShouldBeTrue)
				newList = append(newList, str)

				str, found = s.Pop()
				So(found, ShouldBeTrue)
				newList = append(newList, str)

				_, found = s.Pop()
				So(found, ShouldBeFalse)
				So(s.Len(), ShouldEqual, 0)

				sort.Strings(newList)

				So(newList, ShouldResemble, sl)
			})

			Convey("Can iterate", func() {
				s.Iter(func(val string) bool {
					So(sl, ShouldContain, val)
					return true
				})
			})

			Convey("Can stop iteration early", func() {
				foundOne := false
				s.Iter(func(val string) bool {
					So(foundOne, ShouldBeFalse)
					foundOne = true
					So(sl, ShouldContain, val)
					return false
				})
			})

			Convey("Can dup them", func() {
				dup := s.Dup()
				So(dup, ShouldResemble, s)
				So(dup, ShouldNotEqual, s)
				dup.Add("panwaffles") // the best of both!
				So(dup, ShouldNotResemble, s)
			})
		})
	})

	Convey("Can create with pre-set values", t, func() {
		s := NewFromSlice("hi", "there", "person", "hi")
		So(s.Len(), ShouldEqual, 3)
		So(s.Has("hi"), ShouldBeTrue)
		sl := s.ToSlice()
		sort.Strings(sl)
		So(sl, ShouldResemble, []string{"hi", "person", "there"})
	})
}
