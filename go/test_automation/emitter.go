package test_automation

import (
	"io"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	MSG_TYPE_STEP_STARTED  = "STEP_STARTED"
	MSG_TYPE_STEP_FINISHED = "STEP_FINISHED"
	MSG_TYPE_STEP_DATA     = "STEP_DATA"
)

// Message is a struct used to send Step metadata to Receivers.
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
	Type string `json:"type"`

	// Step is the metadata about the step at creation time. Required for
	// MSG_TYPE_STEP_STARTED.
	Step *StepProperties `json:"step,omitempty"`

	// Result is the result of the step. Required for
	// MSG_TYPE_STEP_FINISHED.
	Result *StepResult `json:"result,omitempty"`

	// Data is arbitrary additional data about the step. Required for
	// MSG_TYPE_STEP_DATA.
	Data interface{} `json:"data,omitempty"`
}

// stepEmitter is used to send metadata about Steps to various Receivers.
type stepEmitter struct {
	receivers map[string]Receiver
	taskId    string
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

// Send a Message indicating that a new Step has started.
func (e *stepEmitter) Start(s *Step) {
	msg := &Message{
		Type:   MSG_TYPE_STEP_STARTED,
		StepId: s.Id,
		Step:   s.StepProperties,
	}
	e.send(msg)
}

// Send a Message with additional data for the current Step.
func (e *stepEmitter) AddStepData(id string, d interface{}) {
	msg := &Message{
		Type:   MSG_TYPE_STEP_DATA,
		StepId: id,
		Data:   d,
	}
	e.send(msg)
}

// Send a Message indicating that the current Step has finished with the given
// StepResult.
func (e *stepEmitter) Finish(id string, result *StepResult) {
	msg := &Message{
		Type:   MSG_TYPE_STEP_FINISHED,
		StepId: id,
		Result: result,
	}
	e.send(msg)
}

// Open a log stream.
func (e *stepEmitter) LogStream(stepId, logId string) io.Writer {
	writers := make([]io.Writer, 0, len(e.receivers))
	for _, r := range e.receivers {
		w, err := r.LogStream(stepId, logId)
		if err != nil {
			panic(err)
		}
		writers = append(writers, w)
	}
	return util.MultiWriter(writers)
}
