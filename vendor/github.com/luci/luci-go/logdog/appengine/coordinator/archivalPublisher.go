// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package coordinator

import (
	"time"

	gcps "cloud.google.com/go/pubsub"
	"github.com/golang/protobuf/proto"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/retry"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/services/v1"
	"golang.org/x/net/context"
)

// ArchivalPublisher is capable of publishing archival requests.
type ArchivalPublisher interface {
	// Publish publishes the supplied ArchiveTask.
	Publish(context.Context, *logdog.ArchiveTask) error

	// NewPublishIndex returns a new publish index. Each publish index is unique
	// within its request.
	NewPublishIndex() uint64
}

type pubsubArchivalPublisher struct {
	// topic is the authenticated Pub/Sub topic handle to publish to.
	topic *gcps.Topic

	// publishIndexFunc is a function that will return a unique publish index
	// for this request.
	publishIndexFunc func() uint64
}

func (p *pubsubArchivalPublisher) Publish(c context.Context, t *logdog.ArchiveTask) error {
	d, err := proto.Marshal(t)
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to marshal task.")
		return err
	}

	// TODO: Route this through some system (e.g., task queue, tumble) that can
	// impose a dispatch delay for the settle period.
	msg := gcps.Message{
		Data: d,
	}

	return retry.Retry(c, retry.Default, func() error {
		log.Fields{
			"project": t.Project,
			"hash":    t.Id,
			"key":     t.Key,
		}.Infof(c, "Publishing archival message for stream.")

		_, err := p.topic.Publish(c, &msg)
		return err
	}, func(err error, d time.Duration) {
		log.Fields{
			log.ErrorKey: err,
			"delay":      d,
		}.Warningf(c, "Failed to publish task. Retrying...")
	})
}

func (p *pubsubArchivalPublisher) NewPublishIndex() uint64 {
	return p.publishIndexFunc()
}
