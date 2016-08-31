// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package model

import (
	"github.com/luci/gae/service/datastore"

	"github.com/luci/luci-go/dm/api/service/v1"
)

// BackDepGroup describes a group of reverse dependencies ('depended-by')
// between Attempts. Its ID is the same as the id of the Attempt that's being
// depended-on by other attempts, and it serves as the parent entity for the
// BackDep model. So:
//
//   Attempt(OTHER_QUEST|2)
//     FwdDep(QUEST|1)
//
//   Attempt(QUEST|1)
//
//   BackDepGroup(QUEST|1)
//     BackDep(OTHER_QUEST|2)
//
// Represents the OTHER_QUEST|2 depending on QUEST|1.
type BackDepGroup struct {
	// Dependee is the "<AttemptID>" that the deps in this group point
	// back FROM.
	Dependee dm.Attempt_ID `gae:"$id"`

	// This is a denormalized version of Attempt.State, used to allow
	// transactional additions to the BackDepGroup to stay within this Entity
	// Group when adding new back deps.
	AttemptFinished bool
}

// BackDep represents a single backwards dependency. Its ID is the same as the
// Attempt that's depending on this one. See BackDepGroup for more context.
type BackDep struct {
	// The attempt id of the attempt that's depending on this dependee.
	Depender dm.Attempt_ID `gae:"$id"`

	// The BackdepGroup for the attempt that is being depended on.
	DependeeGroup *datastore.Key `gae:"$parent"`

	// Propagated is true if the BackDepGroup has AttemptFinished, and this
	// BackDep has been processed by the mutate.RecordCompletion tumble
	// mutation. So if with two attempts A and B, A depends on B, the
	// BackDep{DependeeGroup: B, Depender: A} has Propagated as true when B is
	// finished, and a tumble Mutation has been launched to inform A of that fact.
	Propagated bool
}

// Edge produces a fwdedge object which points from the depending attempt to
// the depended-on attempt.
func (b *BackDep) Edge() *FwdEdge {
	ret := &FwdEdge{From: &b.Depender, To: &dm.Attempt_ID{}}
	if err := ret.To.SetDMEncoded(b.DependeeGroup.StringID()); err != nil {
		panic(err)
	}
	return ret
}
