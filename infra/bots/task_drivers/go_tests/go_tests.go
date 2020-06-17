package main

import (
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

/*
	Run the provided Golang test executables.
*/

var (
	// Required properties for this task.
	projectID = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskID    = flag.String("task_id", "", "ID of this task.")
	taskName  = flag.String("task_name", "", "Name of the task.")
	testDir   = flag.String("test-dir", ".", "Directory containing test executables.")
	workers   = flag.Int("n", runtime.GOMAXPROCS(-1), "Number of test-executing workers to run.")

	// Optional flags.
	local  = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

func main() {
	// Setup.
	ctx := td.StartRun(projectID, taskID, taskName, output, local)
	defer td.EndRun(ctx)
	testDirAbs, err := os_steps.Abs(ctx, *testDir)
	if err != nil {
		td.Fatal(ctx, err)
	}

	// TODO(borenet): How do we reconcile this with golang.Test()?

	// Set up a worker pool.
	var wg sync.WaitGroup
	testCh := make(chan string)
	errCh := make(chan error)
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for testExec := range testCh {
				// TODO(borenet): Test flags?
				if _, err := exec.RunCwd(ctx, ".", testExec); err != nil {
					errCh <- err
				}
			}
		}()
	}

	// Search the test directory, pass any executables along the channel.
	if err := filepath.Walk(testDirAbs, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.Mode().IsRegular() && fi.Mode().Perm()&0111 != 0 {
			testCh <- path
		}
		return nil
	}); err != nil {
		close(testCh)
		td.Fatal(ctx, err)
	}
	close(testCh)

	// Collect the errors.
	var errs []error
	go func() {
		for err := range errCh {
			errs = append(errs, err)
		}
	}()

	// Wait for the workers to finish.
	wg.Wait()
	if len(errs) > 0 {
		td.Fatalf(ctx, "Tests Failed")
	}
}
