// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package isolate

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/luci/luci-go/client/archiver"
	"github.com/luci/luci-go/client/internal/common"
	"github.com/luci/luci-go/common/flag/stringmapflag"
	"github.com/luci/luci-go/common/isolated"
	"github.com/luci/luci-go/common/isolatedclient"
	"github.com/luci/luci-go/common/runtime/tracer"
)

// IsolatedGenJSONVersion is used in the batcharchive json format.
//
// TODO(tandrii): Migrate to batch_archive.go.
const IsolatedGenJSONVersion = 1

// ValidVariable is the regexp of valid isolate variable name.
const ValidVariable = "[A-Za-z_][A-Za-z_0-9]*"

var validVariableMatcher = regexp.MustCompile(ValidVariable)
var variableSubstitutionMatcher = regexp.MustCompile("<\\(" + ValidVariable + "\\)")

// IsValidVariable returns true if the variable is a valid symbol name.
func IsValidVariable(variable string) bool {
	return validVariableMatcher.MatchString(variable)
}

// Tree to be isolated.
type Tree struct {
	Cwd  string
	Opts ArchiveOptions
}

// ArchiveOptions for achiving trees.
type ArchiveOptions struct {
	Isolate         string              `json:"isolate"`
	Isolated        string              `json:"isolated"`
	Blacklist       common.Strings      `json:"blacklist"`
	PathVariables   stringmapflag.Value `json:"path_variables"`
	ExtraVariables  stringmapflag.Value `json:"extra_variables"`
	ConfigVariables stringmapflag.Value `json:"config_variables"`
}

// Init initializes with non-nil values.
func (a *ArchiveOptions) Init() {
	a.Blacklist = common.Strings{}
	a.PathVariables = map[string]string{}
	if common.IsWindows() {
		a.PathVariables["EXECUTABLE_SUFFIX"] = ".exe"
	} else {
		a.PathVariables["EXECUTABLE_SUFFIX"] = ""
	}
	a.ExtraVariables = map[string]string{}
	a.ConfigVariables = map[string]string{}
}

// PostProcess post-processes the flags to fix any compatibility issue.
func (a *ArchiveOptions) PostProcess(cwd string) {
	// Set default blacklist only if none is set.
	if len(a.Blacklist) == 0 {
		// This cannot be generalized as ".*" as there is known use that require
		// a ".pki" directory to be mapped.
		a.Blacklist = common.Strings{
			// Temporary python files.
			"*.pyc",
			// Temporary vim files.
			"*.swp",
			".git",
			".hg",
			".svn",
		}
	}
	if !filepath.IsAbs(a.Isolate) {
		a.Isolate = filepath.Join(cwd, a.Isolate)
	}
	a.Isolate = filepath.Clean(a.Isolate)

	if !filepath.IsAbs(a.Isolated) {
		a.Isolated = filepath.Join(cwd, a.Isolated)
	}
	a.Isolated = filepath.Clean(a.Isolated)

	for k, v := range a.PathVariables {
		// This is due to a Windows + GYP specific issue, where double-quoted paths
		// would get mangled in a way that cannot be resolved unless a space is
		// injected.
		a.PathVariables[k] = strings.TrimSpace(v)
	}
}

// ReplaceVariables replaces any occurrences of '<(FOO)' in 'str' with the
// corresponding variable from 'opts'.
//
// If any substitution refers to a variable that is missing, the returned error will
// refer to the first such variable. In the case of errors, the returned string will
// still contain a valid result for any non-missing substitutions.
func ReplaceVariables(str string, opts *ArchiveOptions) (string, error) {
	var err error
	subst := variableSubstitutionMatcher.ReplaceAllStringFunc(str,
		func(match string) string {
			varName := match[2 : len(match)-1]
			if v, ok := opts.PathVariables[varName]; ok {
				return v
			}
			if v, ok := opts.ExtraVariables[varName]; ok {
				return v
			}
			if v, ok := opts.ConfigVariables[varName]; ok {
				return v
			}
			if err == nil {
				err = errors.New("no value for variable '" + varName + "'")
			}
			return match
		})
	return subst, err
}

// Archive processes a .isolate, generates a .isolated and archive it.
// Returns a *Item to the .isolated.
func Archive(arch *archiver.Archiver, opts *ArchiveOptions) *archiver.Item {
	displayName := filepath.Base(opts.Isolated)
	defer tracer.Span(arch, strings.SplitN(displayName, ".", 2)[0]+":archive", nil)(nil)
	f, err := archive(arch, opts, displayName)
	if err != nil {
		arch.Cancel(err)
		i := &archiver.Item{DisplayName: displayName}
		i.SetErr(err)
		return i
	}
	return f
}

