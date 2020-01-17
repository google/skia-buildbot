// Package docker is for running Dockerfiles.
package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"golang.org/x/oauth2"

	"go.skia.org/infra/go/auth"
	docker_pubsub "go.skia.org/infra/go/docker/build/pubsub"
	sk_exec "go.skia.org/infra/go/exec"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	AUTH_SCOPES = []string{auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_FULL_CONTROL}

	REPOSITORY_HOST = "gcr.io"

	// dockerStepPrefix is a regex that matches Step lines in Docker output.
	dockerStepPrefix = regexp.MustCompile(`^Step \d+\/\d+ : `)

	// dockerCmd is the name of the executable to run Docker. A variable so we
	// can change it at test time.
	dockerCmd = "docker"
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
func (d *Docker) Push(ctx context.Context, tag string) error {
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
func (d *Docker) Run(ctx context.Context, image string, cmd, volumes []string, env map[string]string) error {
	return Run(ctx, image, d.configDir, cmd, volumes, env)
}

// Run "docker build" with the given args.
func (d *Docker) Build(ctx context.Context, args ...string) error {
	return Build(ctx, append([]string{"--config", d.configDir, "build"}, args...)...)
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
func Push(ctx context.Context, tag, configDir string) error {
	pushCmd := fmt.Sprintf("%s --config %s push %s", dockerCmd, configDir, tag)
	_, err := sk_exec.RunSimple(ctx, pushCmd)
	if err != nil {
		return err
	}
	return nil
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
func Run(ctx context.Context, image, configDir string, cmd, volumes []string, env map[string]string) error {
	runArgs := []string{"--config", configDir, "run"}
	for _, v := range volumes {
		runArgs = append(runArgs, "--volume", v)
	}
	if env != nil {
		for k, v := range env {
			runArgs = append(runArgs, "--env", fmt.Sprintf("%s=%s", k, v))
		}
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
	cmdArgs := []string{"--config", configDir, "build", "--pull", "--no-cache", "-t", tag, directory}
	if buildArgs != nil {
		for k, v := range buildArgs {
			cmdArgs = append(cmdArgs, "--build-arg", fmt.Sprintf("%s=%s", k, v))
		}
	}
	return Build(ctx, cmdArgs...)
}

// Run "docker build" with the given arguments.
func Build(ctx context.Context, args ...string) error {
	ctx = td.StartStep(ctx, td.Props(fmt.Sprintf("docker %s", strings.Join(args, " "))))
	defer td.EndStep(ctx)

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
	fmt.Println(fmt.Sprintf("Running: docker %s", strings.Join(args, " ")))
	cmd := exec.CommandContext(ctx, dockerCmd, args...)
	cmd.Env = append(cmd.Env, td.GetEnv(ctx)...)
	cmd.Stderr = os.Stderr

	stdOut, err := cmd.StdoutPipe()
	if err != nil {
		return td.FailStep(ctx, err)
	}
	if err := cmd.Start(); err != nil {
		return td.FailStep(ctx, err)
	}
	logStream := td.NewLogStream(ctx, "docker", td.Info)
	scanner := bufio.NewScanner(stdOut)

	// Spin up a Go routine to parse the step output of the Docker build and
	// funnel that into the right logs.

	// wg let's us wait for the Go routine to finish.
	var wg sync.WaitGroup
	wg.Add(1)

	// logStreamError records any errors that occur writing to the logStream.
	var logStreamError error = nil

	go func() {
		var subStepContext context.Context = nil
		for scanner.Scan() {
			line := scanner.Text()
			// If this matches the regex then StartStep, EndStep the last step,
			// and create a new associated log for the new step.
			if dockerStepPrefix.MatchString(line) {
				if subStepContext != nil {
					td.EndStep(subStepContext)
				}
				subStepContext = td.StartStep(ctx, td.Props(line))
				logStream = td.NewLogStream(subStepContext, line, td.Info)
			} else {
				// Otherwise just write the log line to the current logStream.
				if _, err := logStream.Write([]byte(line)); err != nil {
					logStreamError = err
				}
			}
		}
		// Now that we've processed all output, End the current step.
		if subStepContext != nil {
			td.EndStep(subStepContext)
		}
		wg.Done()
	}()

	// Wait for command to finish.
	if err := cmd.Wait(); err != nil {
		// Wait for log processing Go routine to finish.
		wg.Wait()
		return td.FailStep(ctx, err)
	}

	// Wait for log processing Go routine to finish.
	wg.Wait()

	if logStreamError != nil {
		return td.FailStep(ctx, logStreamError)
	}

	return nil
}

// BuildPushImageFromInfraImage is a utility function that pulls the infra image, runs the specified
// buildCmd on the infra image, builds the specified image+tag, pushes it. After pushing it sends
// a pubsub msg signaling completion.
func BuildPushImageFromInfraImage(ctx context.Context, appName, image, tag, repo, configDir, workDir, infraImageTag string, topic *pubsub.Topic, buildCmd, volumes []string, env, buildArgs map[string]string) error {
	err := td.Do(ctx, td.Props(fmt.Sprintf("Build & Push %s Image", appName)).Infra(), func(ctx context.Context) error {

		// Make sure we have the specified infra image.
		infraImageWithTag := fmt.Sprintf("gcr.io/skia-public/infra:%s", infraImageTag)
		if err := Pull(ctx, infraImageWithTag, configDir); err != nil {
			return err
		}
		// Create the image locally using infraImageWithTag.
		if err := Run(ctx, infraImageWithTag, configDir, buildCmd, volumes, env); err != nil {
			return err
		}
		// Build the image using docker.
		imageWithTag := fmt.Sprintf("%s:%s", image, tag)
		if err := BuildHelper(ctx, workDir, imageWithTag, configDir, buildArgs); err != nil {
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
