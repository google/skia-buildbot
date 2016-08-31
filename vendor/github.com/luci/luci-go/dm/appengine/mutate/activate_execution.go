// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package mutate

import (
	"golang.org/x/net/context"

	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/dm/appengine/model"
	"github.com/luci/luci-go/tumble"
)

// ActivateExecution executes an execution, moving it from the
// SCHEDULING->RUNNING state, and resetting the execution timeout (if any).
type ActivateExecution struct {
	Auth   *dm.Execution_Auth
	NewTok []byte
}

// Root implements tumble.Mutation.
func (a *ActivateExecution) Root(c context.Context) *datastore.Key {
	return model.AttemptKeyFromID(c, a.Auth.Id.AttemptID())
}

// RollForward implements tumble.Mutation
func (a *ActivateExecution) RollForward(c context.Context) (muts []tumble.Mutation, err error) {
	_, e, err := model.ActivateExecution(c, a.Auth, a.NewTok)
	if err == nil {
		err = ResetExecutionTimeout(c, e)
	}
	return
}

func init() {
	tumble.Register((*ActivateExecution)(nil))
}
