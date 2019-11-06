// Package docker is for running Dockerfiles.
package docker

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sync"

	"go.skia.org/infra/task_driver/go/td"
)

var (
	// dockerStepPrefix is a regex that matches Step lines in Docker output.
	dockerStepPrefix = regexp.MustCompile(`^Step \d+\/\d+ : `)

	// dockerCmd is the name of the executable to run Docker. A variable so we
	// can change it at test time.
	dockerCmd = "docker"
)

// Build a Dockerfile.
//
// There must be a Dockerfile in the 'directory' and the resulting output is
// tagged with 'tag'.
func Build(ctx context.Context, directory string, tag string) error {
	ctx = td.StartStep(ctx, td.Props(fmt.Sprintf("docker build -t %s %s", tag, directory)))
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

	cmd := exec.CommandContext(ctx, dockerCmd, "build", "-t", tag, directory)
	cmd.Dir = directory
	cmd.Env = append(cmd.Env, td.GetEnv(ctx)...)

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
