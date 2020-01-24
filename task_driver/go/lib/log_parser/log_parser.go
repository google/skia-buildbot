package log_parser

import (
	"bufio"
	"context"
	"os/exec"
	"strings"
	"sync"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_driver/go/td"
)

// Run runs the given command in the given working directory. It calls the
// provided function to emit sub-steps.
func Run(ctx context.Context, cwd string, cmdLine []string, split bufio.SplitFunc, handleToken func(context.Context, string) error, cleanup func(context.Context) error) error {
	ctx = td.StartStep(ctx, td.Props(strings.Join(cmdLine, " ")))
	defer td.EndStep(ctx)

	// Set up the command.
	cmd := exec.CommandContext(ctx, cmdLine[0], cmdLine[1:]...)
	cmd.Dir = cwd
	cmd.Env = td.GetEnv(ctx)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return td.FailStep(ctx, err)
	}
	if err := cmd.Start(); err != nil {
		return td.FailStep(ctx, err)
	}

	// Spin up a goroutine which parses the output of the command and
	// creates sub-steps.
	var wg sync.WaitGroup
	wg.Add(1)

	// runErr records any errors that occur within the goroutine.
	var runErr error

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		scanner.Split(split)
		for scanner.Scan() {
			token := scanner.Text()
			if err := handleToken(ctx, token); err != nil {
				runErr = skerr.Wrapf(err, "Failed handling token %q", token)
				sklog.Error(runErr.Error())
			}
		}
		if cleanup != nil {
			if err := cleanup(ctx); err != nil {
				runErr = skerr.Wrapf(err, "Failed during cleanup")
				sklog.Error(runErr.Error())
			}
		}
	}()

	// Wait for the command to finish.
	if err := cmd.Wait(); err != nil {
		// Wait for log processing goroutine to finish.
		wg.Wait()
		return td.FailStep(ctx, err)
	}

	// Wait for log processing goroutine to finish.
	wg.Wait()
	if runErr != nil {
		return td.FailStep(ctx, runErr)
	}
	return nil
}
