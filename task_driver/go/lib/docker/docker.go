// Package docker is for running Dockerfiles.
package docker

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	sk_exec "go.skia.org/infra/go/exec"
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
	ctx = td.StartStep(ctx, td.Props(fmt.Sprintf("docker build --pull --no-cache -t %s %s", tag, directory)))
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

	cmdArgs := []string{"--config", configDir, "build", "--pull", "--no-cache", "-t", tag, "."}
	if buildArgs != nil {
		for k, v := range buildArgs {
			cmdArgs = append(cmdArgs, "--build-arg", fmt.Sprintf("%s=%s", k, v))
		}
	}

	cmd := exec.CommandContext(ctx, dockerCmd, cmdArgs...)
	cmd.Dir = directory
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
