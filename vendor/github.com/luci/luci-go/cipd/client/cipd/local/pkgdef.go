// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package local

import (
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/luci/luci-go/cipd/client/cipd/common"
)

// PackageDef defines how exactly to build a package.
//
// It specified  what files to put into it, how to name them, how to name
// the package itself, etc. It is loaded from *.yaml file.
type PackageDef struct {
	// Package defines a name of the package.
	Package string

	// Root defines where to search for files, relative to package file itself.
	Root string

	// InstallMode defines how to deploy the package file: "copy" or "symlink".
	InstallMode InstallMode `yaml:"install_mode"`

	// Data describes what is deployed with the package.
	Data []PackageChunkDef
}

// PackageChunkDef represents one entry in 'data' section of package definition.
//
// It is either a single file, or a recursively scanned directory (with optional
// list of regexps for files to skip).
type PackageChunkDef struct {
	// Dir is a directory to add to the package (recursively).
	Dir string

	// File is a single file to add to the package.
	File string

	// VersionFile defines where to drop JSON file with package version.
	VersionFile string `yaml:"version_file"`

	// Exclude is a list of glob patterns to exclude when scanning a directory.
	Exclude []string
}

// LoadPackageDef loads package definition from a YAML source code.
//
// It substitutes %{...} strings in the definition with corresponding values
// from 'vars' map.
func LoadPackageDef(r io.Reader, vars map[string]string) (PackageDef, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return PackageDef{}, err
	}

	out := PackageDef{}
	if err = yaml.Unmarshal(data, &out); err != nil {
		return PackageDef{}, err
	}

	// Substitute variables in all strings.
	for _, str := range out.strings() {
		*str, err = subVars(*str, vars)
		if err != nil {
			return PackageDef{}, err
		}
	}

	// Validate global package properties.
	if err = common.ValidatePackageName(out.Package); err != nil {
		return PackageDef{}, err
	}
	if err = ValidateInstallMode(out.InstallMode); err != nil {
		return PackageDef{}, err
	}

	versionFile := ""
	for i, chunk := range out.Data {
		// Make sure 'dir' and 'file' etc. aren't used together.
		has := []string{}
		if chunk.File != "" {
			has = append(has, "file")
		}
		if chunk.VersionFile != "" {
			has = append(has, "version_file")
		}
		if chunk.Dir != "" {
			has = append(has, "dir")
		}
		if len(has) == 0 {
			return out, fmt.Errorf("files entry #%d needs 'file', 'dir' or 'version_file' key", i)
		}
		if len(has) != 1 {
			return out, fmt.Errorf("files entry #%d should have only one key, got %q", i, has)
		}
		//'version_file' can appear only once, it must be a clean relative path.
		if chunk.VersionFile != "" {
			if versionFile != "" {
				return out, fmt.Errorf("'version_file' entry can be used only once")
			}
			versionFile = chunk.VersionFile
			if !isCleanSlashPath(versionFile) {
				return out, fmt.Errorf("'version_file' must be a path relative to the package root: %s", versionFile)
			}
		}
	}

	// Default 'root' to a directory with the package def file.
	if out.Root == "" {
		out.Root = "."
	}
	return out, nil
}

