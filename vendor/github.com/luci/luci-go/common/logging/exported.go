// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package logging

import (
	"golang.org/x/net/context"
)

// SetError returns a context with its error field set.
func SetError(c context.Context, err error) context.Context {
	return SetField(c, ErrorKey, err)
}

// IsLogging tests whether the context is configured to log at the specified
// level.
//
// Individual Logger implementations are supposed to call this function when
// deciding whether to log the message.
func IsLogging(c context.Context, l Level) bool {
	return l >= GetLevel(c)
}

// Debugf is a shorthand method to call the current logger's Errorf method.
func Debugf(c context.Context, fmt string, args ...interface{}) {
	Get(c).LogCall(Debug, 1, fmt, args)
}

// Infof is a shorthand method to call the current logger's Errorf method.
func Infof(c context.Context, fmt string, args ...interface{}) {
	Get(c).LogCall(Info, 1, fmt, args)
}

// Warningf is a shorthand method to call the current logger's Errorf method.
func Warningf(c context.Context, fmt string, args ...interface{}) {
	Get(c).LogCall(Warning, 1, fmt, args)
}

// Errorf is a shorthand method to call the current logger's Errorf method.
func Errorf(c context.Context, fmt string, args ...interface{}) {
	Get(c).LogCall(Error, 1, fmt, args)
}

// Logf is a shorthand method to call the current logger's logging method which
// corresponds to the supplied log level.
func Logf(c context.Context, l Level, fmt string, args ...interface{}) {
	Get(c).LogCall(l, 1, fmt, args)
}
