// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package mutate

import (
	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/dm/appengine/model"
	"github.com/luci/luci-go/tumble"
	"golang.org/x/net/context"
)

// Attempt to complete attempts 64-at-a-time. Rely on tumble's
// tail-call optimization to save on transactions.
const completionLimit = 64

// RecordCompletion marks that fact that an Attempt is completed (Finished) on
// its corresponding BackDepGroup, and fires off additional AckFwdDep mutations
// for each incoming dependency that is blocked.
//
// In the case where an Attempt has hundreds or thousands of incoming
// dependencies, the naive implementation of this mutation could easily overfill
// a single datastore transaction. For that reason, the implementation here
// unblocks things 64 edges at a time, and keeps returning itself as a mutation
// until it unblocks less than 64 things (e.g. it does a tail-call).
//
// This relies on tumble's tail-call optimization to be performant in terms of
// the number of transactions, otherwise this would take 1 transaction per
// 64 dependencies. With the TCO, it could do hundreds or thousands of
// dependencies, but it will also be fair to other work (e.g. it will allow
// other Attempts to take dependencies on this Attempt while RecordCompletion
// is in between tail-calls).
type RecordCompletion struct {
	For *dm.Attempt_ID
}

// Root implements tumble.Mutation.
func (r *RecordCompletion) Root(c context.Context) *datastore.Key {
	return datastore.Get(c).KeyForObj(&model.BackDepGroup{Dependee: *r.For})
}

// RollForward implements tumble.Mutation.
func (r *RecordCompletion) RollForward(c context.Context) (muts []tumble.Mutation, err error) {
	ds := datastore.Get(c)

	bdg := &model.BackDepGroup{Dependee: *r.For}
	if err = ds.Get(bdg); err != nil && err != datastore.ErrNoSuchEntity {
		return
	}

	needProp := make([]*model.BackDep, 0, completionLimit)

	q := (datastore.NewQuery("BackDep").
		Ancestor(ds.KeyForObj(bdg)).
		Eq("Propagated", false).
		Limit(completionLimit))

	if err = ds.GetAll(q, &needProp); err != nil {
		return
	}

	if len(needProp) > 0 {
		muts = make([]tumble.Mutation, len(needProp))

		for i, bdep := range needProp {
			bdep.Propagated = true
			muts[i] = &AckFwdDep{bdep.Edge()}
		}

		if len(needProp) == completionLimit {
			// Append ourself if there might be more to do!
			muts = append(muts, r)
		}

		if err = ds.Put(needProp); err != nil {
			return
		}
	}

	if !bdg.AttemptFinished {
		bdg.AttemptFinished = true
		if err = ds.Put(bdg); err != nil {
			return
		}
	}

	return
}

func init() {
	tumble.Register((*RecordCompletion)(nil))
}
