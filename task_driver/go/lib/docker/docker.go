// Package docker is for running Dockerfiles.
package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"go.skia.org/infra/go/auth"
	docker_pubsub "go.skia.org/infra/go/docker/build/pubsub"
	sk_exec "go.skia.org/infra/go/exec"
	"go.skia.org/infra/task_driver/go/lib/log_parser"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	AUTH_SCOPES = []string{auth.ScopeUserinfoEmail, auth.ScopeFullControl}

	REPOSITORY_HOST = "gcr.io"

	// dockerStepRegex is a regex that matches Step lines in Docker output.
	dockerStepRegex = regexp.MustCompile(`^Step \d+\/\d+ : .*`)

	// dockerCmd is the name of the executable to run Docker. A variable so we
	// can change it at test time.
	dockerCmd = "docker"

	// imageSha256Regex is used to parse an image sha256 sum from log
	// output.
	imageSha256Regex = regexp.MustCompile(`sha256:[a-f0-9]{64}`)
)

type Docker struct {
	configDir string
	stop      chan struct{}
}

func New(ctx context.Context, ts oauth2.TokenSource) (*Docker, error) {
	configDir, err := os_steps.TempDir(ctx, "", "")
	if err != nil {
		td.Fatal(ctx, err)
	}
	stop := make(chan struct{})
	ready := make(chan error)
	go func() {
		for {
			now := time.Now()
			tok, err := ts.Token()
			if err == nil {
				err = Login(ctx, tok.AccessToken, REPOSITORY_HOST, configDir)
			}
			if ready != nil {
				ready <- err
				ready = nil
			}
			if err != nil {
				return
			}
			t := time.NewTimer(tok.Expiry.Sub(now))
			select {
			case <-stop:
				stop <- struct{}{}
				return
			case <-t.C:
			}

		}
	}()
	rv := &Docker{
		configDir: configDir,
		stop:      stop,
	}
	err = <-ready
	if err != nil {
		return nil, err
	}
	return rv, nil
}

func (d *Docker) Cleanup(ctx context.Context) error {
	return td.Do(ctx, td.Props("Docker Cleanup").Infra(), func(ctx context.Context) error {
		d.stop <- struct{}{}
		<-d.stop
		return os_steps.RemoveAll(ctx, d.configDir)
	})
}

// Pull a Docker image.
func (d *Docker) Pull(ctx context.Context, imageWithTag string) error {
	return Pull(ctx, imageWithTag, d.configDir)
}

// Push a Docker file.
func (d *Docker) Push(ctx context.Context, tag string) (string, error) {
	return Push(ctx, tag, d.configDir)
}

// Tag the given Docker image.
func (d *Docker) Tag(ctx context.Context, imageID, tag string) error {
	return Tag(ctx, imageID, tag, d.configDir)
}

// Run does a "docker run".
//
// volumes should be in the form of "ARG1:ARG2" where ARG1 is the local directory and ARG2 will be the directory in the image.
// Note the above does a --rm i.e. it automatically removes the container when it exits.
func (d *Docker) Run(ctx context.Context, image string, cmd, volumes, env []string) error {
	return Run(ctx, image, d.configDir, cmd, volumes, env)
}

// Run "docker build" with the given args.
func (d *Docker) Build(ctx context.Context, args ...string) error {
	return Build(ctx, append([]string{"--config", d.configDir, "build"}, args...)...)
}

// Extract the given src from the given image to the given host dest.
func (d *Docker) Extract(ctx context.Context, image, src, dest string) error {
	return td.Do(ctx, td.Props(fmt.Sprintf("Extract %s %s:%s", image, src, dest)), func(ctx context.Context) (rv error) {
		// Create a container from the image with a dummy command.
		containerName := fmt.Sprintf("tmp-%s", uuid.New().String())
		cmd := []string{dockerCmd, "--config", d.configDir, "create", "--name", containerName, image, "dummy-cmd"}
		if _, err := sk_exec.RunCwd(ctx, ".", cmd...); err != nil {
			return err
		}

		// Make sure we remove the container once we're done with it.
		defer func() {
			_, err := sk_exec.RunCwd(ctx, ".", dockerCmd, "rm", "-v", containerName)
			if err != nil {
				rv = err
			}
		}()

		// Perform the copy.
		_, err := sk_exec.RunCwd(ctx, ".", dockerCmd, "cp", "-L", fmt.Sprintf("%s:%s", containerName, src), dest)
		return err
	})
}

// Login to docker to be able to run authenticated commands (Eg: docker.Push).
func Login(ctx context.Context, accessToken, hostname, configDir string) error {
	loginCmd := &sk_exec.Command{
		Name:      dockerCmd,
		Args:      []string{"--config", configDir, "login", "-u", "oauth2accesstoken", "--password-stdin", hostname},
		Stdin:     strings.NewReader(accessToken),
		LogStdout: true,
		LogStderr: true,
	}
	_, err := sk_exec.RunCommand(ctx, loginCmd)
	if err != nil {
		return err
	}
	return nil
}

