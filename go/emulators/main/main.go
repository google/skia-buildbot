package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"go.skia.org/infra/go/emulators"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required properties for this task.
	projectID = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskID    = flag.String("task_id", "", "ID of this task.")
	taskName  = flag.String("task_name", "", "Name of the task.")

	// Optional flags.
	local  = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

func main() {
	ctx := td.StartRun(projectID, taskID, taskName, output, local)
	defer td.EndRun(ctx)

	err := td.Do(ctx, td.Props("Start all emulators"), func(ctx context.Context) error {
		if err := emulators.StartAllEmulators(); err != nil {
			return err
		}
		//defer func() {
		//	if err := emulators.StopAllEmulators(); err != nil {
		//		td.Fatal(ctx, err)
		//	}
		//}()
		time.Sleep(5 * time.Second) // Give emulators time to boot.
		return nil
	})
	if err != nil {
		td.Fatal(ctx, err)
	}

	//if err := emulators.StartAllEmulators(); err != nil {
	//	return err
	//}
	//defer func() {
	//	if err := emulators.StopAllEmulators(); err != nil {
	//		rvErr = err
	//	}
	//}()
	//time.Sleep(5 * time.Second) // Give emulators time to boot.

	err = td.Do(ctx, td.Props("SLEEPING FOR 60 MINUTES"), func(ctx context.Context) error {
		fmt.Println("************** SLEEPING FOR 60 MINUTES *******************")
		time.Sleep(1 * time.Hour)
		return nil
	})
	if err != nil {
		td.Fatal(ctx, err)
	}
}
