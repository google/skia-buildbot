// This executable builds the Docker images based off the WASM executables in the
// gcr.io/skia-public/skia-wasm-release image. It then issues a PubSub notification to have those apps
// tagged and deployed by docker_pushes_watcher.
// See //docker_pushes_watcher/README.md for more.
package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"path"

	sk_exec "go.skia.org/infra/go/exec"

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/auth"
	docker_pubsub "go.skia.org/infra/go/docker/build/pubsub"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_driver/go/lib/auth_steps"
	"go.skia.org/infra/task_driver/go/lib/checkout"
	"go.skia.org/infra/task_driver/go/lib/docker"
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
	_, err = checkout.EnsureGitCheckout(ctx, path.Join(wd, "repo"), rs)
	if err != nil {
		td.Fatal(ctx, err)
	}

	// Setup go.
	ctx = golang.WithEnv(ctx, wd)

	// Create token source with scope for cloud registry (storage) and pubsub.
	ts, err := auth_steps.Init(ctx, *local, auth.ScopeUserinfoEmail, auth.ScopeFullControl, pubsub.ScopePubSub)
	if err != nil {
		td.Fatal(ctx, err)
	}

	// Create pubsub client.
	client, err := pubsub.NewClient(ctx, docker_pubsub.TOPIC_PROJECT_ID, option.WithTokenSource(ts))
	if err != nil {
		td.Fatal(ctx, err)
	}
	topic := client.Topic(docker_pubsub.TOPIC)

	// Figure out which tag to use for docker build and push.
	tag := rs.Revision
	if rs.Issue != "" && rs.Patchset != "" {
		tag = fmt.Sprintf("%s_%s", rs.Issue, rs.Patchset)
	}

	// Create a temporary config dir for Docker.
	configDir, err := ioutil.TempDir("", "")
	if err != nil {
		td.Fatal(ctx, err)
	}
	defer util.RemoveAll(configDir)

	// Login to docker (required to push to docker).
	token, err := ts.Token()
	if err != nil {
		td.Fatal(ctx, err)
	}
	if err := docker.Login(ctx, token.AccessToken, "gcr.io/skia-public/", configDir); err != nil {
		td.Fatal(ctx, err)
	}

	// Run skia-wasm-release image and extract wasm products out of it.
	wasmProductsDir, err := os_steps.TempDir(ctx, "", "")
	if err != nil {
		td.Fatal(ctx, err)
	}
	// Run Doxygen pointing to the location of the checkout and the out dir.
	volumes := []string{
		fmt.Sprintf("%s:/OUT", wasmProductsDir),
	}
	wasmCopyCmd := []string{"/bin/sh", "-c", "cp -r /tmp/* /OUT"}
	releaseImg := fmt.Sprintf("gcr.io/skia-public/skia-wasm-release:%s", tag)
	if err := docker.Run(ctx, releaseImg, configDir, wasmCopyCmd, volumes, nil); err != nil {
		td.Fatal(ctx, err)
	}

	// TODO(kjlubick) Build and push all apps of interest as they are ported.
	if err := testBazel(ctx); err != nil {
		td.Fatal(ctx, err)
	}
	fmt.Printf("TODO(kjlubick): need to publish to pubsub topic %s", topic.String())

	// Remove all temporary files from the host machine. Swarming gets upset if there are root-owned
	// files it cannot clean up.
	cleanupCmd := []string{"/bin/sh", "-c", "rm -rf /OUT/*"}
	if err := docker.Run(ctx, releaseImg, configDir, cleanupCmd, volumes, nil); err != nil {
		td.Fatal(ctx, err)
	}
}

func testBazel(ctx context.Context) error {
	return td.Do(ctx, td.Props("Getting bazel version").Infra(), func(ctx context.Context) error {
		runCmd := &sk_exec.Command{
			Name:      "bazel",
			Args:      []string{"version"},
			LogStdout: true,
			LogStderr: true,
		}
		_, err := sk_exec.RunCommand(ctx, runCmd)
		if err != nil {
			return err
		}
		return nil
	})
}
