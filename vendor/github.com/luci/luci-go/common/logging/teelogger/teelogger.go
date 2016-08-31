// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package teelogger

import (
	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/logging"
)

type teeImpl struct {
	c context.Context // for logging level check
	l []logging.Logger
}

func (t *teeImpl) Debugf(fmt string, args ...interface{}) {
	t.LogCall(logging.Debug, 1, fmt, args)
}

func (t *teeImpl) Infof(fmt string, args ...interface{}) {
	t.LogCall(logging.Info, 1, fmt, args)
}

func (t *teeImpl) Warningf(fmt string, args ...interface{}) {
	t.LogCall(logging.Warning, 1, fmt, args)
}

func (t *teeImpl) Errorf(fmt string, args ...interface{}) {
	t.LogCall(logging.Error, 1, fmt, args)
}

func (t *teeImpl) LogCall(level logging.Level, calldepth int, f string, args []interface{}) {
	if t.c != nil && !logging.IsLogging(t.c, level) {
		return
	}
	for _, logger := range t.l {
		logger.LogCall(level, calldepth+1, f, args)
	}
}

// Use adds a tee logger to the context, using the logger factory in
// the context, as well as the other loggers produced by given factories.
//
// We use factories (instead of logging.Logger instances), since we must be able
// to produce logging.Logger instances bound to contexts to be able to use
// logging levels are fields (they are part of the context state).
func Use(ctx context.Context, factories ...logging.Factory) context.Context {
	if cur := logging.GetFactory(ctx); cur != nil {
		factories = append([]logging.Factory{cur}, factories...)
	}
	return logging.SetFactory(ctx, func(ic context.Context) logging.Logger {
		loggers := make([]logging.Logger, len(factories))
		for i, f := range factories {
			loggers[i] = f(ic)
		}
		return &teeImpl{ic, loggers}
	})
}
