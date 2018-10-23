package display

/*
   The display package provides utilities for displaying Task Drivers in a
   human-friendly way.
*/

import (
	"fmt"
	"sort"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_driver/go/db"
	"go.skia.org/infra/task_driver/go/td"
)

// StepDisplay represents one step in a single run of a Task Driver.
type StepDisplay struct {
	*td.StepProperties
	Result   td.StepResult  `json:"result,omitempty"`
	Errors   []string       `json:"errors,omitempty"`
	Started  time.Time      `json:"started,omitempty"`
	Finished time.Time      `json:"finished,omitempty"`
	Data     []*db.StepData `json:"data,omitempty"`

	Steps []*StepDisplay `json:"steps,omitempty"`
}

// StepSlice is a helper for sorting.
type StepSlice []*StepDisplay

func (s StepSlice) Len() int {
	return len(s)
}

func (s StepSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s StepSlice) Less(i, j int) bool {
	return s[i].Started.Before(s[j].Started)
}

// TaskDriverRunDisplay represents a single run of a Task Driver.
type TaskDriverRunDisplay struct {
	Id string `json:"id"`
	*StepDisplay
}

// TaskDriverForDisplay converts a db.TaskDriver into a TaskDriver, which is
// more human-friendly for display purposes.
func TaskDriverForDisplay(t *db.TaskDriverRun) (*TaskDriverRunDisplay, error) {
	rv := &TaskDriverRunDisplay{
		Id: t.TaskId,
	}

	// Create each StepDisplay.
	sklog.Infof("TaskDriver has %d steps", len(t.Steps))
	steps := make(map[string]*StepDisplay, len(t.Steps))
	var helper func(string) error
	helper = func(id string) error {
		if _, ok := steps[id]; ok {
			return nil
		}
		sklog.Infof("Creating step %s", id)
		orig, ok := t.Steps[id]
		if !ok {
			// TODO(borenet): We should do our best to display anyway.
			return fmt.Errorf("Unknown step %s", id)
		}
		var data []*db.StepData
		if orig.Data != nil {
			data = append(make([]*db.StepData, 0, len(orig.Data)), orig.Data...)
		}
		s := &StepDisplay{
			StepProperties: orig.Properties.Copy(),
			Result:         orig.Result,
			Started:        orig.Started,
			Finished:       orig.Finished,
			Data:           data,
			Steps:          []*StepDisplay{},
		}
		if orig.Properties.Parent != "" {
			if err := helper(orig.Properties.Parent); err != nil {
				// If our parent step is missing, don't bother
				// trying to display this step.
				return err
			}
			// Now parent should be in the steps map.
			parent, ok := steps[orig.Properties.Parent]
			if !ok {
				return fmt.Errorf("Already processed %s but it is not present!", orig.Properties.Parent)
			}
			parent.Steps = append(parent.Steps, s)
		}
		steps[s.Id] = s
		return nil
	}
	for _, s := range t.Steps {
		if s.Properties != nil {
			if err := helper(s.Properties.Id); err != nil {
				// The error will be non-nil if any step is missing;
				// we want to do our best to display even if the pubsub
				// messages arrived out of order and we received a
				// message about a child step before its parent, so just
				// ignore the error.
				sklog.Infof("Error when gathering steps: %s", err)
				continue
			}
			if s.Properties.Id == td.STEP_ID_ROOT {
				sklog.Info("This is the root step!")
				rv.StepDisplay = steps[s.Properties.Id]
			}
		}
	}

	// Sort all steps by start time.
	var sortHelper func(*StepDisplay)
	sortHelper = func(s *StepDisplay) {
		sort.Sort(StepSlice(s.Steps))
		for _, child := range s.Steps {
			sortHelper(child)
		}
	}
	if rv.StepDisplay != nil {
		sortHelper(rv.StepDisplay)
	}

	return rv, nil
}
