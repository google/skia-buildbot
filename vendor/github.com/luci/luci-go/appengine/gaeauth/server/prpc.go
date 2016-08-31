// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package server

import (
	"github.com/luci/luci-go/grpc/prpc"
	"github.com/luci/luci-go/server/auth"
)

func init() {
	prpc.RegisterDefaultAuth(auth.Authenticator{
		&OAuth2Method{Scopes: []string{EmailScope}},
	})
}
