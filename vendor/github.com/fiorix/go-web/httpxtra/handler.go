// Copyright 2013-2014 The go-web authors.  All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// httpxtra is a wrapper for http.Handler that adds extra features to the server:
// - Custom logging
// - Support for listening on TCP or UNIX sockets
// - Support X-Real-IP and X-Forwarded-For as the remote IP if the server sits
//   behind a proxy or load balancer.
package httpxtra

import (
	"net/http"
	"time"
)

// Handler is the http.Handler wrapper with extra features.
type Handler struct {
	Handler  http.Handler
	Logger   LoggerFunc
	XHeaders bool
}

// ServeHTTP is a wrapper for the request, which can modify the value
// of the RemoteAddr and later calls the logger function.
//
// Note that when XHeaders are enabled, the value of RemoteAddr might
// be a copy of the X-Real-IP or X-Forwarded-For HTTP header, which can
// be a comma separated list of IPs.
//
// See http://httpd.apache.org/docs/2.2/mod/mod_proxy.html#x-headers for
// details.
func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t := time.Now()
	lw := LogWriter{ResponseWriter: w}
	if h.Handler == nil {
		h.Handler = http.DefaultServeMux
	}
	if h.XHeaders {
		ip := r.Header.Get("X-Real-IP")
		if ip == "" {
			ip = r.Header.Get("X-Forwarded-For")
		}
		if ip != "" {
			r.RemoteAddr = ip
		}
	}
	h.Handler.ServeHTTP(&lw, r)
	if h.Logger != nil {
		h.Logger(r, t, lw.Status, lw.Bytes)
	}
}
