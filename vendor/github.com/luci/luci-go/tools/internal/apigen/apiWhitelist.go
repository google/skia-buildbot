// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package apigen

import (
	"errors"
	"flag"
	"strings"
)

// apiWhitelist is a flag.Value type that appends to the list on add.
type apiWhitelist []string

var _ flag.Value = (*apiWhitelist)(nil)

func (w *apiWhitelist) String() string {
	if *w == nil {
		return ""
	}
	return strings.Join(*w, ",")
}

func (w *apiWhitelist) Set(s string) error {
	if len(s) == 0 {
		return errors.New("cannot whitelist empty string")
	}

	*w = append(*w, s)
	return nil
}
