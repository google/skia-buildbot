// Package docker is for running Dockerfiles.
package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"cloud.google.com/go/pubsub"

	docker_pubsub "go.skia.org/infra/go/docker/build/pubsub"
	sk_exec "go.skia.org/infra/go/exec"
	"go.skia.org/infra/task_driver/go/lib/log_parser"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// dockerStepPrefix is a regex that matches Step lines in Docker output.
	dockerStepPrefix = regexp.MustCompile(`^Step \d+\/\d+ : `)

	// dockerCmd is the name of the executable to run Docker. A variable so we
	// can change it at test time.
	dockerCmd = "docker"
)

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

// Push a Docker file.
func Push(ctx context.Context, tag, configDir string) error {
	pushCmd := fmt.Sprintf("%s --config %s push %s", dockerCmd, configDir, tag)
	_, err := sk_exec.RunSimple(ctx, pushCmd)
	if err != nil {
		return err
	}
	return nil
}

// Run does a "docker run".
//
// volumes should be in the form of "ARG1:ARG2" where ARG1 is the local directory and ARG2 will be the directory in the image.
// Note the above does a --rm i.e. it automatically removes the container when it exits.
func Run(ctx context.Context, image, cmd, configDir string, volumes []string, env map[string]string) error {
	runArgs := []string{"--config", configDir, "run", "--rm"}
	for _, v := range volumes {
		runArgs = append(runArgs, "--volume", v)
	}
	if env != nil {
		for k, v := range env {
			runArgs = append(runArgs, "--env", fmt.Sprintf("%s=%s", k, v))
		}
	}
	runArgs = append(runArgs, image, "sh", "-c", cmd)
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
func Build(ctx context.Context, directory, tag, configDir string, buildArgs map[string]string) error {
	// Runs "docker build -t <some tag name> ." in 'directory' and streams the
	// output.

	// Parse the output of the build.
	//   Parse lines that start with "Step N/M : ACTION value"
	//
	// Examples:
	//   Step 1/7 : FROM debian:testing-slim
	//   ---> e205e0c9e7f5

	//   Step 2/7 : RUN apt-get update && apt-get upgrade -y && apt-get install -y   git   python    curl
	//   ---> Using cache
	//   ---> 5b8240d40b63

	// OR

	//   Step 2/7 : RUN apt-get update && apt-get upgrade -y && apt-get install -y   git   python    curl
	//   ---> Running in 9402d36e7474

	//   Step 3/7 : RUN mkdir -p --mode=0777 /workspace/__cache
	//   Step 5/7 : ENV CIPD_CACHE_DIR /workspace/__cache
	//   Step 6/7 : USER skia

	cmdArgs := []string{dockerCmd, "--config", configDir, "build", "--pull", "--no-cache", "-t", tag, "."}
	if buildArgs != nil {
		for k, v := range buildArgs {
			cmdArgs = append(cmdArgs, "--build-arg", fmt.Sprintf("%s=%s", k, v))
		}
	}
	var currentStep context.Context = nil
	var currentStepLogs io.Writer
	var parentStepLogs io.Writer
	return log_parser.Run(ctx, directory, cmdArgs, bufio.ScanLines, func(ctx context.Context, line string) error {
		// If this matches the regex then StartStep, EndStep the last step,
		// and create a new associated log for the new step.
		if dockerStepPrefix.MatchString(line) {
			if currentStep != nil {
				td.EndStep(currentStep)
			}
			currentStep = td.StartStep(ctx, td.Props(line))
			currentStepLogs = td.NewLogStream(currentStep, line, td.Info)
		} else {
			// If there is no active sub-step, write the log to the
			// parent step.
			var logs io.Writer
			if currentStepLogs == nil {
				if parentStepLogs == nil {
					parentStepLogs = td.NewLogStream(ctx, "docker", td.Info)
				}
				logs = parentStepLogs
			} else {
				logs = currentStepLogs
			}
			if _, err := logs.Write([]byte(line)); err != nil {
				return err
			}
		}
		return nil
	}, func(ctx context.Context) error {
		// Now that we've processed all output, End the current step.
		if currentStep != nil {
			td.EndStep(currentStep)
		}
		return nil
	})
}

// BuildPushImageFromInfraImage is a utility function that pulls the infra image, runs the specified
// buildCmd on the infra image, builds the specified image+tag, pushes it. After pushing it sends
// a pubsub msg signaling completion.
func BuildPushImageFromInfraImage(ctx context.Context, appName, buildCmd, image, tag, repo, configDir, workDir, infraImageTag string, topic *pubsub.Topic, volumes []string, env, buildArgs map[string]string) error {
	err := td.Do(ctx, td.Props(fmt.Sprintf("Build & Push %s Image", appName)).Infra(), func(ctx context.Context) error {

		// Make sure we have the specified infra image.
		infraImageWithTag := fmt.Sprintf("gcr.io/skia-public/infra:%s", infraImageTag)
		if err := Pull(ctx, infraImageWithTag, configDir); err != nil {
			return err
		}
		// Create the image locally using infraImageWithTag.
		if err := Run(ctx, infraImageWithTag, buildCmd, configDir, volumes, env); err != nil {
			return err
		}
		// Build the image using docker.
		imageWithTag := fmt.Sprintf("%s:%s", image, tag)
		if err := Build(ctx, workDir, imageWithTag, configDir, buildArgs); err != nil {
			return err
		}
		// Push the docker image.
		if err := Push(ctx, imageWithTag, configDir); err != nil {
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
