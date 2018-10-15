package td

import (
	"io"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	MSG_TYPE_STEP_STARTED   MessageType = "STEP_STARTED"
	MSG_TYPE_STEP_FINISHED  MessageType = "STEP_FINISHED"
	MSG_TYPE_STEP_DATA      MessageType = "STEP_DATA"
	MSG_TYPE_STEP_FAILED    MessageType = "STEP_FAILED"
	MSG_TYPE_STEP_EXCEPTION MessageType = "STEP_EXCEPTION"
)

// MessageType indicates the type of a Message.
type MessageType string

// Message is a struct used to send step metadata to Receivers.
type Message struct {
	// StepId indicates the ID for the step. This is required for every
	// Message.
	StepId string `json:"stepId"`

	// TaskId indicates the ID of this task. This is required for every
	// Message.
	TaskId string `json:"taskId"`

	// Timestamp is the time at which the Message was created. This is
	// required for every Message.
	Timestamp time.Time `json:"timestamp"`

	// Type indicates the type of message, which dictates which fields must
	// be filled.
	Type MessageType `json:"type"`

	// Step is the metadata about the step at creation time. Required for
	// MSG_TYPE_STEP_STARTED.
	Step *StepProperties `json:"step,omitempty"`

	// Error is any error which might have occurred. Required for
	// MSG_TYPE_STEP_FAILED and MSG_TYPE_STEP_EXCEPTION.
	Error string `json:"error,omitempty"`

	// Data is arbitrary additional data about the step. Required for
	// MSG_TYPE_STEP_DATA.
	Data interface{} `json:"data,omitempty"`
}

// stepEmitter is used to send metadata about steps to various Receivers.
type stepEmitter struct {
	receivers map[string]Receiver
	taskId    string
}

// newStepEmitter returns a stepEmitter instance.
func newStepEmitter(taskId string, receivers map[string]Receiver) *stepEmitter {
	return &stepEmitter{
		receivers: receivers,
		taskId:    taskId,
	}
}

// Send the given message to all receivers. Does not return an error, even if
// sending fails.
func (e *stepEmitter) send(msg *Message) {
	msg.TaskId = e.taskId
	msg.Timestamp = time.Now().UTC()
	g := util.NewNamedErrGroup()
	for k, v := range e.receivers {
		receiver := v
		g.Go(k, func() error {
			err := receiver.HandleMessage(msg)
			return err
		})
	}
	if err := g.Wait(); err != nil {
		// Just log the error but don't return it.
		// TODO(borenet): How do we handle this?
		sklog.Error(err)
	}
}

// Send a Message indicating that a new step has started.
func (e *stepEmitter) Start(props *StepProperties) {
	msg := &Message{
		Type:   MSG_TYPE_STEP_STARTED,
		StepId: props.Id,
		Step:   props,
	}
	e.send(msg)
}

// Send a Message with additional data for the current step.
func (e *stepEmitter) AddStepData(id string, d interface{}) {
	msg := &Message{
		Type:   MSG_TYPE_STEP_DATA,
		StepId: id,
		Data:   d,
	}
	e.send(msg)
}

// Send a Message indicating that the current step has failed with the given
// error.
func (e *stepEmitter) Failed(id string, err error) {
	msg := &Message{
		Type:   MSG_TYPE_STEP_FAILED,
		StepId: id,
		Error:  err.Error(),
	}
	e.send(msg)
}

// Send a Message indicating that the current step has failed exceptionally.
func (e *stepEmitter) Exception(id string, err error) {
	msg := &Message{
		Type:   MSG_TYPE_STEP_EXCEPTION,
		StepId: id,
		Error:  err.Error(),
	}
	e.send(msg)
}

// Send a Message indicating that the current step has finished.
func (e *stepEmitter) Finish(id string) {
	msg := &Message{
		Type:   MSG_TYPE_STEP_FINISHED,
		StepId: id,
	}
	e.send(msg)
}

// Open a log stream.
func (e *stepEmitter) LogStream(stepId, logId, severity string) io.Writer {
	writers := make([]io.Writer, 0, len(e.receivers))
	for _, r := range e.receivers {
		w, err := r.LogStream(stepId, logId, severity)
		if err != nil {
			panic(err)
		}
		writers = append(writers, w)
	}
	return util.MultiWriter(writers)
}

// Close the stepEmitter.
func (e *stepEmitter) Close() error {
	for _, r := range e.receivers {
		if err := r.Close(); err != nil {
			return err
		}
	}
	return nil
}
