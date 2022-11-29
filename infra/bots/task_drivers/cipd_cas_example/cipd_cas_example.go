package main

import (
	"flag"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/task_driver/go/lib/auth_steps"
	"go.skia.org/infra/task_driver/go/lib/cas"
	"go.skia.org/infra/task_driver/go/lib/cipd"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required properties for this task.
	projectId = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskId    = flag.String("task_id", "", "ID of this task.")
	taskName  = flag.String("task_name", "", "Name of the task.")
	workdir   = flag.String("workdir", ".", "Working directory")

	casFlags  = cas.SetupFlags(nil)
	cipdFlags = cipd.SetupFlags(nil)

	// Optional flags.
	local  = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

func main() {
	// Setup.
	ctx := td.StartRun(projectId, taskId, taskName, output, local)
	defer td.EndRun(ctx)

	client, ts, err := auth_steps.InitHttpClient(ctx, *local, auth.ScopeUserinfoEmail)
	if err != nil {
		td.Fatal(ctx, err)
	}
	wd, err := os_steps.Abs(ctx, *workdir)
	if err != nil {
		td.Fatal(ctx, err)
	}
	if err := cipd.EnsureFromFlags(ctx, client, wd, cipdFlags); err != nil {
		td.Fatal(ctx, err)
	}
	if err := cas.DownloadFromFlags(ctx, wd, ts, casFlags); err != nil {
		td.Fatal(ctx, err)
	}
}
