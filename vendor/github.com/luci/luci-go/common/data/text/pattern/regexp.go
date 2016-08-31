// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package pattern

import "regexp"

type regexpMatch struct {
	r *regexp.Regexp
}

func (m regexpMatch) Match(s string) bool {
	return m.r.MatchString(s)
}

func (m regexpMatch) String() string {
	return "regex:" + m.r.String()
}

// Regexp returns a regular expression-based pattern.
func Regexp(expr *regexp.Regexp) Pattern {
	return regexpMatch{expr}
}
