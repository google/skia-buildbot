// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package frontend

import (
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/luci/luci-go/examples/appengine/helloworld_standard/proto"
)

type greeterService struct{}

func (s *greeterService) SayHello(c context.Context, req *helloworld.HelloRequest) (*helloworld.HelloReply, error) {
	if req.Name == "" {
		return nil, grpc.Errorf(codes.InvalidArgument, "Name unspecified")
	}

	return &helloworld.HelloReply{
		Message: "Hello " + req.Name,
	}, nil
}
