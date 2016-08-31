// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package mutate

import (
	"google.golang.org/grpc/codes"

	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/logging"
	dm "github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/dm/appengine/model"
	"github.com/luci/luci-go/grpc/grpcutil"
	"github.com/luci/luci-go/tumble"
	"golang.org/x/net/context"
)

// FinishAttempt does a couple things:
//   Invalidates the current Execution
//   Moves the state to Finished
//   Creates a new AttemptResult
//   Starts RecordCompletion state machine.
type FinishAttempt struct {
	dm.FinishAttemptReq
}

// Root implements tumble.Mutation
func (f *FinishAttempt) Root(c context.Context) *datastore.Key {
	return model.AttemptKeyFromID(c, f.Auth.Id.AttemptID())
}

// RollForward implements tumble.Mutation
//
// This mutation is called directly from FinishAttempt.
func (f *FinishAttempt) RollForward(c context.Context) (muts []tumble.Mutation, err error) {
	atmpt, ex, err := model.InvalidateExecution(c, f.Auth)
	if err != nil {
		logging.WithError(err).Errorf(c, "could not invalidate execution")
		return
	}

	if err = ResetExecutionTimeout(c, ex); err != nil {
		logging.WithError(err).Errorf(c, "could not reset timeout")
		return
	}

	ar := &model.AttemptResult{
		Attempt: model.AttemptKeyFromID(c, &atmpt.ID),
		Data:    *f.Data,
	}

	rslt := *f.Data
	atmpt.Result.Data = &rslt
	atmpt.Result.Data.Object = ""

	ds := datastore.Get(c)
	err = grpcutil.Annotate(ds.Put(atmpt, ar), codes.Internal).Reason("during Put").Err()
	return
}

func init() {
	tumble.Register((*FinishAttempt)(nil))
}
