// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package distributor

import (
	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/dm/appengine/model"
	"github.com/luci/luci-go/tumble"
	"golang.org/x/net/context"
)

// NotifyExecution is used to finish an execution. Specifically it allows the
// appropriate distributor to HandleNotification, and then when that concludes,
// invokes DM's FinishExecution (see mutate.FinishExecution).
type NotifyExecution struct {
	CfgName      string
	Notification *Notification
}

// Root implements tumble.Mutation.
func (f *NotifyExecution) Root(c context.Context) *datastore.Key {
	return model.ExecutionKeyFromID(c, f.Notification.ID)
}

// RollForward implements tumble.Mutation.
func (f *NotifyExecution) RollForward(c context.Context) (muts []tumble.Mutation, err error) {
	reg := GetRegistry(c)
	dist, _, err := reg.MakeDistributor(c, f.CfgName)
	if err != nil {
		logging.Fields{
			logging.ErrorKey: err,
			"cfg":            f.CfgName,
		}.Errorf(c, "Failed to make distributor")
		return
	}
	rslt, err := dist.HandleNotification(f.Notification)
	if err != nil {
		// TODO(riannucci): check for transient/non-transient
		logging.Fields{
			logging.ErrorKey: err,
			"cfg":            f.CfgName,
		}.Errorf(c, "Failed to handle notification")
		return
	}
	if rslt != nil {
		return reg.FinishExecution(c, f.Notification.ID, rslt)
	}
	return
}

func init() {
	tumble.Register((*NotifyExecution)(nil))
}
