// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package logging

import (
	"flag"

	"golang.org/x/net/context"
)

// Config is a logging configuration structure.
type Config struct {
	Level Level
}

// AddFlags adds common flags to a supplied FlagSet.
func (c *Config) AddFlags(fs *flag.FlagSet) {
	fs.Var(&c.Level, "log-level",
		"The logging level. Valid options are: debug, info, warning, error.")
}

// Set returns a new context configured to use logging level passed via the
// command-line level flag.
func (c *Config) Set(ctx context.Context) context.Context {
	return SetLevel(ctx, c.Level)
}
