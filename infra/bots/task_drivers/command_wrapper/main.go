package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	osexec "os/exec"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_driver/go/lib/auth_steps"
	"go.skia.org/infra/task_driver/go/lib/cas"
	"go.skia.org/infra/task_driver/go/lib/cipd"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required properties for this task.
	projectId       = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskId          = flag.String("task_id", "", "ID of this task.")
	taskName        = flag.String("task_name", "", "Name of the task.")
	workdir         = flag.String("workdir", ".", "Working directory")
	cmdIsTaskDriver = flag.Bool("command-is-task-driver", false, "True if the provided command is a task driver.")

	casFlags  = cas.SetupFlags(nil)
	cipdFlags = cipd.SetupFlags(nil)

	// Optional flags.
	local  = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

func main() {
	// Setup.
	var subCommand []string
	for idx, arg := range os.Args {
		if arg == "--" {
			if len(os.Args) >= idx {
				subCommand = os.Args[idx+1:]
			}
			os.Args = os.Args[:idx]
			break
		}
	}
	if len(subCommand) == 0 {
		sklog.Fatalf("Expected subcommand after \"--\"")
	}
	ctx := td.StartRun(projectId, taskId, taskName, output, local)
	defer td.EndRun(ctx)

	// Download inputs for the task.
	if err := td.Do(ctx, td.Props("Setup").Infra(), func(ctx context.Context) error {
		client, ts, err := auth_steps.InitHttpClient(ctx, *local, auth.ScopeUserinfoEmail)
		if err != nil {
			return err
		}
		wd, err := os_steps.Abs(ctx, *workdir)
		if err != nil {
			return err
		}
		if err := cipd.EnsureFromFlags(ctx, client, wd, cipdFlags); err != nil {
			return err
		}
		if err := cas.DownloadFromFlags(ctx, wd, ts, casFlags); err != nil {
			return err
		}
		return nil
	}); err != nil {
		td.Fatal(ctx, err)
	}

	// Run the requested command.
	cmd := &exec.Command{
		Name:       subCommand[0],
		Args:       subCommand[1:],
		InheritEnv: true,
	}
	var runErr error
	if *cmdIsTaskDriver {
		// If the wrapped command is a task driver, it will generate its own
		// steps. Use the built-in os/exec package to run the command so that we
		// don't generate an unnecessary step for the subprocess.
		osCmd := osexec.CommandContext(ctx, cmd.Name, cmd.Args...)
		osCmd.Env = td.MergeEnv(os.Environ(), []string{fmt.Sprintf("%s=%s", td.EnvVarWrappedStepID, td.StepIDRoot)})
		runErr = osCmd.Run()
	} else {
		_, runErr = exec.RunCommand(ctx, cmd)
	}
	if runErr != nil {
		td.Fatal(ctx, runErr)
	}

	// Upload outputs from the task.
	if err := td.Do(ctx, td.Props("Teardown").Infra(), func(ctx context.Context) error {
		// TODO(borenet): Upload CAS outputs.
		return nil
	}); err != nil {
		td.Fatal(ctx, err)
	}
}
