package main

import (
	"flag"
	"path"
	"path/filepath"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/task_driver/go/lib/checkout"
	"go.skia.org/infra/task_driver/go/lib/golang"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required properties for this task.
	projectID   = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskID      = flag.String("task_id", "", "ID of this task.")
	taskName    = flag.String("task_name", "", "Name of the task.")
	workDirFlag = flag.String("workdir", ".", "Working directory.")
	rbe         = flag.Bool("rbe", false, "Whether to run Bazel on RBE or locally.")

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
	skiaInfraRbeKeyFile := filepath.Join(workDir, "skia_infra_rbe_key", "rbe-ci.json")

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

	// Run "go fmt" and fail it there are any diffs.
	if _, err := golang.Go(ctx, gitDir.Dir(), "fmt", "./..."); err != nil {
		td.Fatal(ctx, err)
	}
	failIfNonEmptyGitDiff()

	// Temporary directory for the Bazel cache.
	//
	// We cannot use the default Bazel cache location ($HOME/.cache/bazel) because:
	//
	//  - The cache can be large (>10G).
	//  - Swarming bots have limited storage space on the root partition (15G).
	//  - Because the above, the Bazel build fails with a "no space left on device" error.
	//  - The Bazel cache under $HOME/.cache/bazel lingers after the tryjob completes, causing the
	//    Swarming bot to be quarantined due to low disk space.
	//  - Generally, it's considered poor hygiene to leave a bot in a different state.
	//
	// The temporary directory created by the below function call lives under /mnt/pd0, which has
	// significantly more storage space, and will be wiped after the tryjob completes.
	//
	// Reference: https://docs.bazel.build/versions/master/output_directories.html#current-layout.
	bazelCacheDir, err := os_steps.TempDir(ctx, "", "bazel-user-cache-*")
	if err != nil {
		td.Fatal(ctx, err)
	}

	// By invoking Bazel via this function, we ensure that we will always use the temporary cache.
	bazel := func(args ...string) {
		command := []string{"bazel", "--output_user_root=" + bazelCacheDir}
		command = append(command, args...)
		if _, err := exec.RunCwd(ctx, gitDir.Dir(), command...); err != nil {
			td.Fatal(ctx, err)
		}
	}

	// Print out the Bazel version for debugging purposes.
	bazel("version")

	// Buildifier formats all BUILD.bazel and .bzl files. We enforce formatting by making the tryjob
	// fail if this step produces any diffs.
	bazel("run", "//:buildifier")
	failIfNonEmptyGitDiff()

	// Regenerate //go_repositories.bzl from //go.mod with Gazelle, and fail if there are any diffs.
	bazel("run", "//:gazelle", "--", "update-repos", "-from_file=go.mod", "-to_macro=go_repositories.bzl%go_repositories")
	failIfNonEmptyGitDiff()

	// Update all Go BUILD targets with Gazelle, and fail if there are any diffs.
	bazel("run", "//:gazelle", "--", "update", ".")
	failIfNonEmptyGitDiff()

	// Build all code in the repository. The tryjob will fail upon any build errors.
	if *rbe {
		bazel("build", "//...", "--config=remote", "--google_credentials="+skiaInfraRbeKeyFile)
	} else {
		bazel("build", "//...")
	}
}
