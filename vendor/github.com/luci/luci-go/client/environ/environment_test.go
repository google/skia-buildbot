// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package environ

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestEnvironment(t *testing.T) {
	Convey(`Environment tests`, t, func() {
		Convey(`An empty enviornment array yields a nil enviornment.`, func() {
			So(Load([]string{}), ShouldBeNil)
		})

		Convey(`An environment consisting of KEY and KEY=VALUE pairs should load correctly.`, func() {
			So(Load([]string{
				"",
				"FOO",
				"BAR=",
				"BAZ=QUX",
				"=QUUX",
			}), ShouldResemble, Environment{
				"FOO": "",
				"BAR": "",
				"BAZ": "QUX",
			})
		})
	})
}
