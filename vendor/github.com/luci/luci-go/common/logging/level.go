// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package logging

import (
	"errors"
	"flag"

	"golang.org/x/net/context"
)

// Level is an enumeration consisting of supported log levels.
type Level int

// Level implements flag.Value.
var _ flag.Value = (*Level)(nil)

// Defined log levels.
const (
	Debug Level = iota
	Info
	Warning
	Error
)

// DefaultLevel is the default Level value.
const DefaultLevel = Info

// Set implements flag.Value.
func (l *Level) Set(v string) error {
	switch v {
	case "debug":
		*l = Debug
	case "info":
		*l = Info
	case "warning":
		*l = Warning
	case "error":
		*l = Error

	default:
		return errors.New("Unknown log level value.")
	}
	return nil
}

// String implements flag.Value.
func (l Level) String() string {
	switch l {
	case Debug:
		return "debug"
	case Info:
		return "info"
	case Warning:
		return "warning"
	case Error:
		return "error"

	default:
		return "unknown"
	}
}

// SetLevel sets the Level for this context.
//
// It can be retrieved with GetLevel(context).
func SetLevel(c context.Context, l Level) context.Context {
	return context.WithValue(c, levelKey, l)
}

// GetLevel returns the Level for this context. It will return DefaultLevel if
// none is defined.
func GetLevel(c context.Context) Level {
	if l, ok := c.Value(levelKey).(Level); ok {
		return l
	}
	return DefaultLevel
}
