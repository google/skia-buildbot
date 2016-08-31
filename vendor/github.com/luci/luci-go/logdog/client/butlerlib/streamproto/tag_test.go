// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package streamproto

import (
	"encoding/json"
	"flag"
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestTagMapFlag(t *testing.T) {
	Convey(`An empty TagMap`, t, func() {
		tm := TagMap{}

		Convey(`SortedKeys will return nil.`, func() {
			So(tm.SortedKeys(), ShouldBeNil)
		})

		Convey(`When used as a flag`, func() {
			fs := flag.NewFlagSet("Testing", flag.ContinueOnError)
			fs.Var(&tm, "tag", "Testing tag.")

			Convey(`Can successfully parse multiple parameters.`, func() {
				err := fs.Parse([]string{"-tag", "foo=FOO", "-tag", "bar=BAR", "-tag", "baz"})
				So(err, ShouldBeNil)
				So(tm, ShouldResemble, TagMap{"foo": "FOO", "bar": "BAR", "baz": ""})

				Convey(`Will build a correct string.`, func() {
					So(tm.String(), ShouldEqual, `bar=BAR,baz,foo=FOO`)
				})
			})

			Convey(`Loaded with {"foo": "bar", "baz": "qux"}`, func() {
				tm["foo"] = "bar"
				tm["baz"] = "qux"

				Convey(`Can be converted into JSON.`, func() {
					d, err := json.Marshal(&tm)
					So(err, ShouldBeNil)
					So(string(d), ShouldEqual, `{"baz":"qux","foo":"bar"}`)

					Convey(`And can be unmarshalled from JSON.`, func() {
						tm := TagMap{}
						err := json.Unmarshal(d, &tm)
						So(err, ShouldBeNil)

						So(tm, ShouldResemble, TagMap{
							"foo": "bar",
							"baz": "qux",
						})
					})
				})
			})

			Convey(`An empty TagMap JSON will unmarshal into nil.`, func() {
				tm := TagMap{}
				err := json.Unmarshal([]byte(`{}`), &tm)
				So(err, ShouldBeNil)
				So(tm, ShouldBeNil)
			})

			for _, s := range []string{
				`[{"woot": "invalid"}]`,
				`[{123: abc}]`,
				`[{"key": "invalidl;tag;name"}]`,
			} {
				Convey(fmt.Sprintf(`Invalid TagMap JSON will fail: %q`, s), func() {
					tm := TagMap{}
					err := json.Unmarshal([]byte(s), &tm)
					So(err, ShouldNotBeNil)
				})
			}
		})
	})
}
