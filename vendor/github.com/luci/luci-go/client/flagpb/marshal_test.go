// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package flagpb

import (
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func msg(keysValues ...interface{}) map[string]interface{} {
	m := make(map[string]interface{}, len(keysValues)/2)
	for i := 0; i < len(keysValues); i += 2 {
		m[keysValues[i].(string)] = keysValues[i+1]
	}
	return m
}

func repeated(a ...interface{}) []interface{} {
	return a
}

func TestMarshal(t *testing.T) {
	t.Parallel()

	Convey("Marshal", t, func() {
		test := func(m map[string]interface{}, flags ...string) {
			Convey(strings.Join(flags, " "), func() {
				actualFlags, err := MarshalUntyped(m)
				So(err, ShouldBeNil)
				So(actualFlags, ShouldResemble, flags)
			})
		}

		test(nil)
		test(
			msg("x", 1),
			"-x", "1")
		test(
			msg("x", "a b"),
			"-x", "a b")
		test(
			msg("x", repeated(1, 2)),
			"-x", "1", "-x", "2")
		test(
			msg("b", true),
			"-b")
		test(
			msg("b", false),
			"-b=false")
		test(
			msg("m", msg("x", 1)),
			"-m.x", "1")
		test(
			msg(
				"m", repeated(msg("x", 1), msg("x", 2)),
			),
			"-m.x", "1", "-m", "-m.x", "2")
	})
}
