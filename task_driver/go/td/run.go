package td

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/luciauth"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sklog/cloudlogging"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"
)

const (
	// PubsubTopicLogs is the PubSub topic name for task driver logs.
	PubsubTopicLogs = "task-driver-logs"

	// logID is the StackDriver log ID for all Task Drivers. Logs are labeled
	// with task ID and step ID as well, and those labels should be used for
	// filtering in most cases.
	logID = "task-driver"

	// StepIDRoot is the fixed ID of the root step.
	StepIDRoot = "root"

	// Environment variables provided to all Swarming tasks.
	envVarSwarmingBot    = "SWARMING_BOT_ID"
	envVarSwarmingServer = "SWARMING_SERVER"
	envVarSwarmingTask   = "SWARMING_TASK_ID"

	// envVarPath represents the PATH environment variable.
	envVarPath = "PATH"
)

var (
	// BaseEnv is the basic set of environment variables provided to all steps.
	BaseEnv = []string{
		"CHROME_HEADLESS=1",
		"GIT_USER_AGENT=git/1.9.1", // I don't think this version matters.
	}

	// Auth scopes required for all task_drivers.
	authScopes = []string{compute.CloudPlatformScope}
)

// RunProperties are properties for a single run of a Task Driver.
type RunProperties struct {
	Local          bool   `json:"local"`
	SwarmingBot    string `json:"swarmingBot,omitempty"`
	SwarmingServer string `json:"swarmingServer,omitempty"`
	SwarmingTask   string `json:"swarmingTask,omitempty"`
}

// Validate implements util.Validator.
func (p *RunProperties) Validate() error {
	if p.Local {
		if p.SwarmingBot != "" {
			return errors.New("SwarmingBot must be empty for local runs!")
		}
		if p.SwarmingServer != "" {
			return errors.New("SwarmingServer must be empty for local runs!")
		}
		if p.SwarmingTask != "" {
			return errors.New("SwarmingTask must be empty for local runs!")
		}
	} else {
		if p.SwarmingBot == "" {
			return errors.New("SwarmingBot is required for non-local runs!")
		}
		if p.SwarmingServer == "" {
			return errors.New("SwarmingServer is required for non-local runs!")
		}
		if p.SwarmingTask == "" {
			return errors.New("SwarmingTask is required for non-local runs!")
		}
	}
	return nil
}

// Copy returns a copy of the RunProperties.
func (p *RunProperties) Copy() *RunProperties {
	if p == nil {
		return nil
	}
	return &RunProperties{
		Local:          p.Local,
		SwarmingBot:    p.SwarmingBot,
		SwarmingServer: p.SwarmingServer,
		SwarmingTask:   p.SwarmingTask,
	}
}

// StartRunWithErr begins a new test automation run, returning any error which
// occurs.
func StartRunWithErr(projectId, taskId, taskName, output *string, local *bool) (context.Context, error) {
	common.Init()

	// TODO(borenet): Catch SIGINT, SIGKILL and report.

	// Gather RunProperties.
	swarmingBot := os.Getenv(envVarSwarmingBot)
	swarmingServer := os.Getenv(envVarSwarmingServer)
	swarmingTask := os.Getenv(envVarSwarmingTask)

	// "reproduce" is supplied by "swarming.py reproduce" and indicates that
	// this is actually a local run, but --local won't have been provided
	// because the command was copied directly from the Swarming task.
	if swarmingTask == "reproduce" || swarmingBot == "reproduce" {
		*local = true
		swarmingBot = ""
		swarmingServer = ""
		swarmingTask = ""
	}
	if *local {
		// Check to make sure we're not actually running in production.
		// Note that the presence of SWARMING_SERVER does not indicate
		// that we're running in production, because it can be used with
		// swarming.py as an alternative to --swarming.
		errTmpl := "--local was supplied but %s environment variable was found. Was --local used by accident?"
		if swarmingBot != "" {
			return nil, fmt.Errorf(errTmpl, envVarSwarmingBot)
		} else if swarmingTask != "" {
			return nil, fmt.Errorf(errTmpl, envVarSwarmingTask)
		}

		// Prevent clobbering real task data for local tasks.
		hostname, err := os.Hostname()
		if err != nil {
			return nil, err
		}
		*taskId = fmt.Sprintf("%s_%s", hostname, uuid.New())
	} else {
		// Check to make sure that we're not running locally and the
		// user forgot to use --local.
		errTmpl := "--local was not supplied but environment variable %s was not found. Did you forget to use --local?"
		if swarmingBot == "" {
			return nil, fmt.Errorf(errTmpl, envVarSwarmingBot)
		} else if swarmingServer == "" {
			return nil, fmt.Errorf(errTmpl, envVarSwarmingServer)
		} else if swarmingTask == "" {
			return nil, fmt.Errorf(errTmpl, envVarSwarmingTask)
		}
	}

	// Validate properties and flags.
	props := &RunProperties{
		Local:          *local,
		SwarmingBot:    swarmingBot,
		SwarmingServer: swarmingServer,
		SwarmingTask:   swarmingTask,
	}
	if err := props.Validate(); err != nil {
		return nil, err
	}
	if !*local {
		if *projectId == "" {
			return nil, fmt.Errorf("Project ID is required.")
		}
		if *taskId == "" {
			return nil, fmt.Errorf("Task ID is required.")
		}
		if *taskName == "" {
			return nil, fmt.Errorf("Task name is required.")
		}
	}

	// Create the token source.
	var ts oauth2.TokenSource
	if *local {
		var err error
		ts, err = google.DefaultTokenSource(context.TODO(), authScopes...)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		ts, err = luciauth.NewLUCIContextTokenSource(authScopes...)
		if err != nil {
			return nil, fmt.Errorf("Failed to obtain LUCI TokenSource: %s", err)
		}
	}

	// Connect receivers.
	receiver := MultiReceiver([]Receiver{
		&DebugReceiver{},
		newReportReceiver(*output),
	})

	// Initialize Cloud Logging.
	ctx := context.Background()
	if *projectId != "" && *taskId != "" && *taskName != "" {
		labels := map[string]string{
			"taskId":   *taskId,
			"taskName": *taskName,
		}
		logger, err := cloudlogging.New(ctx, *projectId, logID, ts, labels)
		if err != nil {
			return nil, err
		}
		cloudLogging, err := NewCloudLoggingReceiver(logger.Logger())
		if err != nil {
			return nil, err
		}
		receiver = append(receiver, cloudLogging)
	}

	// Dump environment variables.
	sklog.Infof("Environment:\n%s", strings.Join(os.Environ(), "\n"))

	// Set up and return the root-level Step.
	ctx = newRun(ctx, receiver, *taskId, *taskName, props)
	return ctx, nil
}

