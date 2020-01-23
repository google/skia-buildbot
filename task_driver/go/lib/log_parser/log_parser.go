package log_parser

import (
	"bufio"
	"context"
	"os/exec"
	"strings"
	"sync"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_driver/go/td"
)

// LogParser runs a command as a step, parsing the output of the command to
// create sub-steps.
type LogParser struct {
	handleToken func(context.Context, string) error
	split       bufio.SplitFunc
}

// NewLogParser returns a LogParser which uses the given function to parse logs.
// If handleFunction returns an error, the Run step will fail but parsing will
// not stop.
func NewLogParser(split bufio.SplitFunc, handleToken func(context.Context, string) error) *LogParser {
	return &LogParser{
		handleToken: handleToken,
		split:       split,
	}
}

// Run runs the given command in the given working directory. It calls the
// provided function to emit sub-steps.
func Run(ctx context.Context, cwd string, cmdLine []string, split bufio.SplitFunc, handleToken func(context.Context, string) error) error {
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

	// Spin up a goroutine which parses the JSON output of "go test" and
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
			if err := handleToken(ctx, scanner.Text()); err != nil {
				runErr = err
				sklog.Errorf("Failed handling token: %s", err)
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
