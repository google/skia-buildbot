// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package clock

import (
	"testing"

	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"
)

func TestTags(t *testing.T) {
	Convey(`An empty Context`, t, func() {
		c := context.Background()

		Convey(`Should have nil tags.`, func() {
			So(Tags(c), ShouldBeNil)
		})

		Convey(`With tag, "A"`, func() {
			c = Tag(c, "A")

			Convey(`Should have tags {"A"}.`, func() {
				So(Tags(c), ShouldResemble, []string{"A"})
			})

			Convey(`And another tag, "B"`, func() {
				c = Tag(c, "B")

				Convey(`Should have tags {"A", "B"}.`, func() {
					So(Tags(c), ShouldResemble, []string{"A", "B"})
				})
			})
		})
	})
}
