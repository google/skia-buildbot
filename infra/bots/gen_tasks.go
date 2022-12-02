// Copyright 2016 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

/*
	Generate the tasks.json file.
*/

import (
	"fmt"
	"path"
	"strings"

	"go.skia.org/infra/go/cas/rbe"
	"go.skia.org/infra/go/cipd"
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
		"Housekeeper-OnDemand-Presubmit":        &cqWithDefaults,
		"Infra-PerCommit-Build-Bazel-RBE":       &cqWithDefaults,
		"Infra-PerCommit-Test-Bazel-RBE":        &cqWithDefaults,
		"Housekeeper-Weekly-UpdateCIPDPackages": noCQ,
		"Infra-PerCommit-Build-Bazel-Local":     noCQ,
		"Infra-PerCommit-Test-Bazel-Local":      noCQ,
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

func usesBazelisk(b *specs.TasksCfgBuilder, t *specs.TaskSpec) {
	t.CipdPackages = append(t.CipdPackages, b.MustGetCipdPackageFromAsset("bazelisk"))
	if t.EnvPrefixes == nil {
		t.EnvPrefixes = map[string][]string{}
	}
	t.EnvPrefixes["PATH"] = append(t.EnvPrefixes["PATH"], "bazelisk")
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
	t := &specs.TaskSpec{
		CasSpec: casWholeRepo,
		Command: []string{
			"/bin/bash", "buildbot/infra/bots/build_task_drivers.sh", specs.PLACEHOLDER_ISOLATED_OUTDIR,
			goos + "_" + goarch,
		},
		Dimensions: linuxGceDimensions(machineTypeMedium),
		// This task is idempotent but unlikely to ever be deduped
		// because it depends on the entire repo...
		Idempotent:     true,
		ServiceAccount: compileServiceAccount,
	}
	usesBazelisk(b, t)
	usesWrapperTaskDriver(b, name, false, t)
	b.MustAddTask(name, t)
	return name
}

// Run the presubmit.
func presubmit(b *specs.TasksCfgBuilder, name string) string {
	pkgs := append([]*specs.CipdPackage{}, specs.CIPD_PKGS_GIT_LINUX_AMD64...)

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
		"--bazel_cache_dir", "/dev/shm/bazel_cache",
		"--bazel_repo_cache_dir", "/mnt/pd0/bazel_repo_cache",
	}

	t := &specs.TaskSpec{
		CasSpec:      casEmpty,
		CipdPackages: pkgs,
		Command:      cmd,
		Dependencies: []string{buildTaskDrivers(b, "Linux", "x86_64")},
		Dimensions:   linuxGceDimensions(machineTypeMedium),
		EnvPrefixes: map[string][]string{
			"PATH": {"cipd_bin_packages"},
		},
		ServiceAccount: compileServiceAccount,
		MaxAttempts:    1,
	}
	// To iterate on the presubmit task driver, comment out the
	// call to usePreBuiltTaskDrivers.
	usesPreBuiltTaskDrivers(b, t)
	usesBazelisk(b, t)
	usesWrapperTaskDriver(b, name, true, t)
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
	usesBazelisk(b, t)
	usesWrapperTaskDriver(b, name, true, t)
	b.MustAddTask(name, t)
	return name
}

// usesPreBuiltTaskDrivers changes the task to use pre-built task
// drivers for efficiency. If you want to iterate on the task driver
// itself, just comment out the line where this is called.
func usesPreBuiltTaskDrivers(b *specs.TasksCfgBuilder, t *specs.TaskSpec) {
	// Determine which task driver we want.
	tdName := path.Base(t.Command[0])

	// Add the CIPD package for the task driver.
	t.CipdPackages = append(t.CipdPackages, cipd.MustGetPackage("skia/tools/"+tdName+"/${platform}"))

	// Update the command to use the task driver from the CIPD package.
	t.Command[0] = "./task_drivers/" + tdName

	// Remove the BuildTaskDrivers task from the dependencies.
	newDeps := make([]string, 0, len(t.Dependencies))
	for _, dep := range t.Dependencies {
		if !strings.HasPrefix(dep, buildTaskDriversName) {
			newDeps = append(newDeps, dep)
		}
	}
	t.Dependencies = newDeps
}

func usesWrapperTaskDriver(b *specs.TasksCfgBuilder, name string, isTaskDriver bool, t *specs.TaskSpec) {
	newCmd := []string{
		"./task_drivers/command_wrapper",
		"--project_id", "skia-swarming-bots",
		"--task_id", specs.PLACEHOLDER_TASK_ID,
		"--task_name", name,
		"--workdir", ".",
	}
	for _, pkg := range t.CipdPackages {
		flag := fmt.Sprintf("%s:%s@%s", pkg.Path, pkg.Name, pkg.Version)
		newCmd = append(newCmd, "--cipd", flag)
	}
	if isTaskDriver {
		newCmd = append(newCmd, "--command-is-task-driver")
	}
	newCmd = append(newCmd, "--")
	newCmd = append(newCmd, t.Command...)
	t.Command = newCmd

	t.CipdPackages = []*specs.CipdPackage{
		cipd.MustGetPackage("skia/tools/command_wrapper/${platform}"),
	}
}

func bazelBuild(b *specs.TasksCfgBuilder, name string, rbe bool) string {
	pkgs := append([]*specs.CipdPackage{}, specs.CIPD_PKGS_GIT_LINUX_AMD64...)

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
		"--bazel_cache_dir", "/dev/shm/bazel_cache",
		"--bazel_repo_cache_dir", "/mnt/pd0/bazel_repo_cache",
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
			"PATH": {"cipd_bin_packages"},
		},
		ServiceAccount: compileServiceAccount,
	}
	// To iterate on the bazel_build_all task driver, comment out the
	// call to usePreBuiltTaskDrivers.
	usesPreBuiltTaskDrivers(b, t)
	usesBazelisk(b, t)
	usesWrapperTaskDriver(b, name, true, t)
	b.MustAddTask(name, t)
	return name
}

func bazelTest(b *specs.TasksCfgBuilder, name string, rbe bool) string {
	pkgs := append([]*specs.CipdPackage{}, specs.CIPD_PKGS_GIT_LINUX_AMD64...)
	pkgs = append(pkgs, specs.CIPD_PKGS_ISOLATE...)

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
		"--bazel_cache_dir", "/dev/shm/bazel_cache",
		"--bazel_repo_cache_dir", "/mnt/pd0/bazel_repo_cache",
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
			},
		},
		ServiceAccount: compileServiceAccount,
	}
	// To iterate on the bazel_build_all task driver, comment out the
	// call to usePreBuiltTaskDrivers.
	usesPreBuiltTaskDrivers(b, t)
	usesBazelisk(b, t)
	usesWrapperTaskDriver(b, name, true, t)
	b.MustAddTask(name, t)
	return name
}

// process generates Tasks and Jobs for the given Job name.
func process(b *specs.TasksCfgBuilder, name string, cqConfig *specs.CommitQueueJobConfig) {
	var priority float64 // Leave as default for most jobs.
	var deps []string

	if strings.Contains(name, "UpdateCIPDPackages") {
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
