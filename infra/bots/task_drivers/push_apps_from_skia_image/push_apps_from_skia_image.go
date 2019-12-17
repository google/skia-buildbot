package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"path"
	"time"

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/auth"
	docker_pubsub "go.skia.org/infra/go/docker/build/pubsub"
	"go.skia.org/infra/go/sklog"
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

const (
	FIDDLER_IMAGE_NAME         = "fiddler-v2"
	DEBUGGER_IMAGE_NAME        = "debugger-v2"
	DEBUGGER_ASSETS_IMAGE_NAME = "debugger-assets-v2"
	API_IMAGE_NAME             = "api-v2"
)

func buildPushImage(ctx context.Context, appName, buildCmd, image, tag, repo, configDir string, topic *pubsub.Topic) error {
	err := td.Do(ctx, td.Props(fmt.Sprintf("Build & Push %s Image", appName)).Infra(), func(ctx context.Context) error {
		// Create temporary dir to use for the image.
		tempDir, err := os_steps.TempDir(ctx, "", "")
		if err != nil {
			return err
		}
		defer os_steps.RemoveAll(ctx, tempDir)

		// Create the image locally using "gcr.io/skia-public/infra:prod".
		env := map[string]string{
			"SKIP_BUILD": "1",
			"ROOT":       "/OUT",
		}
		if err := docker.Run(ctx, "gcr.io/skia-public/infra:prod", fmt.Sprintf("%s:/OUT", tempDir), tag, buildCmd, configDir, env); err != nil {
			return err
		}

		// Build the image using docker.
		imageWithTag := fmt.Sprintf("%s:%s", image, tag)
		buildArgs := map[string]string{
			// TODO(rmistry): Change this to skia-release-v2.
			"SKIA_IMAGE_NAME": "skia-release",
			// TODO(rmistry): Change this to tag.
			"SKIA_IMAGE_TAG": "prod",
		}
		if err := docker.Build(ctx, tempDir, imageWithTag, configDir, buildArgs); err != nil {
			return err
		}

		// Push the docker image.
		if err := docker.Push(ctx, imageWithTag, configDir); err != nil {
			return err
		}

		// Send pubsub msg.
		return publishToTopic(ctx, image, tag, repo, topic)
	})
	return err
}

func buildPushFiddlerImage(ctx context.Context, tag, repo, configDir string, topic *pubsub.Topic) error {
	image := fmt.Sprintf("gcr.io/skia-public/%s", FIDDLER_IMAGE_NAME)
	buildCmd := "cd /home/skia/golib/src/go.skia.org/infra/fiddlek && ./build_fiddler_release"
	return buildPushImage(ctx, "Fiddler", buildCmd, image, tag, repo, configDir, topic)
}

func buildPushDebuggerImage(ctx context.Context, tag, repo, configDir string, topic *pubsub.Topic) error {
	image := fmt.Sprintf("gcr.io/skia-public/%s", DEBUGGER_IMAGE_NAME)
	buildCmd := "cd /home/skia/golib/src/go.skia.org/infra/debugger' && make release_ci"
	return buildPushImage(ctx, "Debugger", buildCmd, image, tag, repo, configDir, topic)
}

func buildPushDebuggerAssetsImage(ctx context.Context, tag, repo, configDir string, topic *pubsub.Topic) error {
	image := fmt.Sprintf("gcr.io/skia-public/%s", DEBUGGER_ASSETS_IMAGE_NAME)
	buildCmd := "cd /home/skia/golib/src/go.skia.org/infra/debugger-assets' && make release_ci"
	return buildPushImage(ctx, "Debugger-Assets", buildCmd, image, tag, repo, configDir, topic)
}

// NOT GOING TO WORK WITHOUT EXTRA STEP FOR DOXYGEN!
func buildPushApiImage(ctx context.Context, tag, repo, configDir string, topic *pubsub.Topic) error {
	image := fmt.Sprintf("gcr.io/skia-public/%s", API_IMAGE_NAME)
	buildCmd := "cd /home/skia/golib/src/go.skia.org/infra/api' && make release_ci"
	return buildPushImage(ctx, "Debugger-Assets", buildCmd, image, tag, repo, configDir, topic)
}

func publishToTopic(ctx context.Context, image, tag, repo string, topic *pubsub.Topic) error {
	return td.Do(ctx, td.Props(fmt.Sprintf("Publish pubsub msg to %s", docker_pubsub.TOPIC)).Infra(), func(ctx context.Context) error {
		// Publish to the pubsub topic.
		b, err := json.Marshal(&docker_pubsub.BuildInfo{
			ImageName: image,
			Tag:       tag,
			Repo:      repo,
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
	})
}

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
	ts, err := auth_steps.Init(ctx, *local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_FULL_CONTROL, pubsub.ScopePubSub)
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

	// FIDDLER - create a new function with a new step for build and push fiddler.
	if err := buildPushFiddlerImage(ctx, tag, rs.Repo, configDir, topic); err != nil {
		td.Fatal(ctx, err)
	}
}
