// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package helloworld

import (
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/luci/luci-go/grpc/prpc/talk/helloworld/proto"
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

var _ helloworld.GreeterServer = (*greeterService)(nil)
