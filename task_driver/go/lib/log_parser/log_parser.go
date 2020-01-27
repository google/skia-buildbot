package log_parser

import (
	"bufio"
	"context"
	"os/exec"
	"strings"

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

	// parseErr records any errors that occur while parsing output.
	var parseErr error

	// Parse the output of the command and create sub-steps.
	scanner := bufio.NewScanner(stdout)
	scanner.Split(split)
	for scanner.Scan() {
		token := scanner.Text()
		if err := handleToken(ctx, token); err != nil {
			parseErr = skerr.Wrapf(err, "Failed handling token %q", token)
			sklog.Error(parseErr.Error())
		}
	}
	if cleanup != nil {
		if err := cleanup(ctx); err != nil {
			parseErr = skerr.Wrapf(err, "Failed during cleanup")
			sklog.Error(parseErr.Error())
		}
	}

	// Wait for the command to finish.
	if err := cmd.Wait(); err != nil {
		return td.FailStep(ctx, err)
	}
	if parseErr != nil {
		return td.FailStep(ctx, parseErr)
	}
	return nil
}
