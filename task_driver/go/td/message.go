package td

import (
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

const (
	MsgType_RunStarted    MessageType = "RUN_STARTED"
	MsgType_StepStarted   MessageType = "STEP_STARTED"
	MsgType_StepFinished  MessageType = "STEP_FINISHED"
	MsgType_StepData      MessageType = "STEP_DATA"
	MsgType_StepFailed    MessageType = "STEP_FAILED"
	MsgType_StepException MessageType = "STEP_EXCEPTION"

	DataType_Log          DataType = "log"
	DataType_Text         DataType = "text"
	DataType_Command      DataType = "command"
	DataType_HttpRequest  DataType = "httpRequest"
	DataType_HttpResponse DataType = "httpResponse"
)

// MessageType indicates the type of a Message.
type MessageType string

// DataType indicates the type of a piece of data attached to a step.
type DataType string

// Message is a struct used to send step metadata to Receivers.
type Message struct {
	// ID is the unique identifier for this message. This is required for every
	// Message.
	ID string `json:"id"`

	// Index is a monotonically increasing index of the message within the
	// Task. Deprecated.
	Index int `json:"index"`

	// StepId indicates the ID for the step. This is required for every
	// Message except MSG_TYPE_RUN_STARTED.
	StepId string `json:"stepId,omitempty"`

	// TaskId indicates the ID of this task. This is required for every
	// Message.
	TaskId string `json:"taskId"`

	// Timestamp is the time at which the Message was created. This is
	// required for every Message.
	Timestamp time.Time `json:"timestamp"`

	// Type indicates the type of message, which dictates which fields must
	// be filled.
	Type MessageType `json:"type"`

	// Run is the metadata about the overall Task Driver run. Required for
	// MSG_TYPE_RUN_STARTED.
	Run *RunProperties `json:"run,omitempty"`

	// Step is the metadata about the step at creation time. Required for
	// MSG_TYPE_STEP_STARTED.
	Step *StepProperties `json:"step,omitempty"`

	// Error is any error which might have occurred. Required for
	// MSG_TYPE_STEP_FAILED and MSG_TYPE_STEP_EXCEPTION.
	Error string `json:"error,omitempty"`

	// Data is arbitrary additional data about the step. Required for
	// MSG_TYPE_STEP_DATA.
	Data interface{} `json:"data,omitempty"`

	// DataType describes the contents of Data. Required for
	// MSG_TYPE_STEP_DATA.
	DataType DataType `json:"dataType,omitempty"`
}

// Return an error if the Message is not valid.
func (m *Message) Validate() error {
	if m.ID == "" {
		return skerr.Fmt("ID is required.")
	}
	if m.TaskId == "" {
		return skerr.Fmt("TaskId is required.")
	} else if util.TimeIsZero(m.Timestamp) {
		return skerr.Fmt("Timestamp is required.")
	}
	switch m.Type {
	case MsgType_RunStarted:
		if m.Run == nil {
			return skerr.Fmt("RunProperties are required for %s", m.Type)
		}
		if err := m.Run.Validate(); err != nil {
			return err
		}
	case MsgType_StepStarted:
		if m.StepId == "" {
			return skerr.Fmt("StepId is required for %s", m.Type)
		}
		if m.Step == nil {
			return skerr.Fmt("StepProperties are required for %s", m.Type)
		}
		if err := m.Step.Validate(); err != nil {
			return err
		}
		if m.StepId != m.Step.Id {
			return skerr.Fmt("StepId must equal Step.Id (%s vs %s)", m.StepId, m.Step.Id)
		}
	case MsgType_StepFinished:
		if m.StepId == "" {
			return skerr.Fmt("StepId is required for %s", m.Type)
		}
	case MsgType_StepData:
		if m.StepId == "" {
			return skerr.Fmt("StepId is required for %s", m.Type)
		}
		if m.Data == nil {
			return skerr.Fmt("Data is required for %s", m.Type)
		}
		switch m.DataType {
		case DataType_Log:
		case DataType_Text:
		case DataType_Command:
		case DataType_HttpRequest:
		case DataType_HttpResponse:
		default:
			return skerr.Fmt("Invalid DataType %q", m.DataType)
		}
	case MsgType_StepFailed:
		if m.StepId == "" {
			return skerr.Fmt("StepId is required for %s", m.Type)
		}
		if m.Error == "" {
			return skerr.Fmt("Error is required for %s", m.Type)
		}
	case MsgType_StepException:
		if m.StepId == "" {
			return skerr.Fmt("StepId is required for %s", m.Type)
		}
		if m.Error == "" {
			return skerr.Fmt("Error is required for %s", m.Type)
		}
	default:
		return skerr.Fmt("Invalid message Type %q", m.Type)
	}
	return nil
}
