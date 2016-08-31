// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"strings"

	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/deploytool/managedfs"
)

// stageGoPath creates a GOPATH-compatible directory consisting of all of the
// GOPATHs configured in the supplied components sources. The path begins with
// "src/", and is rooted in the supplied root.
func stageGoPath(w *work, comp *layoutDeploymentComponent, root *managedfs.Dir) error {
	// Build our GoPath sources. To do this, we will build subdirectories under
	// "src" for the various GoPath components, then symlink the last directory
	// component to the actual GOPATH root.
	//
	// We need to detect path conflicts where one GOPATH checks out into the
	// parent of another GOPATH, e.g.:
	// /foo/bar/baz => A
	// /foo/bar => B
	//
	// We do this by checking intermediate Go paths against our deployment plan
	// incrementally.
	dirs := make(map[string]struct{})
	build := make(map[string]string)
	for _, src := range comp.sources {
		if src.InitResult == nil {
			continue
		}

		for _, gopath := range src.InitResult.GoPath {
			// Make sure our Go package isn't a directory.
			if _, ok := dirs[gopath.GoPackage]; ok {
				return errors.Reason("GOPATH %(package)q is both a package and directory").
					D("package", gopath.GoPackage).Err()
			}

			// Check intermediate paths to make sure there isn't a deployment
			// conflict.
			pkgParts := splitGoPackage(gopath.GoPackage)
			for _, parentPkg := range pkgParts[:len(pkgParts)-1] {
				if _, ok := build[parentPkg]; ok {
					return errors.Reason("GOPATH %(package)q is both a package and directory").
						D("package", parentPkg).Err()
				}
				dirs[parentPkg] = struct{}{}
			}

			// Everything checks out, add this link.
			build[gopath.GoPackage] = src.pathTo(gopath.Path, "")
		}
	}

	srcDir, err := root.EnsureDirectory("src")
	if err != nil {
		return err
	}

	for pkg, src := range build {
		var (
			pkgComponents = strings.Split(pkg, "/")
			d             = srcDir
		)
		for _, comp := range pkgComponents[:len(pkgComponents)-1] {
			var err error
			d, err = d.EnsureDirectory(comp)
			if err != nil {
				return errors.Annotate(err).Reason("could not create GOPATH parent directory [%(path)s]").
					D("path", d).Err()
			}
		}
		link := d.File(pkgComponents[len(pkgComponents)-1])
		if err := link.SymlinkFrom(src, true); err != nil {
			return errors.Annotate(err).Reason("failed to create GOPATH link").Err()
		}
	}
	return nil
}
