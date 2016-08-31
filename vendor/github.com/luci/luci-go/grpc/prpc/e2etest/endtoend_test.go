// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package e2etest

import (
	"testing"

	"github.com/luci/luci-go/common/testing/prpctest"
	"github.com/luci/luci-go/server/auth"
	"golang.org/x/net/context"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

type service struct {
	R   *HelloReply
	err error
}

func (s *service) Greet(c context.Context, req *HelloRequest) (*HelloReply, error) {
	return s.R, s.err
}

func TestEndToEnd(t *testing.T) {
	Convey(`A client/server for the Greet service`, t, func() {
		c := context.Background()
		svc := service{}

		// Create a client/server for Greet service.
		ts := prpctest.Server{}
		ts.Authenticator = auth.Authenticator{}
		RegisterHelloServer(&ts, &svc)
		ts.Start(c)
		defer ts.Close()

		prpcClient, err := ts.NewClient()
		if err != nil {
			panic(err)
		}
		client := NewHelloPRPCClient(prpcClient)

		Convey(`Can round-trip a hello message.`, func() {
			svc.R = &HelloReply{Message: "sup"}

			resp, err := client.Greet(c, &HelloRequest{Name: "round-trip"})
			So(err, ShouldBeRPCOK)
			So(resp, ShouldResemble, svc.R)
		})
	})
}
