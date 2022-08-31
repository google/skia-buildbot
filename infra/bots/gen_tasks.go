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

	"go.skia.org/infra/go/cas/rbe"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/specs"
)

const (
	buildTaskDriversName = "Housekeeper-PerCommit-BuildTaskDrivers"
	buildRecipesName     = "Housekeeper-PerCommit-BundleRecipes"

	casAutorollConfigs = "autoroll-configs"
	casEmpty           = "empty" // TODO(borenet): It'd be nice if this wasn't necessary.
	casRunRecipe       = "run-recipe"
	casRecipes         = "recipes"
	casWholeRepo       = "whole-repo"

	defaultOSLinux   = "Debian-10.3"
	defaultOSWindows = "Windows-Server-17763"

	logdogAnnotationURL = "logdog://logs.chromium.org/skia/${SWARMING_TASK_ID}/+/annotations"

	// machineTypeSmall refers to a 2-core machine.
	machineTypeSmall = "n1-highmem-2"
	// machineTypeMedium refers to a 16-core machine
	machineTypeMedium = "n1-standard-16"
	// machineTypeLarge refers to a 64-core machine.
	machineTypeLarge = "n1-highcpu-64"

	// ignoreSwarmingOutput indicates that the outputs will not be isolated/uploaded by swarming
	ignoreSwarmingOutput = "output_ignored"

	cipdUploaderServiceAccount = "cipd-uploader@skia-swarming-bots.iam.gserviceaccount.com"
	compileServiceAccount      = "skia-external-compile-tasks@skia-swarming-bots.iam.gserviceaccount.com"
	recreateSKPsServiceAccount = "skia-recreate-skps@skia-swarming-bots.iam.gserviceaccount.com"
)

