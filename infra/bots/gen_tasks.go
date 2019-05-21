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
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/specs"
)

const (
	BUILD_TASK_DRIVERS_NAME = "Housekeeper-PerCommit-BuildTaskDrivers"
	BUNDLE_RECIPES_NAME     = "Housekeeper-PerCommit-BundleRecipes"

	DEFAULT_OS       = DEFAULT_OS_LINUX
	DEFAULT_OS_LINUX = "Debian-9.8"

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

	SERVICE_ACCOUNT_COMPILE       = "skia-external-compile-tasks@skia-swarming-bots.iam.gserviceaccount.com"
	SERVICE_ACCOUNT_RECREATE_SKPS = "skia-recreate-skps@skia-swarming-bots.iam.gserviceaccount.com"
)

var (
	// "Constants"

	// Top-level list of all Jobs to run at each commit.
	JOBS = []string{
		"Housekeeper-Nightly-UpdateGoDeps",
		"Housekeeper-OnDemand-Presubmit",
		"Infra-PerCommit-Build",
		"Infra-PerCommit-Small",
		"Infra-PerCommit-Medium",
		"Infra-PerCommit-Large",
		"Infra-PerCommit-Race",
		"Infra-Experimental-Small",
	}

	// Versions of the following copied from
	// https://chrome-internal.googlesource.com/infradata/config/+/master/configs/cr-buildbucket/swarming_task_template_canary.json#42
	// to test the fix for chromium:836196.
	// (In the future we may want to use versions from
	// https://chrome-internal.googlesource.com/infradata/config/+/master/configs/cr-buildbucket/swarming_task_template.json#42)
	// TODO(borenet): Roll these versions automatically!
	CIPD_PKGS_PYTHON = []*specs.CipdPackage{
		{
			Name:    "infra/python/cpython/${platform}",
			Path:    "cipd_bin_packages",
			Version: "version:2.7.14.chromium14",
		},
		{
			Name:    "infra/tools/luci/vpython/${platform}",
			Path:    "cipd_bin_packages",
			Version: "git_revision:96f81e737868d43124b4661cf1c325296ca04944",
		},
	}

	CIPD_PKGS_KITCHEN = append([]*specs.CipdPackage{
		{
			Name:    "infra/tools/luci/kitchen/${platform}",
			Path:    ".",
			Version: "git_revision:d8f38ca9494b5af249942631f9cee45927f6b4bc",
		},
		{
			Name:    "infra/tools/luci-auth/${platform}",
			Path:    "cipd_bin_packages",
			Version: "git_revision:2c805f1c716f6c5ad2126b27ec88b8585a09481e",
		},
	}, CIPD_PKGS_PYTHON...)

	CIPD_PKGS_GIT = []*specs.CipdPackage{
		{
			Name:    "infra/git/${platform}",
			Path:    "cipd_bin_packages",
			Version: "version:2.17.1.chromium15",
		},
		{
			Name:    "infra/tools/git/${platform}",
			Path:    "cipd_bin_packages",
			Version: "git_revision:c9c8a52bfeaf8bc00ece22fdfd447822c8fcad77",
		},
		{
			Name:    "infra/tools/luci/git-credential-luci/${platform}",
			Path:    "cipd_bin_packages",
			Version: "git_revision:2c805f1c716f6c5ad2126b27ec88b8585a09481e",
		},
	}

	CIPD_PKGS_GSUTIL = []*specs.CipdPackage{
		{
			Name:    "infra/gsutil",
			Path:    "cipd_bin_packages",
			Version: "version:4.28",
		},
	}

	CACHES_GO = []*specs.Cache{
		{
			Name: "go_cache",
			Path: "cache/go_cache",
		},
		{
			Name: "gopath",
			Path: "cache/gopath",
		},
	}

	LOGDOG_ANNOTATION_URL = fmt.Sprintf("logdog://logs.chromium.org/%s/%s/+/annotations", PROJECT, specs.PLACEHOLDER_TASK_ID)
)

// relpath returns the relative path to the given file from the config file.
func relpath(f string) string {
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	rv, err := filepath.Rel(dir, path.Join(dir, f))
	if err != nil {
		sklog.Fatal(err)
	}
	return rv
}

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
			"PATH": {"cipd_bin_packages", "cipd_bin_packages/bin"},
		},
		Isolate: "infrabots.isolate",
	})
	return BUNDLE_RECIPES_NAME
}

