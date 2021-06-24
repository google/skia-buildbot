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

	"go.skia.org/infra/go/cas/rbe"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/specs"
)

const (
	BUILD_TASK_DRIVERS_NAME = "Housekeeper-PerCommit-BuildTaskDrivers"
	BUNDLE_RECIPES_NAME     = "Housekeeper-PerCommit-BundleRecipes"

	CAS_AUTOROLL_CONFIGS = "autoroll-configs"
	CAS_EMPTY            = "empty" // TODO(borenet): It'd be nice if this wasn't necessary.
	CAS_RUN_RECIPE       = "run-recipe"
	CAS_RECIPES          = "recipes"
	CAS_WHOLE_REPO       = "whole-repo"

	DEFAULT_OS       = DEFAULT_OS_LINUX
	DEFAULT_OS_LINUX = "Debian-10.3"
	DEFAULT_OS_WIN   = "Windows-Server-17763"

	LOGDOG_ANNOTATION_URL = "logdog://logs.chromium.org/skia/${SWARMING_TASK_ID}/+/annotations"

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

	SERVICE_ACCOUNT_CIPD_UPLOADER = "cipd-uploader@skia-swarming-bots.iam.gserviceaccount.com"
	SERVICE_ACCOUNT_COMPILE       = "skia-external-compile-tasks@skia-swarming-bots.iam.gserviceaccount.com"
	SERVICE_ACCOUNT_RECREATE_SKPS = "skia-recreate-skps@skia-swarming-bots.iam.gserviceaccount.com"
)

var (
	// "Constants"

	// Top-level list of all Jobs to run at each commit.
	JOBS = []string{
		"Housekeeper-Nightly-UpdateGoDeps",
		"Housekeeper-Weekly-UpdateCIPDPackages",
		"Housekeeper-OnDemand-Presubmit",
		"Housekeeper-PerCommit-CIPD-SK",
		"Infra-PerCommit-Build",
		"Infra-PerCommit-Small",
		"Infra-PerCommit-Medium",
		"Infra-PerCommit-Large",
		"Infra-PerCommit-Race",
		"Infra-PerCommit-CreateDockerImage",
		"Infra-PerCommit-Puppeteer",
		"Infra-PerCommit-PushAppsFromInfraDockerImage",
		"Infra-PerCommit-ValidateAutorollConfigs",
		"Infra-PerCommit-Build-Bazel-Local",
		"Infra-PerCommit-Build-Bazel-RBE",
		"Infra-PerCommit-Test-Bazel-Local",
		"Infra-PerCommit-Test-Bazel-RBE",
		"Infra-Experimental-Small-Linux",
		"Infra-Experimental-Small-Win",
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

	CACHES_DOCKER = []*specs.Cache{
		{
			Name: "docker",
			Path: "cache/docker",
		},
	}

	// These properties are required by some tasks, eg. for running
	// bot_update, but they prevent de-duplication, so they should only be
	// used where necessary.
	EXTRA_PROPS = map[string]string{
		"buildbucket_build_id": specs.PLACEHOLDER_BUILDBUCKET_BUILD_ID,
		"patch_issue":          specs.PLACEHOLDER_ISSUE_INT,
		"patch_ref":            specs.PLACEHOLDER_PATCH_REF,
		"patch_repo":           specs.PLACEHOLDER_PATCH_REPO,
		"patch_set":            specs.PLACEHOLDER_PATCHSET_INT,
		"patch_storage":        specs.PLACEHOLDER_PATCH_STORAGE,
		"repository":           specs.PLACEHOLDER_REPO,
		"revision":             specs.PLACEHOLDER_REVISION,
		"task_id":              specs.PLACEHOLDER_TASK_ID,
	}

	CIPD_PLATFORMS = []string{
		"--platform", "@io_bazel_rules_go//go/toolchain:darwin_amd64=mac-amd64",
		"--platform", "@io_bazel_rules_go//go/toolchain:linux_amd64=linux-amd64",
		"--platform", "@io_bazel_rules_go//go/toolchain:windows_amd64=windows-amd64",
	}
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
		"docker_installed:true",
	}
}

