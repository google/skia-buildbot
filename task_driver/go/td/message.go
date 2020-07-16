package td

import (
	"errors"
	"fmt"
	"time"

	"go.skia.org/infra/go/util"
)

const (
	MSG_TYPE_RUN_STARTED    MessageType = "RUN_STARTED"
	MSG_TYPE_STEP_STARTED   MessageType = "STEP_STARTED"
	MSG_TYPE_STEP_FINISHED  MessageType = "STEP_FINISHED"
	MSG_TYPE_STEP_DATA      MessageType = "STEP_DATA"
	MSG_TYPE_STEP_FAILED    MessageType = "STEP_FAILED"
	MSG_TYPE_STEP_EXCEPTION MessageType = "STEP_EXCEPTION"

	DATA_TYPE_LOG           DataType = "log"
	DATA_TYPE_TEXT          DataType = "text"
	DATA_TYPE_COMMAND       DataType = "command"
	DATA_TYPE_HTTP_REQUEST  DataType = "httpRequest"
	DATA_TYPE_HTTP_RESPONSE DataType = "httpResponse"
)

// MessageType indicates the type of a Message.
type MessageType string

// DataType indicates the type of a piece of data attached to a step.
type DataType string

// Message is a struct used to send step metadata to Receivers.
type Message struct {
	// Index is a monotonically increasing index of the message within the
	// Task.
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
	if m.Index == 0 {
		return errors.New("A non-zero index is required.")
	}
	if m.TaskId == "" {
		return errors.New("TaskId is required.")
	} else if util.TimeIsZero(m.Timestamp) {
		return errors.New("Timestamp is required.")
	}
	switch m.Type {
	case MSG_TYPE_RUN_STARTED:
		if m.Run == nil {
			return fmt.Errorf("RunProperties are required for %s", m.Type)
		}
		if err := m.Run.Validate(); err != nil {
			return err
		}
	case MSG_TYPE_STEP_STARTED:
		if m.StepId == "" {
			return fmt.Errorf("StepId is required for %s", m.Type)
		}
		if m.Step == nil {
			return fmt.Errorf("StepProperties are required for %s", m.Type)
		}
		if err := m.Step.Validate(); err != nil {
			return err
		}
		if m.StepId != m.Step.Id {
			return fmt.Errorf("StepId must equal Step.Id (%s vs %s)", m.StepId, m.Step.Id)
		}
	case MSG_TYPE_STEP_FINISHED:
		if m.StepId == "" {
			return fmt.Errorf("StepId is required for %s", m.Type)
		}
	case MSG_TYPE_STEP_DATA:
		if m.StepId == "" {
			return fmt.Errorf("StepId is required for %s", m.Type)
		}
		if m.Data == nil {
			return fmt.Errorf("Data is required for %s", m.Type)
		}
		switch m.DataType {
		case DATA_TYPE_LOG:
		case DATA_TYPE_TEXT:
		case DATA_TYPE_COMMAND:
		case DATA_TYPE_HTTP_REQUEST:
		case DATA_TYPE_HTTP_RESPONSE:
		default:
			return fmt.Errorf("Invalid DataType %q", m.DataType)
		}
	case MSG_TYPE_STEP_FAILED:
		if m.StepId == "" {
			return fmt.Errorf("StepId is required for %s", m.Type)
		}
		if m.Error == "" {
			return fmt.Errorf("Error is required for %s", m.Type)
		}
	case MSG_TYPE_STEP_EXCEPTION:
		if m.StepId == "" {
			return fmt.Errorf("StepId is required for %s", m.Type)
		}
		if m.Error == "" {
			return fmt.Errorf("Error is required for %s", m.Type)
		}
	default:
		return fmt.Errorf("Invalid message Type %q", m.Type)
	}
	return nil
}
