// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package dm

import (
	"time"

	google_pb "github.com/luci/luci-go/common/proto/google"
)

// NewJsonResult creates a new JsonResult object with optional expiration time.
func NewJsonResult(data string, exps ...time.Time) *JsonResult {
	exp := time.Time{}
	switch l := len(exps); {
	case l == 1:
		exp = exps[0]
	case l > 1:
		panic("too many exps")
	}
	return &JsonResult{data, uint32(len(data)), google_pb.NewTimestamp(exp)}
}

// NewDatalessJsonResult creates a new JsonResult object without data and with
// optional expiration time.
func NewDatalessJsonResult(size uint32, exps ...time.Time) *JsonResult {
	exp := time.Time{}
	switch l := len(exps); {
	case l == 1:
		exp = exps[0]
	case l > 1:
		panic("too many exps")
	}
	return &JsonResult{"", size, google_pb.NewTimestamp(exp)}
}

// NewAttemptScheduling creates an Attempt in the SCHEDULING state.
func NewAttemptScheduling() *Attempt {
	return &Attempt{
		Data: &Attempt_Data{
			AttemptType: &Attempt_Data_Scheduling_{
				&Attempt_Data_Scheduling{}}}}
}

// NewAttemptExecuting creates an Attempt in the EXECUTING state.
func NewAttemptExecuting(curExID uint32) *Attempt {
	return &Attempt{
		Data: &Attempt_Data{
			NumExecutions: curExID,
			AttemptType: &Attempt_Data_Executing_{
				&Attempt_Data_Executing{curExID}}}}
}

// NewAttemptWaiting creates an Attempt in the WAITING state.
func NewAttemptWaiting(numWaiting uint32) *Attempt {
	return &Attempt{
		Data: &Attempt_Data{
			AttemptType: &Attempt_Data_Waiting_{
				&Attempt_Data_Waiting{numWaiting}}}}
}

// NewAttemptFinished creates an Attempt in the FINISHED state.
func NewAttemptFinished(result *JsonResult) *Attempt {
	return &Attempt{
		Data: &Attempt_Data{
			AttemptType: &Attempt_Data_Finished_{
				&Attempt_Data_Finished{result}}}}
}

// NewAttemptAbnormalFinish creates an Attempt in the ABNORMAL_FINISH state.
func NewAttemptAbnormalFinish(af *AbnormalFinish) *Attempt {
	return &Attempt{
		Data: &Attempt_Data{
			AttemptType: &Attempt_Data_AbnormalFinish{af}}}
}

// State computes the Attempt_State for the current Attempt_Data
func (d *Attempt_Data) State() Attempt_State {
	if d != nil {
		switch d.AttemptType.(type) {
		case *Attempt_Data_Scheduling_:
			return Attempt_SCHEDULING
		case *Attempt_Data_Executing_:
			return Attempt_EXECUTING
		case *Attempt_Data_Waiting_:
			return Attempt_WAITING
		case *Attempt_Data_Finished_:
			return Attempt_FINISHED
		case *Attempt_Data_AbnormalFinish:
			return Attempt_ABNORMAL_FINISHED
		}
	}
	return Attempt_SCHEDULING
}

// NormalizePartial will nil out the Partial field for this Attempt if all
// Partial fields are false.
func (d *Attempt) NormalizePartial() {
	p := d.GetPartial()
	if p == nil {
		return
	}
	if !(p.Data || p.Executions || p.FwdDeps || p.BackDeps ||
		p.Result != Attempt_Partial_LOADED) {
		d.Partial = nil
	}
}

// Any returns true iff any of the Partial fields are true such that they could
// be successfully loaded on a subsequent query.
func (p *Attempt_Partial) Any() bool {
	return (p.BackDeps || p.Data || p.Executions || p.FwdDeps ||
		p.Result == Attempt_Partial_NOT_LOADED)
}