// Dimensions for Windows GCE instances.
func winGceDimensions(machineType string) []string {
	return []string{
		"pool:Skia",
		fmt.Sprintf("os:%s", DEFAULT_OS_WIN),
		"gpu:none",
		"cpu:x86-64-Haswell_GCE",
		fmt.Sprintf("machine_type:%s", machineType),
	}
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
		CasSpec:      CAS_RECIPES,
		CipdPackages: append(specs.CIPD_PKGS_GIT_LINUX_AMD64, specs.CIPD_PKGS_PYTHON_LINUX_AMD64...),
		Command: []string{
			"/bin/bash", "buildbot/infra/bots/bundle_recipes.sh", specs.PLACEHOLDER_ISOLATED_OUTDIR,
		},
		Dimensions: linuxGceDimensions(MACHINE_TYPE_SMALL),
		EnvPrefixes: map[string][]string{
			"PATH": {
				"cipd_bin_packages",
				"cipd_bin_packages/bin",
				"cipd_bin_packages/cpython",
				"cipd_bin_packages/cpython/bin",
				"cipd_bin_packages/cpython3",
				"cipd_bin_packages/cpython3/bin",
			},
		},
		Idempotent: true,
	})
	return BUNDLE_RECIPES_NAME
}

// buildTaskDrivers generates the task to compile the task driver code to run on
// a given platform.
func buildTaskDrivers(b *specs.TasksCfgBuilder, os, arch string) string {
	// TODO(borenet): Add support for RPI.
	goos := map[string]string{
		"Linux": "linux",
		"Mac":   "darwin",
		"Win":   "windows",
	}[os]
	goarch := map[string]string{
		"x86":    "386",
		"x86_64": "amd64",
	}[arch]
	name := fmt.Sprintf("%s-%s-%s", BUILD_TASK_DRIVERS_NAME, os, arch)
	b.MustAddTask(name, &specs.TaskSpec{
		Caches:       CACHES_GO,
		CasSpec:      CAS_WHOLE_REPO,
		CipdPackages: append(specs.CIPD_PKGS_GIT_LINUX_AMD64, b.MustGetCipdPackageFromAsset("go")),
		Command: []string{
			"/bin/bash", "buildbot/infra/bots/build_task_drivers.sh", specs.PLACEHOLDER_ISOLATED_OUTDIR,
		},
		Dimensions: linuxGceDimensions(MACHINE_TYPE_SMALL),
		Environment: map[string]string{
			"GOOS":   goos,
			"GOARCH": goarch,
		},
		EnvPrefixes: map[string][]string{
			"PATH": {"cipd_bin_packages", "cipd_bin_packages/bin", "go/go/bin"},
		},
		// This task is idempotent but unlikely to ever be deduped
		// because it depends on the entire repo...
		Idempotent: true,
	})
	return name

}

