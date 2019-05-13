package main

import (
	"context"
	"flag"
	"fmt"
	"path"
	"strings"

	"go.skia.org/infra/task_driver/go/lib/checkout"
	"go.skia.org/infra/task_driver/go/lib/golang"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
	"go.skia.org/infra/task_scheduler/go/types"
)

var (
	// Required properties for this task.
	projectId = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskId    = flag.String("task_id", "", "ID of this task.")
	taskName  = flag.String("task_name", "", "Name of the task.")
	workdir   = flag.String("workdir", ".", "Working directory")

	gerritUrl  = flag.String("gerrit_url", "", "URL of the Gerrit instance.")
	repo       = flag.String("repo", "", "URL of the repo.")
	revision   = flag.String("revision", "", "Git revision to test.")
	patchIssue = flag.String("patch_issue", "", "Issue ID, required if this is a try job.")
	patchSet   = flag.String("patch_set", "", "Patch Set ID, required if this is a try job.")

	// Optional flags.
	local  = flag.Bool("local", false, "True if running locally (as opposed to in production)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

func main() {
	// Setup.
	ctx := td.StartRun(projectId, taskId, taskName, output, local)
	defer td.EndRun(ctx)

	// Check out the code.
	if *repo == "" {
		td.Fatalf(ctx, "--repo is required.")
	}
	if *revision == "" {
		td.Fatalf(ctx, "--revision is required.")
	}
	wd, err := os_steps.Abs(ctx, *workdir)
	if err != nil {
		td.Fatal(ctx, err)
	}
	co, err := checkout.EnsureGitCheckout(ctx, path.Join(wd, "repo"), types.RepoState{
		Repo:     *repo,
		Revision: *revision,
		Patch: types.Patch{
			Issue:     *patchIssue,
			PatchRepo: *repo,
			Patchset:  *patchSet,
			Server:    *gerritUrl,
		},
	})
	if err != nil {
		td.Fatal(ctx, err)
	}

	// Setup go.
	golang.Init(wd)

	// Perform steps to update the dependencies.
	if _, err := golang.Go(ctx, co.Dir(), "get", "-u"); err != nil {
		td.Fatal(ctx, err)
	}
	if _, err := golang.Go(ctx, co.Dir(), "build", "./..."); err != nil {
		td.Fatal(ctx, err)
	}
	if _, err := golang.Go(ctx, co.Dir(), "test", "-exec=echo", "./..."); err != nil {
		td.Fatal(ctx, err)
	}

	// If we changed anything, upload a CL.
	diff, err := co.Git(ctx, "diff", "--name-only")
	if err != nil {
		td.Fatal(ctx, err)
	}
	diff = strings.TrimSpace(diff)
	modFiles := strings.Split(diff, "\n")
	if len(modFiles) > 0 {
		if _, err := co.Git(ctx, "commit", "-a", "-m", "Update Go dependencies"); err != nil {
			td.Fatal(ctx, err)
		}

		// We need Depot Tools for "git cl".
		depotTools, err := checkout.EnsureGitCheckout(ctx, path.Join(*workdir, "depot_tools"), types.RepoState{
			Repo:     "https://chromium.googlesource.com/chromium/tools/depot_tools.git",
			Revision: "HEAD",
		})
		if err != nil {
			td.Fatal(ctx, err)
		}
		env := []string{fmt.Sprintf("PATH=%s:%s", depotTools.Dir(), td.PATH_PLACEHOLDER)}
		if err := td.Do(ctx, td.Props("depot tools env").Env(env), func(ctx context.Context) error {
			_, err := co.Git(ctx, "cl", "upload", "-f", "--bypass-hooks", "--bypass-watchlists", "--use-commit-queue")
			return err
		}); err != nil {
			td.Fatal(ctx, err)
		}
	}
}
