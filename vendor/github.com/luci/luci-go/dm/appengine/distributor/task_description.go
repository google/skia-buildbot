// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package distributor

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/luci/gae/service/info"
	"github.com/luci/luci-go/common/gcloud/pubsub"
	dm "github.com/luci/luci-go/dm/api/service/v1"
	"golang.org/x/net/context"
)

// NewTaskDescription builds a new *TaskDescription.
//
// It's intended for use by the DM core logic, and not for use by distributor
// implementations.
func NewTaskDescription(c context.Context, payload *dm.Quest_Desc, exAuth *dm.Execution_Auth,
	previousResult *dm.JsonResult) *TaskDescription {
	return &TaskDescription{
		c:              c,
		payload:        payload,
		executionAuth:  exAuth,
		previousResult: previousResult,
	}
}

// TaskDescription is the parameters for PrepareTask.
type TaskDescription struct {
	c              context.Context
	payload        *dm.Quest_Desc
	executionAuth  *dm.Execution_Auth
	previousResult *dm.JsonResult
}

// PrepareTopic returns the pubsub topic that notifications should be sent to.
//
// It returns the full name of the topic and a token that will be used to route
// PubSub messages back to the Distributor. The publisher to the topic must be
// instructed to put the token into the 'auth_token' attribute of PubSub
// messages. DM will know how to route such messages to D.HandleNotification.
func (t *TaskDescription) PrepareTopic() (topic pubsub.Topic, token string, err error) {
	topic = pubsub.NewTopic(info.Get(t.c).TrimmedAppID(), notifyTopicSuffix)
	if err := topic.Validate(); err != nil {
		panic(fmt.Errorf("failed to validate Topic %q: %s", topic, err))
	}
	token, err = encodeAuthToken(t.c, t.executionAuth.Id,
		t.payload.DistributorConfigName)
	return
}

// PreviousResult is the Result of the last successful Execution for the
// Attempt. This will be nil for the first Execution.
func (t *TaskDescription) PreviousResult() *dm.JsonResult {
	if t.previousResult == nil {
		return nil
	}
	return proto.Clone(t.previousResult).(*dm.JsonResult)
}

// Payload is description of the job to run.
func (t *TaskDescription) Payload() *dm.Quest_Desc {
	return proto.Clone(t.payload).(*dm.Quest_Desc)
}

// ExecutionAuth is the combined execution_id+activation token that the
// execution must use to call ActivateExecution before making further API calls
// into DM.
func (t *TaskDescription) ExecutionAuth() *dm.Execution_Auth {
	ret := *t.executionAuth
	return &ret
}
