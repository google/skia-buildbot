// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package model

import (
	"golang.org/x/net/context"

	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/dm/api/service/v1"
)

// FwdEdge represents a forward-dependency from one attempt to another. The
// From attempt will block until the To attempt completes.
type FwdEdge struct {
	From *dm.Attempt_ID
	To   *dm.Attempt_ID
}

// Fwd returns the Attempt (From) and FwdDep (To) models that this FwdEdge
// represents.
func (e *FwdEdge) Fwd(c context.Context) (*Attempt, *FwdDep) {
	atmpt := &Attempt{ID: *e.From}
	return atmpt, &FwdDep{
		Depender: datastore.Get(c).KeyForObj(atmpt),
		Dependee: *e.To,
	}
}

// Back returns the BackDepGroup (To) and the BackDep (From) models that
// represent the reverse dependency for this FwdEdge.
func (e *FwdEdge) Back(c context.Context) (*BackDepGroup, *BackDep) {
	bdg := &BackDepGroup{Dependee: *e.To}
	return bdg, &BackDep{
		DependeeGroup: datastore.Get(c).KeyForObj(bdg),
		Depender:      *e.From,
	}
}
