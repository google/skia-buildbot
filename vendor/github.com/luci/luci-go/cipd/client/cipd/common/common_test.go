// Copyright 2014 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package common

import (
	"fmt"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestValidatePackageName(t *testing.T) {
	Convey("ValidatePackageName works", t, func() {
		So(ValidatePackageName("good/name"), ShouldBeNil)
		So(ValidatePackageName("good_name"), ShouldBeNil)
		So(ValidatePackageName("123-_/also/good/name"), ShouldBeNil)
		So(ValidatePackageName(""), ShouldNotBeNil)
		So(ValidatePackageName("BAD/name"), ShouldNotBeNil)
		So(ValidatePackageName("bad//name"), ShouldNotBeNil)
		So(ValidatePackageName("bad/name/"), ShouldNotBeNil)
		So(ValidatePackageName("bad.name"), ShouldNotBeNil)
		So(ValidatePackageName("/bad/name"), ShouldNotBeNil)
		So(ValidatePackageName("bad/name\nyeah"), ShouldNotBeNil)
		So(ValidatePackageName("../../yeah"), ShouldNotBeNil)
	})
}

func TestValidateInstanceID(t *testing.T) {
	Convey("ValidateInstanceID works", t, func() {
		So(ValidateInstanceID(""), ShouldNotBeNil)
		So(ValidateInstanceID("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), ShouldBeNil)
		So(ValidateInstanceID("0123456789abcdefaaaaaaaaaaaaaaaaaaaaaaaa"), ShouldBeNil)
		So(ValidateInstanceID("€aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), ShouldNotBeNil)
		So(ValidateInstanceID("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), ShouldNotBeNil)
		So(ValidateInstanceID("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), ShouldNotBeNil)
		So(ValidateInstanceID("gaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), ShouldNotBeNil)
		So(ValidateInstanceID("AAAaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), ShouldNotBeNil)
	})
}

func TestValidateInstanceTag(t *testing.T) {
	Convey("ValidateInstanceTag works", t, func() {
		So(ValidateInstanceTag(""), ShouldNotBeNil)
		So(ValidateInstanceTag("notapair"), ShouldNotBeNil)
		So(ValidateInstanceTag(strings.Repeat("long", 200)+":abc"), ShouldNotBeNil)
		So(ValidateInstanceTag("BADKEY:value"), ShouldNotBeNil)
		So(ValidateInstanceTag("good:tag"), ShouldBeNil)
		So(ValidateInstanceTag("good_tag:"), ShouldBeNil)
		So(ValidateInstanceTag("good:tag:blah"), ShouldBeNil)
		So(ValidateInstanceTag("good_tag:asdad/asdad/adad/a\\asdasdad"), ShouldBeNil)
	})
}

func TestValidatePin(t *testing.T) {
	Convey("ValidatePin works", t, func() {
		So(ValidatePin(Pin{"good/name", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}), ShouldBeNil)
		So(ValidatePin(Pin{"BAD/name", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}), ShouldNotBeNil)
		So(ValidatePin(Pin{"good/name", "aaaaaaaaaaa"}), ShouldNotBeNil)
	})
}

func TestValidatePackageRef(t *testing.T) {
	Convey("ValidatePackageRef works", t, func() {
		So(ValidatePackageRef("some-ref"), ShouldBeNil)
		So(ValidatePackageRef(""), ShouldNotBeNil)
		So(ValidatePackageRef("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), ShouldNotBeNil)
		So(ValidatePackageRef("good:tag"), ShouldNotBeNil)
	})
}

func TestValidateInstanceVersion(t *testing.T) {
	Convey("ValidateInstanceVersion works", t, func() {
		So(ValidateInstanceVersion("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), ShouldBeNil)
		So(ValidateInstanceVersion("good:tag"), ShouldBeNil)
		So(ValidatePackageRef("some-read"), ShouldBeNil)
		So(ValidateInstanceVersion("BADTAG:"), ShouldNotBeNil)
	})
}

func TestPinToString(t *testing.T) {
	Convey("Pin.String works", t, func() {
		So(
			fmt.Sprintf("%s", Pin{"good/name", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}),
			ShouldEqual,
			"good/name:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	})
}

func TestGetInstanceTagKey(t *testing.T) {
	Convey("GetInstanceTagKey works", t, func() {
		So(GetInstanceTagKey("a:b"), ShouldEqual, "a")
		So(GetInstanceTagKey("a:b:c"), ShouldEqual, "a")
		So(GetInstanceTagKey(":b"), ShouldEqual, "")
		So(GetInstanceTagKey(""), ShouldEqual, "")
	})
}
