// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"strings"

	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/deploytool/api/deploy"
	"github.com/luci/luci-go/deploytool/managedfs"
)

func flattenVarToDir(v string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case (r >= 'a' && r <= 'z'),
			(r >= 'A' && r <= 'Z'),
			(r >= '0' && r <= '9'),
			(r == '.'), (r == '_'):
			return r

		default:
			return '_'
		}
	}, v)
}

// buildComponent runs the varuous build scripts defined by the specified
// deployment component.
//
// After running the build scripts, variable substitution will occur on "c"
// to update its directory.
func buildComponent(w *work, c *layoutDeploymentComponent, root *managedfs.Dir) error {
	if src := c.source(); !src.Source.RunScripts {
		return errors.Reason("refusing to run scripts (run_scripts is false for %(source)q)").
			D("source", src.title).Err()
	}

	// Create our build directories and map them to variables.
	c.buildDirs = make(map[string]string, len(c.Build))
	dirs := make([]*managedfs.Dir, len(c.Build))
	for i, b := range c.Build {
		if _, has := c.buildDirs[b.DirKey]; has {
			return errors.Reason("duplicate build key [%(key)s]").D("key", b.DirKey).Err()
		}

		dir, err := root.EnsureDirectory(fmt.Sprintf("%d_%s", i, flattenVarToDir(b.DirKey)))
		if err != nil {
			return errors.Annotate(err).Reason("failed to create build directory").Err()
		}

		dirs[i] = dir
		c.buildDirs[b.DirKey] = dir.String()
	}

	// Apply build directories.
	if err := c.expandPaths(); err != nil {
		return errors.Annotate(err).Reason("failed to expand component paths").Err()
	}

	// Run all of our build scripts in parallel.
	err := w.RunMulti(func(workC chan<- func() error) {
		for i, b := range c.Build {
			i, b := i, b
			workC <- func() error {
				return runComponentBuild(w, c, b, dirs[i])
			}
		}
	})
	if err != nil {
		return errors.Annotate(err).Reason("failed to run component builds").Err()
	}
	return nil
}

func runComponentBuild(w *work, c *layoutDeploymentComponent, b *deploy.Component_Build, root *managedfs.Dir) error {
	switch t := b.Operation.(type) {
	case *deploy.Component_Build_PythonScript_:
		ps := t.PythonScript

		python, err := w.python()
		if err != nil {
			return err
		}

		src := c.source()
		scriptPath := src.pathTo(ps.Path, c.comp.Path)
		buildDir := root.String()

		args := append(make([]string, 0, 2+len(ps.ExtraArgs)),
			src.checkoutPath(),
			buildDir)
		args = append(args, ps.ExtraArgs...)

		if err := python.exec(scriptPath, args...).cwd(buildDir).check(w); err != nil {
			return errors.Annotate(err).Reason("failed to execute [%(scriptPath)s]").D("scriptPath", scriptPath).Err()
		}
		return nil

	default:
		return errors.Reason("unknown build type %(type)T").D("type", t).Err()
	}
}
