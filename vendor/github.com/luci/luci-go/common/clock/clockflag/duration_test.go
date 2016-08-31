// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package clockflag

import (
	"encoding/json"
	"flag"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestDuration(t *testing.T) {
	t.Parallel()

	Convey(`A Duration flag`, t, func() {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		var d Duration
		fs.Var(&d, "duration", "Test duration parameter.")

		Convey(`Parses a 10-second Duration from "10s".`, func() {
			err := fs.Parse([]string{"-duration", "10s"})
			So(err, ShouldBeNil)
			So(d, ShouldEqual, time.Second*10)
			So(d.IsZero(), ShouldBeFalse)
		})

		Convey(`Returns an error when parsing "10z".`, func() {
			err := fs.Parse([]string{"-duration", "10z"})
			So(err, ShouldNotBeNil)
		})

		Convey(`When treated as a JSON field`, func() {
			var s struct {
				D Duration `json:"duration"`
			}

			testJSON := `{"duration":10}`
			Convey(`Fails to unmarshal from `+testJSON+`.`, func() {
				testJSON := testJSON
				err := json.Unmarshal([]byte(testJSON), &s)
				So(err, ShouldNotBeNil)
			})

			Convey(`Marshals correctly to duration string.`, func() {
				testDuration := (5 * time.Second) + (2 * time.Minute)
				s.D = Duration(testDuration)
				testJSON, err := json.Marshal(&s)
				So(err, ShouldBeNil)
				So(string(testJSON), ShouldEqual, `{"duration":"2m5s"}`)

				Convey(`And Unmarshals correctly.`, func() {
					s.D = Duration(0)
					err := json.Unmarshal([]byte(testJSON), &s)
					So(err, ShouldBeNil)
					So(s.D, ShouldEqual, testDuration)
				})
			})
		})
	})
}
