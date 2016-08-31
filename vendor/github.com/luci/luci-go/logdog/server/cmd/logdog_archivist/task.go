// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"github.com/golang/protobuf/proto"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/services/v1"

	gcps "cloud.google.com/go/pubsub"
	"golang.org/x/net/context"
)

// pubsubArchiveTask implements the archivist.Task interface for a ArchiveTask
// Pub/Sub message.
type pubSubArchivistTask struct {
	// subscriptionName is the name of the subscription that this task was pulled
	// from. This is NOT the full subscription path.
	subscriptionName string
	// msg is the message that this task is bound to.
	msg *gcps.Message

	// at is the unmarshalled ArchiveTask from msg.
	at logdog.ArchiveTask

	// consumed is true if this task has been marked for consumption.
	consumed bool
}

func makePubSubArchivistTask(s string, msg *gcps.Message) (*pubSubArchivistTask, error) {
	// If we can't decode the archival task, we can't decide whether or not to
	// delete it, so we will leave it in the queue.
	t := pubSubArchivistTask{
		subscriptionName: s,
		msg:              msg,
	}

	if err := proto.Unmarshal(msg.Data, &t.at); err != nil {
		return nil, err
	}
	return &t, nil
}

func (t *pubSubArchivistTask) UniqueID() string {
	// The Message's AckID is guaranteed to be unique for a single lease.
	return t.msg.AckID
}

func (t *pubSubArchivistTask) Task() *logdog.ArchiveTask {
	return &t.at
}

func (t *pubSubArchivistTask) Consume() {
	t.consumed = true
}

func (t *pubSubArchivistTask) AssertLease(c context.Context) error { return nil }
