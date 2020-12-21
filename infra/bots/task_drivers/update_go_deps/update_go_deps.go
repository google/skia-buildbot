// update_go_deps modifies the go.mod and go.sum files to sync to the most
// recent versions of all listed dependencies.
//
// If the go.mod file is not being updated, check the recent runs of this Task
// Driver to verify that:
//
// 1. It is running at all. If not, there may be a bot capacity problem, or a
//    problem with the Task Scheduler.
// 2. It is succeeding. There are a number of reasons why it might fail, but the
//    most common is that a change has landed in one of the dependencies which
//    is not compatible with the current version of our code. Check the logs for
//    the failing step(s).
// 3. The CL uploaded by this task driver is passing the commit queue and
//    landing. This task driver does not run all of the tests and so the CL it
//    uploads may fail the commit queue for legitimate reasons. Look into the
//    failures and determine what actions to take.
//
// If update_go_deps itself is failing, or if the CL it uploads is failing to
// land, you may need to take one of the following actions:
//
// 1. If possible, update call sites in our repo(s) to match the upstream
//    changes. Include the update to go.mod in the same CL. This is only
//    possible if our repo is the only user of the modified dependency, or if
//    all other users have already updated to account for the change.
// 2. Add an "exclude" directive in go.mod. Ideally, this is temporary and can
//    be removed, eg. when all of our dependencies have updated to account for
//    a breaking change in a shared dependency. If you expect the exclude to be
//    temporary, file a bug and add a comment next to the exclude. Note that
//    only specific versions can be excluded, so we may need to exclude
//    additional versions for the same breaking change as versions are released.
// 3. If the breaking change is intentional and we never expect to be able to
//    update to a newer version of the dependency (eg. a required feature was
//    removed), fork the broken dependency. Update all references in our repo(s)
//    to use the fork, or add a "replace" directive in go.mod. Generally we
//    should file a bug against the dependency first to verify that the breaking
//    change is both intentional and not going to be reversed. Forking implies
//    some amount of maintenance headache (eg. what if the dependency is shared
//    by others which assume they're using the most recent version?), so this
//    should be a last resort.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_driver/go/lib/auth_steps"
	"go.skia.org/infra/task_driver/go/lib/checkout"
	"go.skia.org/infra/task_driver/go/lib/gerrit_steps"
	"go.skia.org/infra/task_driver/go/lib/golang"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/lib/rotations"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required properties for this task.
	gerritProject = flag.String("gerrit_project", "", "Gerrit project name.")
	gerritUrl     = flag.String("gerrit_url", "", "URL of the Gerrit server.")
	projectId     = flag.String("project_id", "", "ID of the Google Cloud project.")
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
	{
		// By default, the Go env includes GOFLAGS=-mod=readonly, which prevents
		// commands from modifying go.mod; in this case, we want to modify it,
		// so unset that variable.
		ctx := td.WithEnv(ctx, []string{"GOFLAGS="})

		// This "go list" command obtains the set of direct dependencies; that
		// is, the modules containing packages which are imported directly by
		// our code.
		var buf bytes.Buffer
		// Print the package import path and the module path for every package
		// imported in this repo.
		const format = "{{if .Module}}{{if not (or .Module.Main .Module.Indirect)}}{{.ImportPath}} {{.Module.Path}}{{end}}{{end}}"
		listCmd := &exec.Command{
			Name:   "go",
			Args:   []string{"list", "-f", format, "all"},
			Dir:    co.Dir(),
			Stdout: &buf,
		}
		if _, err := exec.RunCommand(ctx, listCmd); err != nil {
			td.Fatal(ctx, err)
		}
		// Organize the direct dependencies by module.
		deps := map[string][]string{}
		for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
			split := strings.Split(line, " ")
			if len(split) != 2 {
				td.Fatalf(ctx, "Incorrect format for line; expected \"<package path> <module path>\" but got: %s", line)
			}
			deps[split[1]] = append(deps[split[1]], split[0])
		}
		modules := make([]string, 0, len(deps))
		for module := range deps {
			modules = append(modules, module)
		}
		sort.Strings(modules)

		// Perform the update.
		getCmd := []string{
			"get",
			"-u", // Update the named modules.
			"-t", // Also update modules only used in tests.
			"-d", // Download the updated modules but don't build or install them.
		}
		if err := td.Do(ctx, td.Props("go "+strings.Join(getCmd, " ")), func(ctx context.Context) error {
			// We can't simply "go get" the module, because that will fail if it
			// has no .go files at the root level. It's also possible that
			// packages have been renamed or removed between versions, so
			// "go get" might fail for any single package as well. Try a series
			// of updates, starting with the module proper, and iterating over
			// the packages we import. Stop after the first success.
			var lastErr error
			for _, module := range modules {
				pkgs := deps[module]
				for _, target := range append([]string{module}, pkgs...) {
					_, lastErr = golang.Go(ctx, co.Dir(), append(getCmd, target)...)
					if lastErr == nil {
						break
					}
				}
			}
			return skerr.Wrap(lastErr)
		}); err != nil {
			td.Fatal(ctx, err)
		}

		// Explicitly build the infra module, because "go build ./..." doesn't
		// update go.sum for dependencies of the infra module when run in the
		// Skia repo. We have some Skia bots which install things from the infra
		// repo (eg. task drivers which are used directly and not imported), and
		// go.mod and go.sum need to account for that.
		if _, err := golang.Go(ctx, co.Dir(), "build", "-i", "go.skia.org/infra/..."); err != nil {
			td.Fatal(ctx, err)
		}

		// "go build" may also update dependencies, or its results may
		// change based on the updated dependencies.
		if _, err := golang.Go(ctx, co.Dir(), "build", "./..."); err != nil {
			td.Fatal(ctx, err)
		}

		// Setting -exec=echo causes the tests to not actually run; therefore
		// this compiles the tests but doesn't run them.
		if _, err := golang.Go(ctx, co.Dir(), "test", "-exec=echo", "./..."); err != nil {
			td.Fatal(ctx, err)
		}
	}

	// The below commands run with GOFLAGS=-mod=readonly and thus act as a
	// self-check to ensure that we've updated go.mod and go.sum correctly.

	// Tool dependencies; these should be listed in the top-level tools.go
	// file and should therefore be updated via "go get" above. If this
	// fails, it's likely because one of the tools we're installing is not
	// present in tools.go and therefore not present in go.mod.
	if err := golang.InstallCommonDeps(ctx, co.Dir()); err != nil {
		td.Fatal(ctx, err)
	}

	// The generators may have been updated, so run "go generate".
	if _, err := golang.Go(ctx, co.Dir(), "generate", "./..."); err != nil {
		td.Fatal(ctx, err)
	}

	// Regenerate the licenses file.
	if rs.Repo == common.REPO_SKIA_INFRA {
		if _, err := exec.RunCwd(ctx, filepath.Join(co.Dir(), "licenses"), "make", "regenerate"); err != nil {
			td.Fatal(ctx, err)
		}
	}

	// Regenerate infra/bots/tasks.json in case a dependency changed its
	// behavior.
	if _, err := golang.Go(ctx, filepath.Join(co.Dir(), "infra", "bots"), "run", "./gen_tasks.go"); err != nil {
		td.Fatal(ctx, err)
	}

	// If we changed anything, upload a CL.
	c, err := auth_steps.InitHttpClient(ctx, *local, auth.SCOPE_USERINFO_EMAIL)
	if err != nil {
		td.Fatal(ctx, err)
	}
	reviewers, err := rotations.GetCurrentInfraGardener(ctx, c)
	if err != nil {
		td.Fatal(ctx, err)
	}
	g, err := gerrit_steps.Init(ctx, *local, *gerritUrl)
	if err != nil {
		td.Fatal(ctx, err)
	}
	isTryJob := *local || rs.Issue != ""
	if isTryJob {
		var i int64
		if err := td.Do(ctx, td.Props(fmt.Sprintf("Parse %q as int", rs.Issue)).Infra(), func(ctx context.Context) error {
			var err error
			i, err = strconv.ParseInt(rs.Issue, 10, 64)
			return err
		}); err != nil {
			td.Fatal(ctx, err)
		}
		ci, err := gerrit_steps.GetIssueProperties(ctx, g, i)
		if err != nil {
			td.Fatal(ctx, err)
		}
		if !util.In(ci.Owner.Email, reviewers) {
			reviewers = append(reviewers, ci.Owner.Email)
		}
	}
	if err := gerrit_steps.UploadCL(ctx, g, co, *gerritProject, git.DefaultBranch, rs.Revision, "Update Go Deps", reviewers, isTryJob); err != nil {
		td.Fatal(ctx, err)
	}
}
