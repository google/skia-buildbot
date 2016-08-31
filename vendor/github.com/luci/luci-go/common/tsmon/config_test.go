// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tsmon

import (
	"io/ioutil"
	"os"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestLoadConfig(t *testing.T) {
	Convey("Missing file", t, func() {
		c, err := loadConfig("/does/not/exist")
		So(c.Credentials, ShouldEqual, "")
		So(c.Endpoint, ShouldEqual, "")
		So(err, ShouldNotBeNil)
	})

	Convey("Empty file", t, func() {
		tf, err := ioutil.TempFile("", "config_test")
		if err != nil {
			t.Fail()
		}
		defer tf.Close()
		defer os.Remove(tf.Name())

		c, err := loadConfig(tf.Name())
		So(c.Endpoint, ShouldEqual, "")
		So(c.Credentials, ShouldEqual, "")
		So(c.AutoGenHostname, ShouldEqual, false)
		So(c.Hostname, ShouldEqual, "")
		So(c.Region, ShouldEqual, "")
		So(err, ShouldNotBeNil)
	})

	Convey("Full file", t, func() {
		tf, err := ioutil.TempFile("", "config_test")
		if err != nil {
			t.Fail()
		}
		defer tf.Close()
		defer os.Remove(tf.Name())

		tf.WriteString(`
			{"endpoint":         "foo",
			 "credentials":      "bar",
			 "autogen_hostname": true,
			 "hostname":         "test_host",
			 "region":           "test_region"
			}`)
		tf.Sync()

		c, err := loadConfig(tf.Name())
		So(c.Endpoint, ShouldEqual, "foo")
		So(c.Credentials, ShouldEqual, "bar")
		So(c.AutoGenHostname, ShouldEqual, true)
		So(c.Hostname, ShouldEqual, "test_host")
		So(c.Region, ShouldEqual, "test_region")
		So(err, ShouldBeNil)
	})
}