// kitchenTask returns a specs.TaskSpec instance which uses Kitchen to run a
// recipe.
func kitchenTask(name, recipe, casSpec, serviceAccount string, dimensions []string, extraProps map[string]string, outputDir string) *specs.TaskSpec {
	// TODO(borenet): Currently all callers are for Linux tasks, but that may
	// not always be the case.
	cipd := append([]*specs.CipdPackage{}, specs.CIPD_PKGS_KITCHEN_LINUX_AMD64...)
	properties := map[string]string{
		"buildername":   name,
		"swarm_out_dir": specs.PLACEHOLDER_ISOLATED_OUTDIR,
	}
	for k, v := range extraProps {
		properties[k] = v
	}
	var outputs []string = nil
	if outputDir != OUTPUT_NONE {
		outputs = []string{outputDir}
	}
	python := "cipd_bin_packages/vpython3${EXECUTABLE_SUFFIX}"
	return &specs.TaskSpec{
		Caches: []*specs.Cache{
			{
				Name: "vpython",
				Path: "cache/vpython",
			},
		},
		CasSpec:      casSpec,
		CipdPackages: cipd,
		Command:      []string{python, "-u", "buildbot/infra/bots/run_recipe.py", "${ISOLATED_OUTDIR}", recipe, props(properties), "skia"},
		Dependencies: []string{BUNDLE_RECIPES_NAME},
		Dimensions:   dimensions,
		EnvPrefixes: map[string][]string{
			"PATH": {
				"cipd_bin_packages",
				"cipd_bin_packages/bin",
				"cipd_bin_packages/cpython",
				"cipd_bin_packages/cpython/bin",
				"cipd_bin_packages/cpython3",
				"cipd_bin_packages/cpython3/bin",
			},
			"VPYTHON_VIRTUALENV_ROOT": {"cache/vpython"},
		},
		ExtraTags: map[string]string{
			"log_location": LOGDOG_ANNOTATION_URL,
		},
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

	var task *specs.TaskSpec
	if strings.Contains(name, "Puppeteer") {
		// Puppeteer tests run inside a Docker container, take screenshots and
		// upload them to Gold. Therefore we need Docker, goldctl and EXTRA_PROPS,
		// which include the properties required by goldctl (issue, patchset, etc).
		task = kitchenTask(name, "puppeteer_tests", CAS_WHOLE_REPO, SERVICE_ACCOUNT_COMPILE, linuxGceDimensions(machineType), EXTRA_PROPS, OUTPUT_NONE)
		task.CipdPackages = append(task.CipdPackages, specs.CIPD_PKGS_GOLDCTL...)
		task.IoTimeout = 60 * time.Minute
		task.ExecutionTimeout = 60 * time.Minute
	} else {
		task = kitchenTask(name, "swarm_infra", CAS_WHOLE_REPO, SERVICE_ACCOUNT_COMPILE, linuxGceDimensions(machineType), nil, OUTPUT_NONE)
	}

	task.CipdPackages = append(task.CipdPackages, specs.CIPD_PKGS_GIT_LINUX_AMD64...)
	task.CipdPackages = append(task.CipdPackages, b.MustGetCipdPackageFromAsset("go"))
	task.Caches = append(task.Caches, CACHES_GO...)
	task.CipdPackages = append(task.CipdPackages, b.MustGetCipdPackageFromAsset("node"))
	task.CipdPackages = append(task.CipdPackages, specs.CIPD_PKGS_GSUTIL...)
	if strings.Contains(name, "Large") || strings.Contains(name, "Build") {
		task.CipdPackages = append(task.CipdPackages, b.MustGetCipdPackageFromAsset("protoc"))
		task.CipdPackages = append(task.CipdPackages, b.MustGetCipdPackageFromAsset("mockery"))
		task.EnvPrefixes["PATH"] = append(task.EnvPrefixes["PATH"], "mockery")
	}

	// Cloud datastore tests are assumed to be marked as 'Large'
	if strings.Contains(name, "Large") || strings.Contains(name, "Race") {
		task.CipdPackages = append(task.CipdPackages, specs.CIPD_PKGS_ISOLATE...)
		task.CipdPackages = append(task.CipdPackages, b.MustGetCipdPackageFromAsset("gcloud_linux"))
		task.CipdPackages = append(task.CipdPackages, b.MustGetCipdPackageFromAsset("cockroachdb"))
		task.CipdPackages = append(task.CipdPackages, cipd.MustGetPackage("infra/tools/luci/lucicfg/${platform}"))
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
	for k, v := range EXTRA_PROPS {
		extraProps[k] = v
	}
	task := kitchenTask(name, "run_presubmit", CAS_RUN_RECIPE, SERVICE_ACCOUNT_COMPILE, linuxGceDimensions(MACHINE_TYPE_MEDIUM), extraProps, OUTPUT_NONE)
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
	task.CipdPackages = append(task.CipdPackages, specs.CIPD_PKGS_GIT_LINUX_AMD64...)
	task.CipdPackages = append(task.CipdPackages, &specs.CipdPackage{
		Name:    "infra/recipe_bundles/chromium.googlesource.com/chromium/tools/build",
		Path:    "recipe_bundle",
		Version: "git_revision:57b025298deb13e7af1ce0bc07bab76716e076ff",
	})
	task.Dependencies = []string{} // No bundled recipes for this one.

	// Bazel and Go are needed for the Gazelle, Buildifier and gofmt presubmit checks.
	task.CipdPackages = append(task.CipdPackages, b.MustGetCipdPackageFromAsset("bazel"))
	task.CipdPackages = append(task.CipdPackages, b.MustGetCipdPackageFromAsset("go"))
	task.EnvPrefixes["PATH"] = append(task.EnvPrefixes["PATH"], "bazel/bin", "go/go/bin")

	b.MustAddTask(name, task)
	return name
}

func experimental(b *specs.TasksCfgBuilder, name string) string {
	cipd := []*specs.CipdPackage{}
	if strings.Contains(name, "Win") {
		cipd = append(cipd, specs.CIPD_PKGS_GIT_WINDOWS_AMD64...)
		cipd = append(cipd, specs.CIPD_PKGS_PYTHON_WINDOWS_AMD64...)
	} else {
		cipd = append(cipd, specs.CIPD_PKGS_GIT_LINUX_AMD64...)
		cipd = append(cipd, specs.CIPD_PKGS_PYTHON_LINUX_AMD64...)
	}
	cipd = append(cipd, specs.CIPD_PKGS_GSUTIL...)
	cipd = append(cipd, b.MustGetCipdPackageFromAsset("node"))

	machineType := MACHINE_TYPE_MEDIUM
	var deps []string
	var dims []string
	if strings.Contains(name, "Win") {
		goPkg := b.MustGetCipdPackageFromAsset("go_win")
		goPkg.Path = "go"
		cipd = append(cipd, goPkg)
		deps = append(deps, buildTaskDrivers(b, "Win", "x86_64"))
		dims = winGceDimensions(machineType)
	} else if strings.Contains(name, "Linux") {
		cipd = append(cipd, b.MustGetCipdPackageFromAsset("go"))
		deps = append(deps, buildTaskDrivers(b, "Linux", "x86_64"))
		dims = linuxGceDimensions(machineType)
	}
	t := &specs.TaskSpec{
		Caches:       CACHES_GO,
		CasSpec:      CAS_WHOLE_REPO,
		CipdPackages: cipd,
		Command: []string{
			"./infra_tests",
			"--project_id", "skia-swarming-bots",
			"--task_id", specs.PLACEHOLDER_TASK_ID,
			"--task_name", name,
			"--workdir", ".",
			"--alsologtostderr",
		},
		Dependencies: deps,
		Dimensions:   dims,
		EnvPrefixes: map[string][]string{
			"PATH": {
				"cipd_bin_packages",
				"cipd_bin_packages/bin",
				"cipd_bin_packages/cpython",
				"cipd_bin_packages/cpython/bin",
				"cipd_bin_packages/cpython3",
				"cipd_bin_packages/cpython3/bin",
				"go/go/bin",
			},
		},
		ServiceAccount: SERVICE_ACCOUNT_COMPILE,
	}
	b.MustAddTask(name, t)
	return name
}

func updateGoDeps(b *specs.TasksCfgBuilder, name string) string {
	cipd := append([]*specs.CipdPackage{}, specs.CIPD_PKGS_GIT_LINUX_AMD64...)
	cipd = append(cipd, b.MustGetCipdPackageFromAsset("go"))
	cipd = append(cipd, b.MustGetCipdPackageFromAsset("protoc"))

	machineType := MACHINE_TYPE_MEDIUM
	t := &specs.TaskSpec{
		Caches:       CACHES_GO,
		CasSpec:      CAS_EMPTY,
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
			"--revision", specs.PLACEHOLDER_REVISION,
			"--patch_issue", specs.PLACEHOLDER_ISSUE,
			"--patch_set", specs.PLACEHOLDER_PATCHSET,
			"--patch_server", specs.PLACEHOLDER_CODEREVIEW_SERVER,
			"--alsologtostderr",
		},
		Dependencies: []string{buildTaskDrivers(b, "Linux", "x86_64")},
		Dimensions:   linuxGceDimensions(machineType),
		EnvPrefixes: map[string][]string{
			"PATH": {"cipd_bin_packages", "cipd_bin_packages/bin", "go/go/bin"},
		},
		ServiceAccount: SERVICE_ACCOUNT_RECREATE_SKPS,
	}
	b.MustAddTask(name, t)
	return name
}

func createDockerImage(b *specs.TasksCfgBuilder, name string) string {
	cipd := append([]*specs.CipdPackage{}, specs.CIPD_PKGS_GIT_LINUX_AMD64...)
	cipd = append(cipd, b.MustGetCipdPackageFromAsset("go"))
	cipd = append(cipd, b.MustGetCipdPackageFromAsset("protoc"))

	machineType := MACHINE_TYPE_MEDIUM
	t := &specs.TaskSpec{
		Caches:       append(CACHES_GO, CACHES_DOCKER...),
		CasSpec:      CAS_EMPTY,
		CipdPackages: cipd,
		Command: []string{
			"./build_push_docker_image",
			"--image_name", "gcr.io/skia-public/infra",
			"--dockerfile_dir", "docker",
			"--project_id", "skia-swarming-bots",
			"--task_id", specs.PLACEHOLDER_TASK_ID,
			"--task_name", name,
			"--workdir", ".",
			"--gerrit_project", "buildbot",
			"--gerrit_url", "https://skia-review.googlesource.com",
			"--repo", specs.PLACEHOLDER_REPO,
			"--revision", specs.PLACEHOLDER_REVISION,
			"--patch_issue", specs.PLACEHOLDER_ISSUE,
			"--patch_set", specs.PLACEHOLDER_PATCHSET,
			"--patch_server", specs.PLACEHOLDER_CODEREVIEW_SERVER,
			"--swarm_out_dir", specs.PLACEHOLDER_ISOLATED_OUTDIR,
			"--alsologtostderr",
		},
		Dependencies: []string{buildTaskDrivers(b, "Linux", "x86_64")},
		Dimensions:   linuxGceDimensions(machineType),
		EnvPrefixes: map[string][]string{
			"PATH": {"cipd_bin_packages", "cipd_bin_packages/bin", "go/go/bin"},
		},
		ServiceAccount: SERVICE_ACCOUNT_COMPILE,
	}
	b.MustAddTask(name, t)
	return name
}

func createPushAppsFromInfraDockerImage(b *specs.TasksCfgBuilder, name string) string {
	cipd := append([]*specs.CipdPackage{}, specs.CIPD_PKGS_GIT_LINUX_AMD64...)
	cipd = append(cipd, b.MustGetCipdPackageFromAsset("go"))
	cipd = append(cipd, b.MustGetCipdPackageFromAsset("protoc"))

	machineType := MACHINE_TYPE_MEDIUM
	t := &specs.TaskSpec{
		Caches:       append(CACHES_GO, CACHES_DOCKER...),
		CasSpec:      CAS_EMPTY,
		CipdPackages: cipd,
		Command: []string{
			"./push_apps_from_infra_image",
			"--project_id", "skia-swarming-bots",
			"--task_id", specs.PLACEHOLDER_TASK_ID,
			"--task_name", name,
			"--workdir", ".",
			"--gerrit_project", "buildbot",
			"--gerrit_url", "https://skia-review.googlesource.com",
			"--repo", specs.PLACEHOLDER_REPO,
			"--revision", specs.PLACEHOLDER_REVISION,
			"--patch_issue", specs.PLACEHOLDER_ISSUE,
			"--patch_set", specs.PLACEHOLDER_PATCHSET,
			"--patch_server", specs.PLACEHOLDER_CODEREVIEW_SERVER,
			"--alsologtostderr",
		},
		Dependencies: []string{buildTaskDrivers(b, "Linux", "x86_64"), createDockerImage(b, "Infra-PerCommit-CreateDockerImage")},
		Dimensions:   linuxGceDimensions(machineType),
		EnvPrefixes: map[string][]string{
			"PATH": {"cipd_bin_packages", "cipd_bin_packages/bin", "go/go/bin"},
		},
		ServiceAccount: SERVICE_ACCOUNT_COMPILE,
	}
	b.MustAddTask(name, t)
	return name
}

func updateCIPDPackages(b *specs.TasksCfgBuilder, name string) string {
	cipd := append([]*specs.CipdPackage{}, specs.CIPD_PKGS_GIT_LINUX_AMD64...)
	cipd = append(cipd, b.MustGetCipdPackageFromAsset("go"))
	cipd = append(cipd, b.MustGetCipdPackageFromAsset("mockery"))
	cipd = append(cipd, b.MustGetCipdPackageFromAsset("protoc"))

	machineType := MACHINE_TYPE_MEDIUM
	t := &specs.TaskSpec{
		Caches:       CACHES_GO,
		CasSpec:      CAS_EMPTY,
		CipdPackages: cipd,
		Command: []string{
			"./roll_cipd_packages",
			"--project_id", "skia-swarming-bots",
			"--task_id", specs.PLACEHOLDER_TASK_ID,
			"--task_name", name,
			"--workdir", ".",
			"--gerrit_project", "buildbot",
			"--gerrit_url", "https://skia-review.googlesource.com",
			"--repo", specs.PLACEHOLDER_REPO,
			"--revision", specs.PLACEHOLDER_REVISION,
			"--patch_issue", specs.PLACEHOLDER_ISSUE,
			"--patch_set", specs.PLACEHOLDER_PATCHSET,
			"--patch_server", specs.PLACEHOLDER_CODEREVIEW_SERVER,
			"--alsologtostderr",
		},
		Dependencies: []string{buildTaskDrivers(b, "Linux", "x86_64")},
		Dimensions:   linuxGceDimensions(machineType),
		EnvPrefixes: map[string][]string{
			"PATH": {"cipd_bin_packages", "cipd_bin_packages/bin", "go/go/bin", "mockery"},
		},
		ServiceAccount: SERVICE_ACCOUNT_RECREATE_SKPS,
	}
	b.MustAddTask(name, t)
	return name
}

func validateAutorollConfigs(b *specs.TasksCfgBuilder, name string) string {
	t := &specs.TaskSpec{
		CasSpec: CAS_AUTOROLL_CONFIGS,
		Command: []string{
			"./validate_autoroll_configs",
			"--project_id", "skia-swarming-bots",
			"--task_id", specs.PLACEHOLDER_TASK_ID,
			"--task_name", name,
			"--workdir", ".",
			"--config", "./autoroll/config",
			"--alsologtostderr",
		},
		Dependencies:   []string{buildTaskDrivers(b, "Linux", "x86_64")},
		Dimensions:     linuxGceDimensions(MACHINE_TYPE_SMALL),
		ServiceAccount: SERVICE_ACCOUNT_COMPILE,
	}
	b.MustAddTask(name, t)
	return name
}

func bazelBuild(b *specs.TasksCfgBuilder, name string, rbe bool) string {
	cipd := append([]*specs.CipdPackage{}, specs.CIPD_PKGS_GIT_LINUX_AMD64...)
	cipd = append(cipd, b.MustGetCipdPackageFromAsset("bazel"))
	cipd = append(cipd, b.MustGetCipdPackageFromAsset("go"))
	cipd = append(cipd, b.MustGetCipdPackageFromAsset("mockery"))
	cipd = append(cipd, b.MustGetCipdPackageFromAsset("protoc"))

	cmd := []string{
		"./bazel_build_all",
		"--project_id", "skia-swarming-bots",
		"--task_id", specs.PLACEHOLDER_TASK_ID,
		"--task_name", name,
		"--workdir", ".",
		"--repo", specs.PLACEHOLDER_REPO,
		"--revision", specs.PLACEHOLDER_REVISION,
		"--patch_issue", specs.PLACEHOLDER_ISSUE,
		"--patch_set", specs.PLACEHOLDER_PATCHSET,
		"--patch_server", specs.PLACEHOLDER_CODEREVIEW_SERVER,
		"--alsologtostderr",
	}
	if rbe {
		cipd = append(cipd, specs.CIPD_PKGS_SKIA_INFRA_RBE_KEY...)
		cmd = append(cmd, "--rbe", "--rbe_key", "./skia_infra_rbe_key/rbe-ci.json")
	}

	t := &specs.TaskSpec{
		Caches:       CACHES_GO,
		CasSpec:      CAS_EMPTY,
		CipdPackages: cipd,
		Command:      cmd,
		Dependencies: []string{buildTaskDrivers(b, "Linux", "x86_64")},
		Dimensions:   linuxGceDimensions(MACHINE_TYPE_LARGE),
		EnvPrefixes: map[string][]string{
			"PATH": {"cipd_bin_packages", "cipd_bin_packages/bin", "go/go/bin", "mockery", "bazel/bin"},
		},
		ServiceAccount: SERVICE_ACCOUNT_COMPILE,
	}
	b.MustAddTask(name, t)
	return name
}

func bazelTest(b *specs.TasksCfgBuilder, name string, rbe bool) string {
	cipd := append([]*specs.CipdPackage{}, specs.CIPD_PKGS_GIT_LINUX_AMD64...)
	cipd = append(cipd, specs.CIPD_PKGS_PYTHON_LINUX_AMD64...)
	cipd = append(cipd, specs.CIPD_PKGS_GSUTIL...)
	cipd = append(cipd, specs.CIPD_PKGS_ISOLATE...)
	cipd = append(cipd, b.MustGetCipdPackageFromAsset("bazel"))
	cipd = append(cipd, b.MustGetCipdPackageFromAsset("go"))
	cipd = append(cipd, b.MustGetCipdPackageFromAsset("cockroachdb"))
	cipd = append(cipd, b.MustGetCipdPackageFromAsset("gcloud_linux"))

	cmd := []string{
		"./bazel_test_all",
		"--project_id", "skia-swarming-bots",
		"--task_id", specs.PLACEHOLDER_TASK_ID,
		"--task_name", name,
		"--workdir", ".",
		"--repo", specs.PLACEHOLDER_REPO,
		"--revision", specs.PLACEHOLDER_REVISION,
		"--patch_issue", specs.PLACEHOLDER_ISSUE,
		"--patch_set", specs.PLACEHOLDER_PATCHSET,
		"--patch_server", specs.PLACEHOLDER_CODEREVIEW_SERVER,
		"--buildbucket_build_id", specs.PLACEHOLDER_BUILDBUCKET_BUILD_ID,
		"--alsologtostderr",
	}
	if rbe {
		cipd = append(cipd, specs.CIPD_PKGS_SKIA_INFRA_RBE_KEY...)
		cmd = append(cmd, "--rbe", "--rbe_key", "./skia_infra_rbe_key/rbe-ci.json")
	}

	t := &specs.TaskSpec{
		Caches:       CACHES_GO,
		CasSpec:      CAS_EMPTY,
		CipdPackages: cipd,
		Command:      cmd,
		Dependencies: []string{buildTaskDrivers(b, "Linux", "x86_64")},
		Dimensions:   linuxGceDimensions(MACHINE_TYPE_LARGE),
		EnvPrefixes: map[string][]string{
			"PATH": {
				"cipd_bin_packages",
				"cipd_bin_packages/bin",
				"cipd_bin_packages/cpython",
				"cipd_bin_packages/cpython/bin",
				"cipd_bin_packages/cpython3",
				"cipd_bin_packages/cpython3/bin",
				"go/go/bin",
				"bazel/bin",
				"cockroachdb",
				"gcloud_linux/bin",
			},
		},
		ServiceAccount: SERVICE_ACCOUNT_COMPILE,
	}
	b.MustAddTask(name, t)
	return name
}

func buildAndDeployCIPD(b *specs.TasksCfgBuilder, name, packageName string, targets, includePaths []string) string {
	cipd := []*specs.CipdPackage{
		b.MustGetCipdPackageFromAsset("bazel"),
	}
	cipd = append(cipd, specs.CIPD_PKGS_SKIA_INFRA_RBE_KEY...)
	cmd := []string{
		"./build_and_deploy_cipd",
		"--project_id", "skia-swarming-bots",
		"--task_id", specs.PLACEHOLDER_TASK_ID,
		"--task_name", name,
		"--alsologtostderr",
		"--build_dir", "buildbot",
		"--package_name", packageName,
		"--tag", "git_repo:" + specs.PLACEHOLDER_REPO,
		"--tag", "git_revision:" + specs.PLACEHOLDER_REVISION,
		"--ref", "latest",
		"--rbe", "--rbe_key", "./skia_infra_rbe_key/rbe-ci.json",
	}
	for _, target := range targets {
		cmd = append(cmd, "--target", target)
	}
	for _, includePath := range includePaths {
		cmd = append(cmd, "--include_path", includePath)
	}
	cmd = append(cmd, CIPD_PLATFORMS...)
	t := &specs.TaskSpec{
		CasSpec:      CAS_WHOLE_REPO,
		CipdPackages: cipd,
		Command:      cmd,
		Dependencies: []string{
			buildTaskDrivers(b, "Linux", "x86_64"),
			// TODO(borenet): Replace these with Infra-PerCommit-Test-Bazel-RBE
			// once that becomes the source of truth.
			"Infra-PerCommit-Small",
			"Infra-PerCommit-Medium",
			"Infra-PerCommit-Large",
		},
		Dimensions: linuxGceDimensions(MACHINE_TYPE_LARGE),
		EnvPrefixes: map[string][]string{
			"PATH": {
				"cipd_bin_packages",
				"cipd_bin_packages/bin",
				"bazel/bin",
			},
		},
		ServiceAccount: SERVICE_ACCOUNT_CIPD_UPLOADER,
	}
	b.MustAddTask(name, t)
	return name
}

func buildAndDeploySK(b *specs.TasksCfgBuilder, name string) string {
	return buildAndDeployCIPD(b, name, "skia/tools/sk", []string{"//sk/go/sk:sk"}, []string{"_bazel_bin/sk/go/sk/sk_/sk[.exe]"})
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
	} else if strings.Contains(name, "CreateDockerImage") {
		// Create docker image bot.
		deps = append(deps, createDockerImage(b, name))
	} else if strings.Contains(name, "PushAppsFromInfraDockerImage") {
		deps = append(deps, createPushAppsFromInfraDockerImage(b, name))
	} else if strings.Contains(name, "UpdateCIPDPackages") {
		// Update CIPD packages bot.
		deps = append(deps, updateCIPDPackages(b, name))
	} else if strings.Contains(name, "ValidateAutorollConfigs") {
		deps = append(deps, validateAutorollConfigs(b, name))
	} else if strings.Contains(name, "Build-Bazel-Local") {
		deps = append(deps, bazelBuild(b, name, false /* =rbe */))
	} else if strings.Contains(name, "Build-Bazel-RBE") {
		deps = append(deps, bazelBuild(b, name, true /* =rbe */))
	} else if strings.Contains(name, "Test-Bazel-Local") {
		deps = append(deps, bazelTest(b, name, false /* =rbe */))
	} else if strings.Contains(name, "Test-Bazel-RBE") {
		deps = append(deps, bazelTest(b, name, true /* =rbe */))
	} else if name == "Housekeeper-PerCommit-CIPD-SK" {
		deps = append(deps, buildAndDeploySK(b, name))
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
	} else if strings.Contains(name, "Weekly") {
		trigger = specs.TRIGGER_WEEKLY
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

	// CasSpecs.
	b.MustAddCasSpec(CAS_AUTOROLL_CONFIGS, &specs.CasSpec{
		Root:  ".",
		Paths: []string{"autoroll/config"},
	})
	b.MustAddCasSpec(CAS_EMPTY, specs.EmptyCasSpec)
	b.MustAddCasSpec(CAS_RECIPES, &specs.CasSpec{
		Root: "..",
		Paths: []string{
			"buildbot/infra/config/recipes.cfg",
			"buildbot/infra/bots/bundle_recipes.sh",
			"buildbot/infra/bots/recipes",
			"buildbot/infra/bots/recipes.py",
		},
	})
	b.MustAddCasSpec(CAS_RUN_RECIPE, &specs.CasSpec{
		Root:  "..",
		Paths: []string{"buildbot/infra/bots/run_recipe.py"},
	})
	b.MustAddCasSpec(CAS_WHOLE_REPO, &specs.CasSpec{
		Root:     "..",
		Paths:    []string{"buildbot"},
		Excludes: []string{rbe.ExcludeGitDir},
	})

	b.MustFinish()
}
