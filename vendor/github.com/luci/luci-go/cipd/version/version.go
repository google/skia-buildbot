// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package version provides a way for CIPD packaged Go binaries to discover
// their current package instance ID.
//
// It's safe to link this library into arbitrary executables. It is small and
// doesn't pull in rest of CIPD client code.
package version

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kardianos/osext"
)

var (
	lock                  sync.Mutex
	initialExePath        string
	initialExePathErr     error
	startupVersionFile    Info
	startupVersionFileErr error
)

// Info describes JSON file with package version information that's
// deployed to a path specified in 'version_file' attribute of the manifest.
type Info struct {
	PackageName string `json:"package_name"`
	InstanceID  string `json:"instance_id"`
}

// GetCurrentVersion reads version file from disk.
//
// Note that it may have been updated since the process started. This function
// always reads the latest values. Version file is expected to be found at
// <exe-path>.cipd_version.
//
// Add following lines to package definition yaml to to set this up:
//
//   data:
//     - version_file: .versions/<exe-name>${exe_suffix}.cipd_version
//
// Replace <exe-name> with name of the binary file.
//
// If the version file is missing, returns empty Info{} and no error.
func GetCurrentVersion() (Info, error) {
	if initialExePathErr != nil {
		return Info{}, initialExePathErr
	}
	// For CIPD packages installed using "symlink" method initialExePath may point
	// to the real file in .cipd/* guts. To get the current version of the package
	// we need to work back to the original symlink. No need to do it for packages
	// installed with "copy" method.
	if symlinkPath := recoverSymlinkPath(initialExePath); symlinkPath != "" {
		return getCurrentVersion(symlinkPath)
	}
	return getCurrentVersion(initialExePath)
}

// GetStartupVersion returns value of version file as it was when the process
// has just started.
func GetStartupVersion() (Info, error) {
	lock.Lock()
	defer lock.Unlock()
	return startupVersionFile, startupVersionFileErr
}

// recoverSymlinkPath guesses the path to a symlink in CIPD package site root
// given an absolute path to a file in .cipd/* guts. Returns "" if given path
// is not inside .cipd/*. Knows about .cipd/* directory layout.
func recoverSymlinkPath(p string) string {
	// A/.cipd/pkgs/<name>/<id>/b/c/d => A/b/c/d. (at least 5 components)
	chunks := strings.Split(p, string(filepath.Separator))
	if len(chunks) < 5 {
		return ""
	}
	// Search for .cipd to find site root.
	var i int
	for i = len(chunks) - 1; i >= 0; i-- {
		if chunks[i] == ".cipd" {
			break
		}
	}
	if i == -1 {
		return ""
	}
	// Must have at least ".cipd/pkgs/<name>/<id>/...".
	if len(chunks)-i <= 4 {
		return ""
	}
	// Cut out .cipd/pkgs/<name>/<id> to get A/b/c/d.
	return strings.Join(append(chunks[:i], chunks[i+4:]...), string(filepath.Separator))
}

func getCurrentVersion(exePath string) (Info, error) {
	// <root>/.versions/exename.cipd_version
	p := filepath.Join(filepath.Dir(exePath), ".versions", filepath.Base(exePath)+".cipd_version")
	if vf, _ := readVersionFile(p); vf.InstanceID != "" {
		return vf, nil
	}
	// <root>/exename.cipd_version
	return readVersionFile(exePath + ".cipd_version")
}

// readVersionFile returns parsed version file. Returns empty struct and nil if
// it is missing, error if it can't be read.
func readVersionFile(path string) (Info, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		return Info{}, err
	}
	defer f.Close()
	out := Info{}
	if err = json.NewDecoder(f).Decode(&out); err != nil {
		return Info{}, err
	}
	return out, nil
}

// init is used to read version file as soon as possible during the process
// startup. Version file may change later during process lifetime (e.g. during
// update).
func init() {
	lock.Lock()
	defer lock.Unlock()
	// The executable may move during lifetime of the process (e.g. when being
	// updated). Remember the original location.
	initialExePath, initialExePathErr = osext.Executable()
	if initialExePathErr == nil && !filepath.IsAbs(initialExePath) {
		initialExePathErr = fmt.Errorf("not an abs path: %s", initialExePath)
	}
	// Version file can also be changed. Remember the version of the started
	// executable.
	if initialExePathErr == nil {
		// Don't use GetCurrentVersion since we specifically do not want to use
		// the original symlink.
		startupVersionFile, startupVersionFileErr = getCurrentVersion(initialExePath)
	} else {
		startupVersionFileErr = initialExePathErr
	}
}
