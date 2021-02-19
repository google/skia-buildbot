package main

import (
	"flag"
	"path/filepath"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required properties for this task.
	projectID = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskID    = flag.String("task_id", "", "ID of this task.")
	taskName  = flag.String("task_name", "", "Name of the task.")
	workdir   = flag.String("workdir", ".", "Working directory.")

	// Exactly one of these flags must be provided.
	rbe   = flag.Bool("rbe", false, "Run Bazel on RBE.")
	noRBE = flag.Bool("no_rbe", false, "Run Bazel locally (i.e. not on RBE).")

	// Optional flags.
	local  = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

func main() {
	flag.Parse()
	if (*rbe && *noRBE) || (!*rbe && !*noRBE) {
		sklog.Fatalf("Exactly one of flags --rbe or --no_rbe must be set.")
	}

	// Setup.
	ctx := td.StartRun(projectID, taskID, taskName, output, local)
	defer td.EndRun(ctx)

	// Compute various directory paths.
	wd, err := os_steps.Abs(ctx, *workdir)
	if err != nil {
		td.Fatal(ctx, err)
	}
	repoDir := filepath.Join(wd, "buildbot")             // Repository checkout.
	bazelCacheDir := filepath.Join(wd, "cache", "bazel") // Named cache.

	// // Initialize a fake Git repository. We will use it to detect diffs.
	// //
	// // We receive the code via Isolate, but it doesn't include the .git dir.
	// gitDir := git.GitDir(repoDir)
	// td.Do(ctx, td.Props("Initialize fake Git repository"), func(ctx context.Context) error {
	// 	if gitVer, err := gitDir.Git(ctx, "version"); err != nil {
	// 		td.Fatal(ctx, err)
	// 	} else {
	// 		sklog.Infof("Git version %s", gitVer)
	// 	}
	// 	if _, err := gitDir.Git(ctx, "init"); err != nil {
	// 		td.Fatal(ctx, err)
	// 	}
	// 	if _, err := gitDir.Git(ctx, "config", "--local", "user.name", "Skia bots"); err != nil {
	// 		td.Fatal(ctx, err)
	// 	}
	// 	if _, err := gitDir.Git(ctx, "config", "--local", "user.email", "fake@skia.bots"); err != nil {
	// 		td.Fatal(ctx, err)
	// 	}
	// 	if _, err := gitDir.Git(ctx, "add", "."); err != nil {
	// 		td.Fatal(ctx, err)
	// 	}
	// 	if _, err := gitDir.Git(ctx, "commit", "--no-verify", "-m", "Fake commit to detect diffs"); err != nil {
	// 		td.Fatal(ctx, err)
	// 	}
	// 	return nil
	// })

	// // Causes the tryjob to fail in the presence of diffs, e.g. as a consequence of running Gazelle.
	// failIfNonEmptyGitDiff := func() {
	// 	if _, err := gitDir.Git(ctx, "diff", "--no-ext-diff", "--exit-code"); err != nil {
	// 		td.Fatal(ctx, err)
	// 	}
	// }

	// // Set up go.
	// ctx = golang.WithEnv(ctx, wd)
	// if err := golang.InstallCommonDeps(ctx, repoDir); err != nil {
	// 	td.Fatal(ctx, err)
	// }

	// // Run "go generate" and fail it there are any diffs.
	// if _, err := golang.Go(ctx, repoDir, "generate", "./..."); err != nil {
	// 	td.Fatal(ctx, err)
	// }
	// failIfNonEmptyGitDiff()

	// // Run "go fmt" and fail it there are any diffs.
	// if _, err := golang.Go(ctx, repoDir, "fmt", "./..."); err != nil {
	// 	td.Fatal(ctx, err)
	// }
	// failIfNonEmptyGitDiff()

	// By invoking Bazel via this function, we ensure that we will always use the "bazel" named cache
	// as the Bazel cache, rather than the default $HOME/.cache/bazel location.
	//
	// This is important because:
	//
	//	- The Bazel cache can be large (>10G).
	//  - Swarming bots have limited storage space on the root partition (15G).
	//  - Because the above, the Bazel build fails with a "no space left on device" error when using
	//    the default cache location ($HOME/.cache/bazel).
	//  - The Bazel cache under $HOME/.cache/bazel lingers after the tryjob completes, causing the
	//    Swarming bot to be quarantined due to low disk space.
	//  - Generally, it's considered poor hygiene to leave a bot in a different state.
	//
	// Reference: https://docs.bazel.build/versions/master/output_directories.html#current-layout.
	bazel := func(args ...string) {
		command := []string{"bazel", "--output_user_root=" + bazelCacheDir}
		if *rbe {
			// TODO(lovisolo): Uncomment once we figure out how to authenticate against RBE.
			// command = append(command, "--config=remote")
		}
		command = append(command, args...)
		if _, err := exec.RunCwd(ctx, repoDir, command...); err != nil {
			td.Fatal(ctx, err)
		}
	}

	// Print out the Bazel version for debugging purposes.
	bazel("version")

	// Buildifier formats all BUILD.bazel and .bzl files. We enforce formatting by making the tryjob
	// fail if this step produces any diffs.
	bazel("run", "//:buildifier")
	// failIfNonEmptyGitDiff()

	// // Regenerate //go_repositories.bzl from //go.mod with Gazelle, and fail if there are any diffs.
	// bazel("run", "//:gazelle", "--", "update-repos", "-from_file=go.mod", "-to_macro=go_repositories.bzl%go_repositories")
	// failIfNonEmptyGitDiff()

	// // Update all Go BUILD targets with Gazelle, and fail if there are any diffs.
	// bazel("run", "//:gazelle", "--", "update", ".")
	// failIfNonEmptyGitDiff()

	// Build all code in the repository. The tryjob will fail upon any build errors.
	// bazel("build", "//...")
}
