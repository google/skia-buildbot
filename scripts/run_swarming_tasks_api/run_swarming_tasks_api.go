package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	swarmingapi "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
)

/*
	List/Cancel/Retry all swarming tasks that match the specified tags.
*/

const (
	LIST_CMD         = "list"
	CANCEL_CMD       = "cancel"
	RETRY_CMD        = "retry"
	STDOUT_CMD       = "stdout"
	WORKER_POOL_SIZE = 20
	// Number of tasks to display before displaying confirmation of whether to
	// proceed.
	CONFIRMATION_DISPLAY_LIMIT = 5
)

var (
	cmd      = flag.String("cmd", "", "Which swarming operation to use. Eg: list, cancel, retry, stdout.")
	tags     = common.NewMultiStringFlag("tag", nil, "Colon-separated key/value pair, eg: \"runid:testing\" Tags with which to find matching tasks. Can specify multiple times.")
	pool     = flag.String("pool", "", "Which Swarming pool to use.")
	workdir  = flag.String("workdir", ".", "Working directory. Optional, but recommended not to use CWD.")
	state    = flag.String("state", "PENDING", "State the matching tasks should be in. Possible values are: ALL, BOT_DIED, CANCELED, COMPLETED, COMPLETED_FAILURE, COMPLETED_SUCCESS, DEDUPED, EXPIRED, PENDING, PENDING_RUNNING, RUNNING, TIMED_OUT")
	internal = flag.Bool("internal", false, "Run against internal swarming instance.")

	supportedCmds = map[string]bool{
		LIST_CMD:   true,
		CANCEL_CMD: true,
		RETRY_CMD:  true,
		STDOUT_CMD: true,
	}
)

func main() {
	// Setup, parse args.
	common.Init()

	ctx := context.Background()

	if _, ok := supportedCmds[*cmd]; !ok {
		sklog.Fatalf("--cmd must be one of %v", supportedCmds)
	}
	if len(*tags) == 0 {
		sklog.Fatal("Atleast one --tag is required.")
	}
	if *pool == "" {
		sklog.Fatal("--pool is required.")
	}

	var err error
	*workdir, err = filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}

	// Authenticated HTTP client.
	ts, err := auth.NewDefaultTokenSource(true, swarming.AUTH_SCOPE)
	if err != nil {
		sklog.Fatal(err)
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Swarming API client.
	swarmingServer := swarming.SWARMING_SERVER
	if *internal {
		swarmingServer = swarming.SWARMING_SERVER_PRIVATE
	}
	swarmApi, err := swarming.NewApiClient(httpClient, swarmingServer)
	if err != nil {
		sklog.Fatal(err)
	}

	// Obtain the list of tasks.
	tagsWithPool := []string{fmt.Sprintf("pool:%s", *pool)}
	for _, t := range *tags {
		tagsWithPool = append(tagsWithPool, t)
	}
	tasks, err := swarmApi.ListTasks(ctx, time.Time{}, time.Time{}, tagsWithPool, *state)
	if err != nil {
		sklog.Fatal(err)
	}
	if len(tasks) == 0 {
		sklog.Info("Found no matching tasks.")
		return
	}

	if *cmd == LIST_CMD {
		sklog.Infof("Found %d tasks...", len(tasks))
		for _, t := range tasks {
			sklog.Infof("  %s", getTaskStr(t))
		}
		sklog.Infof("Listed %d tasks.", len(tasks))

	} else if *cmd == CANCEL_CMD {
		resp, err := displayTasksAndConfirm(tasks, "canceling")
		if err != nil {
			sklog.Fatal(err)
		}
		if resp {
			sklog.Infof("Starting cancelation of %d tasks...", len(tasks))
			tasksChannel := getClosedTasksChannel(tasks, false /* dedupNames */)
			var wg sync.WaitGroup
			// Loop through workers in the worker pool.
			for i := 0; i < WORKER_POOL_SIZE; i++ {
				// Increment the WaitGroup counter.
				wg.Add(1)
				// Create and run a goroutine closure that cancels tasks.
				go func() {
					// Decrement the WaitGroup counter when the goroutine completes.
					defer wg.Done()

					for t := range tasksChannel {
						if err := swarmApi.CancelTask(ctx, t.TaskId, false /* killRunning */); err != nil {
							sklog.Errorf("Could not delete %s: %s", getTaskStr(t), err)
							continue
						}
						sklog.Infof("Deleted  %s", getTaskStr(t))
					}
				}()
			}
			// Wait for all spawned goroutines to complete
			wg.Wait()

			sklog.Infof("Cancelled %d tasks.", len(tasks))
		}

	} else if *cmd == RETRY_CMD {
		resp, err := displayTasksAndConfirm(tasks, "retrying")
		if err != nil {
			sklog.Fatal(err)
		}
		if resp {
			sklog.Infof("Starting retries of %d tasks...", len(tasks))
			tasksChannel := getClosedTasksChannel(tasks, true /* dedupNames */)

			var wg sync.WaitGroup
			// Loop through workers in the worker pool.
			for i := 0; i < WORKER_POOL_SIZE; i++ {
				// Increment the WaitGroup counter.
				wg.Add(1)
				// Create and run a goroutine closure that retries tasks.
				go func() {
					// Decrement the WaitGroup counter when the goroutine completes.
					defer wg.Done()

					for t := range tasksChannel {
						if _, err := swarmApi.RetryTask(ctx, t); err != nil {
							sklog.Errorf("Could not retry %s: %s", getTaskStr(t), err)
							continue
						}
						sklog.Infof("Retried  %s", getTaskStr(t))
					}
				}()
			}
			// Wait for all spawned goroutines to complete
			wg.Wait()

			sklog.Infof("Retried %d tasks.", len(tasks))
		}
	} else if *cmd == STDOUT_CMD {
		resp, err := displayTasksAndConfirm(tasks, "downloading stdout")
		if err != nil {
			sklog.Fatal(err)
		}
		if resp {
			sklog.Infof("Starting downloading from %d tasks...", len(tasks))
			tasksChannel := getClosedTasksChannel(tasks, false /* dedupNames */)
			outDir := filepath.Join(*workdir, "stdout")
			if err := os.MkdirAll(outDir, 0755); err != nil {
				sklog.Fatal(err)
			}

			var wg sync.WaitGroup
			// Loop through workers in the worker pool.
			for i := 0; i < WORKER_POOL_SIZE; i++ {
				// Increment the WaitGroup counter.
				wg.Add(1)
				// Create and run a goroutine closure that cancels tasks.
				go func() {
					// Decrement the WaitGroup counter when the goroutine completes.
					defer wg.Done()

					for t := range tasksChannel {
						stdout, err := swarmApi.GetStdoutOfTask(ctx, t.TaskId)
						if err != nil {
							sklog.Errorf("Could not download from %s: %s", getTaskStr(t), err)
							continue
						}
						destFile := filepath.Join(outDir, fmt.Sprintf("%s-%s.txt", t.Request.Name, t.TaskId))
						if err := ioutil.WriteFile(destFile, []byte(stdout.Output), 0644); err != nil {
							sklog.Errorf("Could not write log to %s: %s", destFile, err)
							continue
						}
						sklog.Infof("Downloaded from %s", getTaskStr(t))
					}
				}()
			}
			// Wait for all spawned goroutines to complete
			wg.Wait()

			sklog.Infof("Downloaded stdout from %d tasks into %s.", len(tasks), outDir)
		}
	}
}

