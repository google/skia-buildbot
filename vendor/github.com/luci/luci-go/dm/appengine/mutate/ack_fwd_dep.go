// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package mutate

import (
	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/dm/appengine/model"
	"github.com/luci/luci-go/tumble"
	"golang.org/x/net/context"
)

// AckFwdDep records the fact that a dependency was completed.
type AckFwdDep struct {
	Dep *model.FwdEdge
}

// Root implements tumble.Mutation.
func (f *AckFwdDep) Root(c context.Context) *datastore.Key {
	return model.AttemptKeyFromID(c, f.Dep.From)
}

// RollForward implements tumble.Mutation.
func (f *AckFwdDep) RollForward(c context.Context) (muts []tumble.Mutation, err error) {
	ds := datastore.Get(c)

	atmpt, fdep := f.Dep.Fwd(c)
	err = ds.Get(atmpt, fdep)
	if err != nil {
		return
	}

	if (atmpt.State != dm.Attempt_EXECUTING && atmpt.State != dm.Attempt_WAITING) || atmpt.CurExecution != fdep.ForExecution {
		logging.Errorf(c, "EARLY EXIT: %s: %s v %s", atmpt.State, atmpt.CurExecution, fdep.ForExecution)
		return
	}

	idx := uint32(fdep.BitIndex)

	if !atmpt.DepMap.IsSet(idx) {
		atmpt.DepMap.Set(idx)

		if atmpt.DepMap.All(true) && atmpt.State == dm.Attempt_WAITING {
			if err = atmpt.ModifyState(c, dm.Attempt_SCHEDULING); err != nil {
				return
			}
			atmpt.DepMap.Reset()
			muts = append(muts, &ScheduleExecution{For: f.Dep.From})
		}

		err = ds.Put(atmpt)
	}

	return
}

func init() {
	tumble.Register((*AckFwdDep)(nil))
}
