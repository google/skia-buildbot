package test_automation

import (
	"encoding/json"
	"fmt"
	"io"

	"go.skia.org/infra/go/sklog"
)

// Receiver is an interface used to implement arbitrary receivers of step
// metadata, as steps are run.
type Receiver interface {
	// Handle the given message.
	HandleMessage(*Message) error
}

// DebugReceiver just dumps the messages straight to the log.
type DebugReceiver struct{}

// See documentation for Receiver interface.
func (r *DebugReceiver) HandleMessage(m *Message) error {
	switch m.Type {
	case MSG_TYPE_STEP_STARTED:
		sklog.Infof("STEP_STARTED: %s", m.StepId)
	case MSG_TYPE_STEP_FINISHED:
		sklog.Infof("STEP_FINISHED: %s", m.StepId)
	case MSG_TYPE_STEP_DATA:
		b, err := json.MarshalIndent(m.Data, "", " ")
		if err != nil {
			return err
		}
		sklog.Infof("STEP_DATA: %s: %s", m.StepId, string(b))
	default:
		return fmt.Errorf("Invalid message type %s", m.Type)
	}
	return nil
}

// stepReport is a struct used to collect information about a given step.
type stepReport struct {
	*StepResult
	*StepProperties
	Data  []interface{} `json:"data,omitempty"`
	Steps []*stepReport `json:"steps,omitempty"`
}

// ReportReceiver collects all messages and generates a report when requested.
type ReportReceiver struct {
	root *stepReport
}

// Find the step with the given ID in our list. This helps in case messages
// arrive out of order.
func (r *ReportReceiver) findStep(id string) (*stepReport, error) {
	if r.root == nil {
		return nil, fmt.Errorf("No steps!", id)
	}

	var findHelper func(*stepReport) *stepReport
	findHelper = func(current *stepReport) *stepReport {
		if current.Id == id {
			return current
		}
		for _, sub := range current.Steps {
			found := findHelper(sub)
			if found != nil {
				return found
			}
		}
		return nil
	}
	s := findHelper(r.root)
	if s != nil {
		return s, nil
	}
	return nil, fmt.Errorf("Unknown step ID %q", id)
}

// See documentation for Receiver interface.
func (r *ReportReceiver) HandleMessage(m *Message) error {
	switch m.Type {
	case MSG_TYPE_STEP_STARTED:
		s := &stepReport{
			StepProperties: m.Step,
		}
		if m.Step.Id == STEP_ID_ROOT {
			r.root = s
		} else {
			parent, err := r.findStep(m.Step.Parent)
			if err != nil {
				return err
			}
			parent.Steps = append(parent.Steps, s)
		}
	case MSG_TYPE_STEP_FINISHED:
		s, err := r.findStep(m.StepId)
		if err != nil {
			return err
		}
		s.StepResult = m.Result
	case MSG_TYPE_STEP_DATA:
		s, err := r.findStep(m.StepId)
		if err != nil {
			return err
		}
		s.Data = append(s.Data, m.Data)
	}
	return nil
}

// Write the report in JSON format to the given Writer.
func (r *ReportReceiver) Report(w io.Writer) error {
	b, err := json.MarshalIndent(r.root, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}
