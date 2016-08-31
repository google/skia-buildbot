// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package gologger

import (
	"bytes"
	"fmt"
	"strings"
	"sync"

	"github.com/luci/luci-go/common/logging"
	gol "github.com/op/go-logging"
	"golang.org/x/net/context"
)

const (
	logMessageFieldPadding = 44
)

// goLoggerWrapper is a synchronized wrapper around a go-logging Logger
// instance.
type goLoggerWrapper struct {
	sync.Mutex             // Lock around wrapped logger properties.
	l          *gol.Logger // Wrapped logger.
}

// loggerImpl implements logging.Logger. It optionally binds a goLoggerWrapper
// to a Context.
type loggerImpl struct {
	*goLoggerWrapper // The logger instance to log through.

	c context.Context // Bound context; may be nil if there is no bound context.
}

func (li *loggerImpl) Debugf(format string, args ...interface{}) {
	li.LogCall(logging.Debug, 1, format, args)
}
func (li *loggerImpl) Infof(format string, args ...interface{}) {
	li.LogCall(logging.Info, 1, format, args)
}
func (li *loggerImpl) Warningf(format string, args ...interface{}) {
	li.LogCall(logging.Warning, 1, format, args)
}
func (li *loggerImpl) Errorf(format string, args ...interface{}) {
	li.LogCall(logging.Error, 1, format, args)
}

func (li *loggerImpl) LogCall(l logging.Level, calldepth int, format string, args []interface{}) {
	// Append the fields to the format string.
	if li.c != nil {
		if !logging.IsLogging(li.c, l) {
			return
		}

		if fields := logging.GetFields(li.c); len(fields) > 0 {
			text := formatWithFields(format, fields, args)
			format = strings.Replace(text, "%", "%%", -1)
			args = nil
		}
	}

	li.Lock()
	defer li.Unlock()

	li.l.ExtraCalldepth = (calldepth + 1)
	switch l {
	case logging.Debug:
		li.l.Debugf(format, args...)
	case logging.Info:
		li.l.Infof(format, args...)
	case logging.Warning:
		li.l.Warningf(format, args...)
	case logging.Error:
		li.l.Errorf(format, args...)
	}
}

// formatWithFields renders the supplied format string, adding fields.
func formatWithFields(format string, fields logging.Fields, args []interface{}) string {
	fieldString := fields.String()

	buf := bytes.Buffer{}
	buf.Grow(len(format) + logMessageFieldPadding + len(fieldString))
	fmt.Fprintf(&buf, format, args...)

	padding := 44 - buf.Len()
	if padding < 1 {
		padding = 1
	}
	for i := 0; i < padding; i++ {
		buf.WriteString(" ")
	}
	buf.WriteString(fieldString)
	return buf.String()
}
