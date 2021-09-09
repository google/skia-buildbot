package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/auth"
	docker_pubsub "go.skia.org/infra/go/docker/build/pubsub"
	"go.skia.org/infra/go/sklog"
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
	swarmOutDir   = flag.String("swarm_out_dir", "", "Swarming will isolate everything in this directory.")

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
	imageWithTag := fmt.Sprintf("%s:%s", *imageName, tag)

	// Create a temporary config dir for Docker.
	configDir, err := os_steps.TempDir(ctx, "", "")
	if err != nil {
		td.Fatal(ctx, err)
	}
	defer func() {
		if err := os_steps.RemoveAll(ctx, configDir); err != nil {
			sklog.Errorf("Could not remove %s: %s", configDir, err)
		}
	}()

	buildArgs := map[string]string{
		"HASH": rs.Revision,
	}
	if rs.Issue != "" && rs.Patchset != "" {
		buildArgs["PATCH_REF"] = rs.GetPatchRef()
	}
	// Retry docker commands if there are errors. Sometimes the access token expires between the
	// login and the push.
	NUM_ATTEMPTS := 2
	var dockerErr error
	for i := 0; i < NUM_ATTEMPTS; i++ {
		if dockerErr != nil {
			sklog.Warningf("Retrying because of %s", dockerErr)
			dockerErr = nil
		}

		// Login to docker (required to push to docker).
		token, tokErr := ts.Token()
		if tokErr != nil {
			dockerErr = tokErr
			continue
		}
		if loginErr := docker.Login(ctx, token.AccessToken, *imageName, configDir); loginErr != nil {
			dockerErr = loginErr
			continue
		}

		// Build docker image.
		if buildErr := docker.BuildHelper(ctx, filepath.Join(co.Dir(), *dockerfileDir), imageWithTag, configDir, buildArgs); buildErr != nil {
			dockerErr = buildErr
			continue
		}

		// Push to docker.
		if _, pushErr := docker.Push(ctx, imageWithTag, configDir); pushErr != nil {
			dockerErr = pushErr
			continue
		}

		// Docker cmds were successful, break out of the retry loop.
		break
	}
	if dockerErr != nil {
		td.Fatal(ctx, dockerErr)
	}

	if err := td.Do(ctx, td.Props(fmt.Sprintf("Publish pubsub msg to %s", docker_pubsub.TOPIC)).Infra(), func(ctx context.Context) error {
		// Publish to the pubsub topic.
		b, err := json.Marshal(&docker_pubsub.BuildInfo{
			ImageName: *imageName,
			Tag:       tag,
			Repo:      rs.Repo,
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

	// Write the image name and tag to the swarmOutDir.
	outputPath := filepath.Join(*swarmOutDir, fmt.Sprintf("%s.txt", *taskName))
	if err := os_steps.WriteFile(ctx, outputPath, []byte(imageWithTag), os.ModePerm); err != nil {
		td.Fatal(ctx, err)
	}
}
