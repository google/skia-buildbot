// build_and_deploy_cipd performs a Bazel build of the given targets and uploads
// a CIPD package including the given build products.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
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
	includePaths  = common.NewMultiStringFlag("include_path", nil, "Paths to include, relative to bazel-bin.  Use [.exe] for optional suffix, eg. \"program[.exe]\"")

	// Optional flags.
	buildDir       = flag.String("build_dir", ".", "Directory containing the Bazel workspace to build.")
	cipdServiceURL = flag.String("cipd_service_url", "https://chrome-infra-packages.appspot.com", "CIPD service URL.")
	tags           = common.NewMultiStringFlag("tag", nil, "Tags to apply to the package, in key:value format.")
	refs           = common.NewMultiStringFlag("ref", nil, "Refs to apply to the package.")
	metadata       = common.NewMultiStringFlag("metadata", nil, "Metadata to apply to the package, in key:value format.")
	rbe            = flag.Bool("rbe", false, "Whether to run Bazel on RBE or locally.")
	rbeKey         = flag.String("rbe_key", "", "Path to the service account key to use for RBE.")
	local          = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output         = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

var (
	// executableSuffixRegex is used to parse an --include_path which uses the
	// path[.extension] format.
	executableSuffixRegex = regexp.MustCompile(`(.+)\[(.+)\]`)
)

func main() {
	// Setup.
	ctx := td.StartRun(projectId, taskId, taskName, output, local)
	defer td.EndRun(ctx)

	if *pkgName == "" {
		td.Fatalf(ctx, "--package_name is required.")
	}
	if len(*includePaths) == 0 {
		td.Fatalf(ctx, "At least one --include_path is required.")
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
	for _, md := range *metadata {
		splitPair(ctx, md, ":")
	}

	// Create directories for each of the build platforms.
	pkgs := make([]*pkgSpec, 0, len(*platformsList))
	if err := td.Do(ctx, td.Props("Setup").Infra(), func(ctx context.Context) error {
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
		bzl, cleanup, err := bazel.New(ctx, *buildDir, *local, *rbeKey)
		if err != nil {
			return err
		}
		defer cleanup()

		for _, pkg := range pkgs {
			if err := td.Do(ctx, td.Props("Build "+pkg.cipdPlatform), func(ctx context.Context) error {
				// We're building for multiple platforms, and Bazel writes all
				// of the build products into the same directory regardless of
				// platform, so there's a potential for accidental inclusion of
				// incompatible binaries in the CIPD package, eg. "app.exe" vs
				// "app". "bazel clean" prevents that by emptying the output
				// directory between builds.
				if _, err := bzl.Do(ctx, "clean"); err != nil {
					return err
				}

				// Perform the build.
				args := []string{fmt.Sprintf("--platforms=%s", pkg.bazelPlatform)}
				args = append(args, *targets...)
				doFunc := bzl.Do
				if *rbe {
					doFunc = bzl.DoOnRBE
				}
				if _, err := doFunc(ctx, "build", args...); err != nil {
					return err
				}

				// Copy the outputs to the destination dir.
				for _, path := range *includePaths {
					paths := []string{path}
					m := executableSuffixRegex.FindAllStringSubmatch(path, -1)
					if m != nil {
						paths = []string{m[0][1], m[0][1] + m[0][2]}
					}
					found := false
					for _, path := range paths {
						path := filepath.Join(*buildDir, path)
						if _, err := os_steps.Stat(ctx, path); err == nil {
							dest := filepath.Join(pkg.tmpDir, filepath.Base(path))
							if err := os_steps.CopyFile(ctx, path, dest); err != nil {
								return err
							}
							found = true
							break
						}
					}
					if !found {
						return fmt.Errorf("Unable to find %q; tried %v", path, paths)
					}
				}
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		td.Fatal(ctx, err)
	}

	// Upload the package(s) to CIPD.
	// TODO(borenet): See if we can use the CIPD Go code directly, rather than
	// having to ship a separate binary.
	if err := td.Do(ctx, td.Props("Upload to CIPD"), func(ctx context.Context) error {
		// Upload all of the package instances.
		for _, pkg := range pkgs {
			outputDir, err := os_steps.TempDir(ctx, "", pkg.cipdPlatform+"_output")
			if err != nil {
				return err
			}
			defer func() {
				if err := os_steps.RemoveAll(ctx, outputDir); err != nil {
					td.Fatal(ctx, err)
				}
			}()
			outputFile := filepath.Join(outputDir, "results.json")

			cmd := []string{
				"cipd", "create",
				"-service-url", *cipdServiceURL,
				"-compression-level", "1",
				"-in", pkg.tmpDir,
				"-name", pkg.cipdPkgPath,
				"-json-output", outputFile,
			}
			if _, err := exec.RunCwd(ctx, ".", cmd...); err != nil {
				return err
			}
			b, err := os_steps.ReadFile(ctx, outputFile)
			if err != nil {
				return err
			}
			var result cipdResult
			if err := json.Unmarshal(b, &result); err != nil {
				return err
			}
			pkg.uploadedInstanceID = result.Result.InstanceID
		}
		// Apply refs, tags, and metadata. Do this after all platforms have been
		// built and uploaded to increase the likelihood that the refs and tags
		// get applied to all packages or none. Otherwise it's possible for some
		// platforms to be missing when querying by ref or tag.
		for _, pkg := range pkgs {
			cmd := []string{
				"cipd", "attach", pkg.cipdPkgPath,
				"-service-url", *cipdServiceURL,
				"-version", pkg.uploadedInstanceID,
			}
			for _, md := range *metadata {
				cmd = append(cmd, "-metadata", md)
			}
			for _, ref := range *refs {
				cmd = append(cmd, "-ref", ref)
			}
			for _, tag := range *tags {
				cmd = append(cmd, "-tag", tag)
			}
			if _, err := exec.RunCwd(ctx, ".", cmd...); err != nil {
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

// pkgSpec contains information about how to build and upload an indivdual CIPD
// package instance.
type pkgSpec struct {
	bazelPlatform      string
	cipdPlatform       string
	cipdPkgPath        string
	tmpDir             string
	uploadedInstanceID string
}

// cipdResult describes the structure of CIPD's JSON output.
type cipdResult struct {
	Result struct {
		Package    string `json:"package"`
		InstanceID string `json:"instance_id"`
	} `json:"result"`
}