// StartRun begins a new test automation run, panicking if any setup fails.
func StartRun(projectId, taskId, taskName, output *string, local *bool) context.Context {
	ctx, err := StartRunWithErr(projectId, taskId, taskName, output, local)
	if err != nil {
		sklog.Fatalf("Failed task_driver.StartRun(): %s", err)
	}
	return ctx
}

// EndRun performs any cleanup work for the run. Should be deferred in main().
func EndRun(ctx context.Context) {
	defer util.Close(getCtx(ctx).run)

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
func newRun(ctx context.Context, rec Receiver, taskId, taskName string, props *RunProperties) context.Context {
	r := &run{
		receiver: rec,
		taskId:   taskId,
	}
	r.send(&Message{
		Type: MsgType_RunStarted,
		Run:  props,
	})
	ctx = context.WithValue(ctx, contextKey, &Context{
		run:     r,
		execRun: exec.DefaultRun,
	})
	env := MergeEnv(os.Environ(), BaseEnv)
	ctx = newStep(ctx, StepIDRoot, nil, Props(taskName).Env(env))
	return ctx
}

// Send the given message to the receiver. Does not return an error, even if
// sending fails.
func (r *run) send(msg *Message) {
	msg.ID = uuid.New().String()
	msg.TaskId = r.taskId
	msg.Timestamp = time.Now().UTC()
	if err := msg.Validate(); err != nil {
		sklog.Error(err)
	}
	if err := r.receiver.HandleMessage(msg); err != nil {
		// Just log the error but don't return it.
		// TODO(borenet): How do we handle this?
		sklog.Error(err)
	}
}

// Send a Message indicating that a new step has started.
func (r *run) Start(props *StepProperties) {
	msg := &Message{
		Type:   MsgType_StepStarted,
		StepId: props.Id,
		Step:   props,
	}
	r.send(msg)
}

// Send a Message with additional data for the current step.
func (r *run) AddStepData(id string, typ DataType, d interface{}) {
	msg := &Message{
		Type:     MsgType_StepData,
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
		StepId: id,
		Error:  err.Error(),
	}
	if IsInfraError(err) {
		msg.Type = MsgType_StepException
	} else {
		msg.Type = MsgType_StepFailed
	}
	r.send(msg)
}

// Send a Message indicating that the current step has finished.
func (r *run) Finish(id string) {
	msg := &Message{
		Type:   MsgType_StepFinished,
		StepId: id,
	}
	r.send(msg)
}

// cbWriter is a wrapper around an io.Writer which runs a callback on the first
// call to Write().
type cbWriter struct {
	w  io.Writer
	cb func()
}

// See documentation for io.Writer interface.
func (w *cbWriter) Write(b []byte) (int, error) {
	if w.cb != nil {
		w.cb()
		w.cb = nil
	}
	return w.w.Write(b)
}

// Open a log stream.
func (r *run) LogStream(stepId, logName string, severity Severity) io.Writer {
	logId := uuid.New().String() // TODO(borenet): Come up with a better ID.
	w, err := r.receiver.LogStream(stepId, logId, severity)
	if err != nil {
		panic(err)
	}

	// Wrap the io.Writer with a cbWrite so that we only send step data for
	// log streams which are actually used.
	return &cbWriter{
		w: w,
		cb: func() {
			// Emit step data for the log stream.
			r.AddStepData(stepId, DataType_Log, &LogData{
				Name:     logName,
				Id:       logId,
				Severity: severity.String(),
			})
		},
	}
}

// Close the run.
func (r *run) Close() error {
	return r.receiver.Close()
}
