// Copyright 2021 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// build_and_deploy_cipd performs a Bazel build of the given targets and uploads
// a CIPD package including the given build products.
package main

import (
	"context"
	"flag"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	cipd_pkg "go.chromium.org/luci/cipd/client/cipd/pkg"
	cipd_common "go.chromium.org/luci/cipd/common"

	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/task_driver/go/lib/bazel"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required properties for this task.
	projectId = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskId    = flag.String("task_id", "", "ID of this task.")
	taskName  = flag.String("task_name", "", "Name of the task.")

	pkgName       = flag.String("package_name", "", "Name of the CIPD package.")
	targets       = common.NewMultiStringFlag("target", nil, "Bazel build targets.")
	platformsList = common.NewMultiStringFlag("platform", nil, "Pairs of Bazel build platform and CIPD platform in <bazel platform>=<cipd platform> format.")

	bazelCacheDir     = flag.String("bazel_cache_dir", "", "Path to the Bazel cache directory.")
	bazelRepoCacheDir = flag.String("bazel_repo_cache_dir", "", "Path to the Bazel repository cache directory.")

	// Optional flags.
	buildDir       = flag.String("build_dir", ".", "Directory containing the Bazel workspace to build.")
	cipdServiceURL = flag.String("cipd_service_url", cipd.DefaultServiceURL, "CIPD service URL.")
	tags           = common.NewMultiStringFlag("tag", nil, "Tags to apply to the package, in key:value format.")
	refs           = common.NewMultiStringFlag("ref", nil, "Refs to apply to the package.")
	metadata       = common.NewMultiStringFlag("metadata", nil, "Metadata to apply to the package, in key:value format.")
	rbe            = flag.Bool("rbe", false, "Whether to run Bazel on RBE or locally.")
	rbeConfig      = flag.String("rbe_config", "", "Optional Bazel --config name to use when --rbe is true (defaults to \"remote\").")
	rbeKey         = flag.String("rbe_key", "", "Path to the service account key to use for RBE.")
	local          = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output         = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

func main() {
	// Setup.
	ctx := td.StartRun(projectId, taskId, taskName, output, local)
	defer td.EndRun(ctx)

	if *pkgName == "" {
		td.Fatalf(ctx, "--package_name is required.")
	}
	if len(*targets) == 0 {
		td.Fatalf(ctx, "At least one --target is required.")
	}
	if len(*platformsList) == 0 {
		td.Fatalf(ctx, "At least one --platform is required.")
	}
	for _, tag := range *tags {
		splitPair(ctx, tag, ":")
	}
	metadataMap := make(map[string]string, len(*metadata))
	for _, md := range *metadata {
		k, v := splitPair(ctx, md, ":")
		metadataMap[k] = v
	}

	// Create directories for each of the build platforms.
	pkgs := make([]*pkgSpec, 0, len(*platformsList))
	var cipdClient cipd.CIPDClient
	if err := td.Do(ctx, td.Props("Setup").Infra(), func(ctx context.Context) error {
		var err error
		cipdClient, err = cipd.NewClient(ctx, ".", *cipdServiceURL)
		if err != nil {
			return err
		}
		for _, platform := range *platformsList {
			bzlPlatform, cipdPlatform := splitPair(ctx, platform, "=")
			tmpDir, err := os_steps.TempDir(ctx, "", cipdPlatform)
			if err != nil {
				return err
			}
			pkgs = append(pkgs, &pkgSpec{
				bazelPlatform: bzlPlatform,
				cipdPlatform:  cipdPlatform,
				cipdPkgPath:   path.Join(*pkgName, cipdPlatform),
				tmpDir:        tmpDir,
			})
		}
		return nil
	}); err != nil {
		td.Fatal(ctx, err)
	}
	defer func() {
		if err := td.Do(ctx, td.Props("Cleanup").Infra(), func(ctx context.Context) error {
			var rvErr error
			for _, pkg := range pkgs {
				tmpDir := pkg.tmpDir
				if err := os_steps.RemoveAll(ctx, tmpDir); err != nil {
					rvErr = err
				}
			}
			return rvErr
		}); err != nil {
			td.Fatal(ctx, err)
		}
	}()

	// Perform the build(s).
	if err := td.Do(ctx, td.Props("Build"), func(ctx context.Context) (rvErr error) {
		opts := bazel.BazelOptions{
			CachePath:           *bazelCacheDir,
			RepositoryCachePath: *bazelRepoCacheDir,
			RBEConfig:           *rbeConfig,
		}
		bzl, err := bazel.New(ctx, *buildDir, *rbeKey, opts)
		if err != nil {
			return err
		}
		defer bzl.Cleanup()

		return buildPackages(ctx, bzl, *targets, pkgs, *rbe)
	}); err != nil {
		td.Fatal(ctx, err)
	}

	// Upload the package(s) to CIPD.
	// TODO(borenet): See if we can use the CIPD Go code directly, rather than
	// having to ship a separate binary.
	if err := td.Do(ctx, td.Props("Upload to CIPD"), func(ctx context.Context) error {
		// Upload all of the package instances.
		for _, pkg := range pkgs {
			if err := td.Do(ctx, td.Props(fmt.Sprintf("Upload %s", pkg.cipdPlatform)), func(ctx context.Context) error {
				pin, err := cipdClient.Create(ctx, pkg.cipdPkgPath, pkg.tmpDir, cipd_pkg.InstallModeCopy, nil, nil, nil, nil)
				if err != nil {
					return err
				}
				pkg.pin = pin
				return nil
			}); err != nil {
				return err
			}
		}
		// Apply refs, tags, and metadata. Do this after all platforms have been
		// built and uploaded to increase the likelihood that the refs and tags
		// get applied to all packages or none. Otherwise it's possible for some
		// platforms to be missing when querying by ref or tag.
		for _, pkg := range pkgs {
			if err := td.Do(ctx, td.Props(fmt.Sprintf("Attach %s %s", pkg.cipdPlatform, pkg.pin.String())), func(ctx context.Context) error {
				// If any of the provided tags is already attached to a
				// different instance, stop and return an error.
				for _, tag := range *tags {
					found, err := cipdClient.SearchInstances(ctx, pkg.cipdPkgPath, []string{tag})
					if err != nil {
						return err
					}
					if len(found) == 1 && found[0].InstanceID != pkg.pin.InstanceID {
						return skerr.Fmt("Found existing instance %s of package %s with tag %s", found[0].InstanceID, pkg.cipdPkgPath, tag)
					}
					if len(found) > 1 {
						return skerr.Fmt("Found more than one instance of package %s with tag %s. This may result in failure to retrieve the package by tag due to ambiguity. Please contact the current infra gardener to investigate. To detach tags, see http://go/luci-cipd#detachtags", pkg.cipdPkgPath, tag)
					}
				}
				return cipdClient.Attach(ctx, pkg.pin, *refs, *tags, metadataMap)
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		td.Fatal(ctx, err)
	}
}

// splitPair splits a key and value from a command line flag and Fatals if it
// does not follow the expected format.
func splitPair(ctx context.Context, elem, sep string) (string, string) {
	split := strings.SplitN(elem, sep, 2)
	if len(split) != 2 {
		td.Fatalf(ctx, "Expected <key>%s<value> format for %q", sep, elem)
	}
	return split[0], split[1]
}

// buildPackages builds each requested target for every platform in pkgs and
// copies the outputs reported by `bazelisk cquery --output=files` into each
// platform's temporary directory.
func buildPackages(ctx context.Context, bzl *bazel.Bazel, targets []string, pkgs []*pkgSpec, rbe bool) error {
	// Find the execution_root, in which all outputs will be built.
	output, err := bzl.Do(ctx, "info", "execution_root")
	if err != nil {
		return err
	}
	executionRoot := strings.TrimSpace(output)
	if executionRoot == "" {
		return skerr.Fmt("bazelisk info execution_root returned empty output")
	}

	doFunc := bzl.Do
	if rbe {
		doFunc = bzl.DoOnRBE
	}

	for _, pkg := range pkgs {
		if err := td.Do(ctx, td.Props("Build "+pkg.cipdPlatform), func(ctx context.Context) error {
			// Perform the build.
			platformFlag := fmt.Sprintf("--platforms=%s", pkg.bazelPlatform)
			args := append([]string{platformFlag}, targets...)
			if _, err := doFunc(ctx, "build", args...); err != nil {
				return err
			}

			// Copy the outputs to the destination dir. Run cquery with the same
			// flags as build so Bazel reuses the analysis cache.
			for _, target := range targets {
				output, err := doFunc(ctx, "cquery", platformFlag, "--output=files", target)
				if err != nil {
					return err
				}
				trimmed := strings.TrimSpace(output)
				if trimmed == "" {
					return skerr.Fmt("no output files found for %s (%s)", target, pkg.bazelPlatform)
				}
				outputFiles := strings.Split(trimmed, "\n")
				for _, relPath := range outputFiles {
					relPath = strings.TrimSpace(relPath)
					if relPath == "" {
						continue
					}
					fullPath := filepath.Join(executionRoot, relPath)
					if _, err := os_steps.Stat(ctx, fullPath); err != nil {
						return err
					}
					dest := filepath.Join(pkg.tmpDir, filepath.Base(fullPath))
					if err := os_steps.CopyFile(ctx, fullPath, dest); err != nil {
						return err
					}
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// pkgSpec contains information about how to build and upload an indivdual CIPD
// package instance.
type pkgSpec struct {
	bazelPlatform string
	cipdPlatform  string
	cipdPkgPath   string
	tmpDir        string
	pin           cipd_common.Pin
}
