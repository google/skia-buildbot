package display

/*
   The display package provides utilities for displaying Task Drivers in a
   human-friendly way.
*/

import (
	"fmt"
	"sort"
	"strings"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_driver/go/db"
	"go.skia.org/infra/task_driver/go/td"
)

const (
	ELLIPSES        = "..."
	MAX_ERROR_CHARS = 1000
	MAX_ERROR_LINES = 20
)

// StepDisplay represents one step in a single run of a Task Driver.
type StepDisplay struct {
	*td.StepProperties
	Result   td.StepResult `json:"result,omitempty"`
	Errors   []string      `json:"errors,omitempty"`
	Started  time.Time     `json:"started,omitempty"`
	Finished time.Time     `json:"finished,omitempty"`

	Data []*db.StepData `json:"data,omitempty"`

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
	Id         string            `json:"id"`
	Properties *td.RunProperties `json:"properties"`
	*StepDisplay
}

// TaskDriverForDisplay converts a db.TaskDriver into a TaskDriver, which is
// more human-friendly for display purposes.
func TaskDriverForDisplay(t *db.TaskDriverRun) (*TaskDriverRunDisplay, error) {
	rv := &TaskDriverRunDisplay{
		Id:         t.TaskId,
		Properties: t.Properties.Copy(),
	}

	// Create each StepDisplay.
	steps := make(map[string]*StepDisplay, len(t.Steps))
	var helper func(string) error
	helper = func(id string) error {
		if _, ok := steps[id]; ok {
			return nil
		}
		var errs error // Will hold non-critical errors.
		orig, ok := t.Steps[id]
		if !ok {
			// We should do our best to display anyway. Store the
			// error but keep going.
			errs = multierror.Append(errs, fmt.Errorf("Unknown step %s", id))
		}
		var data []*db.StepData
		if len(orig.Data) > 0 {
			for _, d := range orig.Data {
				// TODO(borenet): We should try to deep-copy the data.
				data = append(data, d)
			}
		}
		var errorMsgs []string
		for _, errMsg := range orig.Errors {
			errorMsgs = append(errorMsgs, truncateError(errMsg))
		}
		s := &StepDisplay{
			StepProperties: orig.Properties.Copy(),
			Result:         orig.Result,
			Errors:         errorMsgs,
			Started:        orig.Started,
			Finished:       orig.Finished,
			Data:           data,
			Steps:          []*StepDisplay{},
		}
		if orig.Properties.Parent != "" {
			if err := helper(orig.Properties.Parent); err != nil {
				// We should do our best to display anyway.
				// Store the error but keep going.
				errs = multierror.Append(errs, err)
			}
			// Now parent should be in the steps map.
			parent, ok := steps[orig.Properties.Parent]
			if !ok {
				// If our parent isn't in the steps map, then
				// don't bother trying to display this step.
				return multierror.Append(errs, fmt.Errorf("Parent step %s is not present!", orig.Properties.Parent))
			}
			parent.Steps = append(parent.Steps, s)
		}
		steps[s.Id] = s
		return errs
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

// truncateError shortens an error message, returning the last part, so as not
// to transfer giant logs to the client. There is an endpoint to retrieve a
// specific error which will return the whole thing.
func truncateError(err string) string {
	lines := strings.Split(err, "\n")
	// Logs often end in a newline; trim any empty line.
	if len(lines) > 0 {
		if lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
	}
	if len(lines) > MAX_ERROR_LINES {
		lines = lines[len(lines)-MAX_ERROR_LINES:]
		lines[0] = ELLIPSES + lines[0]
	}
	rv := strings.Join(lines, "\n")
	if len(rv) > MAX_ERROR_CHARS {
		rv = ELLIPSES + rv[len(rv)-MAX_ERROR_CHARS+len(ELLIPSES):]
	}
	sklog.Infof("Truncating error message of length %d to %d", len(err), len(rv))
	return rv
}
