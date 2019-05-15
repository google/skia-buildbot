package main

import (
	"context"
	"flag"
	"path"
	"strings"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_driver/go/lib/checkout"
	"go.skia.org/infra/task_driver/go/lib/gerrit_steps"
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

	gerritProject = flag.String("gerrit_project", "", "Gerrit project name.")
	gerritUrl     = flag.String("gerrit_url", "", "URL of the Gerrit instance.")
	patchIssue    = flag.String("patch_issue", "", "Issue ID, required if this is a try job.")
	patchSet      = flag.String("patch_set", "", "Patch Set ID, required if this is a try job.")
	repo          = flag.String("repo", "", "URL of the repo.")
	reviewers     = flag.String("reviewers", "", "Comma-separated list of emails to review the CL.")
	revision      = flag.String("revision", "", "Git revision to test.")

	// Optional flags.
	local  = flag.Bool("local", false, "True if running locally (as opposed to in production)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

func main() {
	// Setup.
	ctx := td.StartRun(projectId, taskId, taskName, output, local)
	defer td.EndRun(ctx)

	if *gerritProject == "" {
		td.Fatalf(ctx, "--gerrit_project is required.")
	}
	if *gerritUrl == "" {
		td.Fatalf(ctx, "--gerrit_url is required.")
	}
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

	// Check out the code.
	rs := types.RepoState{
		Repo:     *repo,
		Revision: *revision,
	}
	if *patchIssue != "" {
		rs.Patch = types.Patch{
			Issue:     *patchIssue,
			PatchRepo: *repo,
			Patchset:  *patchSet,
			Server:    *gerritUrl,
		}
	}
	co, err := checkout.EnsureGitCheckout(ctx, path.Join(wd, "repo"), rs)
	if err != nil {
		td.Fatal(ctx, err)
	}

	// Setup go.
	ctx = golang.SetEnv(ctx, wd)

	// Perform steps to update the dependencies.
	// By default, the Go env includes GOFLAGS=-mod=readonly, which prevents
	// commands from modifying go.mod; in this case, we want to modify it,
	// so unset that variable.
	ctx = td.SetEnv(ctx, []string{"GOFLAGS="})
	if _, err := golang.Go(ctx, co.Dir(), "get", "-u"); err != nil {
		td.Fatal(ctx, err)
	}

	// Install some tool dependencies.
	installTargets := []string{
		"github.com/golang/protobuf/protoc-gen-go",
		"github.com/kisielk/errcheck",
		"golang.org/x/tools/cmd/goimports",
		"golang.org/x/tools/cmd/stringer",
		"github.com/vektra/mockery/...",
	}
	for _, target := range installTargets {
		if err := golang.Install(ctx, co.Dir(), "-v", target); err != nil {
			td.Fatal(ctx, err)
		}
	}
	files, err := os_steps.ReadDir(ctx, path.Join(wd, "go", "go", "bin"))
	if err != nil {
		td.Fatal(ctx, err)
	}
	for _, f := range files {
		sklog.Infof(f.Name())
	}

	// These commands may also update dependencies, or their results may
	// change based on the updated dependencies.
	if _, err := golang.Go(ctx, co.Dir(), "build", "./..."); err != nil {
		td.Fatal(ctx, err)
	}
	if _, err := golang.Go(ctx, co.Dir(), "test", "-exec=echo", "./..."); err != nil {
		td.Fatal(ctx, err)
	}
	if _, err := golang.Go(ctx, co.Dir(), "generate", "./..."); err != nil {
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
		g, err := gerrit_steps.Init(ctx, *local, wd, *gerritUrl)
		if err != nil {
			td.Fatal(ctx, err)
		}
		if err := td.Do(ctx, td.Props("Upload CL").Infra(), func(ctx context.Context) error {
			ci, err := gerrit.CreateAndEditChange(ctx, g, *gerritProject, "master", "Update Go deps", *revision, func(ctx context.Context, g gerrit.GerritInterface, ci *gerrit.ChangeInfo) error {
				for _, f := range modFiles {
					contents, err := os_steps.ReadFile(ctx, path.Join(co.Dir(), f))
					if err != nil {
						return err
					}
					if err := g.EditFile(ctx, ci, f, string(contents)); err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				return err
			}
			if err := g.SetReview(ctx, ci, "Ready for review.", map[string]interface{}{
				gerrit.CODEREVIEW_LABEL:  gerrit.CODEREVIEW_LABEL_APPROVE,
				gerrit.COMMITQUEUE_LABEL: gerrit.COMMITQUEUE_LABEL_SUBMIT,
			}, strings.Split(*reviewers, ",")); err != nil {
				return err
			}
			return nil
		}); err != nil {
			td.Fatal(ctx, err)
		}
	}
}
