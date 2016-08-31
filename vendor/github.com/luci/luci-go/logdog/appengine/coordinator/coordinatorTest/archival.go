// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package coordinatorTest

import (
	"sort"
	"sync"

	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/services/v1"
	"github.com/luci/luci-go/logdog/appengine/coordinator"
	"golang.org/x/net/context"
)

// ArchivalPublisher is a testing implementation of a
// coordinator.ArchivalPublisher. It records which archival tasks were
// scheduled and offers accessors to facilitate test assertions.
type ArchivalPublisher struct {
	sync.Mutex

	// Err, if not nil, is the error returned by Publish.
	Err error

	tasks         []*logdog.ArchiveTask
	archivalIndex uint64
}

var _ coordinator.ArchivalPublisher = (*ArchivalPublisher)(nil)

// Publish implements coordinator.ArchivalPublisher.
func (ap *ArchivalPublisher) Publish(c context.Context, at *logdog.ArchiveTask) error {
	ap.Lock()
	defer ap.Unlock()

	if err := ap.Err; err != nil {
		return err
	}

	ap.tasks = append(ap.tasks, at)
	return nil
}

// NewPublishIndex implements coordinator.ArchivalPublisher.
func (ap *ArchivalPublisher) NewPublishIndex() uint64 {
	ap.Lock()
	defer ap.Unlock()

	v := ap.archivalIndex
	ap.archivalIndex++
	return v
}

// Tasks returns the dispatched archival tasks in the order in which they were
// dispatched.
func (ap *ArchivalPublisher) Tasks() []*logdog.ArchiveTask {
	ap.Lock()
	defer ap.Unlock()

	taskCopy := make([]*logdog.ArchiveTask, len(ap.tasks))
	for i, at := range ap.tasks {
		taskCopy[i] = at
	}
	return taskCopy
}

// Hashes returns a sorted list of the stream hashes that have had archival
// tasks submitted.
func (ap *ArchivalPublisher) Hashes() []string {
	tasks := ap.Tasks()

	taskHashes := make([]string, len(tasks))
	for i, at := range tasks {
		taskHashes[i] = at.Id
	}
	sort.Strings(taskHashes)
	return taskHashes
}

// Clear clears recorded tasks.
func (ap *ArchivalPublisher) Clear() {
	ap.Lock()
	defer ap.Unlock()
	ap.tasks = nil
}
