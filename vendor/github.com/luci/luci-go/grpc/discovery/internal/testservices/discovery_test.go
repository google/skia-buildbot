// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package testservices

// This test is not in discovery because it needs to a test services
// in different directories.
// However, a generated service depends on grpc/discovery
// and a test in grpc/discovery depends on the service.
// This creates a a cyclic import.
// To break the cycle, we move this test from grpc/discovery.

import (
	"testing"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/proto/google/descutil"
	"github.com/luci/luci-go/grpc/discovery"

	"google.golang.org/genproto/protobuf"

	. "github.com/smartystreets/goconvey/convey"
)

// force test services registration.
var _ = CalcServer(nil)

func TestDiscovery(t *testing.T) {
	Convey("Discovery", t, func() {

		server, err := discovery.New(
			"discovery.Discovery",
			"testservices.Greeter",
			"testservices.Calc",
		)
		So(err, ShouldBeNil)

		c := context.Background()
		res, err := server.Describe(c, nil)
		So(err, ShouldBeNil)

		So(res.Services, ShouldResemble, []string{
			"discovery.Discovery",
			"testservices.Greeter",
			"testservices.Calc",
		})

		desc := res.Description

		// this checks that file deduplication actually works.
		So(len(desc.File), ShouldEqual, 3)

		_, discoveryIndex := descutil.FindService(desc, "discovery.Discovery")
		So(discoveryIndex, ShouldNotEqual, -1)

		_, calcIndex := descutil.FindService(desc, "testservices.Calc")
		So(calcIndex, ShouldNotEqual, -1)

		file, greeterIndex := descutil.FindService(desc, "testservices.Greeter")
		So(greeterIndex, ShouldNotEqual, -1)
		greeter := file.Service[greeterIndex]

		sayHelloIndex := descutil.FindMethodForService(greeter, "SayHello")
		So(sayHelloIndex, ShouldNotEqual, -1)
		sayHello := greeter.Method[sayHelloIndex]

		So(sayHello.GetInputType(), ShouldEqual, ".testservices.HelloRequest")
		_, obj, _ := descutil.Resolve(desc, "testservices.HelloRequest")
		So(obj, ShouldNotBeNil)
		helloReq := obj.(*descriptor.DescriptorProto)
		So(helloReq, ShouldNotBeNil)
		So(helloReq.Field, ShouldHaveLength, 1)
		So(helloReq.Field[0].GetName(), ShouldEqual, "name")
		So(helloReq.Field[0].GetType(), ShouldEqual, descriptor.FieldDescriptorProto_TYPE_STRING)
	})
}
