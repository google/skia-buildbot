// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package pattern implements lightweight parsable string patterns.
package pattern

import (
	"fmt"
	"regexp"
	"strings"
)

// Pattern can either match or not match a string.
type Pattern interface {
	// String returns the definition of the pattern parsable by Parse.
	String() string
	// Match returns true if s matches this pattern, otherwise false.
	Match(s string) bool
}

// Parse parses a pattern.
//
// Ordered by precedence, s can be:
//
//  - "": matches nothing
//  - "*": matches anything
//  - "<S>" where S does not have a colon: same as "exact:<S>"
//  - "exact:<S>": matches only string S
//  - "text:<S>": same as "exact:<S>" for backward compatibility
//  - "regex:<E>": matches all strings matching regular expression E
//
// Anything else will cause an error.
func Parse(s string) (Pattern, error) {
	switch s {
	case "":
		return None, nil
	case "*":
		return Any, nil
	}

	parts := strings.SplitN(s, ":", 2)
	if len(parts) < 2 {
		return Exact(s), nil
	}

	kind, value := parts[0], parts[1]
	switch kind {
	case "exact", "text":
		return Exact(value), nil
	case "regex":
		switch value {
		case ".", ".*":
			return Any, nil
		case "^$":
			return None, nil
		default:
			r, err := regexp.Compile(value)
			if err != nil {
				return nil, err
			}
			return Regexp(r), nil
		}
	default:
		return nil, fmt.Errorf("unknown pattern kind: %q", kind)
	}
}
