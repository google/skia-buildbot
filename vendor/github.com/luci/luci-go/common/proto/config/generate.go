// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package config contains luci-config protobuf definitions.
//
// The .proto files are copied from
// https://github.com/luci/luci-py/tree/45f3e27/appengine/components/components/config/proto
// with package name changed to "config" because golint does not like
// underscore in package names.
package config

import (
	"github.com/golang/protobuf/proto"
)

var _ = proto.Marshal

//go:generate cproto
