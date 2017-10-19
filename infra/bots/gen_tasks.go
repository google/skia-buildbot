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
	DEFAULT_OS       = DEFAULT_OS_LINUX
	DEFAULT_OS_LINUX = "Debian-9.1"

	// Pool for Skia bots.
	POOL_SKIA = "Skia"
)

var (
	// "Constants"

	// Top-level list of all Jobs to run at each commit.
	JOBS = []string{
		"Infra-PerCommit-Small",
		"Infra-PerCommit-Medium",
		"Infra-PerCommit-Large",
	}

	// TODO(borenet): Roll this version automatically!
	CIPD_PKGS_KITCHEN = []*specs.CipdPackage{
		&specs.CipdPackage{
			Name:    "infra/tools/luci/kitchen/${platform}",
			Path:    "kitchen",
			Version: "git_revision:178957e5b67f42bd0ec25c609febedb66dc5d544",
		},
		&specs.CipdPackage{
			Name:    "infra/git/${platform}",
			Path:    "cipd_bin_packages",
			Version: "version:2.14.1.chromium10",
		},
		&specs.CipdPackage{
			Name:    "infra/python/cpython/${platform}",
			Path:    "cipd_bin_packages",
			Version: "version:2.7.14.chromium14",
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
		&specs.CipdPackage{
			Name:    "infra/tools/buildbucket/${platform}",
			Path:    "cipd_bin_packages",
			Version: "git_revision:43acdc268b04a3eae197db17d81accbdfe928d94",
		},
		&specs.CipdPackage{
			Name:    "infra/tools/cloudtail/${platform}",
			Path:    "cipd_bin_packages",
			Version: "git_revision:43acdc268b04a3eae197db17d81accbdfe928d94",
		},
	}
)

// Apply the default CIPD packages.
func cipd(pkgs []*specs.CipdPackage) []*specs.CipdPackage {
	return append(CIPD_PKGS_KITCHEN, pkgs...)
}

func props(p map[string]string) string {
	j, err := json.Marshal(p)
	if err != nil {
		sklog.Fatal(err)
	}
	return strings.Replace(string(j), "\\u003c", "<", -1)
}

// infra generates an infra test Task. Returns the name of the last Task in the
// generated chain of Tasks, which the Job should add as a dependency.
func infra(b *specs.TasksCfgBuilder, name string) string {
	pkgs := []*specs.CipdPackage{b.MustGetCipdPackageFromAsset("go")}
	if strings.Contains(name, "Large") {
		pkgs = append(pkgs, b.MustGetCipdPackageFromAsset("protoc"))
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
	sklog.Infof("Serialized: %q", string(propsJson))
	b.MustAddTask(name, &specs.TaskSpec{
		CipdPackages: cipd(pkgs),
		Dimensions: []string{
			"pool:Skia",
			fmt.Sprintf("os:%s", DEFAULT_OS_LINUX),
			"gpu:none",
			"cpu:x86-64-Haswell_GCE",
		},
		ExtraArgs: []string{
			"-recipe", "swarm_infra",
			"-properties", string(propsJson),
		},
		Isolate:  "swarm_recipe.isolate",
		Priority: 0.8,
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
