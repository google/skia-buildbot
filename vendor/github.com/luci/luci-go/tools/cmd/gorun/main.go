// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package main defines the `gorun` tool, a shorthand tool to extend the
// "go run"-like convenience to packages.
//
// Unfortunately, "go run" is hard to use when the target has more than one
// ".go" file, and even harder when the target has "_test.go" files.
//
// This is just a bootstrap to "go build /path/to/X && /path/to/X" using a
// temporary directory for build storage.
package main

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/luci/luci-go/common/errors"
)

func mainImpl(args []string) (int, error) {
	if len(args) < 1 {
		return 1, errors.Reason("accepts one argument: package").Err()
	}
	pkg, args := args[0], args[1:]

	// Create a temporary output directory to build into.
	tmpdir, err := ioutil.TempDir("", "luci-gorun")
	if err != nil {
		return 1, errors.Annotate(err).Reason("failed to create temporary directory").Err()
	}
	defer os.RemoveAll(tmpdir)

	exePath := filepath.Join(tmpdir, "gorun_target")
	if runtime.GOOS == "windows" {
		exePath += ".exe"
	}

	// Build the package.
	cmd := exec.Command("go", "build", "-o", exePath, pkg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return 1, errors.Annotate(err).Reason("failed to build: %(package)s").D("package", pkg).Err()
	}

	// Run the package.
	cmd = exec.Command(exePath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			if ws, ok := ee.Sys().(syscall.WaitStatus); ok {
				// Forward exit status.
				return ws.ExitStatus(), nil
			}
		}
		return 1, errors.Annotate(err).Reason("failed to run: %(package)s").D("package", pkg).Err()
	}

	return 0, nil
}

func main() {
	rc, err := mainImpl(os.Args[1:])
	if err != nil {
		_, _ = errors.RenderStack(err).DumpTo(os.Stderr)
	}
	os.Exit(rc)
}
