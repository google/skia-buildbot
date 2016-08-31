// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package nestedflagset

import (
	"flag"
	"fmt"
	"strings"
)

// FlagSet is a flag.Flag implementation that parses into a nested flag.FlagSet.
//
// The FlagSet's flag value is parsed broken into a series of sub-options, which
// are then loaded into the nested flag set (FlagSet.F).
type FlagSet struct {
	F flag.FlagSet // The underlying FlagSet, populated on Parse.
}

var _ flag.Value = (*FlagSet)(nil)

// Parse parses a one-line option string into the underlying flag set.
func (fs *FlagSet) Parse(line string) error {
	var args []string
	for _, token := range lexer(line, ',').split() {
		name, value := token.split()
		args = append(args, fmt.Sprintf("-%s", name))
		if value != "" {
			args = append(args, value)
		}
	}

	return fs.F.Parse(args)
}

// Usage constructs a one-line usage string for all of the options defined in
// Flags.
func (fs *FlagSet) Usage() string {
	var flags []string

	// If there is no "help" flag defined, we will use it to display help/usage
	// (default FlagSet).
	if fs.F.Lookup("help") == nil {
		flags = append(flags, "help")
	}

	fs.F.VisitAll(func(f *flag.Flag) {
		comma := ""
		if len(flags) > 0 {
			comma = ","
		}
		flags = append(flags, fmt.Sprintf("[%s%s]", comma, f.Name))
	})
	return strings.Join(flags, "")
}

// Set implements flags.Value.
func (fs *FlagSet) Set(value string) error {
	return fs.Parse(value)
}

// String implements flags.Value.
func (fs *FlagSet) String() string {
	return fs.Usage()
}
