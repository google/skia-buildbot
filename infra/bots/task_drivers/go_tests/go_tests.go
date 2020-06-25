package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	skexec "go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_driver/go/lib/golang"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

/*
	Run the provided Golang test executables.
*/

var (
	// Required properties for this task.
	projectID   = flag.String("project-id", "", "ID of the Google Cloud project.")
	taskID      = flag.String("task-id", "", "ID of this task.")
	taskName    = flag.String("task-name", "", "Name of the task.")
	testDir     = flag.String("test-dir", ".", "Directory containing test executables.")
	testDataDir = flag.String("test-data-dir", ".", "Directory containing test data.")
	workers     = flag.Int("n", runtime.GOMAXPROCS(-1), "Number of test-executing workers to run.")

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

	// Start the various emulators and set up their associated environment
	// variables.
	var emulatorProcess *os.Process
	if err := td.Do(ctx, td.Props("Start emulators"), func(ctx context.Context) error {
		cmd := exec.CommandContext(ctx, "run_emulators")
		cmd.Stdout = td.NewLogStream(ctx, "stdout", td.Info)
		cmd.Stderr = td.NewLogStream(ctx, "stderr", td.Error)
		if err := cmd.Start(); err != nil {
			return err
		}
		log := func(msg string, args ...interface{}) {
			msg = fmt.Sprintf(msg, args...)
			ts := time.Now().Format("15:04:05Z07:00")
			_, _ = cmd.Stdout.Write([]byte(fmt.Sprintf("[%s] %s", ts, msg)))
		}
		emulatorProcess = cmd.Process
		// Wait for the emulators to be ready.
		// TODO(borenet): Deduplicate.
		waitingFor := util.NewStringSet([]string{
			"localhost:8891",
			"localhost:8892",
			"localhost:8893",
			"localhost:8894",
			"localhost:8895",
		})
		const timeout = 30 * time.Second
		log("Emulators started; waiting up to %s for them to be available.", timeout)
		failAfter := time.Now().Add(timeout)
		for {
			output, err := skexec.RunCwd(ctx, ".", "netstat", "tulp")
			if err != nil {
				return err
			}
			log(output)
			for wf := range waitingFor {
				if strings.Contains(output, wf) {
					log(wf + " is ready")
					delete(waitingFor, wf)
				}
			}
			if len(waitingFor) == 0 {
				log("All emulators are ready.")
				break
			} else if time.Now().After(failAfter) {
				err := fmt.Errorf("Emulators failed to start within %s", timeout)
				log(err.Error())
				return err
			} else {
				log("Waiting for:\n%s", strings.Join(waitingFor.Keys(), "\n"))
				time.Sleep(time.Second)
			}
		}
		return nil
	}); err != nil {
		td.Fatal(ctx, err)
	}
	defer func() {
		if err := td.Do(ctx, td.Props("Stop emulators"), func(ctx context.Context) error {
			return emulatorProcess.Kill()
		}); err != nil {
			td.Fatal(ctx, err)
		}
	}()
	// TODO(borenet): Deduplicate this with scripts/run_emulators.go.
	emulatorEnv := []string{
		"DATASTORE_EMULATOR_HOST=localhost:8891",
		"BIGTABLE_EMULATOR_HOST=localhost:8892",
		"PUBSUB_EMULATOR_HOST=localhost:8893",
		"FIRESTORE_EMULATOR_HOST=localhost:8894",
		"COCKROACHDB_EMULATOR_HOST=localhost:8895",
	}
	ctx = td.WithEnv(ctx, emulatorEnv)

	// TODO(borenet): Test arguments, eg. --small.
	testArgs := []string{"--small", "--medium", "--large"}

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
				t.err = golang.TestExecutable(ctx, *testDataDir, t.path, t.pkgName, testArgs...)
				resultCh <- t
			}
		}()
	}

	// Search the test directory, pass any executables along the channel.
	var findTestsErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		findTestsErr = td.Do(ctx, td.Props("Find Tests"), func(ctx context.Context) error {
			defer func() {
				close(testCh)
			}()
			return filepath.Walk(testDirAbs, func(path string, fi os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if fi.Mode().IsRegular() && fi.Mode().Perm()&0111 != 0 {
					pkg := strings.TrimSuffix(path, ".exe")
					pkg = strings.TrimSuffix(path, ".test")
					pkg = strings.TrimPrefix(pkg, testDirAbs)
					pkg = strings.TrimPrefix(pkg, "/")
					testCh <- &test{
						pkgName: pkg,
						path:    path,
					}
				}
				return nil
			})
		})
	}()

	// Close the resultCh when the above are finished.
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Collect the errors.
	result := make(chan error)
	go func() {
		var errs []error
		for t := range resultCh {
			if t.err != nil {
				errs = append(errs, t.err)
			}
		}
		if findTestsErr != nil {
			errs = append(errs, findTestsErr)
		}
		if len(errs) > 0 {
			result <- errs[0]
		} else {
			result <- nil
		}
	}()

	// Wait for the workers to finish.
	if err := <-result; err != nil {
		td.Fatal(ctx, err)
	}
}