var (
	// jobsToCQStatus lists all infra Jobs and their CQ config to run at each commit.
	jobsToCQStatus = map[string]*specs.CommitQueueJobConfig{
		"Housekeeper-OnDemand-Presubmit":  &cqWithDefaults,
		"Infra-PerCommit-Build-Bazel-RBE": &cqWithDefaults,
		"Infra-PerCommit-Large":           &cqWithDefaults,
		"Infra-PerCommit-Medium":          &cqWithDefaults,
		"Infra-PerCommit-Small":           &cqWithDefaults,
		"Infra-PerCommit-Test-Bazel-RBE":  &cqWithDefaults,

		"Housekeeper-PerCommit-CIPD-Canary":                  noCQ,
		"Housekeeper-PerCommit-CIPD-SK":                      noCQ,
		"Housekeeper-PerCommit-CIPD-ValidateAutorollConfigs": noCQ,
		"Housekeeper-Weekly-UpdateCIPDPackages":              noCQ,
		"Infra-Experimental-Small-Linux":                     noCQ,
		"Infra-Experimental-Small-Win":                       noCQ,
		"Infra-PerCommit-Build-Bazel-Local":                  noCQ,
		"Infra-PerCommit-Race":                               noCQ,
		"Infra-PerCommit-Test-Bazel-Local":                   noCQ,
	}

	// cqWithDefaults means this is a non-experimental CQ job (if it fails, the submission will
	// be blocked) and it will run all the time (and not just when some files are modified).
	cqWithDefaults = specs.CommitQueueJobConfig{}
	// noCQ means this job will not appear on the CQ
	noCQ *specs.CommitQueueJobConfig = nil

	goCaches = []*specs.Cache{
		{
			Name: "go_cache",
			Path: "cache/go_cache",
		},
		{
			Name: "gopath",
			Path: "cache/gopath",
		},
	}

	dockerCaches = []*specs.Cache{
		{
			Name: "docker",
			Path: "cache/docker",
		},
	}

	vpythonCaches = []*specs.Cache{
		{
			Name: "vpython",
			Path: "cache/vpython",
		},
	}

	// These properties are required by some tasks, eg. for running
	// bot_update, but they prevent de-duplication, so they should only be
	// used where necessary.
	extraProperties = map[string]string{
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

	cipdPlatforms = []string{
		"--platform", "@io_bazel_rules_go//go/toolchain:darwin_amd64=mac-amd64",
		"--platform", "@io_bazel_rules_go//go/toolchain:darwin_arm64=mac-arm64",
		"--platform", "@io_bazel_rules_go//go/toolchain:linux_amd64=linux-amd64",
		"--platform", "@io_bazel_rules_go//go/toolchain:linux_arm64=linux-arm64",
		"--platform", "@io_bazel_rules_go//go/toolchain:windows_amd64=windows-amd64",
	}
)

// Dimensions for Linux GCE instances.
func linuxGceDimensions(machineType string) []string {
	return []string{
		"pool:Skia",
		fmt.Sprintf("os:%s", defaultOSLinux),
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
		fmt.Sprintf("os:%s", defaultOSWindows),
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
	b.MustAddTask(buildRecipesName, &specs.TaskSpec{
		CasSpec:      casRecipes,
		CipdPackages: append(specs.CIPD_PKGS_GIT_LINUX_AMD64, specs.Python3LinuxAMD64CIPDPackages()...),
		Command: []string{
			"/bin/bash", "buildbot/infra/bots/bundle_recipes.sh", specs.PLACEHOLDER_ISOLATED_OUTDIR,
		},
		Dimensions: linuxGceDimensions(machineTypeSmall),
		EnvPrefixes: map[string][]string{
			"PATH": {
				"cipd_bin_packages",
				"cipd_bin_packages/bin",
				"cipd_bin_packages/cpython3",
				"cipd_bin_packages/cpython3/bin",
			},
		},
		Idempotent: true,
	})
	return buildRecipesName
}

// buildTaskDrivers generates the task which compiles the task driver code to run on the specified
// the platform.
func buildTaskDrivers(b *specs.TasksCfgBuilder, os, arch string) string {
	// Not all of these configurations are currently used. These are a subset of all options
	// supported by Golang: https://go.dev/doc/install/source#environment
	goos := map[string]string{
		"Linux": "linux",
		"Mac":   "darwin",
		"Win":   "windows",
	}[os]
	goarch := map[string]string{
		"x86":    "386",
		"x86_64": "amd64",
	}[arch]
	name := fmt.Sprintf("%s-%s-%s", buildTaskDriversName, os, arch)
	b.MustAddTask(name, &specs.TaskSpec{
		CasSpec:      casWholeRepo,
		CipdPackages: []*specs.CipdPackage{b.MustGetCipdPackageFromAsset("bazelisk")},
		Command: []string{
			"/bin/bash", "buildbot/infra/bots/build_task_drivers.sh", specs.PLACEHOLDER_ISOLATED_OUTDIR,
			goos + "_" + goarch,
		},
		Dimensions: linuxGceDimensions(machineTypeMedium),
		EnvPrefixes: map[string][]string{
			"PATH": {"bazelisk"},
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
	pkgs := append([]*specs.CipdPackage{}, specs.CIPD_PKGS_KITCHEN_LINUX_AMD64...)
	properties := map[string]string{
		"buildername":   name,
		"swarm_out_dir": specs.PLACEHOLDER_ISOLATED_OUTDIR,
	}
	for k, v := range extraProps {
		properties[k] = v
	}
	var outputs []string = nil
	if outputDir != ignoreSwarmingOutput {
		outputs = []string{outputDir}
	}
	python := "cipd_bin_packages/vpython3${EXECUTABLE_SUFFIX}"
	return &specs.TaskSpec{
		Caches:       vpythonCaches,
		CasSpec:      casSpec,
		CipdPackages: pkgs,
		Command:      []string{python, "-u", "buildbot/infra/bots/run_recipe.py", "${ISOLATED_OUTDIR}", recipe, props(properties), "skia"},
		Dependencies: []string{buildRecipesName},
		Dimensions:   dimensions,
		Environment: map[string]string{
			"RECIPES_USE_PY3": "true",
		},
		EnvPrefixes: map[string][]string{
			"PATH": {
				"cipd_bin_packages",
				"cipd_bin_packages/bin",
				"cipd_bin_packages/cpython3",
				"cipd_bin_packages/cpython3/bin",
			},
			"VPYTHON_VIRTUALENV_ROOT": {"cache/vpython"},
			"VPYTHON_DEFAULT_SPEC":    {"buildbot/.vpython"},
		},
		ExtraTags: map[string]string{
			"log_location": logdogAnnotationURL,
		},
		Outputs:        outputs,
		ServiceAccount: serviceAccount,
	}
}

// infra generates an infra test Task. Returns the name of the last Task in the
// generated chain of Tasks, which the Job should add as a dependency.
func infra(b *specs.TasksCfgBuilder, name string) string {
	machineType := machineTypeMedium
	if strings.Contains(name, "Large") {
		// Using machineTypeLarge for Large tests saves ~2 minutes.
		machineType = machineTypeLarge
	}

	task := kitchenTask(name, "swarm_infra", casWholeRepo, compileServiceAccount, linuxGceDimensions(machineType), nil, ignoreSwarmingOutput)

	task.CipdPackages = append(task.CipdPackages, specs.CIPD_PKGS_GIT_LINUX_AMD64...)
	task.CipdPackages = append(task.CipdPackages, b.MustGetCipdPackageFromAsset("go"))
	task.Caches = append(task.Caches, goCaches...)
	task.CipdPackages = append(task.CipdPackages, b.MustGetCipdPackageFromAsset("node"))
	if strings.Contains(name, "Large") || strings.Contains(name, "Build") {
		task.CipdPackages = append(task.CipdPackages, b.MustGetCipdPackageFromAsset("protoc"))
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
		task.ExecutionTimeout = 2 * time.Hour
		task.IoTimeout = 2 * time.Hour
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
	for k, v := range extraProperties {
		extraProps[k] = v
	}
	task := kitchenTask(name, "run_presubmit", casRunRecipe, compileServiceAccount, linuxGceDimensions(machineTypeMedium), extraProps, ignoreSwarmingOutput)
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
		Version: "git_revision:1a28cb094add070f4beefd052725223930d8c27a",
	})
	task.Dependencies = []string{} // No bundled recipes for this one.

	// Bazelisk and Go are needed for the Gazelle, Buildifier and gofmt presubmit checks.
	task.CipdPackages = append(task.CipdPackages, b.MustGetCipdPackageFromAsset("bazelisk"))
	task.CipdPackages = append(task.CipdPackages, b.MustGetCipdPackageFromAsset("go"))
	task.EnvPrefixes["PATH"] = append(task.EnvPrefixes["PATH"], "bazelisk", "go/go/bin")

	// Setting the python version causes conflicts with some of the packages
	// needed by the presubmit recipe.
	delete(task.EnvPrefixes, "VPYTHON_DEFAULT_SPEC")

	b.MustAddTask(name, task)
	return name
}

func experimental(b *specs.TasksCfgBuilder, name string) string {
	var pkgs []*specs.CipdPackage
	if strings.Contains(name, "Win") {
		pkgs = append(pkgs, specs.CIPD_PKGS_GIT_WINDOWS_AMD64...)
		pkgs = append(pkgs, specs.Python3WindowsAMD64CIPDPackages()...)
	} else {
		pkgs = append(pkgs, specs.CIPD_PKGS_GIT_LINUX_AMD64...)
		pkgs = append(pkgs, specs.Python3LinuxAMD64CIPDPackages()...)
	}
	pkgs = append(pkgs, b.MustGetCipdPackageFromAsset("node"))

	machineType := machineTypeMedium
	var deps []string
	var dims []string
	if strings.Contains(name, "Win") {
		goPkg := b.MustGetCipdPackageFromAsset("go_win")
		goPkg.Path = "go"
		pkgs = append(pkgs, goPkg)
		deps = append(deps, buildTaskDrivers(b, "Win", "x86_64"))
		dims = winGceDimensions(machineType)
	} else if strings.Contains(name, "Linux") {
		pkgs = append(pkgs, b.MustGetCipdPackageFromAsset("go"))
		deps = append(deps, buildTaskDrivers(b, "Linux", "x86_64"))
		dims = linuxGceDimensions(machineType)
	}
	t := &specs.TaskSpec{
		Caches:       goCaches,
		CasSpec:      casWholeRepo,
		CipdPackages: pkgs,
		Command: []string{
			"./infra_tests",
			"--project_id", "skia-swarming-bots",
			"--task_id", specs.PLACEHOLDER_TASK_ID,
			"--task_name", name,
			"--workdir", ".",
		},
		Dependencies: deps,
		Dimensions:   dims,
		EnvPrefixes: map[string][]string{
			"PATH": {
				"cipd_bin_packages",
				"cipd_bin_packages/bin",
				"cipd_bin_packages/cpython3",
				"cipd_bin_packages/cpython3/bin",
				"go/go/bin",
			},
		},
		ServiceAccount: compileServiceAccount,
	}
	b.MustAddTask(name, t)
	return name
}

func updateCIPDPackages(b *specs.TasksCfgBuilder, name string) string {
	pkgs := append([]*specs.CipdPackage{}, specs.CIPD_PKGS_GIT_LINUX_AMD64...)
	pkgs = append(pkgs, b.MustGetCipdPackageFromAsset("go"))
	pkgs = append(pkgs, b.MustGetCipdPackageFromAsset("protoc"))

	machineType := machineTypeMedium
	t := &specs.TaskSpec{
		Caches:       goCaches,
		CasSpec:      casEmpty,
		CipdPackages: pkgs,
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
			"--skip", "cpython3",
		},
		Dependencies: []string{buildTaskDrivers(b, "Linux", "x86_64")},
		Dimensions:   linuxGceDimensions(machineType),
		EnvPrefixes: map[string][]string{
			"PATH": {"cipd_bin_packages", "cipd_bin_packages/bin", "go/go/bin"},
		},
		ServiceAccount: recreateSKPsServiceAccount,
	}
	b.MustAddTask(name, t)
	return name
}

func bazelBuild(b *specs.TasksCfgBuilder, name string, rbe bool) string {
	pkgs := append([]*specs.CipdPackage{}, specs.CIPD_PKGS_GIT_LINUX_AMD64...)
	pkgs = append(pkgs, b.MustGetCipdPackageFromAsset("bazelisk"))

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
	}
	if rbe {
		cmd = append(cmd, "--rbe")
	}

	t := &specs.TaskSpec{
		CasSpec:      casEmpty,
		CipdPackages: pkgs,
		Command:      cmd,
		Dependencies: []string{buildTaskDrivers(b, "Linux", "x86_64")},
		Dimensions:   linuxGceDimensions(machineTypeLarge),
		EnvPrefixes: map[string][]string{
			"PATH": {"cipd_bin_packages", "bazelisk"},
		},
		ServiceAccount: compileServiceAccount,
	}
	b.MustAddTask(name, t)
	return name
}

func bazelTest(b *specs.TasksCfgBuilder, name string, rbe bool) string {
	pkgs := append([]*specs.CipdPackage{}, specs.CIPD_PKGS_GIT_LINUX_AMD64...)
	pkgs = append(pkgs, specs.CIPD_PKGS_ISOLATE...)
	pkgs = append(pkgs, b.MustGetCipdPackageFromAsset("bazelisk"))

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
	}
	if rbe {
		cmd = append(cmd, "--rbe")
	}

	t := &specs.TaskSpec{
		CasSpec:      casEmpty,
		CipdPackages: pkgs,
		Command:      cmd,
		Dependencies: []string{buildTaskDrivers(b, "Linux", "x86_64")},
		Dimensions:   linuxGceDimensions(machineTypeLarge),
		EnvPrefixes: map[string][]string{
			"PATH": {
				"cipd_bin_packages",
				"bazelisk",
			},
		},
		ServiceAccount: compileServiceAccount,
	}
	b.MustAddTask(name, t)
	return name
}

func buildAndDeployCIPD(b *specs.TasksCfgBuilder, name, packageName string, targets, includePaths []string) string {
	pkgs := []*specs.CipdPackage{
		b.MustGetCipdPackageFromAsset("bazelisk"),
	}

	cmd := []string{
		"./build_and_deploy_cipd",
		"--project_id", "skia-swarming-bots",
		"--task_id", specs.PLACEHOLDER_TASK_ID,
		"--task_name", name,
		"--build_dir", "buildbot",
		"--package_name", packageName,
		"--metadata", "git_repo:" + specs.PLACEHOLDER_REPO,
		"--tag", "git_revision:" + specs.PLACEHOLDER_REVISION,
		"--ref", "latest",
		"--rbe",
	}
	for _, target := range targets {
		cmd = append(cmd, "--target", target)
	}
	for _, includePath := range includePaths {
		cmd = append(cmd, "--include_path", includePath)
	}
	cmd = append(cmd, cipdPlatforms...)
	t := &specs.TaskSpec{
		CasSpec:      casWholeRepo,
		CipdPackages: pkgs,
		Command:      cmd,
		Dependencies: []string{
			buildTaskDrivers(b, "Linux", "x86_64"),
			// TODO(borenet): Replace these with Infra-PerCommit-Test-Bazel-RBE
			// once that becomes the source of truth.
			"Infra-PerCommit-Small",
			"Infra-PerCommit-Medium",
			"Infra-PerCommit-Large",
		},
		Dimensions: linuxGceDimensions(machineTypeLarge),
		EnvPrefixes: map[string][]string{
			"PATH": {
				"cipd_bin_packages",
				"cipd_bin_packages/bin",
				"bazelisk",
			},
		},
		ServiceAccount: cipdUploaderServiceAccount,
	}
	b.MustAddTask(name, t)
	return name
}

func buildAndDeployCanary(b *specs.TasksCfgBuilder, name string) string {
	return buildAndDeployCIPD(b, name, "skia/tools/canary", []string{"//infra/bots/task_drivers/canary:canary"}, []string{"_bazel_bin/infra/bots/task_drivers/canary/canary_/canary[.exe]"})
}

func buildAndDeploySK(b *specs.TasksCfgBuilder, name string) string {
	return buildAndDeployCIPD(b, name, "skia/tools/sk", []string{"//sk/go/sk:sk"}, []string{"_bazel_bin/sk/go/sk/sk_/sk[.exe]"})
}

func buildAndDeployValidateAutorollConfigs(b *specs.TasksCfgBuilder, name string) string {
	return buildAndDeployCIPD(b, name, "skia/tools/validate_autoroll_configs", []string{"//infra/bots/task_drivers/validate_autoroll_configs:validate_autoroll_configs"}, []string{"_bazel_bin/infra/bots/task_drivers/validate_autoroll_configs/validate_autoroll_configs_/validate_autoroll_configs[.exe]"})
}

// process generates Tasks and Jobs for the given Job name.
func process(b *specs.TasksCfgBuilder, name string, cqConfig *specs.CommitQueueJobConfig) {
	var priority float64 // Leave as default for most jobs.
	var deps []string

	if strings.Contains(name, "Experimental") {
		// Experimental recipe-less tasks.
		deps = append(deps, experimental(b, name))
	} else if strings.Contains(name, "UpdateCIPDPackages") {
		// Update CIPD packages bot.
		deps = append(deps, updateCIPDPackages(b, name))
	} else if strings.Contains(name, "Build-Bazel-Local") {
		deps = append(deps, bazelBuild(b, name, false /* =rbe */))
	} else if strings.Contains(name, "Build-Bazel-RBE") {
		deps = append(deps, bazelBuild(b, name, true /* =rbe */))
	} else if strings.Contains(name, "Test-Bazel-Local") {
		deps = append(deps, bazelTest(b, name, false /* =rbe */))
	} else if strings.Contains(name, "Test-Bazel-RBE") {
		deps = append(deps, bazelTest(b, name, true /* =rbe */))
	} else if name == "Housekeeper-PerCommit-CIPD-Canary" {
		deps = append(deps, buildAndDeployCanary(b, name))
	} else if name == "Housekeeper-PerCommit-CIPD-SK" {
		deps = append(deps, buildAndDeploySK(b, name))
	} else if name == "Housekeeper-PerCommit-CIPD-ValidateAutorollConfigs" {
		deps = append(deps, buildAndDeployValidateAutorollConfigs(b, name))
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
	} else if strings.Contains(name, "PerCommit-CIPD") {
		trigger = specs.TRIGGER_MAIN_ONLY
	}
	b.MustAddJob(name, &specs.JobSpec{
		Priority:  priority,
		TaskSpecs: deps,
		Trigger:   trigger,
	})

	// Add the CQ spec if it is a CQ job.
	if cqConfig != noCQ {
		b.MustAddCQJob(name, cqConfig)
	}
}

// Regenerate the tasks.json file.
func main() {
	b := specs.MustNewTasksCfgBuilder()

	// Create Tasks and Jobs.
	bundleRecipes(b)
	for name, cq := range jobsToCQStatus {
		process(b, name, cq)
	}

	// CasSpecs.
	b.MustAddCasSpec(casAutorollConfigs, &specs.CasSpec{
		Root:  ".",
		Paths: []string{"autoroll/config"},
	})
	b.MustAddCasSpec(casEmpty, specs.EmptyCasSpec)
	b.MustAddCasSpec(casRecipes, &specs.CasSpec{
		Root: "..",
		Paths: []string{
			"buildbot/.vpython",
			"buildbot/infra/config/recipes.cfg",
			"buildbot/infra/bots/bundle_recipes.sh",
			"buildbot/infra/bots/recipes",
			"buildbot/infra/bots/recipes.py",
		},
	})
	b.MustAddCasSpec(casRunRecipe, &specs.CasSpec{
		Root: "..",
		Paths: []string{
			"buildbot/.vpython",
			"buildbot/infra/bots/run_recipe.py",
		},
	})
	b.MustAddCasSpec(casWholeRepo, &specs.CasSpec{
		Root:     "..",
		Paths:    []string{"buildbot"},
		Excludes: []string{rbe.ExcludeGitDir},
	})

	b.MustFinish()
}
