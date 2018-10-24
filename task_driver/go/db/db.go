package db

import (
	"fmt"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_driver/go/td"
)

// DB is an interface used for storing information about Task Drivers.
type DB interface {
	// InsertTaskDriver inserts the TaskDriverRun instance into the DB.
	InsertTaskDriver(*TaskDriverRun) error

	// GetTaskDriver returns the TaskDriver instance with the given ID. If
	// the Task Driver does not exist, returns nil with no error.
	GetTaskDriver(string) (*TaskDriverRun, error)

	// UpdateTaskDriver runs the given function to atomically update the
	// Task Driver with the given ID. If no TaskDriver with the given ID
	// exists, the DB will create and insert one. The DB should not commit
	// any changes to the TaskDriver unless the func returns with no error.
	UpdateTaskDriver(string, func(*TaskDriverRun) error) error
}

// Step represents one step in a single run of a Task Driver.
type Step struct {
	Properties *td.StepProperties `json:"properties"`
	Data       []*StepData        `json:"data"`
	Started    time.Time          `json:"started"`
	Finished   time.Time          `json:"finished"`
	Result     td.StepResult      `json:"result"`
	Errors     []string           `json:"errors"`
}

// newStep returns a near-empty Step with the minimal amount of information
// needed for us to use it.
func newStep(id string) *Step {
	return &Step{
		Properties: &td.StepProperties{
			Id: id,
		},
	}
}

// StepData represents data attached to a step.
type StepData struct {
	Type td.DataType `json:"type"`
	Data interface{} `json:"data"`
}

// Copy returns a deep copy of the Step.
func (s *Step) Copy() *Step {
	if s == nil {
		return nil
	}
	// Unfortunately, because we don't know the type of StepData.Data, we
	// can't deep copy it.
	var data []*StepData
	if s.Data != nil {
		data = append(make([]*StepData, 0, len(s.Data)), s.Data...)
	}
	return &Step{
		Properties: s.Properties.Copy(),
		Data:       data,
		Started:    s.Started,
		Finished:   s.Finished,
		Result:     s.Result,
		Errors:     util.CopyStringSlice(s.Errors),
	}
}

// TaskDriverRun represents a single run of a Task Driver.
type TaskDriverRun struct {
	TaskId string
	Steps  map[string]*Step
}

// Copy returns a deep copy of the TaskDriverRun.
func (t *TaskDriverRun) Copy() *TaskDriverRun {
	if t == nil {
		return nil
	}
	var steps map[string]*Step
	if t.Steps != nil {
		steps = make(map[string]*Step, len(t.Steps))
		for k, v := range t.Steps {
			steps[k] = v.Copy()
		}
	}
	return &TaskDriverRun{
		TaskId: t.TaskId,
		Steps:  steps,
	}
}

// Update a TaskDriverRun from the given Message.
func UpdateFromMessage(db DB, m *td.Message) error {
	// TODO(borenet): This is not thread-safe!
	return db.UpdateTaskDriver(m.TaskId, func(t *TaskDriverRun) error {
		if t.Steps == nil {
			t.Steps = map[string]*Step{}
			sklog.Infof("Got new task: %s", t.TaskId)
		}
		step, ok := t.Steps[m.StepId]
		if !ok {
			step = newStep(m.StepId)
			t.Steps[m.StepId] = step
		}

		switch m.Type {
		case td.MSG_TYPE_STEP_STARTED:
			// Validation.
			if m.Step == nil {
				return fmt.Errorf("Step properties are required.")
			}
			if m.StepId == "" {
				return fmt.Errorf("Step ID is required.")
			}
			step.Properties = m.Step
			step.Started = m.Timestamp
		case td.MSG_TYPE_STEP_FINISHED:
			// Set the finished time.
			step.Finished = m.Timestamp
			// Set the step result if it isn't set already. If the
			// STEP_FINISHED message arrives before STEP_FAILED, the step
			// will have the wrong result until we process STEP_FAILED.
			if step.Result == "" {
				step.Result = td.STEP_RESULT_SUCCESS
			}
		case td.MSG_TYPE_STEP_DATA:
			step.Data = append(step.Data, &StepData{
				Type: m.DataType,
				Data: m.Data,
			})
		case td.MSG_TYPE_STEP_FAILED:
			// TODO(borenet): If we have both a failure and an exception for
			// the same step, we'll have a race condition depending on what
			// order the messages arrive in.
			step.Result = td.STEP_RESULT_FAILURE
			if m.Error != "" {
				step.Errors = append(step.Errors, m.Error)
			}
		case td.MSG_TYPE_STEP_EXCEPTION:
			// TODO(borenet): If we have both a failure and an exception for
			// the same step, we'll have a race condition depending on what
			// order the messages arrive in.
			step.Result = td.STEP_RESULT_EXCEPTION
			if m.Error != "" {
				step.Errors = append(step.Errors, m.Error)
			}
		}
		return nil
	})
}
