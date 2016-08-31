// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package model

import (
	"fmt"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/clock"
	bf "github.com/luci/luci-go/common/data/bit_field"
	google_pb "github.com/luci/luci-go/common/proto/google"
	dm "github.com/luci/luci-go/dm/api/service/v1"
)

// AttemptRetryState indicates the current state of the Attempt's retry
// counters.
type AttemptRetryState struct {
	Failed   uint32
	Expired  uint32
	TimedOut uint32
	Crashed  uint32
}

// Reset resets all of the AttemptRetryState counters.
func (a *AttemptRetryState) Reset() {
	*a = AttemptRetryState{}
}

// Attempt is the datastore model for a DM Attempt. It has no parent key, but
// it may have the following children entities:
//   * FwdDep
//   * AttemptResult
//
// Additionally, every Attempt has an associated BackDepGroup whose ID equals
// the ID of this Attempt.
type Attempt struct {
	ID dm.Attempt_ID `gae:"$id"`

	Created  time.Time
	Modified time.Time

	State      dm.Attempt_State
	RetryState AttemptRetryState

	// IsAbnormal is true iff State==ABNORMAL_FINISHED, used for walk_graph.
	IsAbnormal bool

	// A lazily-updated boolean to reflect that this Attempt is expired for
	// queries.
	IsExpired bool

	// Contains either data (State==FINISHED) or abnormal_finish (State==ABNORMAL_FINISHED)
	//
	// Does not contain the `data.object` field (which is in the AttemptResult,1 object)
	Result dm.Result `gae:",noindex"`

	// TODO(iannucci): Use CurExecution as a 'deps block version'
	// then we can have an 'ANY' directive which executes the attempt as soon
	// as any of the dependencies are ready. If it adds more deps in ANY mode,
	// the bitmaps get /extended/, and new deps bit indices are added to the
	// existing max.
	// If it adds more deps in ALL mode, it just converts from ANY -> ALL mode
	// and follows the current behavior.

	// CurExecution is the maximum Execution ID for this Attempt so far. Execution
	// IDs are contiguous from [1, CurExecution]. If the State is not currently
	// Executing, then CurExecution represents the execution that JUST finished
	// (or 0 if no Executions have been made yet).
	CurExecution uint32

	// LastSuccessfulExecution is the execution ID of the last successful
	// execution, or 0 if no such execution occured yet.
	LastSuccessfulExecution uint32

	// DepMap is valid only while Attempt is in a State of EXECUTING or WAITING.
	//
	// The size of this field is inspected to deteremine what the next state after
	// EXECUTING is. If the size == 0, it means the Attempt should move to the
	// FINISHED state. Otherwise it means that the Attempt should move to the
	// WAITING state.
	//
	// A bit field value of 0 means that the dep is currently waiting, and a bit
	// value of 1 means that the coresponding dep is satisfined. The Attempt can
	// be unblocked from WAITING back to SCHEDULING when all bits are set to 1.
	DepMap bf.BitField `gae:",noindex" json:"-"`
}

// MakeAttempt is a convenience function to create a new Attempt model in
// the NeedsExecution state.
func MakeAttempt(c context.Context, aid *dm.Attempt_ID) *Attempt {
	now := clock.Now(c).UTC()
	return &Attempt{
		ID:       *aid,
		Created:  now,
		Modified: now,
	}
}

// ModifyState changes the current state of this Attempt and updates its
// Modified timestamp.
func (a *Attempt) ModifyState(c context.Context, newState dm.Attempt_State) error {
	if a.State == newState {
		return nil
	}
	if err := a.State.Evolve(newState); err != nil {
		return err
	}
	now := clock.Now(c).UTC()
	if now.After(a.Modified) {
		a.Modified = now
	} else {
		// Microsecond is the smallest granularity that datastore can store
		// timestamps, so use that to disambiguate: the goal here is that any
		// modification always increments the modified time, and never decrements
		// it.
		a.Modified = a.Modified.Add(time.Microsecond)
	}
	return nil
}

// ToProto returns a dm proto version of this Attempt.
func (a *Attempt) ToProto(withData bool) *dm.Attempt {
	ret := dm.Attempt{Id: &a.ID}
	if withData {
		ret.Data = a.DataProto()
	}
	return &ret
}

// DataProto returns an Attempt.Data message for this Attempt.
func (a *Attempt) DataProto() (ret *dm.Attempt_Data) {
	switch a.State {
	case dm.Attempt_SCHEDULING:
		ret = dm.NewAttemptScheduling().Data
	case dm.Attempt_EXECUTING:
		ret = dm.NewAttemptExecuting(a.CurExecution).Data
	case dm.Attempt_WAITING:
		ret = dm.NewAttemptWaiting(a.DepMap.Size() - a.DepMap.CountSet()).Data
	case dm.Attempt_FINISHED:
		ret = dm.NewAttemptFinished(a.Result.Data).Data
	case dm.Attempt_ABNORMAL_FINISHED:
		ret = dm.NewAttemptAbnormalFinish(a.Result.AbnormalFinish).Data
	default:
		panic(fmt.Errorf("unknown Attempt_State: %s", a.State))
	}
	ret.Created = google_pb.NewTimestamp(a.Created)
	ret.Modified = google_pb.NewTimestamp(a.Modified)
	ret.NumExecutions = a.CurExecution
	return ret
}