// buildTaskDrivers generates the task to compile the task driver code to run on
// all platforms.
func buildTaskDrivers(b *specs.TasksCfgBuilder) string {
	b.MustAddTask(BUILD_TASK_DRIVERS_NAME, &specs.TaskSpec{
		Caches:       CACHES_GO,
		CipdPackages: append(CIPD_PKGS_GIT, b.MustGetCipdPackageFromAsset("go")),
		Command: []string{
			"/bin/bash", "buildbot/infra/bots/build_task_drivers.sh", specs.PLACEHOLDER_ISOLATED_OUTDIR,
		},
		Dimensions: linuxGceDimensions(MACHINE_TYPE_SMALL),
		EnvPrefixes: map[string][]string{
			"PATH": {"cipd_bin_packages", "cipd_bin_packages/bin", "go/go/bin"},
		},
		Isolate: "whole_repo.isolate",
	})
	return BUILD_TASK_DRIVERS_NAME

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
			{
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
			"PATH":                    {"cipd_bin_packages", "cipd_bin_packages/bin"},
			"VPYTHON_VIRTUALENV_ROOT": {"${cache_dir}/vpython"},
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
	task := kitchenTask(name, "swarm_infra", "whole_repo.isolate", SERVICE_ACCOUNT_COMPILE, linuxGceDimensions(machineType), nil, OUTPUT_NONE)
	task.CipdPackages = append(task.CipdPackages, CIPD_PKGS_GIT...)
	task.CipdPackages = append(task.CipdPackages, b.MustGetCipdPackageFromAsset("go"))
	task.Caches = append(task.Caches, CACHES_GO...)
	task.CipdPackages = append(task.CipdPackages, b.MustGetCipdPackageFromAsset("node"))
	task.CipdPackages = append(task.CipdPackages, CIPD_PKGS_GSUTIL...)
	if strings.Contains(name, "Large") || strings.Contains(name, "Build") {
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
	task.Caches = append(task.Caches, []*specs.Cache{
		{
			Name: "git",
			Path: "cache/git",
		},
		{
			Name: "git_cache",
			Path: "cache/git_cache",
		},
	}...)
	task.CipdPackages = append(task.CipdPackages, CIPD_PKGS_GIT...)
	task.CipdPackages = append(task.CipdPackages, &specs.CipdPackage{
		Name:    "infra/recipe_bundles/chromium.googlesource.com/chromium/tools/build",
		Path:    "recipe_bundle",
		Version: "refs/heads/master",
	})
	task.Dependencies = []string{} // No bundled recipes for this one.
	b.MustAddTask(name, task)
	return name
}

func experimental(b *specs.TasksCfgBuilder, name string) string {
	cipd := append([]*specs.CipdPackage{}, CIPD_PKGS_GIT...)
	cipd = append(cipd, CIPD_PKGS_GSUTIL...)
	cipd = append(cipd, b.MustGetCipdPackageFromAsset("go"), b.MustGetCipdPackageFromAsset("node"))

	machineType := MACHINE_TYPE_MEDIUM
	if strings.Contains(name, "Large") {
		// Using MACHINE_TYPE_LARGE for Large tests saves ~2 minutes.
		machineType = MACHINE_TYPE_LARGE
		cipd = append(cipd, b.MustGetCipdPackageFromAsset("protoc"))
	}

	t := &specs.TaskSpec{
		Caches:       CACHES_GO,
		CipdPackages: cipd,
		Command: []string{
			"./infra_tests",
			"--project_id", "skia-swarming-bots",
			"--task_id", specs.PLACEHOLDER_TASK_ID,
			"--task_name", name,
			"--workdir", ".",
			"--alsologtostderr",
		},
		Dependencies: []string{BUILD_TASK_DRIVERS_NAME},
		Dimensions:   linuxGceDimensions(machineType),
		EnvPrefixes: map[string][]string{
			"PATH": {"cipd_bin_packages", "cipd_bin_packages/bin", "go/go/bin"},
		},
		Isolate:        "whole_repo.isolate",
		ServiceAccount: SERVICE_ACCOUNT_COMPILE,
	}
	b.MustAddTask(name, t)
	return name
}

func updateGoDeps(b *specs.TasksCfgBuilder, name string) string {
	cipd := append([]*specs.CipdPackage{}, CIPD_PKGS_GIT...)
	cipd = append(cipd, b.MustGetCipdPackageFromAsset("go"))
	cipd = append(cipd, b.MustGetCipdPackageFromAsset("protoc"))

	machineType := MACHINE_TYPE_MEDIUM
	t := &specs.TaskSpec{
		Caches:       CACHES_GO,
		CipdPackages: cipd,
		Command: []string{
			"./update_go_deps",
			"--project_id", "skia-swarming-bots",
			"--task_id", specs.PLACEHOLDER_TASK_ID,
			"--task_name", name,
			"--workdir", ".",
			"--gerrit_project", "buildbot",
			"--gerrit_url", "https://skia-review.googlesource.com",
			"--repo", specs.PLACEHOLDER_REPO,
			"--reviewers", "borenet@google.com",
			"--revision", specs.PLACEHOLDER_REVISION,
			"--patch_issue", specs.PLACEHOLDER_ISSUE,
			"--patch_set", specs.PLACEHOLDER_PATCHSET,
			"--patch_server", specs.PLACEHOLDER_CODEREVIEW_SERVER,
			"--alsologtostderr",
		},
		Dependencies: []string{BUILD_TASK_DRIVERS_NAME},
		Dimensions:   linuxGceDimensions(machineType),
		EnvPrefixes: map[string][]string{
			"PATH": {"cipd_bin_packages", "cipd_bin_packages/bin", "go/go/bin"},
		},
		Isolate:        "empty.isolate",
		ServiceAccount: SERVICE_ACCOUNT_RECREATE_SKPS,
	}
	b.MustAddTask(name, t)
	return name
}

// process generates Tasks and Jobs for the given Job name.
func process(b *specs.TasksCfgBuilder, name string) {
	var priority float64 // Leave as default for most jobs.
	deps := []string{}

	if strings.Contains(name, "Experimental") {
		// Experimental recipe-less tasks.
		deps = append(deps, experimental(b, name))
	} else if strings.Contains(name, "UpdateGoDeps") {
		// Update Go deps bot.
		deps = append(deps, updateGoDeps(b, name))
	} else {
		// Infra tests.
		if strings.Contains(name, "Infra-PerCommit") {
			deps = append(deps, infra(b, name))
		}
		// Presubmit.
		if strings.Contains(name, "Presubmit") {
			priority = 1
			deps = append(deps, presubmit(b, name))
		}
	}

	// Add the Job spec.
	trigger := specs.TRIGGER_ANY_BRANCH
	if strings.Contains(name, "OnDemand") {
		trigger = specs.TRIGGER_ON_DEMAND
	} else if strings.Contains(name, "Nightly") {
		trigger = specs.TRIGGER_NIGHTLY
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
	buildTaskDrivers(b)
	for _, name := range JOBS {
		process(b, name)
	}

	b.MustFinish()
}
