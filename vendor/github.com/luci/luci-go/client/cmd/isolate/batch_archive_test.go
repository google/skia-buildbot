// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/luci/luci-go/client/internal/common"
	"github.com/luci/luci-go/common/flag/stringmapflag"
	"github.com/maruel/ut"
)

func TestConvertPyToGoArchiveCMDArgs(t *testing.T) {
	t.Parallel()
	data := []struct {
		input    []string
		expected []string
	}{
		// Simple.
		{
			[]string{"--path-variable", "key=value"},
			[]string{"--path-variable", "key=value"},
		},
		{
			[]string{"--path-variable", "key", "value1"},
			[]string{"--path-variable", "key=value1"},
		},
		// That's how python isolate works.
		{
			[]string{"--extra-variable", "key", "and spaces"},
			[]string{"--extra-variable", "key=and spaces"},
		},
		{
			[]string{"--path-variable", "key", "--even-this-value"},
			[]string{"--path-variable", "key=--even-this-value"},
		},
		// Other args.
		{
			[]string{"-x", "--var", "--config-variable", "key", "value"},
			[]string{"-x", "--var", "--config-variable", "key=value"},
		},
		{
			[]string{"--path-variable", "key", "value", "posarg"},
			[]string{"--path-variable", "key=value", "posarg"},
		},
		// Too few args are just ignored.
		{
			[]string{"--path-variable"},
			[]string{"--path-variable"},
		},
		{
			[]string{"--path-variable", "key-and-no-value"},
			[]string{"--path-variable", "key-and-no-value"},
		},
	}
	for i, line := range data {
		ut.AssertEqualIndex(t, i, line.expected, convertPyToGoArchiveCMDArgs(line.input))
	}
}

func TestInvalidArchiveCMD(t *testing.T) {
	t.Parallel()
	_, err := parseArchiveCMD([]string{}, absToOS("e:", "/tmp/bar"))
	ut.AssertEqual(t, "-isolated must be specified", err.Error())
}

func TestArchiveCMDParsing(t *testing.T) {
	t.Parallel()
	args := []string{
		"--isolated", "../biz/bar.isolated",
		"--isolate", "../boz/bar.isolate",
		"--path-variable", "DEPTH", "../..",
		"--path-variable", "PRODUCT_DIR", "../../out/Release",
		"--extra-variable", "version_full=42.0.2284.0",
		"--config-variable", "OS=linux",
	}
	root := absToOS("e:", "/tmp/bar")
	opts, err := parseArchiveCMD(args, root)
	base := filepath.Dir(root)
	ut.AssertEqual(t, filepath.Join(base, "boz", "bar.isolate"), opts.Isolate)
	ut.AssertEqual(t, filepath.Join(base, "biz", "bar.isolated"), opts.Isolated)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, opts.ConfigVariables, stringmapflag.Value{"OS": "linux"})
	if common.IsWindows() {
		ut.AssertEqual(t, opts.PathVariables, stringmapflag.Value{"PRODUCT_DIR": "../../out/Release", "EXECUTABLE_SUFFIX": ".exe", "DEPTH": "../.."})
	} else {
		ut.AssertEqual(t, opts.PathVariables, stringmapflag.Value{"PRODUCT_DIR": "../../out/Release", "EXECUTABLE_SUFFIX": "", "DEPTH": "../.."})
	}
	ut.AssertEqual(t, opts.ExtraVariables, stringmapflag.Value{"version_full": "42.0.2284.0"})
}

// Verify that if the isolate/isolated paths are absolute, we don't
// accidentally interpret them as relative to the cwd.
func TestArchiveAbsolutePaths(t *testing.T) {
	t.Parallel()
	root := absToOS("e:", "/tmp/bar/")
	args := []string{
		"--isolated", root + "foo.isolated",
		"--isolate", root + "foo.isolate",
	}
	opts, err := parseArchiveCMD(args, absToOS("x:", "/var/lib"))
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, root+"foo.isolate", opts.Isolate)
	ut.AssertEqual(t, root+"foo.isolated", opts.Isolated)
}

// Private stuff.

// absToOS converts a POSIX path to OS specific format.
func absToOS(drive, p string) string {
	if common.IsWindows() {
		return drive + strings.Replace(p, "/", "\\", -1)
	}
	return p
}
