package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"path"
	"path/filepath"

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/option"

	"go.skia.org/infra/docker_pushes_watcher/go/util"
	"go.skia.org/infra/go/auth"
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

	dockerfileDir = flag.String("dockerfile_dir", "", "Directory that contains the Dockerfile that should be built and pushed.")
	imageName     = flag.String("image_name", "", "Name of the image to build and push to docker. Eg: gcr.io/skia-public/infra")

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

	// TODO(rmistry): Needed?
	// Setup go.
	ctx = golang.WithEnv(ctx, wd)

	// Create token source with scope for cloud registry (storage) and pubsub.
	ts, err := auth_steps.Init(ctx, *local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_FULL_CONTROL, pubsub.ScopePubSub)
	if err != nil {
		td.Fatal(ctx, err)
	}

	// Create pubsub client.
	client, err := pubsub.NewClient(ctx, util.TOPIC_PROJECT_ID, option.WithTokenSource(ts))
	if err != nil {
		td.Fatal(ctx, err)
	}
	topic := client.Topic(util.TOPIC)
	//exists, err := topic.Exists(ctx)
	//if err != nil {
	//	td.Fatal(ctx, err)
	//}
	//if !exists {
	//	topic, err = client.CreateTopic(ctx, TOPIC)
	//	if err != nil {
	//		td.Fatal(ctx, err)
	//	}
	//}

	// Figure out which tag to use for docker build and push.
	tag := rs.Revision
	if rs.Issue != "" && rs.Patchset != "" {
		tag = fmt.Sprintf("%s_%s", rs.Issue, rs.Patchset)
	}
	// HACK FOR TESTING
	tag = "5cb33b46fddb4f9cd7e428f7f2118ba95e6c3808"
	// HACK FOR TESTING
	imageWithTag := fmt.Sprintf("%s:%s", *imageName, tag)

	// TEMPORARY
	if err := td.Do(ctx, td.Props(fmt.Sprintf("Publish pubsub msg to %s", util.TOPIC)).Infra(), func(ctx context.Context) error {
		// Publish to the pubsub topic.
		b, err := json.Marshal(&util.BuildInfo{
			ImageName: *imageName,
			Tag:       tag,
		})
		if err != nil {
			return err
		}
		msg := &pubsub.Message{
			Data: b,
		}
		res := topic.Publish(ctx, msg)
		if _, err := res.Get(ctx); err != nil {
			return err
		}
		return nil
	}); err != nil {
		td.Fatal(ctx, err)
	}
	// TEMPORARY

	// Build docker image.
	if err := docker.Build(ctx, filepath.Join(co.Dir(), *dockerfileDir), imageWithTag); err != nil {
		td.Fatal(ctx, err)
	}

	// Login to docker (required to push to docker).
	token, err := ts.Token()
	if err != nil {
		td.Fatal(ctx, err)
	}
	if err := docker.Login(ctx, token.AccessToken, *imageName); err != nil {
		td.Fatal(ctx, err)
	}

	// Push to docker.
	if err := docker.Push(ctx, imageWithTag); err != nil {
		td.Fatal(ctx, err)
	}

	if err := td.Do(ctx, td.Props(fmt.Sprintf("Publish pubsub msg to %s", util.TOPIC)).Infra(), func(ctx context.Context) error {
		// Publish to the pubsub topic.
		b, err := json.Marshal(&util.BuildInfo{
			ImageName: *imageName,
			Tag:       tag,
		})
		if err != nil {
			return err
		}
		msg := &pubsub.Message{
			Data: b,
		}
		res := topic.Publish(ctx, msg)
		if _, err := res.Get(ctx); err != nil {
			return err
		}
		return nil
	}); err != nil {
		td.Fatal(ctx, err)
	}
}
