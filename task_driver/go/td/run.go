package td

import (
	"context"
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/logging"
	"github.com/pborman/uuid"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	// PubSub topic name.
	PUBSUB_TOPIC = "task-driver"

	// Special ID of the root step.
	STEP_ID_ROOT = "root"
)

var (
	// Auth scopes required for all task_drivers.
	SCOPES = []string{logging.WriteScope}
)

// StartRunWithErr begins a new test automation run, returning any error which
// occurs.
func StartRunWithErr(projectId, taskId, taskName, output *string, local *bool) (context.Context, error) {
	common.Init()

	// TODO(borenet): Catch SIGINT, SIGKILL and report.
	// Prevent clobbering real task data.
	if *local {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, err
		}
		*taskId = fmt.Sprintf("%s_%s", hostname, uuid.New())
	}
	if *projectId == "" {
		return nil, fmt.Errorf("Project ID is required.")
	}
	if *taskId == "" {
		return nil, fmt.Errorf("Task ID is required.")
	}
	if *taskName == "" {
		return nil, fmt.Errorf("Task name is required.")
	}

	// Dump environment variables.
	sklog.Infof("Environment:\n%s", strings.Join(os.Environ(), "\n"))

	// Connect receivers.
	report := newReportReceiver(*output)
	receivers := map[string]Receiver{
		"DebugReceiver":  &DebugReceiver{},
		"ReportReceiver": report,
	}
	emitter := newStepEmitter(*taskId, receivers)

	// Set up and return the root-level Step.
	ctx := newRun(emitter, *taskName)
	return ctx, nil
}

// StartRun begins a new test automation run, panicking if any setup fails.
func StartRun(projectId, taskId, taskName, output *string, local *bool) context.Context {
	ctx, err := StartRunWithErr(projectId, taskId, taskName, output, local)
	if err != nil {
		sklog.Fatalf("Failed task_driver.Init(): %s", err)
	}
	return ctx
}

// Perform any cleanup work for the run. Should be deferred in main().
func EndRun(ctx context.Context) {
	defer getRun(ctx).done()

	// Mark the root step as finished.
	finishStep(ctx, recover())
}

// run represents a full test automation run.
type run struct {
	done    func()
	emitter *stepEmitter
}

// newRun returns a context.Context representing a Task Driver run, including
// creation of a root step.
func newRun(e *stepEmitter, taskName string) context.Context {
	r := &run{
		done: func() {
			util.Close(e)
		},
		emitter: e,
	}
	ctx := context.Background()
	ctx = setRun(ctx, r)
	ctx = newStep(ctx, STEP_ID_ROOT, nil, Props(taskName))
	return ctx
}
