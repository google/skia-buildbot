// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package mutations

import (
	"fmt"
	"time"

	ds "github.com/luci/gae/service/datastore"
	"github.com/luci/gae/service/info"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/logdog/appengine/coordinator"
	"github.com/luci/luci-go/tumble"
	"golang.org/x/net/context"
)

// CreateArchiveTask is a tumble Mutation that registers a single hierarchy
// component.
type CreateArchiveTask struct {
	// ID is the hash ID of the LogStream whose archive task is being created.
	//
	// Note that the task will apply to the LogStreamState, not the stream
	// entity itself.
	ID coordinator.HashID

	// SettleDelay is the settle delay (see ArchivalParams).
	SettleDelay time.Duration
	// CompletePeriod is the complete period to use (see ArchivalParams).
	CompletePeriod time.Duration

	// Expiration is the delay applied to the archive task via ProcessAfter.
	Expiration time.Time
}

var _ tumble.DelayedMutation = (*CreateArchiveTask)(nil)

// RollForward implements tumble.DelayedMutation.
func (m *CreateArchiveTask) RollForward(c context.Context) ([]tumble.Mutation, error) {
	c = log.SetField(c, "id", m.ID)

	svc := coordinator.GetServices(c)
	ap, err := svc.ArchivalPublisher(c)
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to get archival publisher.")
		return nil, err
	}

	// Get the log stream.
	di := ds.Get(c)
	state := m.logStream().State(di)

	if err := di.Get(state); err != nil {
		if err == ds.ErrNoSuchEntity {
			log.Warningf(c, "Log stream no longer exists.")
			return nil, nil
		}

		log.WithError(err).Errorf(c, "Failed to load archival log stream.")
		return nil, err
	}

	params := coordinator.ArchivalParams{
		RequestID:      info.Get(c).RequestID(),
		SettleDelay:    m.SettleDelay,
		CompletePeriod: m.CompletePeriod,
	}
	if err = params.PublishTask(c, ap, state); err != nil {
		if err == coordinator.ErrArchiveTasked {
			log.Warningf(c, "Archival already tasked, skipping.")
			return nil, nil
		}

		log.WithError(err).Errorf(c, "Failed to publish archival task.")
		return nil, err
	}

	if err := di.Put(state); err != nil {
		log.WithError(err).Errorf(c, "Failed to update datastore.")
		return nil, err
	}

	log.Debugf(c, "Successfully published cleanup archival task.")
	return nil, nil
}

// Root implements tumble.DelayedMutation.
func (m *CreateArchiveTask) Root(c context.Context) *ds.Key {
	return ds.Get(c).KeyForObj(m.logStream())
}

// ProcessAfter implements tumble.DelayedMutation.
func (m *CreateArchiveTask) ProcessAfter() time.Time { return m.Expiration }

// HighPriority implements tumble.DelayedMutation.
func (m *CreateArchiveTask) HighPriority() bool { return false }

// TaskName returns the task's name, which is derived from its log stream ID.
func (m *CreateArchiveTask) TaskName(di ds.Interface) (*ds.Key, string) {
	return di.KeyForObj(m.logStream()), fmt.Sprintf("archive-expired-%s", m.ID)
}

// logStream returns the log stream associated with this task.
func (m *CreateArchiveTask) logStream() *coordinator.LogStream {
	return &coordinator.LogStream{
		ID: m.ID,
	}
}

func init() {
	tumble.Register((*CreateArchiveTask)(nil))
}
