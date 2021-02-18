package main

import (
	"context"
	"flag"
	"path/filepath"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required properties for this task.
	projectID = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskID    = flag.String("task_id", "", "ID of this task.")
	taskName  = flag.String("task_name", "", "Name of the task.")
	workdir   = flag.String("workdir", ".", "Working directory")

	// Optional flags.
	local  = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

func main() {
	// Setup.
	ctx := td.StartRun(projectID, taskID, taskName, output, local)
	defer td.EndRun(ctx)

	// Compute various directory paths.
	wd, err := os_steps.Abs(ctx, *workdir)
	if err != nil {
		td.Fatal(ctx, err)
	}
	repoDir := filepath.Join(wd, "buildbot")

	// Initialize a fake Git repository. We will use it to detect diffs.
	//
	// We receive the code via Isolate, but it doesn't include the .git dir.
	gitDir := git.GitDir(repoDir)
	td.Do(ctx, td.Props("Initialize fake Git repository"), func(ctx context.Context) error {
		if gitVer, err := gitDir.Git(ctx, "version"); err != nil {
			td.Fatal(ctx, err)
		} else {
			sklog.Infof("Git version %s", gitVer)
		}
		if _, err := gitDir.Git(ctx, "init"); err != nil {
			td.Fatal(ctx, err)
		}
		if _, err := gitDir.Git(ctx, "config", "--local", "user.name", "Skia bots"); err != nil {
			td.Fatal(ctx, err)
		}
		if _, err := gitDir.Git(ctx, "config", "--local", "user.email", "fake@skia.bots"); err != nil {
			td.Fatal(ctx, err)
		}
		if _, err := gitDir.Git(ctx, "add", "."); err != nil {
			td.Fatal(ctx, err)
		}
		if _, err := gitDir.Git(ctx, "commit", "--no-verify", "-m", "Fake commit to detect diffs"); err != nil {
			td.Fatal(ctx, err)
		}
		return nil
	})

	// Causes the tryjob to fail in the presence of diffs, e.g. as a consequence of running Gazelle.
	checkNonEmptyGitDiff := func() error {
		_, err := gitDir.Git(ctx, "diff", "--no-ext-diff", "--exit-code")
		return err
	}

	// Temporary directory for the Bazel cache.
	//
	// We cannot use the default Bazel cache location ($HOME/.cache/bazel) because:
	//
	//  - The cache can be large (>10G).
	//  - Swarming bots have limited storage space (15G).
	//  - Because the above, the Bazel build fails with a "no space left on device" error.
	//  - The Bazel cache under $HOME/.cache/bazel lingers after the tryjob completes, causing the
	//    Swarming bot to be quarantined due to low disk space.
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
	bazel := func(args ...string) error {
		// TODO(lovisolo): Add --config=remote after we figure out how to authenticate against RBE.
		command := []string{"bazel", "--output_user_root=" + bazelCacheDir}
		command = append(command, args...)
		_, err := exec.RunCwd(ctx, repoDir, command...)
		return err
	}

	// Print out the Bazel version for debugging.
	if err := bazel("version"); err != nil {
		td.Fatal(ctx, err)
	}

	td.Do(ctx, td.Props("Buildifier: Format BUILD.bazel and *.bzl files"), func(ctx context.Context) error {
		if err := bazel("run", "//:buildifier"); err != nil {
			return err
		}
		if err := checkNonEmptyGitDiff(); err != nil {
			return err
		}
		return nil
	})

	td.Do(ctx, td.Props("Gazelle: Regenerate //go_repositories.bzl from //go.mod"), func(ctx context.Context) error {
		if err := bazel("run", "//:gazelle", "--", "update-repos", "-from_file=go.mod", "-to_macro=go_repositories.bzl%go_repositories"); err != nil {
			return err
		}
		if err := checkNonEmptyGitDiff(); err != nil {
			return err
		}
		return nil
	})

	td.Do(ctx, td.Props("Gazelle: Update BUILD targets for Go code"), func(ctx context.Context) error {
		if err := bazel("run", "//:gazelle", "--", "update", "."); err != nil {
			return err
		}
		if err := checkNonEmptyGitDiff(); err != nil {
			return err
		}
		return nil
	})

	// Build all code in the repository. The tryjob will fail upon any build errors.
	if err := bazel("build", "//..."); err != nil {
		td.Fatal(ctx, err)
	}
}
