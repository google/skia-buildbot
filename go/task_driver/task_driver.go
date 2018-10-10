package task_driver

import (
	"context"
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/logging"
	"github.com/pborman/uuid"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
)

const (
	// PubSub topic name.
	PUBSUB_TOPIC = "task-driver"

	// Special ID of the root step.
	STEP_ID_ROOT = "root"
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

	ctx := context.Background()

	// Initialize Cloud Logging.
	ts, err := auth.NewDefaultTokenSource(*local, logging.WriteScope)
	if err != nil {
		return nil, err
	}
	labels := map[string]string{
		"taskId":   *taskId,
		"taskName": *taskName,
	}
	logger, err := sklog.NewCloudLogger(ctx, *projectId, *taskId, ts, labels)
	if err != nil {
		return nil, err
	}
	sklog.SetLogger(logger)

	// Dump environment variables.
	sklog.Infof("Environment:\n", strings.Join(os.Environ(), "\n"))

	// Connect receivers.
	cloudLogging, err := NewCloudLoggingReceiver(logger.Logger())
	if err != nil {
		return nil, err
	}
	pubSub, err := NewPubSubReceiver(ctx, *projectId, PUBSUB_TOPIC)
	if err != nil {
		return nil, err
	}
	report := newReportReceiver(*output)
	receivers := map[string]Receiver{
		"CloudLoggingReceiver": cloudLogging,
		"DebugReceiver":        &DebugReceiver{},
		"PubSubReceiver":       pubSub,
		"ReportReceiver":       report,
	}
	emitter := newStepEmitter(*taskId, receivers)

	// Set up and return the root-level Step.
	r := &run{
		done: func() {
			logger.Flush()
		},
		emitter: emitter,
		report:  report,
	}
	rv := newStep(STEP_ID_ROOT, r, nil).Start()
	return rv, nil
}

// Perform any cleanup work for the run.
func (r *run) Done() {
	defer r.done()
	if err := r.report.Report(); err != nil {
		sklog.Fatal(err)
	}
}
