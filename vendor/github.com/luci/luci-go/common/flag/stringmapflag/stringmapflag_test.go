// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package stringmapflag

import (
	"encoding/json"
	"flag"
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestValueFlag(t *testing.T) {
	Convey(`An empty Value`, t, func() {
		Convey(`When used as a flag`, func() {
			var tm Value
			fs := flag.NewFlagSet("Testing", flag.ContinueOnError)
			fs.Var(&tm, "tag", "Testing tag.")

			Convey(`Can successfully parse multiple parameters.`, func() {
				err := fs.Parse([]string{"-tag", "foo=FOO", "-tag", "bar=BAR", "-tag", "baz"})
				So(err, ShouldBeNil)
				So(tm, ShouldResemble, Value{"foo": "FOO", "bar": "BAR", "baz": ""})

				Convey(`Will build a correct string.`, func() {
					So(tm.String(), ShouldEqual, `bar=BAR,baz,foo=FOO`)
				})
			})

			Convey(`Will refuse to parse an empty key.`, func() {
				err := fs.Parse([]string{"-tag", "="})
				So(err, ShouldNotBeNil)
			})

			Convey(`Will refuse to parse an empty tag.`, func() {
				err := fs.Parse([]string{"-tag", ""})
				So(err, ShouldNotBeNil)
			})

			Convey(`Will refuse to parse duplicate tags.`, func() {
				err := fs.Parse([]string{"-tag", "foo=FOO", "-tag", "foo=BAR"})
				So(err, ShouldNotBeNil)
			})

		})
	})
}

func Example() {
	v := new(Value)
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Var(v, "option", "Appends a key[=value] option.")

	// Simulate user input.
	if err := fs.Parse([]string{
		"-option", "foo=bar",
		"-option", "baz",
	}); err != nil {
		panic(err)
	}

	m, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println("Parsed options:", string(m))

	// Output:
	// Parsed options: {
	//   "baz": "",
	//   "foo": "bar"
	// }
}
