package main

import (
	"context"
	"flag"
	"path"
	"strings"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/task_driver/go/lib/checkout"
	"go.skia.org/infra/task_driver/go/lib/gerrit_steps"
	"go.skia.org/infra/task_driver/go/lib/golang"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required properties for this task.
	gerritProject = flag.String("gerrit_project", "", "Gerrit project name.")
	gerritUrl     = flag.String("gerrit_url", "", "URL of the Gerrit server.")
	projectId     = flag.String("project_id", "", "ID of the Google Cloud project.")
	reviewers     = flag.String("reviewers", "", "Comma-separated list of emails to review the CL.")
	taskId        = flag.String("task_id", "", "ID of this task.")
	taskName      = flag.String("task_name", "", "Name of the task.")
	workdir       = flag.String("workdir", ".", "Working directory")

	checkoutFlags = checkout.SetupFlags(nil)

	// Optional flags.
	local  = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

func main() {
	// Setup.
	ctx := td.StartRun(projectId, taskId, taskName, output, local)
	defer td.EndRun(ctx)

	rs, err := checkout.GetRepoState(checkoutFlags)
	if err != nil {
		td.Fatal(ctx, err)
	}
	if *gerritProject == "" {
		td.Fatalf(ctx, "--gerrit_project is required.")
	}
	if *gerritUrl == "" {
		td.Fatalf(ctx, "--gerrit_url is required.")
	}

	wd, err := os_steps.Abs(ctx, *workdir)
	if err != nil {
		td.Fatal(ctx, err)
	}

	// Check out the code.
	co, err := checkout.EnsureGitCheckout(ctx, path.Join(wd, "repo"), rs)
	if err != nil {
		td.Fatal(ctx, err)
	}

	// Setup go.
	ctx = golang.WithEnv(ctx, wd)

	// Perform steps to update the dependencies.
	// By default, the Go env includes GOFLAGS=-mod=readonly, which prevents
	// commands from modifying go.mod; in this case, we want to modify it,
	// so unset that variable.
	ctx = td.WithEnv(ctx, []string{"GOFLAGS="})
	if _, err := golang.Go(ctx, co.Dir(), "get", "-u"); err != nil {
		td.Fatal(ctx, err)
	}

	// Install some tool dependencies.
	if err := golang.InstallCommonDeps(ctx, co.Dir()); err != nil {
		td.Fatal(ctx, err)
	}

	// These commands may also update dependencies, or their results may
	// change based on the updated dependencies.
	if _, err := golang.Go(ctx, co.Dir(), "build", "./..."); err != nil {
		td.Fatal(ctx, err)
	}
	// Setting -exec=echo causes the tests to not actually run; therefore
	// this compiles the tests but doesn't run them.
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
	if len(modFiles) > 0 && diff != "" {
		g, err := gerrit_steps.Init(ctx, *local, wd, *gerritUrl)
		if err != nil {
			td.Fatal(ctx, err)
		}
		if err := td.Do(ctx, td.Props("Upload CL").Infra(), func(ctx context.Context) error {
			ci, err := gerrit.CreateAndEditChange(ctx, g, *gerritProject, "master", "Update Go deps", rs.Revision, func(ctx context.Context, g gerrit.GerritInterface, ci *gerrit.ChangeInfo) error {
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
			var labels map[string]interface{}
			if !*local && rs.Issue == "" {
				labels = map[string]interface{}{
					gerrit.CODEREVIEW_LABEL:  gerrit.CODEREVIEW_LABEL_APPROVE,
					gerrit.COMMITQUEUE_LABEL: gerrit.COMMITQUEUE_LABEL_SUBMIT,
				}
			}
			if err := g.SetReview(ctx, ci, "Ready for review.", labels, strings.Split(*reviewers, ",")); err != nil {
				return err
			}
			return nil
		}); err != nil {
			td.Fatal(ctx, err)
		}
	}
}
