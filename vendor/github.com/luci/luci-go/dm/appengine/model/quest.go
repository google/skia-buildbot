// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package model

import (
	"time"

	"golang.org/x/net/context"

	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/clock"
	google_pb "github.com/luci/luci-go/common/proto/google"

	dm "github.com/luci/luci-go/dm/api/service/v1"
)

// NewQuest builds a new Quest object with a correct ID given the current
// contents of the Quest_Desc. It returns an error if the Desc is invalid.
//
// Desc must already be Normalize()'d
func NewQuest(c context.Context, desc *dm.Quest_Desc) *Quest {
	return &Quest{ID: desc.QuestID(), Desc: *desc, Created: clock.Now(c).UTC()}
}

// Quest is the model for a job-to-run. Its questPayload should fully
// describe the job to be done.
type Quest struct {
	// ID is the base64 sha256 of questPayload
	ID string `gae:"$id"`

	Desc    dm.Quest_Desc `gae:",noindex"`
	BuiltBy TemplateInfo  `gae:",noindex"`

	Created time.Time `gae:",noindex"`
}

// QueryAttemptsForQuest returns all Attempt objects that exist for this Quest.
func QueryAttemptsForQuest(c context.Context, qid string) *datastore.Query {
	ds := datastore.Get(c)
	from := ds.MakeKey("Attempt", qid+"|")
	to := ds.MakeKey("Attempt", qid+"~")
	return datastore.NewQuery("Attempt").Gt("__key__", from).Lt("__key__", to)
}

// ToProto converts this Quest into its display equivalent.
func (q *Quest) ToProto() *dm.Quest {
	return &dm.Quest{
		Id:   dm.NewQuestID(q.ID),
		Data: q.DataProto(),
	}
}

// DataProto gets the Quest.Data proto message for this Quest.
func (q *Quest) DataProto() *dm.Quest_Data {
	spec := make([]*dm.Quest_TemplateSpec, len(q.BuiltBy))
	for i := range q.BuiltBy {
		spec[i] = &q.BuiltBy[i]
	}

	return &dm.Quest_Data{
		Created: google_pb.NewTimestamp(q.Created),
		Desc:    &q.Desc,
		BuiltBy: spec,
	}
}
