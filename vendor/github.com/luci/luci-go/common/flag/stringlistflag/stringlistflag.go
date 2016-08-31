// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package stringlistflag provides a flag.Value implementation which resolves
// multiple args into a []string.
package stringlistflag

import (
	"flag"
	"fmt"
	"strings"
)

// Flag is a flag.Value implementation which represents an ordered set of
// strings.
//
// For example, this allows you to construct a flag that would behave like:
//   -myflag Foo
//   -myflag Bar
//   -myflag Bar
//
// And then myflag would be []string{"Foo", "Bar", "Bar"}
type Flag []string

var _ flag.Value = (*Flag)(nil)

func (f Flag) String() string {
	return strings.Join(f, ", ")
}

// Set implements flag.Value's Set function.
func (f *Flag) Set(val string) error {
	if val == "" {
		return fmt.Errorf("must have an argument value")
	}

	*f = append(*f, val)
	return nil
}
