// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package jobsim

import (
	"encoding/json"
	"time"

	"github.com/golang/protobuf/jsonpb"

	"github.com/luci/luci-go/dm/api/distributor/jobsim"
	"github.com/luci/luci-go/dm/api/service/v1"
)

type jobsimStatus string

const (
	jobsimRunnable  jobsimStatus = "runnable"
	jobsimRunning   jobsimStatus = "running"
	jobsimFailed    jobsimStatus = "failed"
	jobsimCancelled jobsimStatus = "cancelled"
	jobsimFinished  jobsimStatus = "finished"
)

type jobsimExecution struct {
	_  string `gae:"$kind,jobsim.Task"`
	ID string `gae:"$id"`

	Status        jobsimStatus `gae:",noindex"`
	StateOrReason string       `gae:",noindex"`

	ExAuth dm.Execution_Auth `gae:",noindex"`

	Calculation jobsim.Phrase `gae:",noindex"`
	CfgName     string        `gae:",noindex"`
}

func getAttemptResult(status jobsimStatus, stateOrReason string) *dm.Result {
	switch status {
	case jobsimRunnable, jobsimRunning:
		return nil

	case jobsimFinished:
		return &dm.Result{
			Data: dm.NewJsonResult(stateOrReason)}
	}

	tr := &dm.Result{AbnormalFinish: &dm.AbnormalFinish{
		Reason: stateOrReason}}
	switch status {
	case jobsimFailed:
		tr.AbnormalFinish.Status = dm.AbnormalFinish_FAILED
	case jobsimCancelled:
		tr.AbnormalFinish.Status = dm.AbnormalFinish_CANCELLED
	}
	return tr
}

func executionResult(success bool, value int64, exp time.Time) *dm.JsonResult {
	data, err := (&jsonpb.Marshaler{}).MarshalToString(&jobsim.Result{
		Success: success, Value: value})
	if err != nil {
		panic(err)
	}
	return dm.NewJsonResult(data, exp)
}

func executionResultFromJSON(data *dm.JsonResult) (ret *jobsim.Result, err error) {
	ret = &jobsim.Result{}
	err = jsonpb.UnmarshalString(data.Object, ret)
	return
}

type notification struct {
	Status        jobsimStatus
	StateOrReason string
}

func (n *notification) toJSON() []byte {
	ret, err := json.Marshal(n)
	if err != nil {
		panic(err)
	}
	return ret
}
