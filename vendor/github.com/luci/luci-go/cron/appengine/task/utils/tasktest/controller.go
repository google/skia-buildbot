// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tasktest

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/luci/luci-go/cron/appengine/task"
)

// TestController implements task.Controller and can be used in unit tests.
type TestController struct {
	OverrideJobID    string // return value of JobID() if not ""
	OverrideInvID    int64  // return value of InvocationID() if not 0
	OverrideInvNonce int64  // return value of InvocationNonce() if not 0

	TaskMessage proto.Message // return value of Task
	TaskState   task.State    // return value of State(), mutated in place
	Client      *http.Client  // return value by GetClient()
	Log         []string      // individual log lines passed to DebugLog()

	SaveCallback         func() error                         // mock for Save()
	PrepareTopicCallback func(string) (string, string, error) // mock for PrepareTopic()
}

// JobID is part of Controller interface.
func (c *TestController) JobID() string {
	if c.OverrideJobID != "" {
		return c.OverrideJobID
	}
	return "some-project/some-job"
}

// InvocationID is part of Controller interface.
func (c *TestController) InvocationID() int64 {
	if c.OverrideInvID != 0 {
		return c.OverrideInvID
	}
	return 1
}

// InvocationNonce is part of Controller interface.
func (c *TestController) InvocationNonce() int64 {
	if c.OverrideInvNonce != 0 {
		return c.OverrideInvNonce
	}
	return 2
}

// Task is part of Controller interface.
func (c *TestController) Task() proto.Message {
	return c.TaskMessage
}

// State is part of Controller interface.
func (c *TestController) State() *task.State {
	return &c.TaskState
}

// DebugLog is part of Controller interface.
func (c *TestController) DebugLog(format string, args ...interface{}) {
	c.Log = append(c.Log, fmt.Sprintf(format, args...))
}

// Save is part of Controller interface.
func (c *TestController) Save() error {
	if c.SaveCallback != nil {
		return c.SaveCallback()
	}
	return errors.New("Save must not be called (not mocked)")
}

// PrepareTopic is part of Controller interface.
func (c *TestController) PrepareTopic(publisher string) (topic string, token string, err error) {
	if c.PrepareTopicCallback != nil {
		return c.PrepareTopicCallback(publisher)
	}
	return "", "", errors.New("PrepareTopic must not be called (not mocked)")
}

// GetClient is part of Controller interface.
func (c *TestController) GetClient(timeout time.Duration) (*http.Client, error) {
	if c.Client != nil {
		return c.Client, nil
	}
	return nil, errors.New("GetClient must not be called (not mocked)")
}

var _ task.Controller = (*TestController)(nil)
