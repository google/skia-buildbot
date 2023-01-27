package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_driver/go/lib/auth_steps"
	"go.skia.org/infra/task_driver/go/lib/cas"
	"go.skia.org/infra/task_driver/go/lib/cipd"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
	"go.skia.org/infra/task_scheduler/go/types"
	"golang.org/x/oauth2"
)

var (
	// Required properties for this task.
	projectId       = flag.String("project_id", "", "ID of the Google Cloud project.")
	wd              = flag.String("workdir", ".", "Working directory")
	cmdIsTaskDriver = flag.Bool("command-is-task-driver", false, "True if the provided command is a task driver.")

	// input and output replace most of the below flags.
	input  = flag.String("input", "", "Path to a JSON file containing a TaskRequest struct.")
	output = flag.String("output", "", "Path to a JSON file to write the TaskResult struct.")

	// These are not required if --input is used.
	taskId    = flag.String("task_id", "", "ID of this task.")
	taskName  = flag.String("task_name", "", "Name of the task.")
	casFlags  = cas.SetupFlags(nil)
	cipdFlags = cipd.SetupFlags(nil)

	// Optional flags.
	local      = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	jsonOutput = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

func main() {
	// Parse flags and read TaskRequest.
	startTs := time.Now()
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
	flag.Parse()
	var req types.TaskRequest
	if *input != "" {
		if err := util.WithReadFile(*input, func(f io.Reader) error {
			return json.NewDecoder(f).Decode(&req)
		}); err != nil {
			sklog.Fatal(err)
		}
	} else {
		req.Command = subCommand

		cipdPkgs, err := cipd.GetPackages(cipdFlags)
		if err != nil {
			sklog.Fatal(err)
		}
		req.CipdPackages = cipdPkgs

		casDownloads, err := cas.GetCASDownloads(casFlags)
		if err != nil {
			sklog.Fatal(err)
		}
		if len(casDownloads) > 1 {
			sklog.Fatalf("Only one CAS digest is supported; found %d", len(casDownloads))
		}
		if len(casDownloads) > 0 {
			req.CasInput = casDownloads[0].Digest
		}

		req.Name = *taskName
		req.TaskSchedulerTaskID = *taskId
	}
	if len(req.Command) == 0 {
		sklog.Fatalf("Expected subcommand as part of TaskRequest or after \"--\"")
	}

	// Start up the Task Driver framework.
	ctx := td.StartRun(projectId, &req.TaskSchedulerTaskID, &req.Name, jsonOutput, local)
	defer td.EndRun(ctx)

	// Setup.
	var ts oauth2.TokenSource
	var workdir string
	if err := td.Do(ctx, td.Props("Setup").Infra(), func(ctx context.Context) error {
		// Create/cleanup the working directory.
		var err error
		workdir, err = os_steps.Abs(ctx, *wd)
		if err != nil {
			return err
		}
		if err := os_steps.RemoveAll(ctx, workdir); err != nil {
			return err
		}
		if err := os_steps.MkdirAll(ctx, workdir); err != nil {
			return err
		}

		// Download CIPD and CAS inputs.
		client, tokenSource, err := auth_steps.InitHttpClient(ctx, *local, auth.ScopeUserinfoEmail)
		if err != nil {
			return err
		}
		ts = tokenSource
		if err := cipd.Ensure(ctx, client, workdir, req.CipdPackages...); err != nil {
			return err
		}
		if err := cas.Download(ctx, workdir, *casFlags.Instance, ts, &cas.CASDownload{
			Path:   ".",
			Digest: req.CasInput,
		}); err != nil {
			return err
		}
		return nil

		// TODO(borenet): Handle TaskRequest.Caches.

	}); err != nil {
		td.Fatal(ctx, err)
	}

	// Create the environment for the sub-command.
	envMap := make(map[string]string, len(req.Env)+len(req.EnvPrefixes))
	for k, v := range req.Env {
		envMap[k] = v
	}
	for k, prefixes := range req.EnvPrefixes {
		vals := make([]string, 0, len(prefixes))
		if v, ok := envMap[k]; ok {
			vals = append(vals, v)
		}
		for _, prefix := range prefixes {
			vals = append(vals, filepath.Join(workdir, prefix))
		}
		envMap[k] = strings.Join(vals, string(os.PathSeparator))
	}
	env := make([]string, 0, len(envMap))
	for k, v := range envMap {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Run the requested command.
	cmd := &exec.Command{
		Name:       subCommand[0],
		Args:       subCommand[1:],
		InheritEnv: len(env) == 0,
		Env:        env,
		Timeout:    req.ExecutionTimeout,
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

	// Clean up after the task.
	if err := td.Do(ctx, td.Props("Teardown").Infra(), func(ctx context.Context) error {
		// Upload CAS outputs. Note that we do this regardless of whether the
		// sub-command succeeded.
		// TODO(borenet): Should we provide a pathway for CAS exclusions?
		casOutput, err := cas.Upload(ctx, workdir, *casFlags.Instance, ts, req.Outputs, nil)
		if err != nil {
			return err
		}

		// Write the TaskResult.
		status := types.TASK_STATUS_SUCCESS
		if runErr != nil {
			// TODO(borenet): We need to determine whether the sub-command
			// failed with a normal or infra error. I'm not sure of the best
			// way to do this; in the past other systems have used a designated
			// exit code to specify an infra error.
			status = types.TASK_STATUS_FAILURE
		}
		// We're just echoing the requested tags back. Because we won't be using
		// a separate DB, we have no need for tags for searching the DB.
		tags := make(map[string][]string, len(req.Tags))
		for _, tag := range req.Tags {
			split := strings.SplitN(tag, ":", 2)
			if len(split) == 2 {
				tags[split[0]] = []string{split[1]}
			}
		}
		result := types.TaskResult{
			CasOutput: casOutput,
			// TODO(borenet): The separate Created and Started timestamps are
			// relics of Swarming, where we'd request a task and it would be
			// Created when Swarming inserted it into its DB but would not be
			// Started until the task was matched to a machine and actually
			// began running. I don't know that we need that distinction in the
			// new world, or at least we may not need to obtain that information
			// from the TaskExecutor. Instead, we can just use the timestamp at
			// which Task Scheduler send the TaskRequest.
			Created:  time.Time{},
			Finished: time.Now(),
			ID:       req.TaskSchedulerTaskID,
			Started:  startTs,
			Status:   status,
			Tags:     tags,
		}
		b, err := json.Marshal(&result)
		if err != nil {
			return err
		}
		if err := os_steps.WriteFile(ctx, *output, b, os.ModePerm); err != nil {
			return err
		}

		return nil
	}); err != nil {
		td.Fatal(ctx, err)
	}

	// Exit according to the success/failure status of the sub-command.
	if runErr != nil {
		td.Fatal(ctx, runErr)
	}
}
