// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package logging

// Null is a logger that silently ignores all messages.
var Null Logger = nullLogger{}

// nullLogger silently ignores all messages.
type nullLogger struct{}

func (nullLogger) Debugf(string, ...interface{})             {}
func (nullLogger) Infof(string, ...interface{})              {}
func (nullLogger) Warningf(string, ...interface{})           {}
func (nullLogger) Errorf(string, ...interface{})             {}
func (nullLogger) LogCall(Level, int, string, []interface{}) {}
