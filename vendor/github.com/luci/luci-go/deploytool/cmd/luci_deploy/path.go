// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/luci/luci-go/common/errors"
)

const (
	// configExt is the configuration file extension.
	configExt = ".cfg"
)

func splitSourceRelativePath(path string) []string {
	return strings.Split(path, "/")
}

// deployToNative converts the depoy tool path, path, to a native path.
//
// base is a native base path to prepend to the generated path. If empty, the
// returned path will be relative.
func deployToNative(base, path string) string {
	parts := splitSourceRelativePath(path)
	path = filepath.Join(parts...)
	if base != "" {
		path = filepath.Join(base, path)
	}
	return path
}

// deployDirname returns the directory name of the source-relative path.
func deployDirname(path string) string {
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[:idx]
	}
	return ""
}

func isHidden(path string) bool {
	return strings.HasPrefix(filepath.Base(path), ".")
}

func fileURLToPath(path string) string {
	if filepath.Separator == '/' {
		return path
	}
	return strings.Replace(path, "/", string(filepath.Separator), -1)
}

func withTempDir(f func(string) error) error {
	// Create a temporary directory.
	tdir, err := ioutil.TempDir("", "luci_deploytool")
	if err != nil {
		return errors.Annotate(err).Reason("failed to create tempdir").Err()
	}
	defer os.RemoveAll(tdir)

	return f(tdir)
}
