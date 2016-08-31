// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package deps

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/luci/gae/service/datastore"
	"github.com/luci/gae/service/datastore/dumper"
	dm "github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/dm/appengine/distributor/fake"
	"github.com/luci/luci-go/dm/appengine/model"
	"github.com/luci/luci-go/dm/appengine/mutate"
	"github.com/luci/luci-go/server/auth"
	"github.com/luci/luci-go/server/auth/authtest"
	"github.com/luci/luci-go/tumble"
	"golang.org/x/net/context"
)

func testSetup() (ttest *tumble.Testing, c context.Context, dist *fake.Distributor, s testDepsServer) {
	ttest, c, dist = fake.Setup(mutate.FinishExecutionFn)
	s = testDepsServer{newDecoratedDeps()}
	return
}

func reader(c context.Context) context.Context {
	return auth.WithState(c, &authtest.FakeState{
		Identity:       "test@example.com",
		IdentityGroups: []string{"reader_group"},
	})
}

func writer(c context.Context) context.Context {
	return auth.WithState(c, &authtest.FakeState{
		Identity:       "test@example.com",
		IdentityGroups: []string{"reader_group", "writer_group"},
	})
}

type testDepsServer struct {
	dm.DepsServer
}

func (s testDepsServer) ensureQuest(c context.Context, name string, aids ...uint32) string {
	desc := fake.QuestDesc(name)
	q := model.NewQuest(c, desc)
	qsts, err := s.EnsureGraphData(writer(c), &dm.EnsureGraphDataReq{
		Quest:        []*dm.Quest_Desc{desc},
		QuestAttempt: []*dm.AttemptList_Nums{{Nums: aids}},
	})
	if err != nil {
		panic(err)
	}
	for qid := range qsts.Result.Quests {
		if qid != q.ID {
			panic(fmt.Errorf("non matching quest ID!? got %q, expected %q", qid, q.ID))
		}
		return qid
	}
	panic("impossible")
}

func dumpDatastore(c context.Context) {
	ds := datastore.Get(c)
	snap := ds.Testable().TakeIndexSnapshot()
	ds.Testable().CatchupIndexes()
	defer ds.Testable().SetIndexSnapshot(snap)

	fmt.Println("dumping datastore")
	dumper.Config{
		PropFilters: dumper.PropFilterMap{
			{"Quest", "Desc"}: func(prop datastore.Property) string {
				desc := &dm.Quest_Desc{}
				if err := proto.Unmarshal(prop.Value().([]byte), desc); err != nil {
					panic(err)
				}
				return desc.String()
			},
			{"Attempt", "State"}: func(prop datastore.Property) string {
				return dm.Attempt_State(int32(prop.Value().(int64))).String()
			},
			{"Execution", "State"}: func(prop datastore.Property) string {
				return dm.Execution_State(int32(prop.Value().(int64))).String()
			},
		},
	}.Query(c, nil)
}