func processing(opts *ArchiveOptions) (int, int, []string, string, *isolated.Isolated, error) {
	content, err := ioutil.ReadFile(opts.Isolate)
	if err != nil {
		return 0, 0, nil, "", nil, err
	}
	cmd, deps, readOnly, isolateDir, err := LoadIsolateForConfig(filepath.Dir(opts.Isolate), content, opts.ConfigVariables)
	if err != nil {
		return 0, 0, nil, "", nil, err
	}

	// Check for variable error before doing anything.
	for i := range cmd {
		if cmd[i], err = ReplaceVariables(cmd[i], opts); err != nil {
			return 0, 0, nil, "", nil, err
		}
	}
	filesCount := 0
	dirsCount := 0
	for i := range deps {
		if deps[i], err = ReplaceVariables(deps[i], opts); err != nil {
			return 0, 0, nil, "", nil, err
		}
		if deps[i][len(deps[i])-1] == os.PathSeparator {
			dirsCount++
		} else {
			filesCount++
		}
	}

	// Convert all dependencies to absolute path and find the root directory to
	// use.
	for i, dep := range deps {
		clean := filepath.Clean(filepath.Join(isolateDir, dep))
		if dep[len(dep)-1] == os.PathSeparator {
			clean += osPathSeparator
		}
		deps[i] = clean
	}
	rootDir := isolateDir
	for _, dep := range deps {
		base := filepath.Dir(dep)
		for {
			rel, err := filepath.Rel(rootDir, base)
			if err != nil {
				return 0, 0, nil, "", nil, err
			}
			if !strings.HasPrefix(rel, "..") {
				break
			}
			newRootDir := filepath.Dir(rootDir)
			if newRootDir == rootDir {
				return 0, 0, nil, "", nil, errors.New("failed to find root dir")
			}
			rootDir = newRootDir
		}
	}
	if rootDir != isolateDir {
		log.Printf("Root: %s", rootDir)
	}

	// Prepare the .isolated file.
	i := &isolated.Isolated{
		Algo:     "sha-1",
		Files:    map[string]isolated.File{},
		ReadOnly: readOnly.ToIsolated(),
		Version:  isolated.IsolatedFormatVersion,
	}
	if len(cmd) != 0 {
		i.Command = cmd
	}
	if rootDir != isolateDir {
		relPath, err := filepath.Rel(rootDir, isolateDir)
		if err != nil {
			return 0, 0, nil, "", nil, err
		}
		i.RelativeCwd = relPath
	}
	// Processing of the .isolate file ended.
	return filesCount, dirsCount, deps, rootDir, i, err
}

func archive(arch *archiver.Archiver, opts *ArchiveOptions, displayName string) (*archiver.Item, error) {
	end := tracer.Span(arch, strings.SplitN(displayName, ".", 2)[0]+":loading", nil)
	filesCount, dirsCount, deps, rootDir, i, err := processing(opts)
	end(tracer.Args{"err": err})
	if err != nil {
		return nil, err
	}
	// Handle each dependency, either a file or a directory..
	fileItems := make([]*archiver.Item, 0, filesCount)
	dirItems := make([]*archiver.Item, 0, dirsCount)
	for _, dep := range deps {
		relPath, err := filepath.Rel(rootDir, dep)
		if err != nil {
			return nil, err
		}
		if dep[len(dep)-1] == os.PathSeparator {
			relPath, err := filepath.Rel(rootDir, dep)
			if err != nil {
				return nil, err
			}
			dirItems = append(dirItems, archiver.PushDirectory(arch, dep, relPath, opts.Blacklist))
		} else {
			// Grab the stats right away.
			info, err := os.Lstat(dep)
			if err != nil {
				return nil, err
			}
			mode := info.Mode()
			if mode&os.ModeSymlink == os.ModeSymlink {
				l, err := os.Readlink(dep)
				if err != nil {
					return nil, err
				}
				i.Files[relPath] = isolated.File{Link: newString(l)}
			} else {
				i.Files[relPath] = isolated.File{Mode: newInt(int(mode.Perm())), Size: newInt64(info.Size())}
				fileItems = append(fileItems, arch.PushFile(relPath, dep, -info.Size()))
			}
		}
	}

	for _, item := range fileItems {
		item.WaitForHashed()
		if err = item.Error(); err != nil {
			return nil, err
		}
		f := i.Files[item.DisplayName]
		f.Digest = item.Digest()
		i.Files[item.DisplayName] = f
	}
	// Avoid duplicated entries in includes.
	// TODO(tandrii): add test to reproduce the problem.
	includesSet := map[isolated.HexDigest]bool{}
	for _, item := range dirItems {
		item.WaitForHashed()
		if err = item.Error(); err != nil {
			return nil, err
		}
		includesSet[item.Digest()] = true
	}
	for digest := range includesSet {
		i.Includes = append(i.Includes, digest)
	}
	// Make the includes list deterministic.
	sort.Sort(i.Includes)

	raw := &bytes.Buffer{}
	if err = json.NewEncoder(raw).Encode(i); err != nil {
		return nil, err
	}

	if err := ioutil.WriteFile(opts.Isolated, raw.Bytes(), 0644); err != nil {
		return nil, err
	}
	return arch.Push(displayName, isolatedclient.NewBytesSource(raw.Bytes()), 0), nil
}
