package main

import (
	"fmt"
	"path/filepath"
	"time"
	//"context"
	"flag"
	"path"
	//"strings"

	"go.skia.org/infra/go/auth"
	//"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/task_driver/go/lib/checkout"
	"go.skia.org/infra/task_driver/go/lib/docker"
	//"go.skia.org/infra/task_driver/go/lib/gerrit_steps"
	"go.skia.org/infra/task_driver/go/lib/auth_steps"
	"go.skia.org/infra/task_driver/go/lib/golang"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// tag name make configurable - skia-release vs skia-wasm-release

	// Required properties for this task.
	gerritProject = flag.String("gerrit_project", "", "Gerrit project name.")
	gerritUrl     = flag.String("gerrit_url", "", "URL of the Gerrit server.")
	projectId     = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskId        = flag.String("task_id", "", "ID of this task.")
	taskName      = flag.String("task_name", "", "Name of the task.")
	workdir       = flag.String("workdir", ".", "Working directory")

	imageName = flag.String("image_name", "", "Name of the image to build and push to docker. Eg: gcr.io/skia-public/infra")
	// tag       = flag.String("tag", "", "Tag name to push the image to docker with. Eg: prod for gcr.io/skia-public/infra:prod")
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
	if *imageName == "" {
		td.Fatalf(ctx, "--image_name is required.")
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
	// Override the tag with issue and patchset!!!
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

	// This works:
	// docker login -u oauth2accesstoken -p "$(gcloud auth print-access-token)" https://gcr.io

	// Login(ctx context.Context, accessToken, hostname string)
	// How to pass in accessToken..
	//ts, err := auth_steps.Init(....)
	//token, err := ts.Token()
	//token.AccessToken

	//ts, err := auth_steps.Init(....)
	//token, err := ts.Token()
	//token.AccessToken

	//if err := docker.Login(ctx, )

	ts, err := auth_steps.Init(ctx, *local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_FULL_CONTROL)
	if err != nil {
		td.Fatal(ctx, err)
	}
	token, err := ts.Token()
	if err != nil {
		td.Fatal(ctx, err)
	}
	if err := docker.Login(ctx, token.AccessToken, "gcr.io/skia-public/testing"); err != nil {
		//if err := docker.Login(ctx, token.AccessToken, "https://gcr.io"); err != nil {
		td.Fatal(ctx, err)
	}

	// TODO(Rmistry): must push a version. Make tag and make version arguments!!
	tag := rs.Revision
	if err := docker.Push(ctx, "gcr.io/skia-public/testing"); err != nil {
		fmt.Println("ABOUT TO THROW ERROR SLEEPING for 1 Hour")
		fmt.Println(err)
		time.Sleep(1 * time.Hour)
		td.Fatal(ctx, err)
	}
}