// FindFiles scans files system and returns files to be added to the package.
//
// It uses a path to package definition file directory ('cwd' argument) to find
// a root of the package.
func (def *PackageDef) FindFiles(cwd string) ([]File, error) {
	// Root of the package is defined relative to package def YAML file.
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return nil, err
	}
	root := filepath.Clean(filepath.Join(absCwd, filepath.FromSlash(def.Root)))

	// Helper to get absolute path to a file given path relative to root.
	makeAbs := func(p string) string {
		return filepath.Join(root, filepath.FromSlash(p))
	}

	// Used to skip duplicates.
	seen := map[string]File{}
	add := func(f File) {
		if seen[f.Name()] == nil {
			seen[f.Name()] = f
		}
	}

	for _, chunk := range def.Data {
		// Handled elsewhere.
		if chunk.VersionFile != "" {
			continue
		}

		// Individual file.
		if chunk.File != "" {
			file, err := WrapFile(makeAbs(chunk.File), root, nil)
			if err != nil {
				return nil, err
			}
			add(file)
			continue
		}

		// A subdirectory to scan (with filtering).
		if chunk.Dir != "" {
			// Absolute path to directory to scan.
			startDir := makeAbs(chunk.Dir)
			// Exclude files as specified in 'exclude' section.
			exclude, err := makeExclusionFilter(startDir, chunk.Exclude)
			if err != nil {
				return nil, err
			}
			// Run the scan.
			files, err := ScanFileSystem(startDir, root, exclude)
			if err != nil {
				return nil, err
			}
			for _, f := range files {
				add(f)
			}
			continue
		}

		// LoadPackageDef does validation, so this should not happen.
		return nil, fmt.Errorf("unexpected definition: %v", chunk)
	}

	// Sort by Name().
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)

	// Final sorted array of File.
	out := make([]File, 0, len(names))
	for _, n := range names {
		out = append(out, seen[n])
	}
	return out, nil
}

// VersionFile defines where to drop JSON file with package version.
func (def *PackageDef) VersionFile() string {
	// It is already validated by LoadPackageDef, so just return it.
	for _, chunk := range def.Data {
		if chunk.VersionFile != "" {
			return chunk.VersionFile
		}
	}
	return ""
}

// makeExclusionFilter produces a predicate that checks an absolute file path
// against a list of regexps.
//
// Regexps are defined against slash separated paths relative to 'startDir'.
// The predicate takes absolute native path, converts it to slash separated path
// relative to 'startDir' and checks against list of regexps in 'patterns'.
// Returns true to exclude a path.
func makeExclusionFilter(startDir string, patterns []string) (ScanFilter, error) {
	if len(patterns) == 0 {
		return nil, nil
	}

	// Compile regular expressions.
	exps := []*regexp.Regexp{}
	for _, expr := range patterns {
		if expr == "" {
			continue
		}
		if expr[0] != '^' {
			expr = "^" + expr
		}
		if expr[len(expr)-1] != '$' {
			expr = expr + "$"
		}
		re, err := regexp.Compile(expr)
		if err != nil {
			return nil, err
		}
		exps = append(exps, re)
	}

	return func(abs string) bool {
		rel, err := filepath.Rel(startDir, abs)
		if err != nil {
			return true
		}
		// Do not evaluate paths outside of startDir or startDir itself.
		rel = filepath.ToSlash(rel)
		if rel == "." || strings.HasPrefix(rel, "../") {
			return false
		}
		for _, exp := range exps {
			if exp.MatchString(rel) {
				return true
			}
		}
		return false
	}, nil
}

////////////////////////////////////////////////////////////////////////////////
// Variable substitution.

var subVarsRe = regexp.MustCompile(`\$\{[^\}]+\}`)

// strings return array of pointers to all strings in PackageDef that can
// contain ${var} variables.
func (def *PackageDef) strings() []*string {
	out := []*string{
		&def.Package,
		&def.Root,
	}
	// Important to use index here, to get a point to a real object, not its copy.
	for i := range def.Data {
		out = append(out, def.Data[i].strings()...)
	}
	return out
}

// strings return array of pointers to all strings in PackageChunkDef that can
// contain ${var} variables.
func (def *PackageChunkDef) strings() []*string {
	out := []*string{
		&def.Dir,
		&def.File,
		&def.VersionFile,
	}
	for i := range def.Exclude {
		out = append(out, &def.Exclude[i])
	}
	return out
}

// subVars replaces "${key}" in strings with values from 'vars' map. Returns
// error if some keys weren't found in 'vars' map.
func subVars(s string, vars map[string]string) (string, error) {
	badKeys := []string{}
	res := subVarsRe.ReplaceAllStringFunc(s, func(match string) string {
		// Strip '${' and '}'.
		key := match[2 : len(match)-1]
		val, ok := vars[key]
		if !ok {
			badKeys = append(badKeys, key)
			return match
		}
		return val
	})
	if len(badKeys) != 0 {
		return res, fmt.Errorf("values for some variables are not provided: %v", badKeys)
	}
	return res, nil
}
