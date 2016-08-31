// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package identity

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestIdentity(t *testing.T) {
	Convey("MakeIdentity works", t, func() {
		id, err := MakeIdentity("anonymous:anonymous")
		So(err, ShouldBeNil)
		So(id, ShouldEqual, Identity("anonymous:anonymous"))
		So(id.Kind(), ShouldEqual, Anonymous)
		So(id.Value(), ShouldEqual, "anonymous")
		So(id.Email(), ShouldEqual, "")

		_, err = MakeIdentity("bad ident")
		So(err, ShouldNotBeNil)
	})

	Convey("Validate works", t, func() {
		So(Identity("user:abc@example.com").Validate(), ShouldBeNil)
		So(Identity("user:").Validate(), ShouldNotBeNil)
		So(Identity(":abc").Validate(), ShouldNotBeNil)
		So(Identity("abc@example.com").Validate(), ShouldNotBeNil)
		So(Identity("user:abc").Validate(), ShouldNotBeNil)
	})

	Convey("Kind works", t, func() {
		So(Identity("user:abc@example.com").Kind(), ShouldEqual, User)
		So(Identity("???").Kind(), ShouldEqual, Anonymous)
	})

	Convey("Value works", t, func() {
		So(Identity("service:abc").Value(), ShouldEqual, "abc")
		So(Identity("???").Value(), ShouldEqual, "anonymous")
	})

	Convey("Email works", t, func() {
		So(Identity("user:abc@example.com").Email(), ShouldEqual, "abc@example.com")
		So(Identity("service:abc").Email(), ShouldEqual, "")
		So(Identity("???").Email(), ShouldEqual, "")
	})
}
