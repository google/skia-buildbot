// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package streamproto

import (
	"encoding/json"
	"flag"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestStreamName(t *testing.T) {
	Convey(`A StreamNameFlag`, t, func() {
		sn := StreamNameFlag("")

		Convey(`When attached to a flag.`, func() {
			fs := flag.FlagSet{}
			fs.Var(&sn, "name", "Test stream name.")

			Convey(`Will successfully parse a valid stream name.`, func() {
				err := fs.Parse([]string{"-name", "test/stream/name"})
				So(err, ShouldBeNil)
				So(sn, ShouldEqual, "test/stream/name")
			})

			Convey(`Will fail to parse an invalid stream name.`, func() {
				err := fs.Parse([]string{"-name", "test;stream;name"})
				So(err, ShouldNotBeNil)
			})
		})

		Convey(`With valid value "test/stream/name"`, func() {
			Convey(`Will marshal into a JSON string.`, func() {
				sn = "test/stream/name"
				d, err := json.Marshal(&sn)
				So(err, ShouldBeNil)
				So(string(d), ShouldEqual, `"test/stream/name"`)

				Convey(`And successfully unmarshal.`, func() {
					sn = ""
					err := json.Unmarshal(d, &sn)
					So(err, ShouldBeNil)
					So(sn, ShouldEqual, "test/stream/name")
				})
			})
		})

		Convey(`JSON with an invalid value "test;stream;name" will fail to Unmarshal.`, func() {
			sn := StreamNameFlag("")
			err := json.Unmarshal([]byte(`"test;stream;name"`), &sn)
			So(err, ShouldNotBeNil)
		})
	})
}
