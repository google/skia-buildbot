package main

import (
	"fmt"
	"path/filepath"
	//"context"
	"flag"
	"path"
	//"strings"

	//"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/task_driver/go/lib/checkout"
	"go.skia.org/infra/task_driver/go/lib/docker"
	//"go.skia.org/infra/task_driver/go/lib/gerrit_steps"
	"go.skia.org/infra/task_driver/go/lib/golang"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
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

	targetDir    = flag.String("target_dir", "", "Directory to execute the target script in. Eg: \"cd debugger\"")
	targetScript = flag.String("target_script", "", "Target script in the repo to run in docker. Eg: \"make release_ci\"")
	// imageName will be the output? eg: "gcr.io/skia-public/infra:prod" ??

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

	// Debugging
	fmt.Println("THIS IS THE REVISION")
	// Will need to figure out when it is a tryjob as well.. Do not let it run if trybot??
	fmt.Println(rs.Revision)
	fmt.Println(rs.Issue)
	fmt.Println(rs.Patchset)
	fmt.Println(rs.Server)
	// Debugging

	// Needed?
	// Setup go.
	ctx = golang.WithEnv(ctx, wd)

	if err := docker.Build(ctx, filepath.Join(co.Dir(), "docker") /* directory */, "gcr.io/skia-public/testing"); err != nil {
		td.Fatal(ctx, err)
	}

	//// Perform steps to update the dependencies.
	//// By default, the Go env includes GOFLAGS=-mod=readonly, which prevents
	//// commands from modifying go.mod; in this case, we want to modify it,
	//// so unset that variable.
	//ctx = td.WithEnv(ctx, []string{"GOFLAGS="})
	//if _, err := golang.Go(ctx, co.Dir(), "get", "-u"); err != nil {
	//	td.Fatal(ctx, err)
	//}

	//// Install some tool dependencies.
	//if err := golang.InstallCommonDeps(ctx, co.Dir()); err != nil {
	//	td.Fatal(ctx, err)
	//}

	//// These commands may also update dependencies, or their results may
	//// change based on the updated dependencies.
	//if _, err := golang.Go(ctx, co.Dir(), "build", "./..."); err != nil {
	//	td.Fatal(ctx, err)
	//}
	//// Setting -exec=echo causes the tests to not actually run; therefore
	//// this compiles the tests but doesn't run them.
	//if _, err := golang.Go(ctx, co.Dir(), "test", "-exec=echo", "./..."); err != nil {
	//	td.Fatal(ctx, err)
	//}
	//if _, err := golang.Go(ctx, co.Dir(), "generate", "./..."); err != nil {
	//	td.Fatal(ctx, err)
	//}

}
