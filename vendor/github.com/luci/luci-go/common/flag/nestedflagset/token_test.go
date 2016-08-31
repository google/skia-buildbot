// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package nestedflagset

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestToken(t *testing.T) {
	Convey(`An empty token`, t, func() {
		tok := token("")

		Convey(`When split`, func() {
			name, value := tok.split()

			Convey(`The "name" field should be zero-valued`, func() {
				So(name, ShouldEqual, "")
			})

			Convey(`The "value" field should be zero-valued`, func() {
				So(value, ShouldEqual, "")
			})
		})
	})

	Convey(`A token whose value is "name"`, t, func() {
		tok := token("name")

		Convey(`When split`, func() {
			name, value := tok.split()

			Convey(`The "name" field should be "name"`, func() {
				So(name, ShouldEqual, "name")
			})

			Convey(`The "value" field should be zero-valued`, func() {
				So(value, ShouldEqual, "")
			})
		})
	})

	Convey(`A token whose value is "name=value"`, t, func() {
		tok := token("name=value")

		Convey(`When split`, func() {
			name, value := tok.split()

			Convey(`The "name" field should be "name"`, func() {
				So(name, ShouldEqual, "name")
			})

			Convey(`The "value" field should be "value"`, func() {
				So(value, ShouldEqual, "value")
			})
		})
	})

	Convey(`A token whose value is "name=value=1=2=3"`, t, func() {
		tok := token("name=value=1=2=3")

		Convey(`When split`, func() {
			name, value := tok.split()

			Convey(`The "name" field should be "name"`, func() {
				So(name, ShouldEqual, "name")
			})

			Convey(`The "value" field should "value=1=2=3"`, func() {
				So(value, ShouldEqual, "value=1=2=3")
			})
		})
	})
}
