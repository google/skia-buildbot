// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package errors

// Wrapped indicates an error that wraps another error.
type Wrapped interface {
	// InnerError returns the wrapped error.
	InnerError() error
}

// Unwrap unwraps a wrapped error recursively, returning its inner error.
//
// If the supplied error is not nil, Unwrap will never return nil. If a
// wrapped error reports that its InnerError is nil, that error will be
// returned.
func Unwrap(err error) error {
	for {
		wrap, ok := err.(Wrapped)
		if !ok {
			return err
		}

		inner := wrap.InnerError()
		if inner == nil {
			break
		}
		err = inner
	}
	return err
}
