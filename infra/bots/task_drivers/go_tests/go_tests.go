package main

import (
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"go.skia.org/infra/task_driver/go/lib/golang"
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

	// TODO(borenet): Test arguments, eg. --short.
	testArgs := []string{}

	// Set up a worker pool.
	var wg sync.WaitGroup
	type test struct {
		pkgName string
		path    string
		err     error
	}
	testCh := make(chan *test)
	resultCh := make(chan *test)
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range testCh {
				// TODO(borenet): Test flags?
				t.err = golang.TestExecutable(ctx, t.path, t.pkgName, testArgs...)
				resultCh <- t
			}
		}()
	}

	// Search the test directory, pass any executables along the channel.
	if err := filepath.Walk(testDirAbs, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.Mode().IsRegular() && fi.Mode().Perm()&0111 != 0 {
			pkg := strings.TrimSuffix(path, ".exe")
			pkg = strings.TrimSuffix(path, ".test")
			pkg = strings.TrimPrefix(pkg, testDirAbs)
			testCh <- &test{
				pkgName: pkg,
				path:    path,
			}
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
		for t := range resultCh {
			if t.err != nil {
				errs = append(errs, t.err)
			}
		}
	}()

	// Wait for the workers to finish.
	wg.Wait()
	if len(errs) > 0 {
		td.Fatalf(ctx, "Tests Failed")
	}
}
