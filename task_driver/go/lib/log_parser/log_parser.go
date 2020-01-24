package log_parser

import (
	"bufio"
	"context"
	"io"
	"strings"
	"sync"

	"go.skia.org/infra/go/exec"
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
	stdoutR, stdoutW := io.Pipe()
	cmd := exec.Command{
		Name:   cmdLine[0],
		Args:   cmdLine[1:],
		Dir:    cwd,
		Env:    td.GetEnv(ctx),
		Stdout: stdoutW,
	}

	// Spin up a goroutine which parses the JSON output of "go test" and
	// creates sub-steps.
	var wg sync.WaitGroup
	wg.Add(1)

	// runErr records any errors that occur within the goroutine.
	var runErr error

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdoutR)
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
	_, err := exec.RunCommand(ctx, &cmd)
	closeErr := stdoutW.Close()
	// Wait for log processing goroutine to finish.
	wg.Wait()
	if err != nil {
		return td.FailStep(ctx, err)
	}
	if closeErr != nil {
		return td.FailStep(ctx, closeErr)
	}
	if runErr != nil {
		return td.FailStep(ctx, runErr)
	}
	return nil
}
