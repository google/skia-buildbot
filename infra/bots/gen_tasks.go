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
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/specs"
)

const (
	BUNDLE_RECIPES_NAME = "Housekeeper-PerCommit-BundleRecipes"

	DEFAULT_OS       = DEFAULT_OS_LINUX
	DEFAULT_OS_LINUX = "Debian-9.4"

	// Small is a 2-core machine.
	MACHINE_TYPE_SMALL = "n1-highmem-2"
	// Medium is a 16-core machine
	MACHINE_TYPE_MEDIUM = "n1-standard-16"
	// Large is a 64-core machine.
	MACHINE_TYPE_LARGE = "n1-highcpu-64"

	// Swarming output dirs.
	OUTPUT_NONE = "output_ignored" // This will result in outputs not being isolated.

	// Pool for Skia bots.
	POOL_SKIA = "Skia"

	PROJECT = "skia"

	SERVICE_ACCOUNT_COMPILE = "skia-external-compile-tasks@skia-swarming-bots.iam.gserviceaccount.com"
)

var (
	// "Constants"

	// Top-level list of all Jobs to run at each commit.
	JOBS = []string{
		"Housekeeper-OnDemand-Presubmit",
		"Infra-PerCommit-Small",
		"Infra-PerCommit-Medium",
		"Infra-PerCommit-Large",
		"Infra-PerCommit-Race",
	}

	// Versions of the following copied from
	// https://chrome-internal.googlesource.com/infradata/config/+/master/configs/cr-buildbucket/swarming_task_template_canary.json#42
	// to test the fix for chromium:836196.
	// (In the future we may want to use versions from
	// https://chrome-internal.googlesource.com/infradata/config/+/master/configs/cr-buildbucket/swarming_task_template.json#42)
	// TODO(borenet): Roll these versions automatically!
	CIPD_PKGS_PYTHON = []*specs.CipdPackage{
		&specs.CipdPackage{
			Name:    "infra/python/cpython/${platform}",
			Path:    "cipd_bin_packages",
			Version: "version:2.7.14.chromium14",
		},
		&specs.CipdPackage{
			Name:    "infra/tools/luci/vpython/${platform}",
			Path:    "cipd_bin_packages",
			Version: "git_revision:b6cdec8586c9f8d3d728b1bc0bd4331330ba66fc",
		},
	}

	CIPD_PKGS_KITCHEN = append([]*specs.CipdPackage{
		&specs.CipdPackage{
			Name:    "infra/tools/luci/kitchen/${platform}",
			Path:    ".",
			Version: "git_revision:546aae39f1fb9dce9add528e2011afa574535ecd",
		},
		&specs.CipdPackage{
			Name:    "infra/tools/luci-auth/${platform}",
			Path:    "cipd_bin_packages",
			Version: "git_revision:e1abc57be62d198b5c2f487bfb2fa2d2eb0e867c",
		},
	}, CIPD_PKGS_PYTHON...)

	CIPD_PKGS_GIT = []*specs.CipdPackage{
		&specs.CipdPackage{
			Name:    "infra/git/${platform}",
			Path:    "cipd_bin_packages",
			Version: "version:2.17.0.chromium15",
		},
		&specs.CipdPackage{
			Name:    "infra/tools/git/${platform}",
			Path:    "cipd_bin_packages",
			Version: "git_revision:e1abc57be62d198b5c2f487bfb2fa2d2eb0e867c",
		},
		&specs.CipdPackage{
			Name:    "infra/tools/luci/git-credential-luci/${platform}",
			Path:    "cipd_bin_packages",
			Version: "git_revision:e1abc57be62d198b5c2f487bfb2fa2d2eb0e867c",
		},
	}

	CIPD_PKGS_GSUTIL = []*specs.CipdPackage{
		&specs.CipdPackage{
			Name:    "infra/gsutil",
			Path:    "cipd_bin_packages",
			Version: "version:4.28",
		},
	}

	LOGDOG_ANNOTATION_URL = fmt.Sprintf("logdog://logs.chromium.org/%s/%s/+/annotations", PROJECT, specs.PLACEHOLDER_TASK_ID)
)

