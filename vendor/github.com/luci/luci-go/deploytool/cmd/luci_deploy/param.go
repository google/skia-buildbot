// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"regexp"
	"strings"

	"github.com/luci/luci-go/common/errors"
)

// paramRE is the regular expression for deployment parameter substitution.
//
// Keys can contain alphanumeric letters plus underscores and periods.
// Substitution is represented as ${KEY}.
var paramRE = regexp.MustCompile(`\${([a-zA-Z0-9_.]+)}`)

// substitute substitutes any parameter expressions with a value from the
// supplied substitution map. If a parameter is referenced but not defined,
// an error will be returned.
func substitute(vp *string, subs map[string]string) error {
	var (
		v       = *vp
		matches = paramRE.FindAllStringSubmatchIndex(v, -1)
		parts   = make([]string, 0, (len(matches)*2)+1)
		sidx    = 0
	)
	for _, m := range matches {
		// m will have 4 entries:
		// v[m[0]:m[1]] is the variable expression that was matched.
		// v[m[2]:m[3]] is the paramter key (first capture group)
		parts = append(parts, v[sidx:m[0]])
		sidx = m[1]

		key := v[m[2]:m[3]]
		if val, ok := subs[key]; ok {
			parts = append(parts, val)
		} else {
			// panic to immediately stop matching. This will be caught at the top of
			// this function.
			return errors.Reason("undefined parameter %(key)q").D("key", key).Err()
		}
	}

	// Finish the string and return.
	parts = append(parts, v[sidx:])
	*vp = strings.Join(parts, "")
	return nil
}
