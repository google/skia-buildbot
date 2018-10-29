package td

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/pborman/uuid"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
	compute "google.golang.org/api/compute/v1"
)

const (
	// PubSub topic name for task driver metadata.
	PUBSUB_TOPIC = "task-driver"
	// PubSub topic name for task driver logs.
	PUBSUB_TOPIC_LOGS = "task-driver-logs"

	// Log ID for all Task Drivers. Logs are labeled with task ID and step
	// ID as well, and those labels should be used for filtering in most
	// cases.
	LOG_ID = "task-driver"

	// Special ID of the root step.
	STEP_ID_ROOT = "root"
)

var (
	// Auth scopes required for all task_drivers.
	SCOPES = []string{compute.CloudPlatformScope}
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

	ctx := context.Background()

	// Initialize Cloud Logging.
	var ts oauth2.TokenSource
	if *local {
		var err error
		ts, err = auth.NewDefaultTokenSource(*local, SCOPES...)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		ts, err = auth.NewLUCIContextTokenSource(SCOPES...)
		if err != nil {
			return nil, fmt.Errorf("Failed to obtain LUCI TokenSource: %s", err)
		}
	}
	labels := map[string]string{
		"taskId":   *taskId,
		"taskName": *taskName,
	}
	logger, err := sklog.NewCloudLogger(ctx, *projectId, LOG_ID, ts, labels)
	if err != nil {
		return nil, err
	}
	sklog.SetLogger(logger)

	// Dump environment variables.
	sklog.Infof("Environment:\n%s", strings.Join(os.Environ(), "\n"))

	// Connect receivers.
	cloudLogging, err := NewCloudLoggingReceiver(logger.Logger())
	if err != nil {
		return nil, err
	}
	report := newReportReceiver(*output)
	receiver := MultiReceiver([]Receiver{
		cloudLogging,
		&DebugReceiver{},
		report,
	})

	// Set up and return the root-level Step.
	ctx = newRun(ctx, receiver, *taskId, *taskName)
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
	defer util.Close(getRun(ctx))

	// Mark the root step as finished.
	finishStep(ctx, recover())
}

// run represents a full test automation run.
type run struct {
	receiver Receiver
	taskId   string
}

// newRun returns a context.Context representing a Task Driver run, including
// creation of a root step.
func newRun(ctx context.Context, rec Receiver, taskId, taskName string) context.Context {
	r := &run{
		receiver: rec,
		taskId:   taskId,
	}
	ctx = setRun(ctx, r)
	ctx = newStep(ctx, STEP_ID_ROOT, nil, Props(taskName))
	return ctx
}

// Send the given message to the receiver. Does not return an error, even if
// sending fails.
func (r *run) send(msg *Message) {
	msg.TaskId = r.taskId
	msg.Timestamp = time.Now().UTC()
	if err := r.receiver.HandleMessage(msg); err != nil {
		// Just log the error but don't return it.
		// TODO(borenet): How do we handle this?
		sklog.Error(err)
	}
}

// Send a Message indicating that a new step has started.
func (r *run) Start(props *StepProperties) {
	msg := &Message{
		Type:   MSG_TYPE_STEP_STARTED,
		StepId: props.Id,
		Step:   props,
	}
	r.send(msg)
}

// Send a Message with additional data for the current step.
func (r *run) AddStepData(id string, typ DataType, d interface{}) {
	msg := &Message{
		Type:     MSG_TYPE_STEP_DATA,
		StepId:   id,
		Data:     d,
		DataType: typ,
	}
	r.send(msg)
}

// Send a Message indicating that the current step has failed with the given
// error.
func (r *run) Failed(id string, err error) {
	msg := &Message{
		Type:   MSG_TYPE_STEP_FAILED,
		StepId: id,
		Error:  err.Error(),
	}
	r.send(msg)
}

// Send a Message indicating that the current step has failed exceptionally.
func (r *run) Exception(id string, err error) {
	msg := &Message{
		Type:   MSG_TYPE_STEP_EXCEPTION,
		StepId: id,
		Error:  err.Error(),
	}
	r.send(msg)
}

// Send a Message indicating that the current step has finished.
func (r *run) Finish(id string) {
	msg := &Message{
		Type:   MSG_TYPE_STEP_FINISHED,
		StepId: id,
	}
	r.send(msg)
}

// Open a log stream.
func (r *run) LogStream(stepId, logName, severity string) io.Writer {
	logId := uuid.New() // TODO(borenet): Come up with a better ID.
	rv, err := r.receiver.LogStream(stepId, logId, severity)
	if err != nil {
		panic(err)
	}

	// Emit step data for the log stream.
	r.AddStepData(stepId, DATA_TYPE_LOG, &LogData{
		Name:     logName,
		Id:       logId,
		Severity: severity,
	})
	return rv
}

// Close the run.
func (r *run) Close() error {
	return r.receiver.Close()
}
