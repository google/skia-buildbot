package main

import (
	"flag"
	"path"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/task_driver/go/lib/bazel"
	"go.skia.org/infra/task_driver/go/lib/checkout"
	"go.skia.org/infra/task_driver/go/lib/golang"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required properties for this task.
	projectID     = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskID        = flag.String("task_id", "", "ID of this task.")
	taskName      = flag.String("task_name", "", "Name of the task.")
	workDirFlag   = flag.String("workdir", ".", "Working directory.")
	rbe           = flag.Bool("rbe", false, "Whether to run Bazel on RBE or locally.")
	rbeKey        = flag.String("rbe_key", "", "Path to the service account key to use for RBE.")
	ramdiskSizeGb = flag.Int("ramdisk_gb", 40, "Size of ramdisk to use, in GB.")

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

	// Compute various paths.
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

	// Set up go.
	ctx = golang.WithEnv(ctx, workDir)
	if err := golang.InstallCommonDeps(ctx, gitDir.Dir()); err != nil {
		td.Fatal(ctx, err)
	}

	// Run "go generate" and fail it there are any diffs.
	if _, err := golang.Go(ctx, gitDir.Dir(), "generate", "./..."); err != nil {
		td.Fatal(ctx, err)
	}
	failIfNonEmptyGitDiff()

	// Run "errcheck" and fail if there are any findings.
	//
	// For some reason, exec.RunCwd cannot find the errcheck binary without an absolute path, which
	// is weird because /mnt/pd0/s/w/ir/gopath/bin is included in $PATH, so we provide an absolute
	// path to the errcheck binary.
	if _, err := exec.RunCwd(ctx, path.Join(workDir, "repo"), path.Join(workDir, "gopath", "bin", "errcheck"), "-ignore", ":Close", "go.skia.org/infra/..."); err != nil {
		td.Fatal(ctx, err)
	}

	// Set up Bazel.
	var (
		bzl        *bazel.Bazel
		bzlCleanup func()
	)
	if !*rbe && !*local {
		// Infra-PerCommit-Build-Bazel-Local uses a ramdisk as the Bazel cache. I/O on GCE VMs can be
		// very slow, which that can cause said task to time out.
		bzl, bzlCleanup, err = bazel.NewWithRamdisk(ctx, gitDir.Dir(), *rbeKey, *ramdiskSizeGb)
	} else {
		bzl, err = bazel.New(ctx, gitDir.Dir(), *local, *rbeKey)
		bzlCleanup = func() {}
	}
	if err != nil {
		td.Fatal(ctx, err)
	}
	defer bzlCleanup()

	// Print out the Bazel version for debugging purposes.
	if _, err := bzl.Do(ctx, "version"); err != nil {
		td.Fatal(ctx, err)
	}

	// Run "go fmt" and fail it there are any diffs.
	if _, err := bzl.Do(ctx, "run", "//:gofmt", "--", "-s", "-w", "."); err != nil {
		td.Fatal(ctx, err)
	}
	failIfNonEmptyGitDiff()

	// Buildifier formats all BUILD.bazel and .bzl files. We enforce formatting by making the tryjob
	// fail if this step produces any diffs.
	if _, err := bzl.Do(ctx, "run", "//:buildifier"); err != nil {
		td.Fatal(ctx, err)
	}
	failIfNonEmptyGitDiff()

	// Regenerate //go_repositories.bzl from //go.mod with Gazelle, and fail if there are any diffs.
	if _, err := bzl.Do(ctx, "run", "//:gazelle", "--", "update-repos", "-from_file=go.mod", "-to_macro=go_repositories.bzl%go_repositories"); err != nil {
		td.Fatal(ctx, err)
	}
	failIfNonEmptyGitDiff()

	// Update all Go BUILD targets with Gazelle, and fail if there are any diffs.
	if _, err := bzl.Do(ctx, "run", "//:gazelle", "--", "update", "."); err != nil {
		td.Fatal(ctx, err)
	}
	failIfNonEmptyGitDiff()

	// Run "npm cache clean -f".
	if _, err := exec.RunCwd(ctx, path.Join(workDir, "repo"), "npm", "cache", "clean", "-f"); err != nil {
		td.Fatal(ctx, err)
	}

	// Build all code in the repository. The tryjob will fail upon any build errors.
	doFunc := bzl.Do
	if *rbe {
		doFunc = bzl.DoOnRBE
	}
	if _, err := doFunc(ctx, "build", "//..."); err != nil {
		td.Fatal(ctx, err)
	}
}
