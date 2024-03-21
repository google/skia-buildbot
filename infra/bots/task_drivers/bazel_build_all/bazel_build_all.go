package main

import (
	"flag"
	"path"

	"go.skia.org/infra/task_driver/go/lib/bazel"
	"go.skia.org/infra/task_driver/go/lib/checkout"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required properties for this task.
	projectID         = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskID            = flag.String("task_id", "", "ID of this task.")
	taskName          = flag.String("task_name", "", "Name of the task.")
	workDirFlag       = flag.String("workdir", ".", "Working directory.")
	rbeKey            = flag.String("rbe_key", "", "Path to the service account key to use for RBE.")
	ramdiskSizeGb     = flag.Int("ramdisk_gb", 40, "Size of ramdisk to use, in GB.")
	bazelCacheDir     = flag.String("bazel_cache_dir", "", "Path to the Bazel cache directory.")
	bazelRepoCacheDir = flag.String("bazel_repo_cache_dir", "", "Path to the Bazel repository cache directory.")

	checkoutFlags = checkout.SetupFlags(nil)

	// Optional flags.
	local  = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

func main() {
	flag.Parse()

	// Setup.
	ctx := td.StartRun(projectID, taskID, taskName, output, local)
	defer td.EndRun(ctx)

	// Compute work dir path.
	workDir, err := os_steps.Abs(ctx, *workDirFlag)
	if err != nil {
		td.Fatal(ctx, err)
	}

	// Check out the code.
	repoState, err := checkout.GetRepoState(checkoutFlags)
	if err != nil {
		td.Fatal(ctx, err)
	}
	gitDir, err := checkout.EnsureGitCheckout(ctx, path.Join(workDir, "repo"), repoState)
	if err != nil {
		td.Fatal(ctx, err)
	}

	// Causes the tryjob to fail in the presence of diffs, e.g. as a consequence of running Gazelle.
	failIfNonEmptyGitDiff := func() {
		if _, err := gitDir.Git(ctx, "diff", "--no-ext-diff", "--exit-code"); err != nil {
			td.Fatal(ctx, err)
		}
	}

	// Set up Bazel.
	opts := bazel.BazelOptions{
		CachePath:           *bazelCacheDir,
		RepositoryCachePath: *bazelRepoCacheDir,
	}
	bzl, err := bazel.New(ctx, gitDir.Dir(), *rbeKey, opts)
	if err != nil {
		td.Fatal(ctx, err)
	}

	// Print out the Bazel version for debugging purposes.
	if _, err := bzl.Do(ctx, "version"); err != nil {
		td.Fatal(ctx, err)
	}

	// Regenerate "//:pnpm-lock.yaml" and fail if there are any diffs.
	//
	// This is an experimental attempt at preventing spurious //pnpm-lock.yaml diffs such as:
	//
	//     ...
	//         /puppeteer@19.2.2:
	//           resolution: {integrity: sha512-m1T5Mog5qu5+dMBptWYTn6pXRdnFbydbVUCthqwbfd8/kOiMlzZBR9ywjX79LpvI1Sj+/z8+FKeIsjnMul8ZYA==}
	//           engines: {node: '>=14.1.0'}
	//     -     deprecated: < 21.5.0 is no longer supported
	//     +     deprecated: < 21.3.7 is no longer supported
	//           requiresBuild: true
	//     ...
	//
	// The below command should regenerate //pnpm-lock.yaml based on the contents of //package.json
	// and //package-lock.json. It is basead on the sample pnpm invocations in
	// https://docs.aspect.build/rulesets/aspect_rules_js/docs/pnpm/#update_pnpm_lock.
	//
	// If we still see spurious diffs, another thing we could try is to empty the contents of
	// //pnpm-lock.yaml (e.g. with "echo > pnpm-lock.yaml"). This should cause said file to be
	// regenerated upon the next Bazel build, and was tested empirically by lovisolo@.
	if _, err := bzl.DoOnRBE(ctx, "run", "@pnpm//:pnpm", "--", "--dir", gitDir.Dir(), "install", "--lockfile-only"); err != nil {
		td.Fatal(ctx, err)
	}
	failIfNonEmptyGitDiff()

	// Run "go generate" and fail it there are any diffs.
	if _, err := bzl.DoOnRBE(ctx, "run", "//:go", "--", "generate", "./..."); err != nil {
		td.Fatal(ctx, err)
	}
	failIfNonEmptyGitDiff()

	// Run "errcheck" and fail it there are any findings.
	if _, err := bzl.DoOnRBE(ctx, "run", "//:errcheck", "--", "-ignore", ":Close", "go.skia.org/infra/..."); err != nil {
		td.Fatal(ctx, err)
	}

	// Run "gofmt" and fail it there are any diffs.
	if _, err := bzl.DoOnRBE(ctx, "run", "//:gofmt", "--", "-s", "-w", "."); err != nil {
		td.Fatal(ctx, err)
	}
	failIfNonEmptyGitDiff()

	// Buildifier formats all BUILD.bazel and .bzl files. We enforce formatting by making the tryjob
	// fail if this step produces any diffs.
	if _, err := bzl.DoOnRBE(ctx, "run", "//:buildifier"); err != nil {
		td.Fatal(ctx, err)
	}
	failIfNonEmptyGitDiff()

	// Regenerate //go_repositories.bzl from //go.mod with Gazelle, and fail if there are any diffs.
	if _, err := bzl.DoOnRBE(ctx, "run", "//:gazelle", "--", "update-repos", "-from_file=go.mod", "-to_macro=go_repositories.bzl%go_repositories"); err != nil {
		td.Fatal(ctx, err)
	}
	failIfNonEmptyGitDiff()

	// Update all Go BUILD targets with Gazelle, and fail if there are any diffs.
	if _, err := bzl.DoOnRBE(ctx, "run", "//:gazelle", "--", "update", "."); err != nil {
		td.Fatal(ctx, err)
	}
	failIfNonEmptyGitDiff()

	// Build all code in the repository. The tryjob will fail upon any build errors.
	//
	// We invoke Bazel with --remote_download_minimal to avoid "no space left on device errors". See
	// https://bazel.build/reference/command-line-reference#flag--remote_download_minimal.
	if _, err := bzl.DoOnRBE(ctx, "build", "//...", "--remote_download_minimal"); err != nil {
		td.Fatal(ctx, err)
	}
}
