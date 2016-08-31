// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package auth

import (
	"net/http"
)

// NewModifyingTransport returns a transport that can modify headers of
// http.Request's that pass through it (e.g. by appending authentication
// information).
//
// Go docs explicitly prohibit this kind of behavior, but everyone cheat and
// implement it anyway. E.g. https://github.com/golang/oauth2.
//
// Works only with Go >=1.5 transports (that use Request.Cancel instead of
// deprecated Transport.CancelRequest). For same reason doesn't implement
// CancelRequest itself.
func NewModifyingTransport(base http.RoundTripper, modifier func(*http.Request) error) http.RoundTripper {
	return &modifyingTransport{
		base:     base,
		modifier: modifier,
	}
}

type modifyingTransport struct {
	base     http.RoundTripper
	modifier func(*http.Request) error
}

func (t *modifyingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := *req
	clone.Header = make(http.Header, len(req.Header))
	for k, v := range req.Header {
		clone.Header[k] = v
	}
	if err := t.modifier(&clone); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(&clone)
}
