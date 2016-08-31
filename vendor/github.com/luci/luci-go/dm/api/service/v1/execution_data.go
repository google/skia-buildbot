// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package dm

import "errors"

// NewExecutionScheduling creates an Execution in the SCHEDULING state.
func NewExecutionScheduling() *Execution {
	return &Execution{
		Data: &Execution_Data{
			ExecutionType: &Execution_Data_Scheduling_{
				&Execution_Data_Scheduling{}}}}
}

// NewExecutionRunning creates an Execution in the RUNNING state.
func NewExecutionRunning() *Execution {
	return &Execution{
		Data: &Execution_Data{
			ExecutionType: &Execution_Data_Running_{
				&Execution_Data_Running{}}}}
}

// NewExecutionStopping creates an Execution in the STOPPING state.
func NewExecutionStopping() *Execution {
	return &Execution{
		Data: &Execution_Data{
			ExecutionType: &Execution_Data_Stopping_{
				&Execution_Data_Stopping{}}}}
}

// NewExecutionFinished creates an Execution in the FINISHED state.
func NewExecutionFinished(result *JsonResult) *Execution {
	return &Execution{
		Data: &Execution_Data{
			ExecutionType: &Execution_Data_Finished_{
				&Execution_Data_Finished{result}}}}
}

// NewExecutionAbnormalFinish creates an Execution in the ABNORMAL_FINISH state.
func NewExecutionAbnormalFinish(af *AbnormalFinish) *Execution {
	return &Execution{
		Data: &Execution_Data{
			ExecutionType: &Execution_Data_AbnormalFinish{af}}}
}

// State computes the Execution_State for the current Execution_Data
func (d *Execution_Data) State() Execution_State {
	if d != nil {
		switch d.ExecutionType.(type) {
		case *Execution_Data_Scheduling_:
			return Execution_SCHEDULING
		case *Execution_Data_Running_:
			return Execution_RUNNING
		case *Execution_Data_Stopping_:
			return Execution_STOPPING
		case *Execution_Data_Finished_:
			return Execution_FINISHED
		case *Execution_Data_AbnormalFinish:
			return Execution_ABNORMAL_FINISHED
		}
	}
	return Execution_SCHEDULING
}

// Normalize returns an error iff the Execution_Auth has invalid form (e.g.
// contains nils).
func (a *Execution_Auth) Normalize() error {
	if a == nil {
		return errors.New("Execution_Auth is nil")
	}
	if a.Id == nil {
		return errors.New("Execution_Auth.Id is nil")
	}
	return nil
}