// Dimensions for Linux GCE instances.
func linuxGceDimensions(machineType string) []string {
	return []string{
		"pool:Skia",
		fmt.Sprintf("os:%s", DEFAULT_OS_LINUX),
		"gpu:none",
		"cpu:x86-64-Haswell_GCE",
		fmt.Sprintf("machine_type:%s", machineType),
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
	d := make(map[string]interface{}, len(p)+1)
	for k, v := range p {
		d[k] = interface{}(v)
	}
	d["$kitchen"] = struct {
		DevShell bool `json:"devshell"`
		GitAuth  bool `json:"git_auth"`
	}{
		DevShell: true,
		GitAuth:  true,
	}

	j, err := json.Marshal(d)
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
			"/bin/bash", "buildbot/infra/bots/bundle_recipes.sh", specs.PLACEHOLDER_ISOLATED_OUTDIR,
		},
		Dimensions: linuxGceDimensions(MACHINE_TYPE_SMALL),
		EnvPrefixes: map[string][]string{
			"PATH": []string{"cipd_bin_packages", "cipd_bin_packages/bin"},
		},
		Isolate: "infrabots.isolate",
	})
	return BUNDLE_RECIPES_NAME
}

// kitchenTask returns a specs.TaskSpec instance which uses Kitchen to run a
// recipe.
func kitchenTask(name, recipe, isolate, serviceAccount string, dimensions []string, extraProps map[string]string, outputDir string) *specs.TaskSpec {
	cipd := append([]*specs.CipdPackage{}, CIPD_PKGS_KITCHEN...)
	properties := map[string]string{
		"buildername":   name,
		"patch_issue":   specs.PLACEHOLDER_ISSUE,
		"patch_ref":     specs.PLACEHOLDER_PATCH_REF,
		"patch_repo":    specs.PLACEHOLDER_PATCH_REPO,
		"patch_set":     specs.PLACEHOLDER_PATCHSET,
		"patch_storage": specs.PLACEHOLDER_PATCH_STORAGE,
		"repository":    specs.PLACEHOLDER_REPO,
		"revision":      specs.PLACEHOLDER_REVISION,
		"swarm_out_dir": specs.PLACEHOLDER_ISOLATED_OUTDIR,
	}
	for k, v := range extraProps {
		properties[k] = v
	}
	var outputs []string = nil
	if outputDir != OUTPUT_NONE {
		outputs = []string{outputDir}
	}
	return &specs.TaskSpec{
		Caches: []*specs.Cache{
			&specs.Cache{
				Name: "vpython",
				Path: "cache/vpython",
			},
		},
		CipdPackages: cipd,
		Command: []string{
			"./kitchen${EXECUTABLE_SUFFIX}", "cook",
			"-checkout-dir", "recipe_bundle",
			"-mode", "swarming",
			"-luci-system-account", "system",
			"-cache-dir", "cache",
			"-temp-dir", "tmp",
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
			"-recipe", recipe,
			"-properties", props(properties),
			"-logdog-annotation-url", LOGDOG_ANNOTATION_URL,
		},
		Dependencies: []string{BUNDLE_RECIPES_NAME},
		Dimensions:   dimensions,
		EnvPrefixes: map[string][]string{
			"PATH":                    []string{"cipd_bin_packages", "cipd_bin_packages/bin"},
			"VPYTHON_VIRTUALENV_ROOT": []string{"${cache_dir}/vpython"},
		},
		ExtraTags: map[string]string{
			"log_location": LOGDOG_ANNOTATION_URL,
		},
		Isolate:        isolate,
		Outputs:        outputs,
		ServiceAccount: serviceAccount,
	}
}

