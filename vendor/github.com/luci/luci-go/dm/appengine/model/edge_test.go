// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package model

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"

	"github.com/luci/gae/impl/memory"
	"github.com/luci/gae/service/datastore"

	"github.com/luci/luci-go/dm/api/service/v1"
)

func TestBackdepEdge(t *testing.T) {
	t.Parallel()

	Convey("BackDep.Edge", t, func() {
		bd := &BackDep{
			*dm.NewAttemptID("depender", 5),
			datastore.MakeKey("aid", "ns", "BackDepGroup", "quest|fffffffe"),
			true,
		}
		So(bd.Edge(), ShouldResemble, &FwdEdge{
			dm.NewAttemptID("depender", 5),
			dm.NewAttemptID("quest", 1),
		})
	})
}

func TestFwdDepEdge(t *testing.T) {
	t.Parallel()

	Convey("FwdDep.Edge", t, func() {
		bd := &FwdDep{
			Dependee: *dm.NewAttemptID("quest", 1),
			Depender: datastore.MakeKey("aid", "ns", "Attempt", "depender|fffffffa"),
		}
		So(bd.Edge(), ShouldResemble, &FwdEdge{
			dm.NewAttemptID("depender", 5),
			dm.NewAttemptID("quest", 1),
		})
	})
}

func TestFwdEdge(t *testing.T) {
	t.Parallel()

	Convey("FwdEdge", t, func() {
		c := memory.Use(context.Background())
		e := &FwdEdge{
			dm.NewAttemptID("from", 1),
			dm.NewAttemptID("to", 1),
		}

		Convey("Fwd", func() {
			atmpt, fwddep := e.Fwd(c)
			So(atmpt.ID.Quest, ShouldEqual, "from")
			So(atmpt.ID.Id, ShouldEqual, 1)
			So(fwddep.Dependee.Quest, ShouldEqual, "to")
			So(fwddep.Dependee.Id, ShouldEqual, 1)
			So(fwddep.Depender.String(), ShouldEqual, `dev~app::/Attempt,"from|fffffffe"`)
		})

		Convey("Back", func() {
			bdg, bdep := e.Back(c)
			So(bdg.Dependee.Quest, ShouldEqual, "to")
			So(bdg.Dependee.Id, ShouldEqual, 1)
			So(bdep.Depender.Quest, ShouldEqual, "from")
			So(bdep.Depender.Id, ShouldEqual, 1)
			So(bdep.DependeeGroup.String(), ShouldEqual, `dev~app::/BackDepGroup,"to|fffffffe"`)
		})

	})
}

func TestFwdDepsFromList(t *testing.T) {
	t.Parallel()

	Convey("FwdDepsFromList", t, func() {
		c := memory.Use(context.Background())
		list := &dm.AttemptList{To: map[string]*dm.AttemptList_Nums{
			"a": {Nums: []uint32{1}},
			"b": {Nums: []uint32{1, 2}},
			"c": {Nums: []uint32{1}},
		}}
		base := dm.NewAttemptID("quest", 1)

		ds := datastore.Get(c)

		root := ds.MakeKey("Attempt", "quest|fffffffe")

		So(FwdDepsFromList(c, base, list), ShouldResemble, []*FwdDep{
			{Depender: root, Dependee: *dm.NewAttemptID("a", 1), BitIndex: 0},
			{Depender: root, Dependee: *dm.NewAttemptID("b", 1), BitIndex: 1},
			{Depender: root, Dependee: *dm.NewAttemptID("b", 2), BitIndex: 2},
			{Depender: root, Dependee: *dm.NewAttemptID("c", 1), BitIndex: 3},
		})
		So(FwdDepKeysFromList(c, base, list), ShouldResemble, []*datastore.Key{
			ds.MakeKey("Attempt", "quest|fffffffe", "FwdDep", "a|fffffffe"),
			ds.MakeKey("Attempt", "quest|fffffffe", "FwdDep", "b|fffffffe"),
			ds.MakeKey("Attempt", "quest|fffffffe", "FwdDep", "b|fffffffd"),
			ds.MakeKey("Attempt", "quest|fffffffe", "FwdDep", "c|fffffffe"),
		})
	})

}
