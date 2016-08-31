// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

//go:generate cproto

// Package logpb contains LogDog protobuf source and generated protobuf data.
//
// The package name here must match the protobuf package name, as the generated
// files will reside in the same directory.
package logpb

import (
	"github.com/golang/protobuf/proto"
)

var _ = proto.Marshal
