package task_driver

import (
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/logging"
	"github.com/pborman/uuid"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
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

// run represents a full test automation run.
type run struct {
	done    func()
	emitter *stepEmitter
	report  *ReportReceiver
}

// Init begins a new test automation run.
func Init(projectId, taskId, taskName, output *string, local *bool) (*Step, error) {
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
	r := &run{
		done:    func() {},
		emitter: emitter,
		report:  report,
	}
	rv := newStep(STEP_ID_ROOT, r, nil).Start()
	return rv, nil
}

// MustInit begins a new test automation run, panicking if any setup fails.
func MustInit(projectId, taskId, taskName, output *string, local *bool) *Step {
	s, err := Init(projectId, taskId, taskName, output, local)
	if err != nil {
		sklog.Fatalf("Failed task_driver.Init(): %s", err)
	}
	return s
}

// Perform any cleanup work for the run.
func (r *run) Done() {
	defer r.done()
	if err := r.report.Report(); err != nil {
		sklog.Fatal(err)
	}
}
