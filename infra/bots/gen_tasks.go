// Copyright 2016 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

/*
	Generate the tasks.json file.
*/

import (
	"fmt"
	"strings"

	"go.skia.org/infra/go/cas/rbe"
	"go.skia.org/infra/task_scheduler/go/specs"
)

const (
	buildTaskDriversName = "Housekeeper-PerCommit-BuildTaskDrivers"

	casAutorollConfigs = "autoroll-configs"
	casEmpty           = "empty" // TODO(borenet): It'd be nice if this wasn't necessary.
	casWholeRepo       = "whole-repo"

	defaultOSLinux   = "Debian-10.3"
	defaultOSWindows = "Windows-Server-17763"

	// machineTypeMedium refers to a 16-core machine
	machineTypeMedium = "n1-standard-16"
	// machineTypeLarge refers to a 64-core machine.
	machineTypeLarge = "n1-highcpu-64"

	cipdUploaderServiceAccount = "cipd-uploader@skia-swarming-bots.iam.gserviceaccount.com"
	compileServiceAccount      = "skia-external-compile-tasks@skia-swarming-bots.iam.gserviceaccount.com"
	recreateSKPsServiceAccount = "skia-recreate-skps@skia-swarming-bots.iam.gserviceaccount.com"
)

var (
	// jobsToCQStatus lists all infra Jobs and their CQ config to run at each commit.
	jobsToCQStatus = map[string]*specs.CommitQueueJobConfig{
		"Housekeeper-OnDemand-Presubmit":  &cqWithDefaults,
		"Infra-PerCommit-Build-Bazel-RBE": &cqWithDefaults,
		"Infra-PerCommit-Test-Bazel-RBE":  &cqWithDefaults,

		"Housekeeper-PerCommit-CIPD-Canary":                  noCQ,
		"Housekeeper-PerCommit-CIPD-SK":                      noCQ,
		"Housekeeper-PerCommit-CIPD-ValidateAutorollConfigs": noCQ,
		"Housekeeper-Weekly-UpdateCIPDPackages":              noCQ,
		"Infra-Experimental-Small-Linux":                     noCQ,
		"Infra-Experimental-Small-Win":                       noCQ,
		"Infra-PerCommit-Build-Bazel-Local":                  noCQ,
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

	vpythonCaches = []*specs.Cache{
		{
			Name: "vpython",
			Path: "cache/vpython",
		},
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

// Run the presubmit.
func presubmit(b *specs.TasksCfgBuilder, name string) string {
	pkgs := append([]*specs.CipdPackage{}, specs.CIPD_PKGS_GIT_LINUX_AMD64...)
	pkgs = append(pkgs, b.MustGetCipdPackageFromAsset("bazelisk"))

	cmd := []string{
		"./presubmit",
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

	t := &specs.TaskSpec{
		CasSpec:      casEmpty,
		CipdPackages: pkgs,
		Command:      cmd,
		Dependencies: []string{buildTaskDrivers(b, "Linux", "x86_64")},
		Dimensions:   linuxGceDimensions(machineTypeMedium),
		EnvPrefixes: map[string][]string{
			"PATH": {"cipd_bin_packages", "bazelisk"},
		},
		ServiceAccount: compileServiceAccount,
		MaxAttempts:    1,
	}
	b.MustAddTask(name, t)
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
			// We don't use any outputs of the Test job, we just want to make sure that we
			// do not upload to CIPD if our tests are failing.
			"Infra-PerCommit-Test-Bazel-RBE",
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
	for name, cq := range jobsToCQStatus {
		process(b, name, cq)
	}

	// CasSpecs.
	b.MustAddCasSpec(casAutorollConfigs, &specs.CasSpec{
		Root:  ".",
		Paths: []string{"autoroll/config"},
	})
	b.MustAddCasSpec(casEmpty, specs.EmptyCasSpec)
	b.MustAddCasSpec(casWholeRepo, &specs.CasSpec{
		Root:     "..",
		Paths:    []string{"buildbot"},
		Excludes: []string{rbe.ExcludeGitDir},
	})

	b.MustFinish()
}
