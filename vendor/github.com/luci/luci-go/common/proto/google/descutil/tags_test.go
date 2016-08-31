// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package descutil

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/golang/protobuf/proto"
	pb "google.golang.org/genproto/protobuf"

	. "github.com/smartystreets/goconvey/convey"
)

func TestTagsMatchProto(t *testing.T) {
	t.Parallel()

	propertiesForField := func(props *proto.StructProperties, name string) *proto.Properties {
		for _, p := range props.Prop {
			if p.Name == name {
				return p
			}
		}
		panic(fmt.Errorf("no property for %q", name))
	}

	Convey(`Hard-coded tag constants match protobuf.`, t, func() {
		Convey(`For FileDescriptorProto`, func() {
			props := proto.GetProperties(reflect.TypeOf(pb.FileDescriptorProto{}))

			So(propertiesForField(props, "Package").Tag, ShouldEqual, FileDescriptorProtoPackageTag)
			So(propertiesForField(props, "MessageType").Tag, ShouldEqual, FileDescriptorProtoMessageTag)
			So(propertiesForField(props, "EnumType").Tag, ShouldEqual, FileDescriptorProtoEnumTag)
			So(propertiesForField(props, "Service").Tag, ShouldEqual, FileDescriptorProtoServiceTag)
		})

		Convey(`For ServiceDescriptorProto`, func() {
			props := proto.GetProperties(reflect.TypeOf(pb.ServiceDescriptorProto{}))

			So(propertiesForField(props, "Method").Tag, ShouldEqual, ServiceDescriptorProtoMethodTag)
		})

		Convey(`For DescriptorProto`, func() {
			props := proto.GetProperties(reflect.TypeOf(pb.DescriptorProto{}))

			So(propertiesForField(props, "Field").Tag, ShouldEqual, DescriptorProtoFieldTag)
			So(propertiesForField(props, "NestedType").Tag, ShouldEqual, DescriptorProtoNestedTypeTag)
			So(propertiesForField(props, "EnumType").Tag, ShouldEqual, DescriptorProtoEnumTypeTag)
			So(propertiesForField(props, "OneofDecl").Tag, ShouldEqual, DescriptorProtoOneOfTag)
		})

		Convey(`For EnumDescriptorProto`, func() {
			props := proto.GetProperties(reflect.TypeOf(pb.EnumDescriptorProto{}))

			So(propertiesForField(props, "Value").Tag, ShouldEqual, EnumDescriptorProtoValueTag)
		})
	})
}
