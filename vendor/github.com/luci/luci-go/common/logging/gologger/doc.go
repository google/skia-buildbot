// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package gologger is a compatibility layer between go-logging library and
// luci-go/common/logging.
//
// Use it if you want to log to files or console.
//
// Default usage (logging to stderr):
//
//   import (
//     "github.com/luci/luci-go/common/logging"
//     "github.com/luci/luci-go/common/logging/gologger"
//   )
//
//   ...
//
//   ctx := context.Background()
//   ctx = gologger.StdConfig.Use(ctx)
//   logging.Infof(ctx, "Hello %s", "world")
//
//
// If you want more control over where log goes or how it looks, instantiate
// custom LoggerConfig struct:
//
//   logCfg := gologger.LoggerConfig{Out: logFileWriter}
//   ctx = logCfg.Use(ctx)
//
//
// If you know what you are doing you even can prepare go-logging.Logger
// instance yourself and plug it in into luci-go/common/logging:
//
//   logCfg := gologger.LoggerConfig{Logger: goLoggingLogger}
//   ctx = logCfg.Use(ctx)
//
//
// Note that you almost never want to change logging level of the go-logging
// logger (that's why LoggerConfig doesn't expose it), use logging.SetLevel
// instead to do filtering before messages hit go-logging logger.
package gologger
