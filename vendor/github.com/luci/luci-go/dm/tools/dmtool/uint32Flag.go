// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"flag"
	"strconv"
)

type uint32Flag uint32

func (f *uint32Flag) Set(s string) error {
	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return err
	}
	*f = uint32Flag((uint32)(v))
	return nil
}

func (f *uint32Flag) String() string {
	return strconv.FormatUint(uint64(*f), 10)
}

var _ flag.Value = (*uint32Flag)(nil)
