// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"strings"

	"github.com/luci/luci-go/common/proto/google/descutil"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/grpc/discovery"
	"github.com/luci/luci-go/grpc/prpc"
	"google.golang.org/genproto/protobuf"
)

type serverDescription struct {
	*discovery.DescribeResponse
}

func loadDescription(c context.Context, client *prpc.Client) (*serverDescription, error) {
	dc := discovery.NewDiscoveryPRPCClient(client)
	res, err := dc.Describe(c, &discovery.Void{})
	if err != nil {
		return nil, fmt.Errorf("could not load server description: %s", err)
	}

	return &serverDescription{res}, nil
}

// resolveInputMessage resolves input message type of a method.
func (d *serverDescription) resolveInputMessage(service, method string) (*descriptor.DescriptorProto, error) {
	_, obj, _ := descutil.Resolve(d.Description, service)
	serviceDesc, ok := obj.(*descriptor.ServiceDescriptorProto)
	if !ok {
		return nil, fmt.Errorf("service %q not found", service)
	}

	mi := descutil.FindMethodForService(serviceDesc, method)
	if mi == -1 {
		return nil, fmt.Errorf("method %q in service %q not found", method, service)
	}

	msgName := serviceDesc.Method[mi].GetInputType()
	msgName = strings.TrimPrefix(msgName, ".")
	return d.resolveMessage(msgName)
}

func (d *serverDescription) resolveMessage(name string) (*descriptor.DescriptorProto, error) {
	_, obj, _ := descutil.Resolve(d.Description, name)
	msg, ok := obj.(*descriptor.DescriptorProto)
	if !ok {
		return nil, fmt.Errorf("message %q not found", name)
	}
	return msg, nil
}
