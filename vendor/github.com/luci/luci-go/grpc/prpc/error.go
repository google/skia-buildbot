// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package prpc

import (
	"fmt"
)

// protocolError is returned if a pRPC request is malformed.
type protocolError struct {
	err    error
	status int // HTTP status to use in response.
}

func (e *protocolError) Error() string {
	return fmt.Sprintf("pRPC: %s", e.err)
}

// withStatus wraps an error with an HTTP status.
// If err is nil, returns nil.
func withStatus(err error, status int) *protocolError {
	if _, ok := err.(*protocolError); ok {
		panic("protocolError in protocolError")
	}
	if err == nil {
		return nil
	}
	return &protocolError{err, status}
}

// errorf creates a new protocol error.
func errorf(status int, format string, a ...interface{}) *protocolError {
	return withStatus(fmt.Errorf(format, a...), status)
}
