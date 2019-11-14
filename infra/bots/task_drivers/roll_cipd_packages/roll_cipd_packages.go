package main

import (
	"context"
	"flag"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_driver/go/lib/auth_steps"
	"go.skia.org/infra/task_driver/go/lib/checkout"
	"go.skia.org/infra/task_driver/go/lib/golang"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

const (
	// Tag indicating the most recently uploaded version of a CIPD package.
	TAG_LATEST = "latest"
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

	// Read packages from cipd.ensure.
	var pkgs []*cipd.Package
	if err := td.Do(ctx, td.Props("Read cipd.ensure").Infra(), func(ctx context.Context) error {
		pkgs, err = cipd.ParseEnsureFile(filepath.Join(co.Dir(), "cipd.ensure"))
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		td.Fatal(ctx, err)
	}
	sort.Sort(cipd.PackageSlice(pkgs))

	// Find the latest versions of the desired packages.
	c, err := auth_steps.InitHttpClient(ctx, *local, auth.SCOPE_USERINFO_EMAIL)
	if err != nil {
		td.Fatal(ctx, err)
	}
	var cc *cipd.Client
	if err := td.Do(ctx, td.Props("Create CIPD client").Infra(), func(ctx context.Context) error {
		cc, err = cipd.NewClient(c, *workdir)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		td.Fatal(ctx, err)
	}
	if err := td.Do(ctx, td.Props("Get latest package versions").Infra(), func(ctx context.Context) error {
		for _, pkg := range pkgs {
			// Fake a platform.
			name := strings.Replace(pkg.Name, "${platform}", "linux-amd64")
			if err := td.Do(ctx, td.Props(fmt.Sprintf("Resolve %s", name)).Infra(), func(ctx context.Context) error {
				pin, err := cc.ResolveVersion(ctx, name, TAG_LATEST)
				if err != nil {
					return err
				}
				desc, err := cc.Describe(ctx, name, pin.InstanceID)
				if err != nil {
					return err
				}
				sklog.Infof("%s: %+v", name, desc)
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		td.Fatal(ctx, err)
	}

	// If we changed anything, upload a CL.
	/*diff, err := co.Git(ctx, "diff", "--name-only")
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
			var labels map[string]int
			if !*local && rs.Issue == "" {
				labels = map[string]int{
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
	}*/
}
