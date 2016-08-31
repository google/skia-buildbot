// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package prpc

import (
	"google.golang.org/grpc"
)

type service struct {
	desc    *grpc.ServiceDesc
	methods map[string]*method
	impl    interface{}
}
