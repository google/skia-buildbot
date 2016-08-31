// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package gaesecrets

import (
	"testing"

	"github.com/luci/gae/impl/memory"
	"github.com/luci/luci-go/server/secrets"
	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"
)

func TestWorks(t *testing.T) {
	Convey("gaesecrets.Store works", t, func() {
		c := Use(memory.Use(context.Background()), nil)

		// Autogenerates one.
		s1, err := secrets.GetSecret(c, "key1")
		So(err, ShouldBeNil)
		So(len(s1.Current.ID), ShouldEqual, 8)
		So(len(s1.Current.Blob), ShouldEqual, 32)

		// Returns same one.
		s2, err := secrets.GetSecret(c, "key1")
		So(err, ShouldBeNil)
		So(s2, ShouldResemble, s1)
	})
}
