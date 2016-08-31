// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package nestedflagset

import (
	"strings"
)

// Token is a single name[=value] string.
type token string

// split breaks a token into its name and value components.
func (t token) split() (name, value string) {
	split := strings.SplitN(string(t), "=", 2)
	name = split[0]
	if len(split) == 2 {
		value = split[1]
	}
	return
}
