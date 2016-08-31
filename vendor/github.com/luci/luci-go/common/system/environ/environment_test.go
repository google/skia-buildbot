// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package environ

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestEnvironmentConversion(t *testing.T) {
	t.Parallel()

	Convey(`Source environment slice translates correctly to/from an Env.`, t, func() {
		env := New([]string{
			"",
			"FOO",
			"BAR=BAZ",
			"bar=baz",
			"qux=quux=quuuuuuux",
		})
		So(env, ShouldResemble, Env{
			"":    "",
			"FOO": "FOO",
			"BAR": "BAR=BAZ",
			"bar": "bar=baz",
			"qux": "qux=quux=quuuuuuux",
		})

		So(env.Sorted(), ShouldResemble, []string{
			"",
			"BAR=BAZ",
			"FOO",
			"bar=baz",
			"qux=quux=quuuuuuux",
		})
	})
}

func TestEnvironmentManipulation(t *testing.T) {
	t.Parallel()

	Convey(`A testing Env`, t, func() {
		env := Env{
			"PYTHONPATH": "PYTHONPATH=/foo:/bar:/baz",
			"http_proxy": "http_proxy=http://example.com",
			"novalue":    "novalue",
		}

		Convey(`Can Get values.`, func() {
			v, ok := env.Get("PYTHONPATH")
			So(ok, ShouldBeTrue)
			So(v, ShouldEqual, "/foo:/bar:/baz")

			v, ok = env.Get("http_proxy")
			So(ok, ShouldBeTrue)
			So(v, ShouldEqual, "http://example.com")

			v, ok = env.Get("novalue")
			So(ok, ShouldBeTrue)
			So(v, ShouldEqual, "")
		})

		Convey(`Will note missing values.`, func() {
			_, ok := env.Get("pythonpath")
			So(ok, ShouldBeFalse)

			_, ok = env.Get("")
			So(ok, ShouldBeFalse)
		})

		Convey(`Can update its values.`, func() {
			orig := env.Clone()

			// Update PYTHONPATH, confirm that it updated correctly.
			v, _ := env.Get("PYTHONPATH")
			env.Set("PYTHONPATH", "/override:"+v)
			So(env.Sorted(), ShouldResemble, []string{
				"PYTHONPATH=/override:/foo:/bar:/baz",
				"http_proxy=http://example.com",
				"novalue",
			})

			// Test that the clone didn't change.
			So(orig.Sorted(), ShouldResemble, []string{
				"PYTHONPATH=/foo:/bar:/baz",
				"http_proxy=http://example.com",
				"novalue",
			})
		})
	})
}