// Pull a Docker image.
func Pull(ctx context.Context, imageWithTag, configDir string) error {
	pullCmd := fmt.Sprintf("%s --config %s pull %s", dockerCmd, configDir, imageWithTag)
	_, err := sk_exec.RunSimple(ctx, pullCmd)
	if err != nil {
		return err
	}
	return nil
}

// Push a Docker image.
func Push(ctx context.Context, tag, configDir string) (string, error) {
	out, err := sk_exec.RunCwd(ctx, ".", dockerCmd, "--config", configDir, "push", tag)
	if err != nil {
		return "", err
	}
	m := imageSha256Regex.FindStringSubmatch(out)
	if len(m) == 1 {
		return m[0], nil
	}
	return "", nil
}

// Tag the given Docker image.
func Tag(ctx context.Context, imageID, tag, configDir string) error {
	_, err := sk_exec.RunCwd(ctx, ".", "docker", "--config", configDir, "tag", imageID, tag)
	return err
}

// Run does a "docker run".
//
// volumes should be in the form of "ARG1:ARG2" where ARG1 is the local directory and ARG2 will be the directory in the image.
// Note the above does a --rm i.e. it automatically removes the container when it exits.
func Run(ctx context.Context, image, configDir string, cmd, volumes, env []string) error {
	runArgs := []string{"--config", configDir, "run"}
	for _, v := range volumes {
		runArgs = append(runArgs, "--volume", v)
	}
	for _, e := range env {
		runArgs = append(runArgs, "--env", e)
	}
	runArgs = append(runArgs, image)
	runArgs = append(runArgs, cmd...)
	runCmd := &sk_exec.Command{
		Name:      dockerCmd,
		Args:      runArgs,
		LogStdout: true,
		LogStderr: true,
	}
	_, err := sk_exec.RunCommand(ctx, runCmd)
	if err != nil {
		return err
	}
	return nil
}

// Build a Dockerfile.
//
// There must be a Dockerfile in the 'directory' and the resulting output is
// tagged with 'tag'.
func BuildHelper(ctx context.Context, directory, tag, configDir string, buildArgs map[string]string) error {
	cmdArgs := []string{"--config", configDir, "build", "--pull", "-t", tag, directory}
	if buildArgs != nil {
		for k, v := range buildArgs {
			cmdArgs = append(cmdArgs, "--build-arg", fmt.Sprintf("%s=%s", k, v))
		}
	}
	return Build(ctx, cmdArgs...)
}

// Build runs "docker build <args>" in 'directory' and streams the
// output. The log output is parsed into sub-steps for each line starting with
// "Step N/M : ACTION value"
//
// Examples:
//   Step 1/7 : FROM debian:testing-slim
//   ---> e205e0c9e7f5
//   Step 2/7 : RUN apt-get update && apt-get upgrade -y && apt-get install -y   git   python    curl
//   ---> Using cache
//   ---> 5b8240d40b63
//
// OR
//
//   Step 2/7 : RUN apt-get update && apt-get upgrade -y && apt-get install -y   git   python    curl
//   ---> Running in 9402d36e7474
//   Step 3/7 : RUN mkdir -p --mode=0777 /workspace/__cache
//   Step 5/7 : ENV CIPD_CACHE_DIR /workspace/__cache
//   Step 6/7 : USER skia
func Build(ctx context.Context, args ...string) error {
	return log_parser.RunRegexp(ctx, dockerStepRegex, ".", append([]string{dockerCmd}, args...))
}

// BuildPushImageFromInfraImage is a utility function that pulls the infra image, runs the specified
// buildCmd on the infra image, builds the specified image+tag, pushes it. After pushing it sends
// a pubsub msg signaling completion.
func BuildPushImageFromInfraImage(ctx context.Context, appName, image, tag, repo, configDir, workDir, infraImageTag string, topic *pubsub.Topic, cmd, volumes, env []string, buildArgs map[string]string) error {
	err := td.Do(ctx, td.Props(fmt.Sprintf("Build & Push %s Image", appName)).Infra(), func(ctx context.Context) error {

		// Make sure we have the specified infra image.
		infraImageWithTag := fmt.Sprintf("gcr.io/skia-public/infra:%s", infraImageTag)
		if err := Pull(ctx, infraImageWithTag, configDir); err != nil {
			return err
		}
		// Create the image locally using infraImageWithTag.
		if err := Run(ctx, infraImageWithTag, configDir, cmd, volumes, env); err != nil {
			return err
		}
		// Build the image using docker.
		imageWithTag := fmt.Sprintf("%s:%s", image, tag)
		if err := BuildHelper(ctx, workDir, imageWithTag, configDir, buildArgs); err != nil {
			return err
		}
		// Push the docker image.
		if _, err := Push(ctx, imageWithTag, configDir); err != nil {
			return err
		}
		// Send pubsub msg.
		return publishToTopic(ctx, image, tag, repo, topic)
	})
	return err
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
