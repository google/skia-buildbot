// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package flagpb

import (
	"github.com/luci/luci-go/common/proto/google/descutil"

	"google.golang.org/genproto/protobuf"
)

// Resolver resolves type names.
type Resolver interface {
	// Resolve resolves a type name to
	// *descriptor.DescriptorProto or *descriptor.EnumDescriptorProto.
	Resolve(name string) interface{}
}

// NewResolver creates a resolver for all types in a file descriptor set.
// Resolving time complexity is linear.
func NewResolver(set *descriptor.FileDescriptorSet) Resolver {
	return &descriptorSetResolver{set}
}

type descriptorSetResolver struct {
	set *descriptor.FileDescriptorSet
}

func (r *descriptorSetResolver) Resolve(name string) interface{} {
	_, o, _ := descutil.Resolve(r.set, name)
	switch o := o.(type) {
	case *descriptor.DescriptorProto, *descriptor.EnumDescriptorProto:
		return o
	default:
		return nil
	}
}
