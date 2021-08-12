package main

import (
	"flag"

	"go.skia.org/infra/task_driver/go/td"
)

/*
   Task Driver panic example.

   Run like this:

   $ go run ./panic.go --project_id=skia-swarming-bots --task_name=basic_example -o - --local
*/

var (
	// Required flags for all TaskDrivers.
	projectId = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskId    = flag.String("task_id", "", "ID of this task.")
	taskName  = flag.String("task_name", "", "Name of the task.")
	output    = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
	local     = flag.Bool("local", false, "True if running locally (as opposed to in production)")
)

func main() {
	ctx := td.StartRun(projectId, taskId, taskName, output, local)
	defer td.EndRun(ctx)

	panic("this is a panic")
}
