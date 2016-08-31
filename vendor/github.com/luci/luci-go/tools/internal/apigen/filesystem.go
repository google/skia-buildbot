// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package apigen

import (
	"errors"
	"fmt"
	"go/build"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// editFunc is a function called when a file is copied by copyFile.
//
// If editFunc returns a nil byte array, the copy will be skipped.
//
// On success, editFunc returns the new file data to write.
type editFunc func(relPath string, data []byte) ([]byte, error)

// getPackagePath searches through GOPATH to find the filesystem path of the
// named package.
//
// This is complicated by the fact that the named package might not exist. In
// this case, the package's path will be traversed until one of its parent's
// paths is found.
func getPackagePath(p string) (string, error) {
	pkg := strings.Split(p, "/")

	for i := len(pkg) - 1; i > 0; i-- {
		p, err := build.Import(strings.Join(pkg[:i], "/"), "", build.FindOnly)
		if err != nil {
			continue
		}
		return augPath(p.Dir, pkg[i:]...), nil
	}
	return "", errors.New("could not find package path")
}

// augPath joins a series of path elements to a base path.
func augPath(base string, parts ...string) string {
	cpath := make([]string, 0, len(parts)+1)
	cpath = append(cpath, base)
	cpath = append(cpath, parts...)
	return filepath.Join(cpath...)
}

// installSource recursively copies an API generator output directory to a
// package location.
func installSource(src, dst string, edit editFunc) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relpath, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path [%s]: %s", path, err)
		}

		dstpath := filepath.Join(dst, relpath)
		switch {
		case info.IsDir():
			// Make sure the directory exists in the target filesystem.
			if err := ensureDirectory(dstpath); err != nil {
				return fmt.Errorf("failed to ensure directory [%s]: %s", dstpath, err)
			}

		case !info.Mode().IsRegular():
			// Skip non-regular files.
			break

		default:
			// Copy the file from source to destination.
			if err := copyFile(path, dstpath, relpath, edit); err != nil {
				return fmt.Errorf("failed to copy file ([%s] => [%s]): %s", path, dstpath, err)
			}
		}

		return nil
	})
}

// ensureDirectory ensures that the supplied directory exists, creating it and
// its parent directories as-needed.
func ensureDirectory(path string) error {
	return os.MkdirAll(path, 0755)
}

// copyFile copies the contents of a single file to a destination.
func copyFile(src, dst string, relPath string, edit editFunc) error {
	data, err := ioutil.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source: %s", err)
	}

	if edit != nil {
		data, err = edit(relPath, data)
		if err != nil {
			return fmt.Errorf("edit error: %s", err)
		}

	}
	if data == nil {
		return nil
	}
	return ioutil.WriteFile(dst, data, 0644)
}