// infra generates an infra test Task. Returns the name of the last Task in the
// generated chain of Tasks, which the Job should add as a dependency.
func infra(b *specs.TasksCfgBuilder, name string) string {
	machineType := MACHINE_TYPE_MEDIUM
	if strings.Contains(name, "Large") {
		// Using MACHINE_TYPE_LARGE for Large tests saves ~2 minutes.
		machineType = MACHINE_TYPE_LARGE
	}
	task := kitchenTask(name, "swarm_infra", "infrabots.isolate", SERVICE_ACCOUNT_COMPILE, linuxGceDimensions(machineType), nil, OUTPUT_NONE)
	task.CipdPackages = append(task.CipdPackages, CIPD_PKGS_GIT...)
	task.CipdPackages = append(task.CipdPackages, b.MustGetCipdPackageFromAsset("go"))
	task.CipdPackages = append(task.CipdPackages, b.MustGetCipdPackageFromAsset("node"))
	task.CipdPackages = append(task.CipdPackages, CIPD_PKGS_GSUTIL...)
	if strings.Contains(name, "Large") {
		task.CipdPackages = append(task.CipdPackages, b.MustGetCipdPackageFromAsset("protoc"))
	}

	// Cloud datastore tests are assumed to be marked as 'Large'
	if strings.Contains(name, "Large") || strings.Contains(name, "Race") {
		task.CipdPackages = append(task.CipdPackages, b.MustGetCipdPackageFromAsset("gcloud_linux"))
	}

	// Re-run failing bots but not when testing for race conditions.
	task.MaxAttempts = 2
	if strings.Contains(name, "Race") {
		task.MaxAttempts = 1
		task.IoTimeout = 1 * time.Hour
	}
	b.MustAddTask(name, task)
	return name
}

// Run the presubmit.
func presubmit(b *specs.TasksCfgBuilder, name string) string {
	extraProps := map[string]string{
		"category":         "cq",
		"patch_gerrit_url": "https://skia-review.googlesource.com",
		"patch_project":    "buildbot",
		"patch_ref":        fmt.Sprintf("refs/changes/%s/%s/%s", specs.PLACEHOLDER_ISSUE_SHORT, specs.PLACEHOLDER_ISSUE, specs.PLACEHOLDER_PATCHSET),
		"reason":           "CQ",
		"repo_name":        "skia_buildbot",
	}
	task := kitchenTask(name, "run_presubmit", "empty.isolate", SERVICE_ACCOUNT_COMPILE, linuxGceDimensions(MACHINE_TYPE_MEDIUM), extraProps, OUTPUT_NONE)

	replaceArg := func(key, value string) {
		found := false
		for idx, arg := range task.Command {
			if arg == key {
				task.Command[idx+1] = value
				found = true
			}
		}
		if !found {
			task.Command = append(task.Command, key, value)
		}
	}
	replaceArg("-repository", "https://chromium.googlesource.com/chromium/tools/build")
	replaceArg("-revision", "HEAD")
	task.Caches = append(task.Caches, []*specs.Cache{
		&specs.Cache{
			Name: "git",
			Path: "cache/git",
		},
		&specs.Cache{
			Name: "git_cache",
			Path: "cache/git_cache",
		},
	}...)
	task.CipdPackages = append(task.CipdPackages, CIPD_PKGS_GIT...)
	task.Dependencies = []string{} // No bundled recipes for this one.
	b.MustAddTask(name, task)
	return name
}

// process generates Tasks and Jobs for the given Job name.
func process(b *specs.TasksCfgBuilder, name string) {
	var priority float64 // Leave as default for most jobs.
	deps := []string{}

	// Infra tests.
	if strings.Contains(name, "Infra-PerCommit") {
		deps = append(deps, infra(b, name))
	}
	// Presubmit.
	if strings.Contains(name, "Presubmit") {
		priority = 1
		deps = append(deps, presubmit(b, name))
	}

	// Add the Job spec.
	trigger := specs.TRIGGER_ANY_BRANCH
	if strings.Contains(name, "OnDemand") {
		trigger = specs.TRIGGER_ON_DEMAND
	}
	b.MustAddJob(name, &specs.JobSpec{
		Priority:  priority,
		TaskSpecs: deps,
		Trigger:   trigger,
	})
}

// Regenerate the tasks.json file.
func main() {
	b := specs.MustNewTasksCfgBuilder()

	// Create Tasks and Jobs.
	bundleRecipes(b)
	for _, name := range JOBS {
		process(b, name)
	}

	b.MustFinish()
}
