// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

//go:generate cproto -discovery=false

// Package deploy contains protobuf source and generated protobuf data for the
// LUCI deployment tool.
//
// The package name here must match the protobuf package name, as the generated
// files will reside in the same directory.
package deploy

import (
	"github.com/golang/protobuf/proto"
)

var _ = proto.Marshal
