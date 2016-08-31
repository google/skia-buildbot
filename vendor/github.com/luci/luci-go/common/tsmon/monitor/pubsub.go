// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"net/http"

	"cloud.google.com/go/pubsub"
	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/api/option"

	gcps "github.com/luci/luci-go/common/gcloud/pubsub"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/tsmon/types"
)

type pubSubMonitor struct {
	topic *pubsub.Topic
}

// NewPubsubMonitor returns a Monitor that sends metrics to the Cloud Pub/Sub
// API.
//
// The provided client should implement sufficient authentication to send
// Cloud Pub/Sub requests.
func NewPubsubMonitor(ctx context.Context, client *http.Client, topic gcps.Topic) (Monitor, error) {
	project, name := topic.Split()

	psClient, err := pubsub.NewClient(ctx, project, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	return &pubSubMonitor{
		topic: psClient.Topic(name),
	}, nil
}

func (m *pubSubMonitor) ChunkSize() int {
	// PubSub publish request must be less than 10 MB in size. Using 1000 here
	// assumes one cell serializes to <10Kb.
	return 1000
}

func (m *pubSubMonitor) Send(ctx context.Context, cells []types.Cell) error {
	collection := SerializeCells(cells)

	data, err := proto.Marshal(collection)
	if err != nil {
		return err
	}

	ids, err := m.topic.Publish(ctx, &pubsub.Message{Data: data})
	if err != nil {
		logging.Errorf(ctx, "PubSub publish error - %s", err)
		return err
	}
	logging.Debugf(ctx, "Sent %d tsmon cells to PubSub, message id: %v", len(cells), ids)
	return nil
}
