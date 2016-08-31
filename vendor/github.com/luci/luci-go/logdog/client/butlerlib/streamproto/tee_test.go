// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package streamproto

import (
	"encoding/json"
	"flag"
	"math"
	"os"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

// Test the TeeType struct.
func TestTeeType(t *testing.T) {
	Convey(`A TeeNone type should return a nil writer`, t, func() {
		So(TeeNone.Writer(), ShouldBeNil)
	})

	Convey(`A TeeStdout type should return STDOUT file.`, t, func() {
		So(TeeStdout.Writer(), ShouldEqual, os.Stdout)
	})

	Convey(`A TeeStderr type should return STDERR file.`, t, func() {
		So(TeeStderr.Writer(), ShouldEqual, os.Stderr)
	})

	Convey(`An invalid value should panic.`, t, func() {
		So(func() { TeeType(math.MaxUint32).Writer() }, ShouldPanic)
	})
}

func TestTeeTypeFlag(t *testing.T) {
	Convey(`A TeeType flag`, t, func() {
		var value TeeType

		fs := flag.NewFlagSet("Testing", flag.ContinueOnError)
		fs.Var(&value, "tee_type", "TeeType test.")

		Convey(`Can be loaded as a flag.`, func() {
			err := fs.Parse([]string{"-tee_type", "stdout"})
			So(err, ShouldBeNil)
			So(value, ShouldEqual, TeeStdout)
		})

		Convey(`Will unmmarshal from JSON.`, func() {
			var s struct {
				Value TeeType `json:"value"`
			}

			err := json.Unmarshal([]byte(`{"value":"stderr"}`), &s)
			So(err, ShouldBeNil)
			So(s.Value, ShouldEqual, TeeStderr)
		})

		Convey(`Will marshal to JSON.`, func() {
			var s struct {
				Value TeeType `json:"value"`
			}
			s.Value = TeeNone

			v, err := json.Marshal(&s)
			So(err, ShouldBeNil)
			So(string(v), ShouldEqual, `{"value":"none"}`)
		})
	})
}