func displayTasksAndConfirm(tasks []*swarmingapi.SwarmingRpcsTaskRequestMetadata, verb string) (bool, error) {
	sklog.Infof("Found %d tasks. Displaying the first %d:", len(tasks), CONFIRMATION_DISPLAY_LIMIT)
	for _, t := range tasks[:util.MinInt(CONFIRMATION_DISPLAY_LIMIT, len(tasks))] {
		sklog.Infof("  %s", getTaskStr(t))
	}
	sklog.Infof("Would you like to proceed with %s %d tasks? ('y' or 'n')", verb, len(tasks))
	return askForConfirmation()
}

func askForConfirmation() (bool, error) {
	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		return false, err
	}
	if response == "y" {
		return true, nil
	} else if response == "n" {
		return false, nil
	} else {
		sklog.Info("Please type 'y' or 'n' and then press enter:")
		return askForConfirmation()
	}
}

func getClosedTasksChannel(tasks []*swarmingapi.SwarmingRpcsTaskRequestMetadata, dedupNames bool) chan *swarmingapi.SwarmingRpcsTaskRequestMetadata {
	// Create channel that contains specified tasks. This channel will
	// be consumed by the worker pool.
	tasksChannel := make(chan *swarmingapi.SwarmingRpcsTaskRequestMetadata, len(tasks))
	// Dictionary that will be used for deduping by names if dedupNames is true.
	taskNames := map[string]bool{}

	for _, t := range tasks {
		if dedupNames {
			if _, ok := taskNames[t.Request.Name]; ok {
				// Already encountered this task. Continue.
				sklog.Errorf("Found another %s: %s", t.Request.Name, t.TaskId)
				continue
			}
		}
		tasksChannel <- t
		taskNames[t.Request.Name] = true
	}
	close(tasksChannel)
	return tasksChannel
}

func getTaskStr(t *swarmingapi.SwarmingRpcsTaskRequestMetadata) string {
	return fmt.Sprintf("Task id: %s  Created: %s  Name: %s", t.TaskId, t.Request.CreatedTs, t.Request.Name)
}
