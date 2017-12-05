// Copyright 2016 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

/*
	Generate the tasks.json file.
*/

import (
	"encoding/json"
	"fmt"
	"strings"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/specs"
)

const (
	BUNDLE_RECIPES_NAME = "Housekeeper-PerCommit-BundleRecipes"

	DEFAULT_OS       = DEFAULT_OS_LINUX
	DEFAULT_OS_LINUX = "Debian-9.2"

	// Pool for Skia bots.
	POOL_SKIA = "Skia"
	PROJECT   = "skia"
)

var (
	// "Constants"

	// Top-level list of all Jobs to run at each commit.
	JOBS = []string{
		"Infra-PerCommit-Small",
		"Infra-PerCommit-Medium",
		"Infra-PerCommit-Large",
		"Infra-PerCommit-Race",
	}

	// TODO(borenet): Roll these versions automatically!
	CIPD_PKGS_KITCHEN = []*specs.CipdPackage{
		&specs.CipdPackage{
			Name:    "infra/tools/luci/kitchen/${platform}",
			Path:    ".",
			Version: "git_revision:178957e5b67f42bd0ec25c609febedb66dc5d544",
		},
		&specs.CipdPackage{
			Name:    "infra/python/cpython/${platform}",
			Path:    "cipd_bin_packages",
			Version: "version:2.7.14.chromium14",
		},
		&specs.CipdPackage{
			Name:    "infra/tools/luci/vpython/${platform}",
			Path:    "cipd_bin_packages",
			Version: "git_revision:25b0564a204da2bfd6346f26a59b9efb8cfc2212",
		},
		&specs.CipdPackage{
			Name:    "infra/tools/authutil/${platform}",
			Path:    "cipd_bin_packages",
			Version: "git_revision:9c63809842a277ce10a86afd51b61c639a665d11",
		},
	}

	CIPD_PKGS_GIT = []*specs.CipdPackage{
		&specs.CipdPackage{
			Name:    "infra/git/${platform}",
			Path:    "cipd_bin_packages",
			Version: "version:2.14.1.chromium10",
		},
		&specs.CipdPackage{
			Name:    "infra/tools/git/${platform}",
			Path:    "cipd_bin_packages",
			Version: "git_revision:fa7a52f4741f5e04bba0dfccc9b8456dc572c60b",
		},
		&specs.CipdPackage{
			Name:    "infra/tools/luci/git-credential-luci/${platform}",
			Path:    "cipd_bin_packages",
			Version: "git_revision:fa7a52f4741f5e04bba0dfccc9b8456dc572c60b",
		},
	}

	CIPD_PKGS_PYTHON = []*specs.CipdPackage{
		&specs.CipdPackage{
			Name:    "infra/python/cpython/${platform}",
			Path:    "cipd_bin_packages",
			Version: "version:2.7.14.chromium14",
		},
		&specs.CipdPackage{
			Name:    "infra/tools/luci/vpython/${platform}",
			Path:    "cipd_bin_packages",
			Version: "git_revision:25b0564a204da2bfd6346f26a59b9efb8cfc2212",
		},
	}
)

// Dimensions for Linux GCE instances.
func linuxGceDimensions() []string {
	return []string{
		"pool:Skia",
		fmt.Sprintf("os:%s", DEFAULT_OS_LINUX),
		"gpu:none",
		"cpu:x86-64-Haswell_GCE",
	}
}

// Apply the default CIPD packages.
func cipd(pkgs []*specs.CipdPackage) []*specs.CipdPackage {
	// We also need Git.
	rv := append(CIPD_PKGS_KITCHEN, CIPD_PKGS_GIT...)
	return append(rv, pkgs...)
}

// Create a properties JSON string.
func props(p map[string]string) string {
	j, err := json.Marshal(p)
	if err != nil {
		sklog.Fatal(err)
	}
	return strings.Replace(string(j), "\\u003c", "<", -1)
}

