package td

import (
	"time"
)

const (
	MSG_TYPE_STEP_STARTED   MessageType = "STEP_STARTED"
	MSG_TYPE_STEP_FINISHED  MessageType = "STEP_FINISHED"
	MSG_TYPE_STEP_DATA      MessageType = "STEP_DATA"
	MSG_TYPE_STEP_FAILED    MessageType = "STEP_FAILED"
	MSG_TYPE_STEP_EXCEPTION MessageType = "STEP_EXCEPTION"

	DATA_TYPE_LOG           DataType = "log"
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

	// DataType describes the contents of Data. Required for
	// MSG_TYPE_STEP_DATA.
	DataType DataType `json:"dataType,omitempty"`
}
