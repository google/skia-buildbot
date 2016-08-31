// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"flag"
	"strings"
)

var (
	// (*streamServerURI) must implement flag.Value.
	_ = flag.Value((*streamServerURI)(nil))
)

// Implements flag.Value.
func (u *streamServerURI) String() string {
	return string(*u)
}

// Set implements flag.Value.
func (u *streamServerURI) Set(v string) error {
	uri := streamServerURI(v)
	if err := uri.Validate(); err != nil {
		return err
	}
	*u = uri
	return nil
}

func parseStreamServer(v string) (string, string) {
	parts := strings.SplitN(v, ":", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}
