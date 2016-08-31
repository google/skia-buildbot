// Copyright 2013-2014 The go-web authors.  All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package httpxtra

import (
	"net"
	"net/http"
	"strings"
)

// ListenAndServe can listen on both TCP and UNIX sockets.
func ListenAndServe(srv http.Server) error {
	var proto string
	if srv.Addr == "" {
		srv.Addr = ":http"
	}
	if strings.Contains(srv.Addr, "/") {
		proto = "unix"
	} else {
		proto = "tcp"
	}
	l, e := net.Listen(proto, srv.Addr)
	if e != nil {
		return e
	}
	return srv.Serve(l)
}
