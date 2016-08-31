// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package common

import (
	"io"
	"os"
	"runtime"

	"github.com/luci/luci-go/common/system/terminal"
)

// IsDirectory returns true if path is a directory and is accessible.
func IsDirectory(path string) bool {
	fileInfo, err := os.Stat(path)
	return err == nil && fileInfo.IsDir()
}

// IsWindows returns True when running on the best OS there is.
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// IsTerminal returns true if the specified io.Writer is a terminal.
func IsTerminal(out io.Writer) bool {
	f, ok := out.(*os.File)
	if !ok {
		return false
	}
	return terminal.IsTerminal(int(f.Fd()))
}
