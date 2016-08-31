// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package descutil

import (
	"testing"

	"io/ioutil"

	"github.com/golang/protobuf/proto"
	. "github.com/smartystreets/goconvey/convey"
	pb "google.golang.org/genproto/protobuf"
)

func TestUtil(t *testing.T) {
	t.Parallel()

	Convey("Util", t, func() {
		descFileBytes, err := ioutil.ReadFile("util_test.desc")
		So(err, ShouldBeNil)

		var desc pb.FileDescriptorSet
		err = proto.Unmarshal(descFileBytes, &desc)
		So(err, ShouldBeNil)

		So(desc.File, ShouldHaveLength, 1)
		file := desc.File[0]
		So(file.GetName(), ShouldEqual, "github.com/luci/luci-go/common/proto/google/descutil/util_test.proto")

		Convey("Resolve works", func() {
			names := []string{
				"descutil.E1",
				"descutil.E1.V0",

				"descutil.M1",
				"descutil.M1.f1",

				"descutil.M2.f1",
				"descutil.M2.f2",

				"descutil.M3.O1",
				"descutil.M3.f1",
				"descutil.M3.O2",

				"descutil.S1",
				"descutil.S1.R1",
				"descutil.S2.R2",

				"descutil.NestedMessageParent",
				"descutil.NestedMessageParent.NestedMessage",
				"descutil.NestedMessageParent.NestedMessage.f1",
				"descutil.NestedMessageParent.NestedEnum",
				"descutil.NestedMessageParent.NestedEnum.V0",
			}
			for _, n := range names {
				Convey(n, func() {
					actualFile, obj, _ := Resolve(&desc, n)
					So(actualFile, ShouldEqual, file)
					So(obj, ShouldNotBeNil)
				})
			}

			Convey("wrong name", func() {
				actualFile, obj, path := Resolve(&desc, "foo")
				So(actualFile, ShouldBeNil)
				So(obj, ShouldBeNil)
				So(path, ShouldBeNil)
			})
		})
	})
}
