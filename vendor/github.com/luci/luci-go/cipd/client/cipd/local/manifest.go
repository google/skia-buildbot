// Copyright 2014 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package local

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
)

// SiteServiceDir is a name of the directory inside an installation root
// reserved for cipd stuff.
const SiteServiceDir = ".cipd"

const (
	// packageServiceDir is a name of the directory inside the package
	// reserved for cipd stuff.
	packageServiceDir = ".cipdpkg"

	// manifestName is a name of the manifest file inside the package.
	manifestName = packageServiceDir + "/manifest.json"

	// manifestFormatVersion is a version to write to the manifest file.
	manifestFormatVersion = "1"
)

// InstallMode defines how to install a package.
type InstallMode string

const (
	// InstallModeSymlink is default (for backward compatibility).
	//
	// In this mode all files are extracted to .cipd/*/... and then symlinked to
	// the site root directory. Version switch happens atomically.
	InstallModeSymlink InstallMode = "symlink"

	// InstallModeCopy is used when files should be directly copied.
	//
	// This mode is always used on Windows (and can be optionally) used on
	// other OSes. If installation is aborted midway, the package may end up
	// in inconsistent state.
	InstallModeCopy InstallMode = "copy"
)

// Manifest defines structure of manifest.json file.
type Manifest struct {
	FormatVersion string      `json:"format_version"`
	PackageName   string      `json:"package_name"`
	VersionFile   string      `json:"version_file,omitempty"` // where to put JSON with info about deployed package
	InstallMode   InstallMode `json:"install_mode,omitempty"` // how to install: "copy" or "symlink"
	Files         []FileInfo  `json:"files,omitempty"`        // present only in deployed manifest
}

// FileInfo is JSON-ish struct with info extracted from File interface.
type FileInfo struct {
	// Name is slash separated file path relative to a package root.
	Name string `json:"name"`

	// Size is a size of the file. 0 for symlinks.
	Size uint64 `json:"size"`

	// Executable is true if the file is executable.
	//
	// Only used for Linux\Mac archives. False for symlinks.
	Executable bool `json:"executable,omitempty"`

	// Symlink is a path the symlink points to or "" if the file is not a symlink.
	Symlink string `json:"symlink,omitempty"`
}

// VersionFile describes JSON file with package version information that's
// deployed to a path specified in 'version_file' attribute of the manifest.
type VersionFile struct {
	PackageName string `json:"package_name"`
	InstanceID  string `json:"instance_id"`
}

// ValidateInstallMode returns non nil if install mode is invalid.
//
// Valid modes are: "" (client will pick platform default), "copy"
// (aka InstallModeCopy), "symlink" (aka InstallModeSymlink).
func ValidateInstallMode(mode InstallMode) error {
	if mode == "" || mode == InstallModeCopy || mode == InstallModeSymlink {
		return nil
	}
	return fmt.Errorf("invalid install mode %q", mode)
}

// Set is called by 'flag' package when parsing command line options.
func (m *InstallMode) Set(value string) error {
	val := InstallMode(value)
	if err := ValidateInstallMode(val); err != nil {
		return err
	}
	*m = val
	return nil
}

// String is needed to conform to flag.Value interface.
func (m InstallMode) String() string {
	return string(m)
}

// readManifest reads and decodes manifest JSON from io.Reader.
func readManifest(r io.Reader) (manifest Manifest, err error) {
	blob, err := ioutil.ReadAll(r)
	if err == nil {
		err = json.Unmarshal(blob, &manifest)
	}
	return
}

// writeManifest encodes and writes manifest JSON to io.Writer.
func writeManifest(m *Manifest, w io.Writer) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