// bundleRecipes generates the task to bundle and isolate the recipes.
func bundleRecipes(b *specs.TasksCfgBuilder) string {
	b.MustAddTask(BUNDLE_RECIPES_NAME, &specs.TaskSpec{
		CipdPackages: append(CIPD_PKGS_GIT, CIPD_PKGS_PYTHON...),
		Command: []string{
			"/bin/bash", "bundle_recipes.sh", specs.PLACEHOLDER_ISOLATED_OUTDIR,
		},
		Dimensions: linuxGceDimensions(),
		Isolate:    "infrabots.isolate",
		Priority:   0.7,
	})
	return BUNDLE_RECIPES_NAME
}

// infra generates an infra test Task. Returns the name of the last Task in the
// generated chain of Tasks, which the Job should add as a dependency.
func infra(b *specs.TasksCfgBuilder, name string) string {
	bundle := bundleRecipes(b)

	pkgs := []*specs.CipdPackage{b.MustGetCipdPackageFromAsset("go")}
	if strings.Contains(name, "Large") {
		pkgs = append(pkgs, b.MustGetCipdPackageFromAsset("protoc"))
	}
	attempts := 2
	if strings.Contains(name, "Race") {
		attempts = 1
	}
	propsJson := props(map[string]string{
		"repository":    specs.PLACEHOLDER_REPO,
		"buildername":   name,
		"mastername":    "fake-master",
		"buildnumber":   "2",
		"slavename":     "fake-buildslave",
		"nobuildbot":    "True",
		"swarm_out_dir": specs.PLACEHOLDER_ISOLATED_OUTDIR,
		"patch_storage": specs.PLACEHOLDER_PATCH_STORAGE,
		"patch_issue":   specs.PLACEHOLDER_ISSUE,
		"patch_set":     specs.PLACEHOLDER_PATCHSET,
	})
	b.MustAddTask(name, &specs.TaskSpec{
		CipdPackages: cipd(pkgs),
		Command: []string{
			"./kitchen${EXECUTABLE_SUFFIX}", "cook",
			"-checkout-dir", "recipe_bundle",
			"-mode", "swarming",
			"-luci-system-account", "system",
			"-cache-dir", "cache",
			"-temp-dir", "tmp",
			"-set-env-abspath", "VPYTHON_VIRTUALENV_ROOT=cache/vpython",
			"-prefix-path-env", "go",
			"-prefix-path-env", "cipd_bin_packages",
			"-prefix-path-env", "cipd_bin_packages/bin",
			"-known-gerrit-host", "android.googlesource.com",
			"-known-gerrit-host", "boringssl.googlesource.com",
			"-known-gerrit-host", "chromium.googlesource.com",
			"-known-gerrit-host", "dart.googlesource.com",
			"-known-gerrit-host", "fuchsia.googlesource.com",
			"-known-gerrit-host", "go.googlesource.com",
			"-known-gerrit-host", "llvm.googlesource.com",
			"-known-gerrit-host", "pdfium.googlesource.com",
			"-known-gerrit-host", "skia.googlesource.com",
			"-known-gerrit-host", "webrtc.googlesource.com",
			"-output-result-json", "${ISOLATED_OUTDIR}/build_result_filename",
			"-workdir", ".",
			"-recipe", "swarm_infra",
			"-properties", string(propsJson),
			"-logdog-annotation-url", fmt.Sprintf("logdog://logs.chromium.org/%s/%s/+/annotations", PROJECT, specs.PLACEHOLDER_TASK_ID),
		},
		Dependencies: []string{bundle},
		Dimensions:   linuxGceDimensions(),
		Isolate:      "infrabots.isolate",
		Priority:     0.8,
		MaxAttempts:  attempts,
	})
	return name
}

// process generates Tasks and Jobs for the given Job name.
func process(b *specs.TasksCfgBuilder, name string) {
	deps := []string{}

	// Infra tests.
	if strings.Contains(name, "Infra-PerCommit") {
		deps = append(deps, infra(b, name))
	}

	// Add the Job spec.
	b.MustAddJob(name, &specs.JobSpec{
		Priority:  0.8,
		TaskSpecs: deps,
	})
}

// Regenerate the tasks.json file.
func main() {
	b := specs.MustNewTasksCfgBuilder()

	// Create Tasks and Jobs.
	for _, name := range JOBS {
		process(b, name)
	}

	b.MustFinish()
}
