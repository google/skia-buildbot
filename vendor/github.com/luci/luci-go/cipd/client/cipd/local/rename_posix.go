// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// +build !windows

package local

import (
	"os"
)

func atomicRename(source, target string) error {
	return os.Rename(source, target)
}
